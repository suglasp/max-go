// move_test.go : testing unit for move.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package units

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
)

// newTank builds a placed tank with a known speed budget for movement tests.
func newTank(t *testing.T, mgr *Manager, speed, gx, gy int, angle uint8) *game.UnitInfo {
	t.Helper()
	u := game.NewUnitInfo(assets.Tank, 0, 1, angle)
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribSpeed, speed)
	u.Speed = speed
	if mgr != nil {
		mgr.Deploy(u, gx, gy)
	} else {
		u.SetPosition(gx, gy)
	}
	return u
}

// run drives a MoveOrder to completion (or until blocked) and returns the
// terminal status and the number of ticks taken, guarding against runaway.
func run(t *testing.T, m *MoveOrder) (MoveStatus, int) {
	t.Helper()
	for i := 0; i < 100000; i++ {
		switch s := m.Tick(); s {
		case MoveDone, MoveBlocked:
			return s, i + 1
		}
	}
	t.Fatal("move did not terminate")
	return MoveBlocked, 0
}

func clearCost(x, y int) int { return 4 } // every cell costs Clear

func TestMoveStraightLineArrives(t *testing.T) {
	mgr := NewManager()
	u := newTank(t, mgr, 10, 1, 1, 2) // already facing east
	steps := []paths.Step{
		{To: game.Point{X: 2, Y: 1}, Dir: 2},
		{To: game.Point{X: 3, Y: 1}, Dir: 2},
		{To: game.Point{X: 4, Y: 1}, Dir: 2},
	}
	m := NewMoveOrder(mgr, u, steps, clearCost, false)
	status, _ := run(t, m)
	if status != MoveDone {
		t.Fatalf("status = %v, want MoveDone", status)
	}
	if u.GridX != 4 || u.GridY != 1 {
		t.Errorf("ended at (%d,%d), want (4,1)", u.GridX, u.GridY)
	}
	if u.X != 4*tileSize+tileSize/2 || u.Y != 1*tileSize+tileSize/2 {
		t.Errorf("pixel pos (%d,%d) not snapped to cell center", u.X, u.Y)
	}
	// The unit must be bucketed again at the destination, not the origin.
	if got := mgr.Cells.At(4, 1); len(got) != 1 || got[0] != u {
		t.Errorf("unit not in destination cell hash")
	}
	if got := mgr.Cells.At(1, 1); len(got) != 0 {
		t.Errorf("unit still in origin cell hash")
	}
	if u.Order != game.OrderAwait {
		t.Errorf("order = %v, want OrderAwait after finishing", u.Order)
	}
}

func TestMoveTurnsBeforeDriving(t *testing.T) {
	u := newTank(t, nil, 10, 1, 1, 0) // facing north, must turn to east
	steps := []paths.Step{{To: game.Point{X: 2, Y: 1}, Dir: 2}}
	m := NewMoveOrder(nil, u, steps, clearCost, false)
	// First ticks are pure turning, no movement.
	turns := 0
	for {
		if s := m.Tick(); s == MoveTurning {
			turns++
			if u.Moved != 0 {
				t.Fatal("unit moved while still turning")
			}
			continue
		}
		break
	}
	if turns != 2 { // N -> NE -> E
		t.Errorf("turns = %d, want 2 (N->NE->E)", turns)
	}
	if u.Angle != 2 {
		t.Errorf("final angle = %d, want 2 (E)", u.Angle)
	}
}

