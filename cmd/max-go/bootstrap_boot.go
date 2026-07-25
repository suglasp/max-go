// bootstrap_boot.go : finds and copies M.A.X. data during the first run, then opens the menu.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// This phase runs when the resolved data directory has no MAX.RES.
// It uses Ebitengine's embedded debug font because the game fonts are part of the data being copied.
package main

import (
	"image"
	"image/color"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"github.com/suglasp/max-go/internal/bootstrap"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/settings"
)

// bootPhase is the bootstrap screen's state.
type bootPhase int

const (
	bootSearching bootPhase = iota // probing GOG/Steam or scanning a typed root
	bootIsCopying                    // Copy is running, the progress bar is live
	bootPathInput                  // the hunt found nothing: the user types a path
	bootHasFailed                     // terminal error screen, Escape quits
)

// inputAction is what a key press means to the path input line.
type inputAction int

const (
	inputNone   inputAction = iota
	inputSubmit             // Return with a non empty path
	inputCancel             // Escape gives up on the input screen
)

// inputCap bounds typed paths well beyond any practical path length.
const inputCap = 240

// bootScreen is the bootstrap UI state. The worker goroutine and the frame
// hook both mutate it, the overlay reads it, so every access locks.
type bootScreen struct {
	mu     sync.Mutex
	phase  bootPhase
	status string   // one line describing what is happening now
	source string   // where the data comes from once located
	done   int      // Copy progress: files processed so far
	total  int      // Copy progress: total files
	file   string   // Copy progress: current file name
	input  []byte   // the typed path (path input phase)
	notice string   // extra line on the input screen (a failed rescan)
	fail   []string // the terminal error screen lines
}

// setSearching flips to the searching phase with a status line.
func (s *bootScreen) setSearching(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = bootSearching
	s.status = status
}

// setCopying enters the copying phase for the located data folder.
func (s *bootScreen) setCopying(src string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = bootIsCopying
	s.source = src
	s.done, s.total, s.file = 0, 0, ""
}

// setPathInput opens the path prompt and explains why the search failed.
func (s *bootScreen) setPathInput(notice string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = bootPathInput
	s.notice = notice
}

// setFailed flips to the terminal error screen showing the given lines.
func (s *bootScreen) setFailed(lines ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = bootHasFailed
	s.fail = lines
}

// currentPhase returns the phase under the lock.
func (s *bootScreen) currentPhase() bootPhase {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.phase
}

// progress is the bootstrap.Progress callback feeding the copy bar.
func (s *bootScreen) progress(done, total int, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done, s.total, s.file = done, total, name
}

// handleInputKey updates the typed path for one input code. Printable ASCII
// appends, Backspace trims, Return submits, and Escape cancels. Other codes are
// ignored, and the returned path is valid only for inputSubmit.
func (s *bootScreen) handleInputKey(code int) (inputAction, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case code == platform.KeyEscape:
		return inputCancel, ""

	case code == platform.KeyReturn:
		path := strings.TrimSpace(string(s.input))
		// A path pasted from Explorer's "Copy as path" carries quotes.
		if len(path) >= 2 && strings.HasPrefix(path, `"`) && strings.HasSuffix(path, `"`) {
			path = strings.TrimSpace(path[1 : len(path)-1])
		}
		if path == "" {
			return inputNone, ""
		}
		s.input = nil
		return inputSubmit, path

	case code == platform.KeyBackspace:
		if len(s.input) > 0 {
			s.input = s.input[:len(s.input)-1]
		}

	case code >= ' ' && code <= '~':
		if len(s.input) < inputCap {
			s.input = append(s.input, byte(code))
		}
	}

	return inputNone, ""
}

