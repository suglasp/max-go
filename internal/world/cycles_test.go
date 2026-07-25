// cycles_test.go : testing unit for cycles.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package world

import (
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
)

func numberedPal() gfx.Palette {
	var p gfx.Palette
	for i := 0; i < gfx.PaletteSize; i++ {
		p.SetRGB(i, uint8(i%64), uint8((i+1)%64), uint8((i+2)%64))
	}
	return p
}

func TestCycleStepRight(t *testing.T) {
	p := numberedPal()
	orig := p
	c := ColorCycle{First: 13, Last: 16, Right: true, FPS: 6}

	CycleStep(&p, c, nil)
	// The color of Last moved to First, everything else shifted up.
	if r, g, b := p.RGB(13); r != 16%64 || g != 17%64 || b != 18%64 {
		t.Errorf("entry 13 = %d,%d,%d, want old entry 16", r, g, b)
	}
	if r, _, _ := p.RGB(14); r != 13 {
		t.Errorf("entry 14 should hold old entry 13, got r=%d", r)
	}
	// Entries outside the range untouched.
	for _, i := range []int{12, 17} {
		r, g, b := p.RGB(i)
		or, og, ob := orig.RGB(i)
		if r != or || g != og || b != ob {
			t.Errorf("entry %d outside the cycle changed", i)
		}
	}

	// A full period restores the original palette.
	for i := 0; i < 3; i++ {
		CycleStep(&p, c, nil)
	}
	if p != orig {
		t.Error("full rotation did not restore the palette")
	}
}

func TestCycleStepLeft(t *testing.T) {
	p := numberedPal()
	orig := p
	c := ColorCycle{First: 9, Last: 12, Right: false, FPS: 9}

	CycleStep(&p, c, nil)
	// The color of First moved to Last, everything else shifted down.
	if r, _, _ := p.RGB(12); r != 9 {
		t.Errorf("entry 12 should hold old entry 9, got r=%d", r)
	}
	if r, _, _ := p.RGB(9); r != 10 {
		t.Errorf("entry 9 should hold old entry 10, got r=%d", r)
	}
	for i := 0; i < 3; i++ {
		CycleStep(&p, c, nil)
	}
	if p != orig {
		t.Error("full rotation did not restore the palette")
	}
}

func TestCyclePulse(t *testing.T) {
	p := numberedPal()
	c := ColorCycle{First: 31, Last: 31, Right: true, FPS: 6}
	var pulse uint8

	want := []uint8{63, 31, 15, 7, 63}
	for i, w := range want {
		CycleStep(&p, c, &pulse)
		if _, g, _ := p.RGB(31); g != w {
			t.Errorf("step %d: green = %d, want %d", i, g, w)
		}
	}
	// Red and blue stay.
	if r, _, b := p.RGB(31); r != 31%64 || b != 33%64 {
		t.Errorf("red/blue changed: %d, %d", r, b)
	}
}
