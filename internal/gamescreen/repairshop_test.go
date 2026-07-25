// repairshop_test.go : tests repairshop.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import "testing"

func TestReloadAmount(t *testing.T) {
	cases := []struct {
		maxAmmo, ammo, raw, want int
	}{
		{6, 6, 10, 0}, // already full
		{6, 0, 10, 6}, // raw covers a full reload
		{6, 2, 3, 3},  // raw is the limit (2 -> 5)
		{6, 0, 0, 0},  // no raw
		{0, 0, 10, 0}, // not a combat unit
		{6, 5, 10, 1}, // top off the last shell
	}
	for _, c := range cases {
		if got := reloadAmount(c.maxAmmo, c.ammo, c.raw); got != c.want {
			t.Errorf("reloadAmount(max=%d,ammo=%d,raw=%d) = %d, want %d", c.maxAmmo, c.ammo, c.raw, got, c.want)
		}
	}
}

func TestRepairHeal(t *testing.T) {
	cases := []struct {
		maxHits, hits, raw, wantHeal, wantCost int
	}{
		{40, 40, 10, 0, 0},   // already full
		{40, 0, 100, 40, 10}, // raw covers full repair: 40 hits / 4 = 10 raw
		{40, 0, 5, 20, 5},    // raw is the limit: 5 raw -> 20 hits
		{40, 38, 10, 2, 1},   // small repair rounds the cost up
		{40, 10, 0, 0, 0},    // no raw
	}
	for _, c := range cases {
		heal, cost := repairHeal(c.maxHits, c.hits, c.raw)
		if heal != c.wantHeal || cost != c.wantCost {
			t.Errorf("repairHeal(max=%d,hits=%d,raw=%d) = (%d,%d), want (%d,%d)",
				c.maxHits, c.hits, c.raw, heal, cost, c.wantHeal, c.wantCost)
		}
		if cost > c.raw {
			t.Errorf("repairHeal cost %d exceeds available raw %d", cost, c.raw)
		}
	}
}
