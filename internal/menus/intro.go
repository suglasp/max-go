// intro.go : plays the landing animation before the main menu and for the -random startup mode.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/suglasp/max-go/internal/audio"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/intro"
	"github.com/suglasp/max-go/internal/mve"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
)

// RunIntro plays the boot intro sequence (the MAXINT movie, then the ILOGO and
// MLOGO splash screens) through the pure internal/intro orchestrator. It drives
// the gnw screen via an introStage and reads resources from both the RES
// archive and data directory because the intro movie is a standalone data file.
// This screen does not handle logo or menu fades.
//
// C++: Movie_PlayIntro + menu_draw_logo at boot (main.cpp / movie.cpp).
func RunIntro(ctx *Context, opts intro.Options) error {
	drainMouse(ctx) // ignore the click that opened the screen
	ctx.Sys.MouseHide()
	defer ctx.Sys.MouseShow()
	if ctx.Audio != nil {
		ctx.Audio.StopMusic()
	}

	// Widescreen: run the intro on the wide logical screen so the splash can
	// paint into the pillarbox banners (intro.showLogo fills them with a
	// blurred logo backdrop). Restored to 4:3 before the menu shows.
	if ctx.Plat.Widescreen() {
		ctx.Sys.ResizeScreen(ctx.Plat, platform.WideWidth, platform.ScreenHeight)
		defer ctx.Sys.ResizeScreen(ctx.Plat, platform.ScreenWidth, platform.ScreenHeight)
	}
	screenW, screenH := ctx.Sys.ScreenSize()
	stage := &introStage{
		sys:     ctx.Sys,
		plat:    ctx.Plat,
		win:     ctx.Sys.Buf(mainWindow), // cached: Buf locks, so fetch it outside the frame lock
		scratch: gfx.New(screenW, screenH),
		format:  mve.AudioFormat{Rate: 22050, Channels: 2, Bits: 16},
		audio:   ctx.Audio,
	}
	defer stage.CloseAudio()
	// Start the menu music as the splash begins, so it leads seamlessly into the
	// menu (whose own PlayMusic("MAIN_MSC") then does nothing; no restart).
	if opts.OnSplash == nil && ctx.Audio != nil {
		opts.OnSplash = func() { ctx.Audio.PlayMusic(menuMusicTag, true) }
	}
	reader := introReader{lib: ctx.Lib, dir: ctx.DataDir}
	err := intro.Run(stage, reader, opts)
	drainMouse(ctx) // so a skip click does not leak into the menu under the cursor
	return err
}

// drainMouse blocks until no mouse button is held, ticking the screen so the
// frame keeps presenting.
func drainMouse(ctx *Context) {
	for ctx.Plat.Mouse().Buttons != 0 {
		ctx.Sys.GetInput()
	}
}

// introStage is the gnw backed host surface intro.Run draws through: the movie
// and logos are composited into a scratch buffer, then copied into the main
// window and presented one gnw frame at a time.
type introStage struct {
	sys         *gnw.System
	plat        *platform.Platform
	win         *gfx.Buffer // the main window buffer (cached; Buf locks sys.mu)
	scratch     *gfx.Buffer
	skip        bool
	prevButtons uint32 // for edge detecting a fresh left click (held buttons don't skip again)

	format       mve.AudioFormat
	audio        *audio.Manager
	audioStarted bool
}

// Frame returns the scratch buffer that internal/intro draws into.
func (s *introStage) Frame() *gfx.Buffer {
	return s.scratch
}

// Present copies the scratch frame to the main window and polls skip input.
// A skip is any key press or a fresh LEFT click, detected on the edge, so holding the
// button down (e.g. the click that skipped the movie) does not keep skipping
// the splash that follows.
func (s *introStage) Present() {
	s.sys.Lock()
	s.win.Copy(s.scratch)
	s.sys.Unlock()
	if s.sys.GetInput() != -1 { // yield one frame; any key skips
		s.skip = true
	}
	cur := s.plat.Mouse().Buttons
	if cur&platform.MousePressLeft != 0 && s.prevButtons&platform.MousePressLeft == 0 {
		s.skip = true // a fresh left click down, not a held button
	}
	s.prevButtons = cur
}

// SetPalette installs the current movie or logo palette.
func (s *introStage) SetPalette(p *gfx.Palette) {
	s.plat.SetPalette(p)
}

