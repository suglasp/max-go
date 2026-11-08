// gameconfig_test.go : tests game-configuration option lists and value formatting.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/settings"
	"github.com/suglasp/max-go/internal/testutil"
)

// newTestGameConfigMenu builds a gameConfigMenu on a headless gnw
// System with the same static art and panel snapshots GameConfig sets
// up, minus buttons and palette (no platform needed).
func newTestGameConfigMenu(t *testing.T) *gameConfigMenu {
	t.Helper()
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	a, err := assets.Load(lib, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	fs, err := fonts.LoadSet(dir)
	if err != nil {
		t.Fatal(err)
	}
	screen := gfx.New(platform.ScreenWidth, platform.ScreenHeight)
	sys := gnw.New(screen, fs)
	ctx := &Context{Sys: sys, Lib: lib, Assets: a, DataDir: dir}

	m := &gameConfigMenu{ctx: ctx, mode: GameConfigModeCustom,
		g: settings.Game{VictoryType: 0, VictoryLimit: 100}}

	pic, err := ctx.loadBig("OPTNFRM")
	if err != nil {
		t.Fatal(err)
	}
	ctx.drawBigAt(pic, 0, 0)
	for _, r := range gcEditRects() {
		img, err := ctx.loadSimple("PREFEDIT")
		if err != nil {
			t.Fatal(err)
		}
		ctx.drawSimpleAt(mainWindow, img, r.ULX, r.ULY, true)
	}
	m.panelRects = [6]gfx.Rect{
		gfx.NewRect(6, 38, 205, 227), gfx.NewRect(216, 38, 415, 227), gfx.NewRect(426, 38, 625, 227),
		gfx.NewRect(6, 233, 205, 422), gfx.NewRect(216, 233, 415, 422), gfx.NewRect(426, 233, 625, 422),
	}
	for i, r := range m.panelRects {
		m.panels[i] = ctx.snapshotArea(r)
	}
	m.titles = [6]string{"Computer Opponent", "Turn Timers", "Play Mode",
		"Starting Credit", "Resource Levels", "Victory Condition"}

	sys.Fonts().SetFont(fonts.Font5)
	m.drawPanels()
	sys.Fonts().SetFont(fonts.Font1) // the loop leaves Font1 current
	return m
}

// countColor counts pixels of palette index c inside r in the main
// window buffer.
func countColor(t *testing.T, sys *gnw.System, r gfx.Rect, c uint8) int {
	t.Helper()
	buf := sys.Buf(mainWindow)
	sys.Lock()
	defer sys.Unlock()
	n := 0
	for y := r.ULY; y <= r.LRY; y++ {
		for x := r.ULX; x <= r.LRX; x++ {
			if buf.At(x, y) == c {
				n++
			}
		}
	}
	return n
}

// TestVictoryEditCommitShowsNumber drives the victory limit edit field
// through the same call sequence the GameConfig input loop performs on
// click, digit, and Enter, then checks that the white limit remains visible.
func TestVictoryEditCommitShowsNumber(t *testing.T) {
	m := newTestGameConfigMenu(t)
	sys := m.ctx.Sys
	box := gcEditRects()[0]

	// The idle limit value is drawn in the metal color (0xA2); it only turns
	// white while being edited (vanilla TextEdit convention).
	if countColor(t, sys, box, 0xA2) == 0 {
		t.Fatalf("initial limit not visible in %v", box)
	}

	// Click on the 'turns' field (control 46), type a digit, Enter.
	m.event(46)
	if m.textEdit == nil {
		t.Fatal("text edit did not open")
	}
	m.textEdit.HandleKey('2')
	m.commitEdit(true)

	if countColor(t, sys, box, 0xA2) == 0 {
		t.Errorf("limit not visible after commit in %v", box)
	}
	if m.g.VictoryLimit != 1002 || m.g.VictoryType != 0 {
		t.Errorf("committed limit = %d type = %d, want 1002 type 0",
			m.g.VictoryLimit, m.g.VictoryType)
	}
}

// TestVictoryEditUsesOwnFont checks that the open edit field keeps Font5 while
// the menu loop uses Font1. Keystroke repaints must stay within the
// Font5 text band and must not spill a Font1 sized fill below it.
func TestVictoryEditUsesOwnFont(t *testing.T) {
	m := newTestGameConfigMenu(t)
	sys := m.ctx.Sys

	h5 := sys.Fonts().Get(fonts.Font5).Height
	h1 := sys.Fonts().Get(fonts.Font1).Height
	if h1 <= h5 {
		t.Fatalf("retail fonts: Font1 height %d not larger than Font5 height %d", h1, h5)
	}

	box := gcEditRects()[0]
	fieldX, fieldY, fieldW := box.ULX+10, box.ULY+5, box.Width()-20

	m.event(46) // open the edit; field snapshot taken under Font5
	// The band a Font1 sized repaint would overwrite but a Font5 one
	// leaves alone.
	band := gfx.NewRect(fieldX, fieldY+h5, fieldX+fieldW-1, fieldY+h1-1)
	before := m.ctx.snapshotArea(band)

	// Keystrokes arrive with Font1 current, as in the GameConfig loop.
	sys.Fonts().SetFont(fonts.Font1)
	if !m.textEdit.HandleKey('2') {
		t.Fatal("digit not consumed")
	}

	after := m.ctx.snapshotArea(band)
	for y := 0; y < before.H; y++ {
		for x := 0; x < before.W; x++ {
			if before.At(x, y) != after.At(x, y) {
				t.Fatalf("pixel (%d,%d) below the Font5 band changed: %d -> %d (field repainted with wrong font)",
					band.ULX+x, band.ULY+y, before.At(x, y), after.At(x, y))
			}
		}
	}
}