// draw renders the bootstrap overlay at the 640x480 menu resolution and
// centers its contents within the current bounds.
func (s *bootScreen) draw(screen *ebiten.Image) {
	s.mu.Lock()
	defer s.mu.Unlock()

	width := screen.Bounds().Dx()

	// The debug font advances 6px per glyph, 16px per line.
	centered := func(text string, y int) {
		ebitenutil.DebugPrintAt(screen, text, (width-len(text)*6)/2, y)
	}

	switch s.phase {
	case bootSearching:
		centered("M.A.X. Go", 160)
		centered(s.status, 208)

	case bootIsCopying:
		centered("Bootstraping ...", 160)
		centered("Copying game data from:", 192)
		centered(s.source, 208)
		drawProgressBar(screen, (width-400)/2, 240, 400, 12, s.done, s.total)
		if s.total > 0 {
			centered(strings.TrimSpace(s.file), 264)
		}

	case bootPathInput:
		centered("M.A.X. game data not found", 128)
		centered(s.notice, 160)
		centered("Type the folder of your M.A.X. installation and press Enter.", 192)
		centered("The folder is scanned for MAX.RES. Escape gives up.", 208)
		ebitenutil.DebugPrintAt(screen, "> "+string(s.input)+"_", 32, 248)

	case bootHasFailed:
		y := 176
		for _, line := range s.fail {
			centered(line, y)
			y += 16
		}
	}
}

// drawProgressBar paints a white bordered bar filled proportionally to
// done/total, using plain rectangle fills (no game art exists yet).
func drawProgressBar(screen *ebiten.Image, x, y, w, h, done, total int) {
	fillRect(screen, x, y, w, h, color.White)
	fillRect(screen, x+1, y+1, w-2, h-2, color.Black)
	if total > 0 {
		fillRect(screen, x+2, y+2, (w-4)*done/total, h-4, color.White)
	}
}

// fillRect fills one axis aligned rectangle on the screen.
func fillRect(screen *ebiten.Image, x, y, w, h int, c color.Color) {
	if w <= 0 || h <= 0 {
		return
	}
	screen.SubImage(image.Rect(x, y, x+w, y+h)).(*ebiten.Image).Fill(c)
}

// explicitFailLines is the error screen for an explicitly configured data dir
// with no MAX.RES: the boot never hunts in that case, it points the user at
// the three data dir sources.
func explicitFailLines(dir string) []string {
	return []string{
		"Game data not found: " + dir,
		"MAX.RES is missing at the configured data location.",
		"",
		"Point data_dir in max-go.json, the MAX_DATA_DIR environment",
		"variable or the -datadir flag at your M.A.X. installation folder.",
		"",
		"Press Escape to quit.",
	}
}

// huntFailLines is the error screen once the hunt and the typed path both
// gave up. The first line is the wording the project spec fixes.
func huntFailLines(detail string) []string {
	lines := []string{"Bootstrap failed! Install M.A.X. Gog or Steam edition"}
	if detail != "" {
		lines = append(lines, "", detail)
	}
	lines = append(lines, "", "Press Escape to quit.")
	return lines
}

// locateGameData resolves one worker request: an empty root runs the automatic
// GOG/Steam hunt, anything else scans the typed root for MAX.RES. On failure it
// opens the path prompt and reports false.
func locateGameData(scr *bootScreen, root string) (string, bool) {
	if root == "" {
		scr.setSearching("Searching for an installed M.A.X. (GOG / Steam) ...")
		inst, err := bootstrap.Locate(bootstrap.NewProber())
		if err != nil {
			dbg.Printf("bootstrap: %v", err)
			scr.setPathInput("No GOG or Steam install of M.A.X. was found automatically.")
			return "", false
		}
		dbg.Printf("bootstrap: located %s install at %s", inst.Source, inst.DataDir)
		return inst.DataDir, true
	}

	scr.setSearching("Scanning " + root + " ...")
	dir, ok := bootstrap.FindDataDir(root)
	if !ok {
		scr.setPathInput("No MAX.RES found under: " + root)
		return "", false
	}
	dbg.Printf("bootstrap: found game data at %s", dir)
	return dir, true
}

// copyDataAndPrepareMenu copies the located game data into dataDir, then brings
// up what a normal boot loads before the menu (palette, fonts, the menu itself).
// Any failure lands on the terminal error screen and returns nil.
func copyDataAndPrepareMenu(p *platform.Platform, scr *bootScreen, cfg *settings.Settings, cfgPath, src, dataDir string, flags launchFlags) *menuBoot {
	scr.setCopying(src)
	copied, err := bootstrap.Copy(src, dataDir, scr.progress)
	if err != nil {
		dbg.Printf("bootstrap: copy failed: %v", err)
		scr.setFailed(huntFailLines(err.Error())...)
		return nil
	}
	dbg.Printf("bootstrap: copied %d files into %s", copied, dataDir)

	scr.setSearching("Starting M.A.X. ...")
	loadGamePalette(p, *cfg)

	fontSet, err := fonts.LoadSet(dataDir)
	if err != nil {
		scr.setFailed(huntFailLines("fonts: " + err.Error())...)
		return nil
	}

	boot, err := prepareMainMenu(p, cfg, cfgPath, fontSet, menuJumpName(flags), *flags.skipintro, *flags.nomovie, *flags.nosplash)
	if err != nil {
		scr.setFailed(huntFailLines(err.Error())...)
		return nil
	}

	return boot
}