// Skipped reports whether a skip input arrived in the current stage.
func (s *introStage) Skipped() bool {
	return s.skip
}

// ConsumeSkip clears the skip flag between stages, so skipping the movie
// advances to (rather than skips) the splash.
func (s *introStage) ConsumeSkip() {
	s.skip = false
}

// NowMS returns the platform tick count used for movie frame timing.
func (s *introStage) NowMS() uint64 {
	return platform.Ticks()
}

// SetAudioFormat receives the MVE stream format from internal/intro.
func (s *introStage) SetAudioFormat(format mve.AudioFormat) {
	if format.Rate > 0 && format.Bits == 16 && (format.Channels == 1 || format.Channels == 2) {
		s.format = format
	}
}

// QueueAudio feeds movie PCM into the byte stream pulled by Ebitengine/Oto.
func (s *introStage) QueueAudio(pcm []int16) {
	if len(pcm) == 0 || s.audio == nil {
		return
	}
	if !s.audioStarted {
		dbg.Printf("intro: start movie PCM %+v", s.format)
		s.audio.StartMoviePCM(audio.PCMFormat{Rate: s.format.Rate, Channels: s.format.Channels, Bits: s.format.Bits})
		s.audioStarted = true
	}
	s.audio.QueueMoviePCM(pcm)
}

// StopMovieAudio stops the movie PCM stream and releases its player. intro.Run
// calls it the moment the movie ends so its buffered tail does not bleed into
// the splash; it is idempotent (the deferred CloseAudio then does nothing).
func (s *introStage) StopMovieAudio() {
	if s.audio != nil && s.audioStarted {
		s.audio.StopMoviePCM()
		s.audioStarted = false
	}
}

// CloseAudio stops the movie audio stream and releases its player.
func (s *introStage) CloseAudio() {
	s.StopMovieAudio()
}

// introReader resolves intro resources from the RES archive first, then from
// standalone data dir files (the intro movie MAXINT.MVE is not a RES entry).
type introReader struct {
	lib *resfile.Library
	dir string
}

// Read resolves a tag from MAX.RES or a matching loose data dir file.
func (r introReader) Read(tag string) ([]byte, error) {
	if data, err := r.lib.Read(tag); err == nil {
		return data, nil
	}
	for _, name := range []string{tag, tag + ".MVE"} {
		if data, err := os.ReadFile(resfile.DataPath(r.dir, name)); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("intro: resource %q not found in RES or %s", tag, r.dir)
}

// RunMVE plays a single MVE file (by path or data dir tag) inside the gnw
// event loop. It is used for mission win or lose cut scenes at game over.
// A key press or left click skips playback. Returns nil if the file does not
// exist (does nothing silently so skirmish games don't break).
//
// C++: menu.cpp - menu_draw_mission_story_screen / Movie_Play
func RunMVE(ctx *Context, tag string) error {
	path := tag
	if filepath.Ext(path) == "" {
		path += ".MVE"
	}
	if !filepath.IsAbs(path) {
		path = resfile.DataPath(ctx.DataDir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		dbg.Printf("RunMVE %q: file not found, skipping", path)
		return nil // no file, does nothing silently
	}

	drainMouse(ctx)
	ctx.Sys.MouseHide()
	defer ctx.Sys.MouseShow()

	// Size the frame from the live screen: a mid game cut scene can play while
	// the game view is wide (854), and a 640 scratch would only cover part of it.
	screenW, screenH := ctx.Sys.ScreenSize()
	stage := &introStage{
		sys:     ctx.Sys,
		plat:    ctx.Plat,
		win:     ctx.Sys.Buf(mainWindow),
		scratch: gfx.New(screenW, screenH),
		format:  mve.AudioFormat{Rate: 22050, Channels: 2, Bits: 16},
		audio:   ctx.Audio,
	}
	defer stage.CloseAudio()

	r := singleReader{data: data}
	err = intro.Run(stage, r, intro.Options{PlayMovie: true})
	drainMouse(ctx)
	return err
}

// singleReader serves a single preloaded blob for any tag lookup, used by RunMVE.
type singleReader struct{ data []byte }

func (r singleReader) Read(_ string) ([]byte, error) {
	return r.data, nil
}
