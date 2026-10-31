// combat_test.go : tests targeting, damage, range, and shot resolution.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package combat

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// allLand / allWater are synthetic surfaces for the naval rule tests.
type allLand struct{}

func (allLand) SurfaceAt(x, y int) uint8 { return world.SurfaceLand }

type allWater struct{}

func (allWater) SurfaceAt(x, y int) uint8 { return world.SurfaceWater }

// combatant builds a unit of a type/team at a cell with the given attack,
// armor, hits, rounds, range, ammo stats and the type's category flags.
func combatant(typ assets.UnitType, team, gx, gy int, flags game.Flags, attack, armor, hits, rounds, rng, ammo int) *game.UnitInfo {
	u := game.NewUnitInfo(typ, team, 0, 2)
	u.Flags = flags
	u.GridX, u.GridY = gx, gy
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribAttack, attack)
	u.Values.Set(game.AttribArmor, armor)
	u.Values.Set(game.AttribHits, hits)
	u.Values.Set(game.AttribRounds, rounds)
	u.Values.Set(game.AttribRange, rng)
	u.Values.Set(game.AttribAmmo, ammo)
	u.InitState()
	return u
}

func tank(team, gx, gy int) *game.UnitInfo {
	return combatant(assets.Tank, team, gx, gy, game.FlagMobileLandUnit|game.FlagSelectable,
		/*attack*/ 18 /*armor*/, 16 /*hits*/, 36 /*rounds*/, 1 /*range*/, 4 /*ammo*/, 6)
}

// TestDamageFormula checks the armour subtraction with its floor of 1 and the
// multi hit potential scaling.
func TestDamageFormula(t *testing.T) {
	a := tank(0, 0, 0)
	d := tank(1, 1, 0)

	// 18 attack - 16 armor = 2.
	if got := Damage(a, d, 0); got != 2 {
		t.Errorf("damage = %d, want 2 (18 attack - 16 armor)", got)
	}
	// Floor of 1 when armour exceeds attack.
	d.Values.Set(game.AttribArmor, 50)
	if got := Damage(a, d, 0); got != 1 {
		t.Errorf("damage = %d, want 1 (armour floor)", got)
	}
	// attackPotential 2 -> attack * (5-2)/4 = 18*3/4 = 13; vs armour 16 -> floor 1.
	d.Values.Set(game.AttribArmor, 5)
	if got := Damage(a, d, 2); got != 13-5 {
		t.Errorf("potential-2 damage = %d, want %d", Damage(a, d, 2), 13-5)
	}
}

// TestSubmarineHalvedByBomber checks the bomber vs submarine halving.
func TestSubmarineHalvedByBomber(t *testing.T) {
	bomber := combatant(assets.Bomber, 0, 0, 0, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable,
		30, 0, 20, 1, 5, 4)
	sub := combatant(assets.Submarne, 1, 1, 0, game.FlagMobileSeaUnit|game.FlagSelectable,
		20, 4, 24, 1, 4, 6)
	// 30/2 = 15, minus 4 armor = 11.
	if got := Damage(bomber, sub, 0); got != 11 {
		t.Errorf("bomber-vs-sub damage = %d, want 11 (halved then armour)", got)
	}
}

// TestAttackBothDirections checks that two tanks trade damage and consume their
// shots and ammunition.
func TestAttackBothDirections(t *testing.T) {
	a := tank(0, 0, 0)
	b := tank(1, 1, 0) // adjacent, within range 4

	r1, ok := AttackForce(a, b, allLand{}, false)
	if !ok || r1.Damage != 2 || r1.Destroyed {
		t.Fatalf("A->B = %+v ok=%v, want damage 2 not destroyed", r1, ok)
	}
	if b.Hits != 34 {
		t.Errorf("B hits = %d, want 34", b.Hits)
	}
	if a.Shots != 0 || a.Ammo != 5 {
		t.Errorf("A shots/ammo = %d/%d, want 0/5 after firing", a.Shots, a.Ammo)
	}

	r2, ok := AttackForce(b, a, allLand{}, false)
	if !ok || r2.Damage != 2 {
		t.Fatalf("B->A = %+v ok=%v, want damage 2", r2, ok)
	}
	if a.Hits != 34 {
		t.Errorf("A hits = %d, want 34 after return fire", a.Hits)
	}
}

