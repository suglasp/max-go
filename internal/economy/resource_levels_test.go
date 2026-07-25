// resource_levels_test.go : tests setup levels and resource density.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package economy

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestResourceParamsForLevels checks the setup level (0 Poor, 1 Average, 2 Rich)
// to allocator tunable mapping against the values the original computes in
// menu_update_resource_levels. Raw sets the base min/max band, fuel and gold add
// to it, and the mixed separation keys off raw+fuel.
func TestResourceParamsForLevels(t *testing.T) {
	tests := []struct {
		name             string
		raw, fuel, gold  int
		min, max, mixed  int
		rawDiff, goldLow int
	}{
		// Average across the board (the settings default) is the middle of every band.
		{"average", 1, 1, 1, 9, 20, 40, 5, 8},
		// Poor everything: the smallest deposits, widest separation.
		{"poor", 0, 0, 0, 6, 17, 49, 6, 5},
		// Rich everything: the largest deposits, tightest separation.
		{"rich", 2, 2, 2, 12, 23, 33, 4, 12},
		// Out of range values clamp to Average, matching the "average" row.
		{"clamped", 5, -1, 9, 9, 20, 40, 5, 8},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := ResourceParamsForLevels(tc.raw, tc.fuel, tc.gold)
			if p.MinResources != tc.min || p.MaxResources != tc.max {
				t.Errorf("min/max = %d/%d, want %d/%d", p.MinResources, p.MaxResources, tc.min, tc.max)
			}
			if p.MixedSeparation != tc.mixed {
				t.Errorf("mixed separation = %d, want %d", p.MixedSeparation, tc.mixed)
			}
			if p.RawConcDiff != tc.rawDiff {
				t.Errorf("raw concentrate diffusion = %d, want %d", p.RawConcDiff, tc.rawDiff)
			}
			if p.GoldConcLow != tc.goldLow {
				t.Errorf("gold concentrate low = %d, want %d", p.GoldConcLow, tc.goldLow)
			}
		})
	}
}

// TestRicherLevelsYieldMoreDeposits checks that Rich settings place more total
// material than Poor settings for the same map and seed.
func TestRicherLevelsYieldMoreDeposits(t *testing.T) {
	const w, h = 48, 48
	starts := []game.Point{{X: 12, Y: 12}, {X: 36, Y: 36}}

	total := func(p ResourceParams) int {
		rmap := NewResourceMap(w, h)
		AllocateResourcesWith(rmap, w, h, starts, game.NewRandomizer(0xBEEF), p)
		sum := 0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if _, n := rmap.DepositAt(x, y); n > 0 {
					sum += n
				}
			}
		}
		return sum
	}

	poor := total(ResourceParamsForLevels(0, 0, 0))
	rich := total(ResourceParamsForLevels(2, 2, 2))
	if rich <= poor {
		t.Errorf("rich total %d should exceed poor total %d", rich, poor)
	}
}
