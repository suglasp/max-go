// demo.go : contains the legacy Interplay GNW demo loop and its launch helpers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	/* "os": note: unused code in Go port, the palette read is gfx.LoadBasePalette */

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/settings"
	"github.com/suglasp/max-go/internal/sprites"
)

// show or hide std console
var consoleShown bool

func showConsoleOnce(cfg settings.Settings) {
	if consoleShown {
		return
	}

	consoleShown = true
	// Must run after RunGame has begun:
	// Ebitengine hides the console window during its own initialization
	if cfg.Debug.Console {
		dbg.ShowConsole()
		dbg.Printf("debug console enabled")
	}
}

// Input codes emitted by the demo's buttons.
const (
	demoQuit = 9000 + iota
	demoBlank
	demoRadioA
	demoRadioB
	demoRadioC
)

// demoLogic runs in the GNW logic goroutine: windows, text, buttons,
// radio group, text edit and cursor over the test pattern, the
// visual verification for chunks 5-8.
func demoLogic(p *platform.Platform, sys *gnw.System, cfg settings.Settings) error {
	sys.Fonts().SetFont(fonts.Font1)

	// Test pattern as the background window's content.
	bg := sys.Buf(1)
	sys.Lock()
	drawPattern(bg)
	sys.Unlock()

	win := sys.AddWindow(170, 130, 300, 220, 32, 0)
	sys.ShadedBox(win, 0, 0, 299, 219, 255, 0)
	sys.ShadedBox(win, 2, 2, 297, 217, 0, 255)
	sys.Print(win, "M.A.X. Go - GNW demo", 0, 20, 12, 0x04)
	sys.Print(win, "ESC quits, F12 screenshot", 0, 20, 32, 255|fonts.FlagUnderline)

	// Image button from the real game art (BLANK_UP/BLANK_DN).
	if lib, err := resfile.OpenDir(mustDataDir(cfg)); err == nil {
		upData, err1 := lib.Read("BLANK_UP")
		dnData, err2 := lib.Read("BLANK_DN")

		if err1 == nil && err2 == nil {
			up, _ := sprites.DecodeSimple(upData)
			dn, _ := sprites.DecodeSimple(dnData)
			if up != nil && dn != nil {
				sys.AddButton(win, 50, 56, up.W, up.H,
					gnw.NoValue, gnw.NoValue, gnw.NoValue, demoBlank,
					up.Buffer(), dn.Buffer(), nil, 0)
				sys.Print(win, "blank", 0, 130, 64, 0)
			}
		}

		lib.Close()
	}

	// Radio group of three sticky buttons.
	mk := func(x int, val int, c uint8) gnw.ButtonID {
		img := gfx.New(24, 16)
		img.Fill(c)
		dn := gfx.New(24, 16)
		dn.Fill(c)
		dn.DrawBox(0, 0, 23, 15, 255)
		dn.DrawBox(1, 1, 22, 14, 255)
		return sys.AddButton(win, x, 100, 24, 16, gnw.NoValue, gnw.NoValue, gnw.NoValue, val, img, dn, nil, 0)
	}

	radios := []gnw.ButtonID{
		mk(50, demoRadioA, 64), mk(90, demoRadioB, 96), mk(130, demoRadioC, 160),
	}
	sys.GroupRadioButtons(radios, nil)
	sys.SetRestState(radios[0], true)
	sys.Print(win, "radio", 0, 170, 102, 0)

	// Beveled text button generated from the font.
	sys.AddTextButton(win, 50, 130, gnw.NoValue, gnw.NoValue, gnw.NoValue, demoQuit, "Quit", 0)

	// Text edit field.
	sys.Print(win, "name:", 0, 50, 168, 0)
	sys.Fill(win, 110, 166, 140, 20, 20)
	te := sys.NewTextEdit(win, 112, 168, 136, 14, 0x04, fonts.Font1)
	te.SetText("Player 1")
	te.SetActive(true)

	sys.MouseShow()

	for {
		showConsoleOnce(cfg)
		cycleDemoPalette(p)
		key := sys.GetInput()
		switch {
		case key == -1:
			// idle frame

		case key == platform.KeyEscape, key == demoQuit:
			return gnw.ErrQuit

		case te.HandleKey(key):
			// typed into the edit field

		default:
			dbg.Printf("input: %d", key)
		}
	}
}

