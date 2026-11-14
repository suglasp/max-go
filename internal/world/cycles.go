// cycles.go : the M.A.X. palette color cycle animator - advances water caustics, shore foam, and beacon blink each frame.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package world

import "github.com/suglasp/max-go/internal/gfx"

// ColorCycle is one animated palette index range of the terrain
// palette (water caustics, shore foam, the blinking beacon entry).
// The game screen advances each cycle at its FPS by calling CycleStep;
// pixels never change, only the palette rotates.
//
// C++: struct ColorCycleData (world.hpp)
type ColorCycle struct {
	First, Last int
	Right       bool // rotate towards higher indices (rotate_direction == 1)
	FPS         int  // steps per second (TIMER_FPS_TO_MS(FPS) ms apart)
}

// ColorCycles is the fixed table of terrain palette animations.
//
// C++: World::InitColorCycles (world.cpp)
var ColorCycles = [...]ColorCycle{
	{9, 12, false, 9},
	{13, 16, true, 6},
	{17, 20, true, 9},
	{21, 24, true, 6},
	{25, 30, true, 2},
	{31, 31, true, 6},
	{96, 102, true, 8},
	{103, 109, true, 8},
	{110, 116, true, 10},
	{117, 122, true, 6},
	{123, 127, true, 6},
}

// CycleStep advances one color cycle a single step on the palette.
// Cycles with a single entry pulse the entry's green channel instead of
// rotating; pulse must point at a byte persisted between steps (zero
// value starts the pulse at full brightness).
//
// C++: World::UpdateColorAnimations (world.cpp)
func CycleStep(pal *gfx.Palette, c ColorCycle, pulse *uint8) {
	if c.Last <= c.First {
		if *pulse <= 7 {
			*pulse = 63
		} else {
			*pulse /= 2
		}
		r, _, b := pal.RGB(c.First)
		pal.SetRGB(c.First, r, *pulse, b)
		return
	}
	if c.Right {
		r, g, b := pal.RGB(c.Last)
		for i := c.Last; i > c.First; i-- {
			pr, pg, pb := pal.RGB(i - 1)
			pal.SetRGB(i, pr, pg, pb)
		}
		pal.SetRGB(c.First, r, g, b)
	} else {
		r, g, b := pal.RGB(c.First)
		for i := c.First; i < c.Last; i++ {
			nr, ng, nb := pal.RGB(i + 1)
			pal.SetRGB(i, nr, ng, nb)
		}
		pal.SetRGB(c.Last, r, g, b)
	}
}

// ColorCycler runs the terrain color cycles on their own clocks: one timestamp
// per cycle plus the pulse byte. The zero value is ready to use.
type ColorCycler struct {
	stamps [len(ColorCycles)]uint64
	pulse  uint8
}

// Advance steps every cycle whose interval has passed at tick now (in
// milliseconds) and reports whether the palette changed, so the caller
// installs it only then.
//
// C++: World::UpdateColorAnimations driven from the game tick (world.cpp)
func (c *ColorCycler) Advance(pal *gfx.Palette, now uint64) bool {
	changed := false

	for i, cycle := range ColorCycles {
		if now-c.stamps[i] >= uint64(1000/cycle.FPS) {
			c.stamps[i] = now
			CycleStep(pal, cycle, &c.pulse)
			changed = true
		}
	}

	return changed
}
