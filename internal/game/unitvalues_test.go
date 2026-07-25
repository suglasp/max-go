// unitvalues_test.go : tests attribute access, versions, cloning, and equality.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
)

func sampleAttribs() assets.UnitAttributes {
	return assets.UnitAttributes{
		TurnsToBuild: 5, HitPoints: 30, Armor: 4, Attack: 12, MoveAndFire: true,
		Speed: 16, Fuel: 25, Range: 6, Rounds: 1, Scan: 8, Storage: 0, Ammo: 1, Area: 0,
	}
}

func TestNewUnitValuesMapsAttribs(t *testing.T) {
	v := NewUnitValues(sampleAttribs())
	cases := []struct {
		a    Attrib
		want int
	}{
		{AttribTurns, 5}, {AttribHits, 30}, {AttribArmor, 4}, {AttribAttack, 12},
		{AttribMoveAndFire, 1}, {AttribSpeed, 16}, {AttribFuel, 25}, {AttribRange, 6},
		{AttribRounds, 1}, {AttribScan, 8}, {AttribStorage, 0}, {AttribAmmo, 1},
		{AttribAttackRadius, 0}, {AttribAgentAdjust, 0},
	}
	for _, c := range cases {
		if got := v.Get(c.a); got != c.want {
			t.Errorf("attrib %d = %d, want %d", c.a, got, c.want)
		}
	}
	if v.Version() != 1 {
		t.Errorf("fresh version = %d, want 1", v.Version())
	}
}

func TestUnitValuesSetAddVersion(t *testing.T) {
	v := NewUnitValues(sampleAttribs())
	v.Set(AttribAttack, 20)
	if v.Get(AttribAttack) != 20 {
		t.Fatalf("Set attack = %d, want 20", v.Get(AttribAttack))
	}
	v.Add(AttribAttack, 3)
	if v.Get(AttribAttack) != 23 {
		t.Fatalf("Add attack = %d, want 23", v.Get(AttribAttack))
	}

	// UpdateVersion does nothing until the set is in use.
	v.UpdateVersion()
	if v.Version() != 1 {
		t.Errorf("version advanced while not in use: %d", v.Version())
	}
	v.MarkInUse()
	v.UpdateVersion()
	v.UpdateVersion()
	if v.Version() != 3 {
		t.Errorf("version = %d, want 3 after two in-use bumps", v.Version())
	}
	v.MarkNotInUse()
	v.UpdateVersion()
	if v.Version() != 3 {
		t.Errorf("version advanced after MarkNotInUse: %d", v.Version())
	}
}

func TestUnitValuesCloneIsIndependent(t *testing.T) {
	v := NewUnitValues(sampleAttribs())
	c := v.Clone()
	c.Set(AttribHits, 99)
	if v.Get(AttribHits) == 99 {
		t.Error("Clone shares storage with the original")
	}
}

// TestUnitValuesEqualIgnoresFuel checks the original operator== behavior, which
// excludes fuel from equality.
func TestUnitValuesEqualIgnoresFuel(t *testing.T) {
	a := NewUnitValues(sampleAttribs())
	b := a.Clone()
	if !a.Equal(b) {
		t.Fatal("identical clones not equal")
	}
	b.Set(AttribFuel, a.Get(AttribFuel)+50)
	if !a.Equal(b) {
		t.Error("fuel difference broke equality (should be ignored, per C++)")
	}
	b.Set(AttribArmor, a.Get(AttribArmor)+1)
	if a.Equal(b) {
		t.Error("armor difference did not break equality")
	}
}

func TestUnitValuesBadAttribPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic on out-of-range attribute")
		}
	}()
	NewUnitValues(sampleAttribs()).Get(AttribCount)
}
