// sprite_palette_test.go : the sprite cache rebuilds its shadow LUT when the world palette changes.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package units

import (
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
)

// paletteRamp builds a palette whose entries step through a colour so two ramps with
// different steps produce measurably different brightness tables.
func paletteRamp(step uint8) *gfx.Palette {
	p := &gfx.Palette{}
	for i := 0; i < gfx.PaletteSize; i++ {
		v := uint8((i * int(step)) % 64)
		p.SetRGB(i, v, uint8((v+16)%64), uint8((v+32)%64))
	}
	return p
}

// TestSpriteCacheSetPaletteRebuildsShadow: an in game load can swap to a world whose
// terrain palette differs, so the cache must recompute its shadow darkening LUT. Before
// the fix the LUT stayed keyed to the first palette and unit shadows rendered in the
// wrong hue on the new world.
func TestSpriteCacheSetPaletteRebuildsShadow(t *testing.T) {
	palA := paletteRamp(1)
	palB := paletteRamp(3)

	c := NewSpriteCache(&stubReader{data: map[string][]byte{}}, palA)
	if *c.shadow != *palA.BrightnessTable(5) {
		t.Fatal("a fresh cache should carry palette A's shadow LUT")
	}

	c.SetPalette(palB)
	if *c.shadow != *palB.BrightnessTable(5) {
		t.Error("SetPalette did not rebuild the shadow LUT for the new palette")
	}
	if *c.shadow == *palA.BrightnessTable(5) {
		t.Error("the shadow LUT still matches the old palette after SetPalette")
	}
}