// mustDataDir resolves the directory that supplied the demo fonts.
func mustDataDir(cfg settings.Settings) string {
	dir, _ := cfg.ResolveDataDir()

	return dir
}

// loadGamePalette installs COLOR.PAL when the game data is available.
func loadGamePalette(p *platform.Platform, cfg settings.Settings) {
	dir, err := cfg.ResolveDataDir()

	if err != nil {
		return
	}

	pal, err := gfx.LoadBasePalette(dir)
	if err != nil {
		dbg.Printf("no game palette: %v", err)
		return
	}
	/* // note: unused code in Go port
	// The read and parse block is gfx.LoadBasePalette; one log line covers a missing and a bad file.
	data, err := os.ReadFile(resfile.DataPath(dir, "COLOR.PAL"))
	if err != nil {
		dbg.Printf("no game palette: %v", err)
		return
	}

	pal, err := gfx.ParsePalFile(data)
	if err != nil {
		dbg.Printf("bad COLOR.PAL: %v", err)
		return
	}

	p.SetPalette(&pal.Palette)
	*/

	p.SetPalette(&pal)

	dbg.Printf("loaded COLOR.PAL from %s", dir)
}

// demoPal is the -demo palette. cycleDemoPalette rotates one range to
// demonstrate palette animation.
var demoPal gfx.Palette

func setupDemo(p *platform.Platform, cfg settings.Settings) {
	// Use the game palette when available, or build a synthetic fallback.
	if dir, err := cfg.ResolveDataDir(); err == nil {
		// A missing or invalid file leaves the zero palette, and the synthetic one below takes over.
		demoPal, _ = gfx.LoadBasePalette(dir)
		/* // note: unused code in Go port
		// The read and parse block is gfx.LoadBasePalette.
		if data, err := os.ReadFile(resfile.DataPath(dir, "COLOR.PAL")); err == nil {
			if pal, err := gfx.ParsePalFile(data); err == nil {
				demoPal = pal.Palette
			}
		}
		*/
	}

	if demoPal == (gfx.Palette{}) {
		for i := 0; i < gfx.PaletteSize; i++ {
			v := uint8(i % 64)
			demoPal.SetRGB(i, v, uint8(63-v), uint8((i/4)%64))
		}
	}

	p.SetPalette(&demoPal)

	// In plain (plain (not GNW)) mode the pattern goes straight to the screen.
	drawPattern(p.Screen())

	dbg.Printf("demo pattern ready (F12 for screenshot, ESC quits)")
}

// drawPattern renders the test pattern: all 256 colors as a 16x16
// grid plus primitives.
func drawPattern(s *gfx.Buffer) {
	for i := 0; i < 256; i++ {
		cell := s.Sub((i%16)*30+80, (i/16)*25+40, 30, 25)
		cell.Fill(uint8(i))
	}

	s.DrawBox(78, 38, 561, 441, 255)
	s.DrawCircle(320, 240, 100, 255)
	s.DrawLine(80, 40, 559, 439, 255)
	s.DrawShadedBox(60, 20, 579, 459, 254, 253)
}

var lastCycle uint64

func cycleDemoPalette(p *platform.Platform) {
	// Rotate palette entries 96..127 (one of the game's animated ranges) every 100 ms.
	if platform.Elapsed(lastCycle) < 100 {
		return
	}

	lastCycle = platform.Ticks()

	r, g, b := demoPal.RGB(127)
	for i := 127; i > 96; i-- {
		pr, pg, pb := demoPal.RGB(i - 1)
		demoPal.SetRGB(i, pr, pg, pb)
	}

	demoPal.SetRGB(96, r, g, b)

	for i := 96; i <= 127; i++ {
		cr, cg, cb := demoPal.RGB(i)
		p.SetPaletteColor(i, cr, cg, cb)
	}
}
