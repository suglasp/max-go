// budget_test.go : tests the shared movement and firing budget.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
)

// budgetAttribs builds a unit with 2 shots per turn and 8 speed: each shot costs
// half the speed bar, so the move/fire budget trades cleanly 1-for-1.
func budgetAttribs(moveAndFire bool) assets.UnitAttributes {
	return assets.UnitAttributes{
		TurnsToBuild: 5, HitPoints: 30, Armor: 4, Attack: 12, MoveAndFire: moveAndFire,
		Speed: 8, Fuel: 25, Range: 6, Rounds: 2, Scan: 8, Storage: 0, Ammo: 4, Area: 0,
	}
}

// TestSpendShotSpeed checks that each shot consumes its share of Speed. A unit
// with two rounds and eight Speed spends four Speed per shot.
func TestSpendShotSpeed(t *testing.T) {
	u := &UnitInfo{Values: NewUnitValues(budgetAttribs(false))}
	u.Speed, u.Shots = 8, 2

	u.Shots-- // first shot fired
	u.SpendShotSpeed()
	if u.Speed != 4 {
		t.Fatalf("speed after first shot = %d, want 4", u.Speed)
	}

	u.Shots-- // second shot fired
	u.SpendShotSpeed()
	if u.Speed != 0 {
		t.Fatalf("speed after second shot = %d, want 0", u.Speed)
	}
}

// TestSpendShotSpeedFloorsAtZero: a partly spent mover that fires does not go
// negative on speed.
func TestSpendShotSpeedFloorsAtZero(t *testing.T) {
	u := &UnitInfo{Values: NewUnitValues(budgetAttribs(false))}
	u.Speed, u.Shots = 1, 2 // only 1 movement left, a shot costs 4

	u.Shots--
	u.SpendShotSpeed()
	if u.Speed != 0 {
		t.Errorf("speed = %d, want 0 (floored, not negative)", u.Speed)
	}
}

// TestClampShotsToSpeed checks that movement reduces the remaining shot budget.
func TestClampShotsToSpeed(t *testing.T) {
	u := &UnitInfo{Values: NewUnitValues(budgetAttribs(false))}
	u.Speed, u.Shots = 8, 2

	u.Speed = 4 // half the movement spent
	u.ClampShotsToSpeed()
	if u.Shots != 1 {
		t.Fatalf("shots after half move = %d, want 1", u.Shots)
	}

	u.Speed = 0 // all movement spent
	u.ClampShotsToSpeed()
	if u.Shots != 0 {
		t.Fatalf("shots after full move = %d, want 0", u.Shots)
	}
}

// TestClampShotsNeverRaises checks that the clamp cannot restore spent shots.
func TestClampShotsNeverRaises(t *testing.T) {
	u := &UnitInfo{Values: NewUnitValues(budgetAttribs(false))}
	u.Speed, u.Shots = 8, 0 // out of shots but full movement

	u.ClampShotsToSpeed()
	if u.Shots != 0 {
		t.Errorf("shots = %d, want 0 (clamp must not raise)", u.Shots)
	}
}

// TestMoveAndFireExempt checks that MOVE_AND_FIRE units spend neither budget.
func TestMoveAndFireExempt(t *testing.T) {
	u := &UnitInfo{Values: NewUnitValues(budgetAttribs(true))}
	u.Speed, u.Shots = 8, 2

	u.Shots--
	u.SpendShotSpeed()
	if u.Speed != 8 {
		t.Errorf("move-and-fire speed after a shot = %d, want 8 (unchanged)", u.Speed)
	}

	u.Speed = 0
	u.ClampShotsToSpeed()
	if u.Shots != 1 {
		t.Errorf("move-and-fire shots after a full move = %d, want 1 (unchanged)", u.Shots)
	}
}
