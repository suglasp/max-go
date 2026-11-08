// intro.go : plays the boot movie and splash logos through an injected host stage.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package intro orchestrates the original M.A.X. boot movie and logo sequence through an injected host stage.
package intro

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/mve"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/sprites"
)

const (
	defaultLogoHoldMS = 3000

	// logoFadeMS is how long a splash logo dissolves in from black (a palette
	// ramp from all black up to the logo's palette). C++: menu_draw_logo
	// fading in at boot.
	logoFadeMS = 1500

	movieTag = "MAXINT"
	logo1Tag = "ILOGO"
	logo2Tag = "MLOGO"

	// movieLeadFrames bounds how far the audio decoded ahead may lead the
	// presented video (~0.5s at the movie's ~15 fps): enough to absorb a present
	// hitch, but small so streaming backends with a fixed queue (SoLoud's
	// Queue caps at 32 sources) are not overrun by chunks fed ahead.
	movieLeadFrames = 8
)

var _ Reader = (*resfile.Library)(nil)

// Stage is the host surface the sequence draws through.
type Stage interface {
	// Frame returns the writable frame buffer for the next movie/logo frame.
	Frame() *gfx.Buffer
	// Present shows the current frame and polls skip input.
	Present()
	// SetPalette installs the active 8-bit palette.
	SetPalette(p *gfx.Palette)
	// QueueAudio queues signed 16-bit movie PCM for playback.
	QueueAudio(pcm []int16)
	// Skipped reports whether a skip input (key or left click) has arrived in
	// the current stage.
	Skipped() bool
	// ConsumeSkip clears the skip flag so it does not carry into the next stage
	// skipping the movie advances to the splash without also skipping it.
	ConsumeSkip()
	// NowMS returns the current monotonic tick count in milliseconds.
	NowMS() uint64
}

// Options selects which boot stages run. ShowInterplayLogo adds ILOGO between the movie and MLOGO; the retail sequence leaves it disabled.
// OnSplash runs before the logos so the host can start menu music without restarting it when the menu opens.
type Options struct {
	PlayMovie         bool   // play MAXINT
	ShowLogos         bool   // show boot logos
	ShowInterplayLogo bool   // include optional ILOGO
	LogoHoldMS        uint64 // logo hold duration
	OnSplash          func() // runs before the logos
}

// Reader is the minimal resource accessor used by Run.
type Reader interface {
	Read(tag string) ([]byte, error)
}

// Run plays the boot movie, then ILOGO and MLOGO, through stage.
func Run(stage Stage, lib Reader, opts Options) error {
	if stage == nil {
		return errors.New("intro: nil stage")
	}
	if lib == nil {
		return errors.New("intro: nil reader")
	}

	var errs []error
	moviePlayed := false
	if opts.PlayMovie {
		played, err := playMovie(stage, lib)
		if err != nil {
			errs = append(errs, err)
		}
		moviePlayed = played
		// Stop queued movie audio before the splash, including PCM decoded ahead of a skipped frame.
		if stopper, ok := stage.(interface{ StopMovieAudio() }); ok {
			stopper.StopMovieAudio()
		}
		// Skipping the movie advances to the splash; it must not also skip the
		// splash, so the skip is consumed here.
		stage.ConsumeSkip()
	}

	if opts.ShowLogos {
		// Cue the menu music as the splash begins, so it is already playing when
		// the menu appears (the menu's own PlayMusic then does nothing and does
		// not restart the track).
		if opts.OnSplash != nil {
			opts.OnSplash()
		}
		hold := opts.LogoHoldMS
		if hold == 0 {
			hold = defaultLogoHoldMS
		}
		// The Interplay logo (ILOGO) is intentionally NOT shown in the retail
		// flow: only when explicitly opted in for inspection.
		if moviePlayed && opts.ShowInterplayLogo {
			if err := showLogo(stage, lib, logo1Tag, hold); err != nil {
				errs = append(errs, err)
			}
			stage.ConsumeSkip()
		}
		if err := showLogo(stage, lib, logo2Tag, hold); err != nil {
			errs = append(errs, err)
		}
		stage.ConsumeSkip()
	}

	return errors.Join(errs...)
}

