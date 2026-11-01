// upgrade_test.go : tests upgrade steps, costs, discounts, and offsets.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
)

// TestUpgradeStep checks the step size scales with the spec base value.
func TestUpgradeStep(t *testing.T) {
	cases := []struct {
		base, want int
	}{{5, 1}, {10, 2}, {24, 2}, {25, 5}, {49, 5}, {50, 10}, {100, 10}}
	for _, c := range cases {
		if got := UpgradeStep(AttribAttack, c.base); got != c.want {
			t.Errorf("UpgradeStep(base=%d) = %d, want %d", c.base, got, c.want)
		}
	}
}

// TestUpgradeCostMonotonic checks the per step cost is positive and rises as the
// attribute climbs above its base (the curve raised to the 7.5 power).
func TestUpgradeCostMonotonic(t *testing.T) {
	base := 10
	c1 := UpgradeCost(AttribAttack, base, 0, base)   // first step from base
	c2 := UpgradeCost(AttribAttack, base+2, 0, base) // next step (step=2)
	c3 := UpgradeCost(AttribAttack, base+4, 0, base) // step after that
	if c1 < 1 {
		t.Fatalf("first upgrade cost = %d, want >= 1", c1)
	}
	if !(c1 <= c2 && c2 <= c3) {
		t.Errorf("cost not non-decreasing: %d, %d, %d", c1, c2, c3)
	}
}

// TestUpgradeCostVanillaPrices checks first step prices against the vanilla DOS Buy Upgrade
// screen (retail ATTRIBS bases, no research). Speed and ammo pin those factors.
func TestUpgradeCostVanillaPrices(t *testing.T) {
	cases := []struct {
		name      string
		attribute Attrib
		base      int
		want      int
	}{
		{"scout attack", AttribAttack, 12, 8},
		{"scout shots", AttribRounds, 1, 720},
		{"scout range", AttribRange, 3, 61},
		{"scout ammo", AttribAmmo, 10, 5},
		{"scout speed", AttribSpeed, 12, 8},
		{"tank ammo", AttribAmmo, 14, 3},
		{"tank speed", AttribSpeed, 6, 8},
		{"tank armor", AttribArmor, 10, 5},
		{"surveyor scan", AttribScan, 3, 61},
		{"depot hits", AttribHits, 56, 4},
	}
	for _, c := range cases {
		if got := UpgradeCost(c.attribute, c.base, 0, c.base); got != c.want {
			t.Errorf("%s: UpgradeCost(base=%d) = %d, want %d", c.name, c.base, got, c.want)
		}
	}
}

// TestUpgradeResearchFactorDiscount checks research makes an upgrade cheaper.
func TestUpgradeResearchFactorDiscount(t *testing.T) {
	base := 20
	plain := UpgradeCost(AttribHits, base+10, 0, base)
	rf := UpgradeResearchFactor(3, base) // level 3 research
	discounted := UpgradeCost(AttribHits, base+10, rf, base)
	if discounted >= plain {
		t.Errorf("research factor did not lower cost: plain=%d discounted=%d (rf=%d)", plain, discounted, rf)
	}
}

// TestUpgradeOffsets checks the per team per type offset store round trips and
// allocates lazily.
func TestUpgradeOffsets(t *testing.T) {
	var ti TeamInfo
	if got := ti.UpgradeOffset(assets.Tank, AttribArmor); got != 0 {
		t.Fatalf("fresh offset = %d, want 0", got)
	}
	ti.SetUpgradeOffset(assets.Tank, AttribArmor, 6)
	ti.SetUpgradeOffset(assets.Tank, AttribAttack, 4)
	if got := ti.UpgradeOffset(assets.Tank, AttribArmor); got != 6 {
		t.Errorf("armor offset = %d, want 6", got)
	}
	if got := ti.UpgradeOffset(assets.Tank, AttribAttack); got != 4 {
		t.Errorf("attack offset = %d, want 4", got)
	}
	// A different type is independent.
	if got := ti.UpgradeOffset(assets.Scout, AttribArmor); got != 0 {
		t.Errorf("scout armor offset = %d, want 0", got)
	}
}