// TestAttackDestroys checks a target reaching zero hits reports Destroyed.
func TestAttackDestroys(t *testing.T) {
	a := combatant(assets.Tank, 0, 0, 0, game.FlagMobileLandUnit|game.FlagSelectable,
		100, 0, 36, 1, 4, 6)
	b := tank(1, 1, 0)
	b.Hits = 5 // already wounded
	r, ok := AttackForce(a, b, allLand{}, false)
	if !ok || !r.Destroyed {
		t.Fatalf("expected kill, got %+v ok=%v", r, ok)
	}
	if r.Damage != 5 { // damage clamped to remaining hits
		t.Errorf("damage = %d, want 5 (clamped to remaining hits)", r.Damage)
	}
	if b.Hits != 0 {
		t.Errorf("B hits = %d, want 0", b.Hits)
	}
}

// TestOutOfRangeAndAmmo checks the gates: out of range, no shots, friendly.
func TestGatesBlockAttack(t *testing.T) {
	a := tank(0, 0, 0)
	far := tank(1, 10, 0) // range 4, distance 10 -> out of range
	if _, ok := AttackForce(a, far, allLand{}, false); ok {
		t.Error("out-of-range attack should be rejected")
	}

	near := tank(1, 1, 0)
	a.Shots = 0
	if _, ok := AttackForce(a, near, allLand{}, false); ok {
		t.Error("attack with no shots should be rejected")
	}

	a.Shots = 1
	friendly := tank(0, 1, 0)
	if _, ok := AttackForce(a, friendly, allLand{}, false); ok {
		t.Error("attack on a friendly unit should be rejected")
	}
}

// TestAntiAirOnlyHitsAir checks target class validation: an anti aircraft unit
// cannot target a ground tank but can target a flyer; a tank cannot hit a
// not hovering flyer.
func TestTargetClassValidation(t *testing.T) {
	aa := combatant(assets.AntiAir, 0, 0, 0, game.FlagMobileLandUnit|game.FlagSelectable, 10, 0, 20, 1, 5, 4)
	groundTank := tank(1, 1, 0)
	// An airborne flyer carries HOVERING; a not hovering air unit would be a
	// landed one, remapped to a land target.
	flyer := combatant(assets.Fighter, 1, 1, 0, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, 10, 0, 10, 1, 5, 4)

	if CanTarget(aa, groundTank, allLand{}) {
		t.Error("anti-air should not target a ground unit")
	}
	if !CanTarget(aa, flyer, allLand{}) {
		t.Error("anti-air should target a flyer")
	}
	if CanTarget(groundTank, flyer, allLand{}) {
		t.Error("a tank should not target a non-hovering flyer")
	}
}

// TestAirUnitCanAttack checks the single target air attack used in place of the
// C++ flyover sortie. Bombers and Fighters may attack the target classes their
// weapons cover.
func TestAirUnitCanAttack(t *testing.T) {
	// A bomber (air->ground) attacks a ground target in range.
	bomber := combatant(assets.Bomber, 0, 0, 0, game.FlagMobileAirUnit|game.FlagSelectable, 14, 0, 20, 1, 6, 4)
	groundTank := tank(1, 2, 0)
	if !CanAttack(bomber, groundTank, allLand{}) {
		t.Error("a bomber should be able to attack a ground target (interim single-target air attack)")
	}
	// A bomber targets ground/sea/structures, not air.
	flyerTgt := combatant(assets.Fighter, 1, 2, 0, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, 10, 0, 10, 1, 5, 4)
	if CanTarget(bomber, flyerTgt, allLand{}) {
		t.Error("a bomber should not target an air unit")
	}
	// A fighter (air->air) can target an air unit.
	fighter := combatant(assets.Fighter, 0, 0, 0, game.FlagMobileAirUnit|game.FlagSelectable, 12, 0, 12, 1, 6, 4)
	airTgt := combatant(assets.Awac, 1, 2, 0, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, 0, 0, 18, 0, 0, 0)
	if !CanTarget(fighter, airTgt, allLand{}) {
		t.Error("a fighter should be able to target an air unit")
	}
}

// TestNavalAttackNeedsWater checks a corvette can only fire at a target on
// water/coast.
func TestNavalAttackNeedsWater(t *testing.T) {
	corv := combatant(assets.Corvette, 0, 0, 0, game.FlagMobileSeaUnit|game.FlagSelectable, 12, 0, 20, 1, 5, 4)
	target := combatant(assets.Battlshp, 1, 1, 0, game.FlagMobileSeaUnit|game.FlagSelectable, 30, 5, 30, 1, 5, 4)

	if !CanTarget(corv, target, allWater{}) {
		t.Error("corvette should fire at a ship over water")
	}
	if CanTarget(corv, target, allLand{}) {
		t.Error("corvette should not fire when the target cell is land")
	}
}