// playMovie decodes MAXINT, forwards palette/audio changes, and presents frames.
func playMovie(stage Stage, lib Reader) (bool, error) {
	if stage.Skipped() {
		return false, nil
	}
	data, tag, err := readFirst(lib, movieTag, "MAXINT.MVE", "INTROFLC")
	if err != nil {
		return false, fmt.Errorf("intro: load movie %s: %w", movieTag, err)
	}
	if alias := strings.TrimRight(string(data), "\x00\r\n\t "); len(alias) > 0 && len(alias) < 64 && !bytes.HasPrefix(data, []byte("Interplay MVE File")) {
		if aliased, aliasTag, aliasErr := readFirst(lib, alias); aliasErr == nil {
			data, tag = aliased, aliasTag
		}
	}
	movie, err := mve.Open(bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("intro: decode movie %s: %w", tag, err)
	}
	if audioStage, ok := stage.(interface{ SetAudioFormat(mve.AudioFormat) }); ok {
		audioStage.SetAudioFormat(movie.AudioFormat())
	}

	// Decode ahead in a goroutine and queue each frame's audio as soon as it is
	// decoded, so the audio stream is never starved by jitter on the video side (the
	// gnw present/composite pace). The main loop then presents the buffered
	// video frames at the movie's own timing. The channel bounds how far audio
	// may lead video (a few frames is enough to ride out a hitch).
	type videoFrame struct {
		pix     []byte
		w, h    int
		pal     *gfx.Palette
		delayMS int
	}
	frames := make(chan videoFrame, movieLeadFrames)
	done := make(chan struct{})
	// Drain the decoder before returning: defers run LIFO, so close(done)
	// signals the goroutine first, then wg.Wait() blocks until it has exited.
	// This guarantees no late QueueAudio can fire after the caller stops the
	// movie audio (which would otherwise restart the PCM stream).
	var wg sync.WaitGroup
	defer wg.Wait()
	defer close(done)
	wg.Add(1)
	var decodeErr error
	go func() {
		defer wg.Done()
		defer close(frames)
		for {
			frame, err := movie.NextFrame()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				decodeErr = fmt.Errorf("intro: movie %s frame: %w", tag, err)
				return
			}
			if len(frame.Audio) > 0 {
				stage.QueueAudio(frame.Audio) // queued ahead of presentation
			}
			vf := videoFrame{w: frame.W, h: frame.H, delayMS: frame.DelayMS}
			vf.pix = append([]byte(nil), frame.Pix...) // NextFrame reuses the pixel buffer
			if frame.PaletteSet {
				p := paletteFromMovie(frame.Palette)
				vf.pal = &p
			}
			select {
			case frames <- vf:
			case <-done:
				return
			}
		}
	}()

	// Pace video on an absolute schedule so presentation time does not accumulate into drift and starve the audio player.
	// Play in the movie's OWN window: clear any stale screen underneath (a game or
	// menu frame, when a cinematic is launched mid game) so a movie smaller than
	// fullscreen shows black borders instead of compositing over whatever was
	// behind it.
	stage.Frame().Fill(0)
	next := stage.NowMS()
	for vf := range frames {
		if stage.Skipped() {
			return true, nil
		}
		if vf.pal != nil {
			stage.SetPalette(vf.pal)
		}
		blitCentered(stage.Frame(), gfx.FromPix(vf.pix, vf.w, vf.h))
		stage.Present()
		next += uint64(max(vf.delayMS, 0))
		if waitUntil(stage, next) {
			return true, nil
		}
	}
	return true, decodeErr
}

// showLogo decodes one big sprite logo, presents it, and holds for holdMS.
func showLogo(stage Stage, lib Reader, tag string, holdMS uint64) error {
	if stage.Skipped() {
		return nil
	}
	data, err := lib.Read(tag)
	if err != nil {
		return fmt.Errorf("intro: load logo %s: %w", tag, err)
	}
	img, err := sprites.DecodeBig(data)
	if err != nil {
		return fmt.Errorf("intro: decode logo %s: %w", tag, err)
	}
	// Blit the logo onto a cleared frame with the palette black, then ramp the
	// palette up so the image dissolves in. The fade always plays in full (so it
	// is actually seen, even right after a movie skip); any input that lands
	// during it is then consumed, so the following hold waits for a fresh click
	// (or the timer) before continuing to the menu.
	frame := stage.Frame()
	frame.Fill(0)
	if frame.W*3 > frame.H*4 {
		// A frame wider than 4:3 means the host widened it for widescreen:
		// fill the side banners with a zoomed, blurred copy of the logo
		// instead of leaving them black.
		blitCentered(frame, blurLogoMem(img.Buffer(), &img.Palette, frame.W, frame.H))
	}
	blitCentered(frame, img.Buffer())
	fadeInPalette(stage, &img.Palette, logoFadeMS)
	stage.ConsumeSkip()
	waitUntil(stage, stage.NowMS()+holdMS)
	return nil
}

// fadeInPalette ramps the active palette from black up to target over durMS,
// presenting each step. The frame's pixels are left in place, so an already
// blitted image dissolves in. It always runs the full duration (it does not
// abort on a skip) so the fade is always seen.
func fadeInPalette(stage Stage, target *gfx.Palette, durMS uint64) {
	start := stage.NowMS()
	for {
		e := stage.NowMS() - start
		if e >= durMS {
			break
		}
		var p gfx.Palette
		for i := range p {
			p[i] = uint8(uint64(target[i]) * e / durMS)
		}
		stage.SetPalette(&p)
		stage.Present()
	}
	stage.SetPalette(target)
	stage.Present()
}

// waitUntil keeps the stage presenting until the target tick or a skip input.
func waitUntil(stage Stage, until uint64) bool {
	for stage.NowMS() < until {
		if stage.Skipped() {
			return true
		}
		stage.Present()
	}
	return stage.Skipped()
}

// blitCentered copies src into the center of dst, clipping at dst edges.
func blitCentered(dst, src *gfx.Buffer) {
	if dst == nil || src == nil {
		return
	}
	x := (dst.W - src.W) / 2
	y := (dst.H - src.H) / 2
	for sy := 0; sy < src.H; sy++ {
		dy := y + sy
		if dy < 0 || dy >= dst.H {
			continue
		}
		srow := src.Row(sy)
		for sx, c := range srow {
			dx := x + sx
			if dx < 0 || dx >= dst.W {
				continue
			}
			dst.Set(dx, dy, c)
		}
	}
}

// paletteFromMovie converts the MVE palette array into the engine palette type.
func paletteFromMovie(src [gfx.PaletteSize * gfx.PaletteStride]byte) gfx.Palette {
	var p gfx.Palette
	copy(p[:], src[:])
	return p
}

// readFirst returns the first readable resource from a list of fallback tags.
func readFirst(lib Reader, tags ...string) ([]byte, string, error) {
	var errs []error
	for _, tag := range tags {
		data, err := lib.Read(tag)
		if err == nil {
			return data, tag, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", tag, err))
	}
	return nil, "", errors.Join(errs...)
}

/* // note: unused code in Go port
// the helper repeated the builtin max; callers use it directly.
// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
*/
