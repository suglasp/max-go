// media.go : plays MVE movies and FLIC animations for debug views and the intro.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	ebiaudio "github.com/hajimehoshi/ebiten/v2/audio"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/flic"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/mve"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/settings"
)

// runMovieDebug plays an MVE file for decoder inspection. It presents frames on DelayMS while a decoder goroutine queues audio ahead of playback.
func runMovieDebug(p *platform.Platform, cfg settings.Settings, name string) error {
	path, err := resolveMediaPath(cfg, name, ".MVE")
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	mv, err := mve.Open(f)
	if err != nil {
		return err
	}

	format := mv.AudioFormat()
	dbg.Printf("mve: %s %dx%d audio=%+v", path, mv.W(), mv.H(), format)

	audioStream, audioPlayer, err := startMovieAudio(format)
	if err != nil {
		return err
	}

	if audioPlayer != nil {
		defer audioPlayer.Close()
		defer audioStream.Close()
	}

	frames, decErr, done, producerDone := startMovieDecoder(mv, audioStream, format)
	defer func() {
		close(done)
		<-producerDone
	}()

	var (
		frame  *mve.Frame
		nextAt uint64
		palSet bool
	)

	p.Screen().Fill(0)

	p.SetFrameHook(func(p *platform.Platform) error {
		if err := drainEscapeInput(p); err != nil {
			return err
		}

		now := platform.Ticks()
		if frame != nil && now < nextAt {
			return nil
		}

		select {
		case fr, ok := <-frames:
			if !ok {
				if derr := <-decErr; derr != nil && !errors.Is(derr, io.EOF) {
					return derr
				}
				return ebiten.Termination
			}
			frame = fr

		default:
			// Hold the current picture until the decoder produces another frame.
			return nil
		}

		if frame.PaletteSet || !palSet {
			applyFramePalette(p, frame.Palette[:])
			palSet = true
		}

		drawCenteredFrame(p.Screen(), frame.Pix, frame.W, frame.H)

		delay := frame.DelayMS
		if delay <= 0 {
			delay = 33
		}

		nextAt = now + uint64(delay)

		return nil
	})

	return p.Run()
}

// startMovieAudio opens the playback device for a movie's audio format. Both
// returns are nil when the format is unsupported, which leaves the movie silent.
func startMovieAudio(format mve.AudioFormat) (*movieAudioStream, *ebiaudio.Player, error) {
	audioCtx, audioOK := movieAudioContext(format)
	if !audioOK {
		dbg.Printf("mve: audio disabled for unsupported format %+v", format)
	}

	var (
		audioStream *movieAudioStream
		audioPlayer *ebiaudio.Player
	)

	if audioCtx != nil {
		var err error

		audioStream = newMovieAudioStream()
		audioPlayer, err = audioCtx.NewPlayer(audioStream)
		if err != nil {
			return nil, nil, err
		}
		audioPlayer.SetBufferSize(250 * time.Millisecond)
		audioPlayer.SetVolume(1)
		audioPlayer.Play()
	}

	return audioStream, audioPlayer, nil
}

// startMovieDecoder decodes on a separate goroutine. The bounded frame channel limits memory, while queued audio absorbs update jitter. Closing done stops it and producerDone signals completion.
func startMovieDecoder(mv *mve.Movie, audioStream *movieAudioStream, format mve.AudioFormat) (chan *mve.Frame, chan error, chan struct{}, chan struct{}) {
	frames := make(chan *mve.Frame, 32)
	decErr := make(chan error, 1)
	done := make(chan struct{})
	producerDone := make(chan struct{})

	go func() {
		defer close(producerDone)
		for {
			fr, derr := mv.NextFrame()
			if derr != nil {
				decErr <- derr
				close(frames)
				return
			}
			if audioStream != nil && len(fr.Audio) > 0 {
				audioStream.Write(moviePCMBytes(fr.Audio, format))
			}
			select {
			case frames <- fr:
			case <-done:
				return
			}
		}
	}()

	return frames, decErr, done, producerDone
}

func movieAudioContext(format mve.AudioFormat) (*ebiaudio.Context, bool) {
	if format.Rate <= 0 || format.Bits != 16 || (format.Channels != 1 && format.Channels != 2) {
		return nil, false
	}

	if ctx := ebiaudio.CurrentContext(); ctx != nil {
		if ctx.SampleRate() != format.Rate {
			return nil, false
		}
		return ctx, true
	}

	return ebiaudio.NewContext(format.Rate), true
}

func moviePCMBytes(samples []int16, format mve.AudioFormat) []byte {
	channels := format.Channels
	if channels <= 0 {
		channels = 2
	}

	outChannels := channels
	if outChannels == 1 {
		outChannels = 2
	}

	out := make([]byte, 0, len(samples)*2*outChannels/channels)
	for _, sample := range samples {
		lo, hi := byte(sample), byte(uint16(sample)>>8)
		out = append(out, lo, hi)
		if channels == 1 {
			out = append(out, lo, hi)
		}
	}

	return out
}

