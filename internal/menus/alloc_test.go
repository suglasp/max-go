// alloc_test.go : tests allocation-bar hazard-tape spans.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import "testing"

// TestTapeSpanZeroDepositFullBar checks a resource with no
// deposit (maxv == 0) is fully covered by the hazard tape. The same rule applies
// to raw, fuel, and gold.
func TestTapeSpanZeroDepositFullBar(t *testing.T) {
	const barW, tapeW = 240, 240
	for _, res := range []string{"raw", "fuel", "gold"} {
		x, w := tapeSpan(0, 0, barW, tapeW) // maxv == 0: no deposit
		if x != 0 || w != barW {
			t.Errorf("%s zero-deposit tape = (x=%d, w=%d), want the whole bar (0, %d)", res, x, w, barW)
		}
	}
}

// TestTapeSpanHeadroomAndNoTape checks the partial lock and fully minable cases.
func TestTapeSpanHeadroomAndNoTape(t *testing.T) {
	const barW, tapeW = 240, 240

	// A fuel deposit of 16 with a lock of 4 covers [4..16]: 75% of the
	// bar, beginning at x = 60 with a width of 180.
	if x, w := tapeSpan(16, 4, barW, tapeW); x != 60 || w != 180 {
		t.Errorf("fuel headroom tape = (x=%d, w=%d), want (60, 180)", x, w)
	}

	// A fully minable deposit (lock == maxv) has no tape.
	if x, w := tapeSpan(16, 16, barW, tapeW); w != 0 {
		t.Errorf("fully-minable tape = (x=%d, w=%d), want width 0", x, w)
	}
}

// TestTapeSpanCapsToSpriteWidth checks the tape width never exceeds the sprite width.
func TestTapeSpanCapsToSpriteWidth(t *testing.T) {
	const barW = 240
	if _, w := tapeSpan(0, 0, barW, 100); w != 100 {
		t.Errorf("zero-deposit tape width = %d, want capped to the 100px sprite", w)
	}
}
