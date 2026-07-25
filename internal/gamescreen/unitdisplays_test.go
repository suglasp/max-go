// unitdisplays_test.go : tests unitdisplays.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Tests for the per unit display overlays (the COLORS/STATUS/HITS/AMMO panel
// toggles): the clipped fill/box primitives, the bar fill colour thresholds, and
// the hit point bar rendering into a buffer.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
)

// TestBarFillColor checks the green/yellow/red thresholds match the original's
// value_max/2 and value_max/4 cut offs.
func TestBarFillColor(t *testing.T) {
	cases := []struct {
		actual, max int
		want        uint8
	}{
		{100, 100, gfx.ColorGreen}, // full -> green
		{60, 100, gfx.ColorGreen},  // above half -> green
		{50, 100, gfx.ColorYellow}, // exactly half -> not above half -> yellow
		{30, 100, gfx.ColorYellow}, // above a quarter -> yellow
		{25, 100, gfx.ColorRed},    // exactly a quarter -> not above -> red
		{1, 100, gfx.ColorRed},     // nearly empty -> red
	}
	for _, c := range cases {
		// Each fraction should land on the documented shade.
		if got := barFillColor(c.actual, c.max); got != c.want {
			t.Errorf("barFillColor(%d,%d) = %d, want %d", c.actual, c.max, got, c.want)
		}
	}
}

// TestFillRectClippedBounds confirms a fill partly outside the buffer paints only
// the in bounds part and never panics.
func TestFillRectClippedBounds(t *testing.T) {
	buf := gfx.New(8, 8)
	// A rect straddling the bottom right corner: only the overlap should fill.
	fillRectClipped(buf, 6, 6, 5, 5, gfx.ColorRed)
	if buf.At(7, 7) != gfx.ColorRed {
		t.Fatalf("in-bounds corner not filled: got %d", buf.At(7, 7))
	}
	// A wholly out of bounds rect must do nothing (and not panic).
	fillRectClipped(buf, 100, 100, 4, 4, gfx.ColorGreen)
	// A negative origin rect clips its left/top: (0,0) should fill, off buffer ignored.
	fillRectClipped(buf, -2, -2, 3, 3, gfx.ColorBlue)
	if buf.At(0, 0) != gfx.ColorBlue {
		t.Fatalf("negative-origin clip did not fill (0,0): got %d", buf.At(0, 0))
	}
}

// TestBoxClippedEdges checks the hollow box draws its four edges but leaves the
// interior untouched.
func TestBoxClippedEdges(t *testing.T) {
	buf := gfx.New(8, 8)
	boxClipped(buf, 1, 1, 5, 5, gfx.ColorYellow) // edges at x/y 1 and 5
	// A corner and a mid edge pixel must be painted.
	if buf.At(1, 1) != gfx.ColorYellow || buf.At(3, 5) != gfx.ColorYellow {
		t.Fatalf("box edge not drawn: corner=%d edge=%d", buf.At(1, 1), buf.At(3, 5))
	}
	// The interior must stay clear (0).
	if buf.At(3, 3) != 0 {
		t.Fatalf("box interior filled: got %d", buf.At(3, 3))
	}
}

// TestDrawUnitBarsHits renders a half health unit's hit bar and confirms a black
// trough plus a yellow fill (half health is in the yellow band) appear.
func TestDrawUnitBarsHits(t *testing.T) {
	buf := gfx.New(64, 64)
	buf.Fill(0x99) // a non black background so the black trough is detectable
	v := game.NewUnitValues(assets.UnitAttributes{})
	v.Set(game.AttribHits, 20)
	u := &game.UnitInfo{
		Type:   assets.Tank,
		Values: v,
		Hits:   10, // half of 20 -> yellow band
	}
	// Only HITS on; a 32px footprint at the top left of the buffer.
	s := &Screen{showHits: true}
	s.drawUnitBars(buf, u, 0, 0, 0, 32)
	// Somewhere in the bar region there must be a black trough pixel and a yellow
	// fill pixel.
	var sawBlack, sawYellow bool
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			switch buf.At(x, y) {
			case gfx.ColorBlack:
				sawBlack = true
			case gfx.ColorYellow:
				sawYellow = true
			}
		}
	}
	if !sawBlack || !sawYellow {
		t.Fatalf("hit bar not rendered: black=%v yellow=%v", sawBlack, sawYellow)
	}
}