func TestMoveStopsWhenOutOfSpeed(t *testing.T) {
	// Speed 1 affords a single Clear step (normalized cost (4+3)/4 = 1),
	// then stops; the second step needs another point it doesn't have.
	u := newTank(t, nil, 1, 1, 1, 2)
	steps := []paths.Step{
		{To: game.Point{X: 2, Y: 1}, Dir: 2},
		{To: game.Point{X: 3, Y: 1}, Dir: 2},
	}
	m := NewMoveOrder(nil, u, steps, clearCost, false)
	status, _ := run(t, m)
	if status != MoveBlocked {
		t.Fatalf("status = %v, want MoveBlocked", status)
	}
	if m.Index != 1 {
		t.Errorf("index = %d, want 1 (one cell entered)", m.Index)
	}
	if u.GridX != 2 {
		t.Errorf("ended at grid x %d, want 2", u.GridX)
	}
	if u.Speed != 0 {
		t.Errorf("speed = %d, want 0 after exhausting budget", u.Speed)
	}
	// Refill and it can finish the route next "turn".
	u.ResetMovement()
	if u.Speed != 1 {
		t.Fatalf("speed after ResetMovement = %d, want 1", u.Speed)
	}
	status, _ = run(t, m)
	if status != MoveDone || u.GridX != 3 {
		t.Errorf("after refill status=%v grid x=%d, want MoveDone at 3", status, u.GridX)
	}
}

func TestMoveBlockedByImpassableCell(t *testing.T) {
	u := newTank(t, nil, 10, 1, 1, 2)
	steps := []paths.Step{{To: game.Point{X: 2, Y: 1}, Dir: 2}}
	m := NewMoveOrder(nil, u, steps, func(x, y int) int { return 0 }, false)
	status, _ := run(t, m)
	if status != MoveBlocked {
		t.Fatalf("status = %v, want MoveBlocked for impassable next cell", status)
	}
	if u.GridX != 1 {
		t.Errorf("unit advanced into an impassable cell (grid x %d)", u.GridX)
	}
}

func TestCalculateStepSpeedBudget(t *testing.T) {
	// Cost 12 (water) diagonal = 18; normalized = (18+3)/4 = 5. With speed 6
	// it succeeds leaving speed 1; the move_fraction carries 5*4-18 = 2.
	u := newTank(t, nil, 6, 0, 0, 0)
	if !calculateStep(u, 12, true, false) {
		t.Fatal("expected step to be affordable")
	}
	if u.Speed != 1 {
		t.Errorf("speed = %d, want 1", u.Speed)
	}
	if u.MoveFraction != 2 {
		t.Errorf("move_fraction = %d, want 2", u.MoveFraction)
	}
}

// TestCalculateStepBanksLeftoverSpeed checks Paths_CalculateStep: an unaffordable step ADDS the
// leftover speed (x4) to the banked move_fraction instead of overwriting it, and spends the speed.
func TestCalculateStepBanksLeftoverSpeed(t *testing.T) {
	// Cost 12 straight, 2 already banked: 10 left, normalized 3, more than the 1 speed point.
	u := newTank(t, nil, 1, 0, 0, 0)
	u.MoveFraction = 2
	if calculateStep(u, 12, false, false) {
		t.Fatal("expected the step to be unaffordable")
	}
	if u.MoveFraction != 6 {
		t.Errorf("move_fraction = %d, want 6 (2 banked + 1 speed x4)", u.MoveFraction)
	}
	if u.Speed != 0 {
		t.Errorf("speed = %d, want 0 after the failed step", u.Speed)
	}
}

// TestCalculateStepClampsShots locks the move side of the shared move/fire budget:
// a unit with 2 rounds and 8 speed that spends one Clear step (cost 4 -> 1 speed,
// 8->7) no longer has the budget for both shots, so its shot count clamps from
// 2 to 1.
func TestCalculateStepClampsShots(t *testing.T) {
	u := newTank(t, nil, 8, 0, 0, 2)
	u.Values.Set(game.AttribRounds, 2)
	u.Shots = 2
	if !calculateStep(u, 4, false, false) {
		t.Fatal("expected the step to be affordable")
	}
	if u.Speed != 7 {
		t.Fatalf("speed = %d, want 7", u.Speed)
	}
	if u.Shots != 1 {
		t.Errorf("shots after move = %d, want 1 (clamped to remaining speed)", u.Shots)
	}
}