// movieAudioStream feeds decoded PCM to the movie player. Read returns silence on underrun because blocking would deadlock shutdown.
type movieAudioStream struct {
	mu     sync.Mutex
	buf    []byte
	off    int
	closed bool
}

func newMovieAudioStream() *movieAudioStream {
	return &movieAudioStream{}
}

func (s *movieAudioStream) Write(p []byte) {
	if len(p) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	if s.off > 0 && (s.off == len(s.buf) || s.off > 64*1024) {
		copy(s.buf, s.buf[s.off:])
		s.buf = s.buf[:len(s.buf)-s.off]
		s.off = 0
	}

	s.buf = append(s.buf, p...)
}

func (s *movieAudioStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.off >= len(s.buf) {
		if s.closed {
			return 0, io.EOF
		}
		// Underrun: emit silence rather than block, keeping the device
		// fed until the decoder queues more PCM.
		for i := range p {
			p[i] = 0
		}
		return len(p), nil
	}

	n := copy(p, s.buf[s.off:])
	s.off += n
	if s.off == len(s.buf) {
		s.buf = s.buf[:0]
		s.off = 0
	}

	return n, nil
}

func (s *movieAudioStream) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

// drainEscapeInput eats this frame's pending key presses, stopping the loop
// with ebiten.Termination when Escape is among them.
func drainEscapeInput(p *platform.Platform) error {
	for {
		switch key := p.GetInput(); key {
		case -1:
			return nil
		case platform.KeyEscape:
			return ebiten.Termination
		}
	}
}

// applyFramePalette pushes one decoded frame palette to the display.
func applyFramePalette(p *platform.Platform, palette []byte) {
	var pal gfx.Palette
	copy(pal[:], palette)
	p.SetPalette(&pal)
}

// drawCenteredFrame letterboxes one decoded video frame onto the screen,
// cropping the middle when the frame is larger than the display.
func drawCenteredFrame(screen *gfx.Buffer, pix []byte, frameW, frameH int) {
	screen.Fill(0)

	srcX, srcY := 0, 0
	dstX := max(0, (platform.ScreenWidth-frameW)/2)
	dstY := max(0, (platform.ScreenHeight-frameH)/2)
	w, h := frameW, frameH

	if w > platform.ScreenWidth {
		srcX = (w - platform.ScreenWidth) / 2
		w = platform.ScreenWidth
	}

	if h > platform.ScreenHeight {
		srcY = (h - platform.ScreenHeight) / 2
		h = platform.ScreenHeight
	}

	src := gfx.FromPix(pix, frameW, frameH).Sub(srcX, srcY, w, h)
	screen.Sub(dstX, dstY, w, h).Copy(src)
}

// runFlicDebug plays a FLIC file for decoder inspection and restarts it at the end.
func runFlicDebug(p *platform.Platform, cfg settings.Settings, name string) error {
	path, err := resolveMediaPath(cfg, name, ".FLC")
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	anim, err := flic.Open(f)
	if err != nil {
		return err
	}

	dbg.Printf("flic: %s %+v", path, anim.Header())

	var (
		frame  *flic.Frame
		nextAt uint64
		palSet bool
	)

	p.Screen().Fill(0)

	p.SetFrameHook(func(p *platform.Platform) error {
		if err := drainEscapeInput(p); err != nil {
			return err
		}

		now := platform.Ticks()
		if frame != nil && now < nextAt {
			return nil
		}

		frame, err = anim.NextFrame()
		if errors.Is(err, io.EOF) {
			if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
				return seekErr
			}
			anim, err = flic.Open(f)
			if err != nil {
				return err
			}
			frame, err = anim.NextFrame()
		}

		if err != nil {
			return err
		}

		if frame.PaletteSet || !palSet {
			applyFramePalette(p, frame.Palette[:])
			palSet = true
		}

		drawCenteredFrame(p.Screen(), frame.Pix, frame.W, frame.H)
		delay := frame.DelayMS
		if delay <= 0 {
			delay = 67
		}

		nextAt = now + uint64(delay)

		return nil
	})

	return p.Run()
}

// resolveMediaPath turns a movie or animation name into a playable path.
// Absolute paths and existing files pass through. Bare names get defaultExt and resolve against the game data directory.
func resolveMediaPath(cfg settings.Settings, name string, defaultExt string) (string, error) {
	if filepath.IsAbs(name) {
		return name, nil
	}

	if _, err := os.Stat(name); err == nil {
		return name, nil
	}

	if filepath.Ext(name) == "" {
		name += defaultExt
	}

	dir, err := cfg.ResolveDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, name), nil
}