// TestResetCombatRefillsShots checks the per turn shot refill, capped by ammo.
func TestResetCombatRefillsShots(t *testing.T) {
	u := combatant(assets.Tank, 0, 0, 0, game.FlagMobileLandUnit, 18, 16, 36, 3, 4, 2)
	u.Shots = 0
	u.ResetCombat()
	// ammo 2 < rounds 3 -> shots capped to ammo.
	if u.Shots != 2 {
		t.Errorf("shots = %d, want 2 (capped by ammo)", u.Shots)
	}
	u.Ammo = 5
	u.ResetCombat()
	if u.Shots != 3 {
		t.Errorf("shots = %d, want 3 (full rounds)", u.Shots)
	}
}

// landAt is a water map with land on the listed cells, for the naval line of fire tests.
type landAt map[[2]int]bool

func (m landAt) SurfaceAt(x, y int) uint8 {
	if m[[2]int{x, y}] {
		return world.SurfaceLand
	}
	return world.SurfaceWater
}

// coastAt is a water map with coast on the listed cells.
type coastAt map[[2]int]bool

func (m coastAt) SurfaceAt(x, y int) uint8 {
	if m[[2]int{x, y}] {
		return world.SurfaceCoast
	}
	return world.SurfaceWater
}

// TestNavalLineOfFire checks klei 23/08/26 Access_IsWithinAttackRange: a Submarine or Corvette fires only
// along water or coast; only the cells strictly between shooter and target count.
func TestNavalLineOfFire(t *testing.T) {
	for _, c := range []struct {
		name   string
		typ    assets.UnitType
		from   [2]int
		to     [2]int
		surf   SurfaceSource
		wanted bool
	}{
		{"open water", assets.Corvette, [2]int{0, 2}, [2]int{4, 2}, landAt{}, true},
		{"island in between", assets.Corvette, [2]int{0, 2}, [2]int{4, 2}, landAt{{2, 2}: true}, false},
		{"submarine too", assets.Submarne, [2]int{0, 2}, [2]int{4, 2}, landAt{{3, 2}: true}, false},
		{"coast in between", assets.Corvette, [2]int{0, 2}, [2]int{4, 2}, coastAt{{2, 2}: true}, true},
		{"own cell not walked", assets.Corvette, [2]int{0, 2}, [2]int{4, 2}, landAt{{0, 2}: true}, true},
		{"target cell not walked", assets.Corvette, [2]int{0, 2}, [2]int{4, 2}, landAt{{4, 2}: true}, true},
		{"adjacent", assets.Corvette, [2]int{0, 2}, [2]int{1, 2}, landAt{}, true},
		{"diagonal blocked", assets.Corvette, [2]int{0, 0}, [2]int{3, 3}, landAt{{1, 1}: true}, false},
		{"diagonal clear beside land", assets.Corvette, [2]int{0, 0}, [2]int{3, 3}, landAt{{1, 0}: true, {0, 1}: true}, true},
		{"not a ship", assets.Tank, [2]int{0, 2}, [2]int{4, 2}, landAt{{2, 2}: true}, true},
		{"out of range", assets.Corvette, [2]int{0, 2}, [2]int{6, 2}, landAt{}, false},
		{"no surface", assets.Corvette, [2]int{0, 2}, [2]int{4, 2}, nil, true},
	} {
		u := combatant(c.typ, 0, 0, 0, game.FlagMobileSeaUnit|game.FlagSelectable, 12, 0, 20, 1, 5, 4)
		u.SetPosition(c.from[0], c.from[1])
		if got := WithinAttackRange(u, c.to[0], c.to[1], 5, c.surf); got != c.wanted {
			t.Errorf("%s: WithinAttackRange = %v, want %v", c.name, got, c.wanted)
		}
	}

	// CanAttack follows the line: an island between a corvette and a ship blocks the shot.
	corv := combatant(assets.Corvette, 0, 0, 0, game.FlagMobileSeaUnit|game.FlagSelectable, 12, 0, 20, 1, 5, 4)
	corv.SetPosition(0, 2)
	ship := combatant(assets.Battlshp, 1, 0, 0, game.FlagMobileSeaUnit|game.FlagSelectable, 30, 5, 30, 1, 5, 4)
	ship.SetPosition(4, 2)
	if !CanAttack(corv, ship, landAt{}) {
		t.Error("corvette should fire at a ship over open water")
	}
	if CanAttack(corv, ship, landAt{{2, 2}: true}) {
		t.Error("corvette should not fire across an island")
	}
}
