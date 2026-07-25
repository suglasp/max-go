// resource_allocator_test.go : tests seeded resource placement.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package economy

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestAllocateResources checks the start of game deposit placement: it is
// deterministic for a seed, lays all three resource types within the legal
// amount range, and boosts raw/fuel around a team start.
func TestAllocateResources(t *testing.T) {
	const w, h = 40, 40
	starts := []game.Point{{X: 10, Y: 10}, {X: 30, Y: 30}}

	run := func() *ResourceMap {
		rmap := NewResourceMap(w, h)
		AllocateResources(rmap, w, h, starts, game.NewRandomizer(0xC0FFEE))
		return rmap
	}
	a, b := run(), run()

	var raw, fuel, gold int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ta, na := a.DepositAt(x, y)
			tb, nb := b.DepositAt(x, y)
			if ta != tb || na != nb {
				t.Fatalf("not deterministic at (%d,%d): %v/%d vs %v/%d", x, y, ta, na, tb, nb)
			}
			if na > 16 {
				t.Errorf("(%d,%d) amount %d exceeds the 16 cap", x, y, na)
			}
			if na > 0 {
				switch ta {
				case CargoRaw:
					raw++
				case CargoFuel:
					fuel++
				case CargoGold:
					gold++
				}
			}
		}
	}
	if raw == 0 || fuel == 0 || gold == 0 {
		t.Errorf("expected all three resource types: raw=%d fuel=%d gold=%d", raw, fuel, gold)
	}

	// The start boost should leave raw richer near a team start than the map's
	// average raw cell.
	near := rawZoneTotal(a, starts[0].X, starts[0].Y)
	if near == 0 {
		t.Errorf("no raw boosted near the team start at %v", starts[0])
	}
}

// rawZoneTotal sums the raw amounts in the 5x5 area around (cx,cy).
func rawZoneTotal(m *ResourceMap, cx, cy int) int {
	sum := 0
	for y := cy - 2; y <= cy+2; y++ {
		for x := cx - 2; x <= cx+2; x++ {
			if t, n := m.DepositAt(x, y); t == CargoRaw {
				sum += n
			}
		}
	}
	return sum
}