// handleBootInput drains this frame's key presses against the current phase:
// Escape leaves the error screen and the path prompt feeds typed roots to the
// worker. Keys during a search or copy are ignored, including the Escape the
// window X pushes, so a running copy is never cut short.
func handleBootInput(p *platform.Platform, scr *bootScreen, tryDirs chan<- string) error {
	for {
		key := p.GetInput()
		if key < 0 {
			return nil
		}

		switch scr.currentPhase() {
		case bootHasFailed:
			if key == platform.KeyEscape {
				return ebiten.Termination
			}

		case bootPathInput:
			action, path := scr.handleInputKey(key)
			switch action {
			case inputSubmit:
				select {
				case tryDirs <- path:
					scr.setSearching("Scanning " + path + " ...")
				default:
					// Drop the submission when a scan is already queued.
				}
			case inputCancel:
				scr.setFailed(huntFailLines("")...)
			}

		default:
			// Searching or copying: every key is ignored here.
		}
	}
}

// runDataBootstrap handles a missing MAX.RES. Explicit data directories show
// an error, while the default directory triggers a search and copy. Once the
// data is ready, it loads the menu resources and transfers the running loop.
func runDataBootstrap(p *platform.Platform, cfg *settings.Settings, cfgPath string, flags launchFlags) error {
	scr := &bootScreen{}
	p.SetOverlay(scr.draw)

	dataDir, err := cfg.ResolveDataDir()
	if err != nil {
		return err
	}

	if cfg.DataDirExplicit() {
		// An explicit data directory reports the missing file without searching
		// elsewhere.
		dbg.Printf("bootstrap: MAX.RES missing at the explicit data dir %s", dataDir)
		scr.setFailed(explicitFailLines(dataDir)...)
		p.SetFrameHook(func(p *platform.Platform) error {
			showConsoleOnce(*cfg)
			for {
				switch key := p.GetInput(); key {
				case -1:
					return nil
				case platform.KeyEscape:
					return ebiten.Termination
				}
			}
		})
		return p.Run()
	}

	// The menu handoff: the worker prepares it, the frame hook attaches it,
	// and its resources are released when the loop ends.
	var (
		menuMu sync.Mutex
		menu   *menuBoot
	)
	defer func() {
		menuMu.Lock()
		defer menuMu.Unlock()
		if menu != nil {
			menu.close()
		}
	}()

	readyCh := make(chan *menuBoot, 1)

	// tryDirs feeds the worker: "" = the automatic GOG/Steam hunt, anything
	// else = a typed root to scan for MAX.RES.
	tryDirs := make(chan string, 1)
	tryDirs <- ""

	// The worker does everything slow off the update goroutine: probing,
	// scanning, copying, then the palette/fonts/menu load. None of it makes
	// ebiten calls, so it is platform safe here.
	go func() {
		for root := range tryDirs {
			src, ok := locateGameData(scr, root)
			if !ok {
				// The path prompt is up; wait for the next typed root.
				continue
			}

			if boot := copyDataAndPrepareMenu(p, scr, cfg, cfgPath, src, dataDir, flags); boot != nil {
				readyCh <- boot
			}
			return
		}
	}()

	p.SetFrameHook(func(p *platform.Platform) error {
		showConsoleOnce(*cfg)

		select {
		case boot := <-readyCh:
			menuMu.Lock()
			menu = boot
			menuMu.Unlock()
			p.SetOverlay(nil)
			// Swapping the hook from inside the running hook is the one safe
			// way to do it while the loop runs (see gnw.System.Attach).
			boot.attach(p)
			return nil
		default:
		}

		return handleBootInput(p, scr, tryDirs)
	})

	return p.Run()
}
