// blur_test.go : tests the indexed-color splash backdrop blur.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package intro

import (
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
)

// testPalette builds a palette with a red, a blue, and a purple entry so the
// blur seam between red and blue has an exact midpoint to map to.
func testPalette() *gfx.Palette {
	var pal gfx.Palette
	pal.SetRGB(1, 60, 0, 0)  // red
	pal.SetRGB(2, 0, 0, 60)  // blue
	pal.SetRGB(3, 30, 0, 30) // the red/blue midpoint
	return &pal
}

// TestBlurLogoMemDimensionsAndUniform checks the output size and that a one
// color logo maps back to its own palette entry everywhere.
func TestBlurLogoMemDimensionsAndUniform(t *testing.T) {
	pal := testPalette()
	logo := gfx.New(64, 48)
	logo.Fill(1) // all red

	backdrop := blurLogoMem(logo, pal, 86, 48)
	if backdrop.W != 86 || backdrop.H != 48 {
		t.Fatalf("backdrop size = %dx%d, want 86x48", backdrop.W, backdrop.H)
	}
	// Blurring a constant image changes nothing, so every pixel must map back
	// to the exact red entry.
	for i, c := range backdrop.Pix {
		if c != 1 {
			t.Fatalf("pixel %d = %d, want palette entry 1 (red)", i, c)
		}
	}
}

// TestBlurLogoMemBlendsSeam checks that the blur really mixes colors: a half
// red half blue logo must keep pure edges and map the seam to the midpoint entry.
func TestBlurLogoMemBlendsSeam(t *testing.T) {
	pal := testPalette()
	logo := gfx.New(64, 48)
	for y := 0; y < logo.H; y++ {
		for x := 0; x < logo.W; x++ {
			if x < logo.W/2 {
				logo.Set(x, y, 1) // red left half
			} else {
				logo.Set(x, y, 2) // blue right half
			}
		}
	}

	backdrop := blurLogoMem(logo, pal, 86, 48)
	midY := backdrop.H / 2
	if c := backdrop.At(2, midY); c != 1 {
		t.Errorf("left edge = %d, want 1 (pure red)", c)
	}
	if c := backdrop.At(backdrop.W-3, midY); c != 2 {
		t.Errorf("right edge = %d, want 2 (pure blue)", c)
	}
	if c := backdrop.At(backdrop.W/2, midY); c != 3 {
		t.Errorf("seam center = %d, want 3 (the blurred midpoint)", c)
	}
}

// TestBlurLogoMemNilLogo checks the guard: a missing logo yields a blank
// backdrop of the requested size instead of a panic.
func TestBlurLogoMemNilLogo(t *testing.T) {
	backdrop := blurLogoMem(nil, testPalette(), 86, 48)
	if backdrop.W != 86 || backdrop.H != 48 {
		t.Fatalf("backdrop size = %dx%d, want 86x48", backdrop.W, backdrop.H)
	}
	for i, c := range backdrop.Pix {
		if c != 0 {
			t.Fatalf("pixel %d = %d, want 0 on a nil logo", i, c)
		}
	}
}
