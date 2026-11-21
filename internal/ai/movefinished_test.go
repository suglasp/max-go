// movefinished_test.go : testing unit for movefinished.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// haltedFixture is the explore fixture's map with an inited AI and a team 1 tank at (4,12) holding a route to
// (18,12), as a halt on an enemy sighting leaves it.
func haltedFixture(t *testing.T) (*exploreFixture, *AI, *game.UnitInfo) {
	t.Helper()

	f := newExploreFixture(t)
	a := New(testUnitFactory)
	a.inited = true

	tank := armedTank(1, 1, 4, 12, f.units)
	f.g.SetHeldDestination(tank, 18, 12)

	return f, a, tank
}

// TestMoveFinishedDrivesOnInTheSameTurn checks the drive on: a halted unit with moves left and no enemy in range
// takes its held route again at once, toward the same destination.
func TestMoveFinishedDrivesOnInTheSameTurn(t *testing.T) {
	f, a, tank := haltedFixture(t)

	a.MoveFinished(f.g, tank)

	goal, moving := moveGoal(f.g, tank)

	if !moving || goal.X != 18 || goal.Y != 12 {
		t.Errorf("goal %v (moving %v), want a move to the held destination (18,12)", goal, moving)
	}

	if f.g.HeldRoute(tank) != nil {
		t.Error("the held route survived the drive on; it must be consumed")
	}
}

// TestMoveFinishedFiresAndKeepsTheRoute checks the fire branch: an enemy in range draws the shot, no move is laid and
// the held route stays for the next turn.
func TestMoveFinishedFiresAndKeepsTheRoute(t *testing.T) {
	f, a, tank := haltedFixture(t)
	tank.Values.Set(game.AttribRounds, 2) // two shots a turn, so a shot leaves moves a drive on could still use
	tank.ResetCombat()

	enemy := spotted(f, 90, 4, 15) // 3 cells off, in range 4
	hitsBefore := enemy.Hits
	ammoBefore := tank.Ammo

	a.MoveFinished(f.g, tank)

	if enemy.Hits >= hitsBefore && tank.Ammo >= ammoBefore {
		t.Error("the tank did not fire on the enemy in range")
	}

	if f.g.HasOrder(tank) {
		t.Error("a unit that fired also drove on; it is fire or move")
	}

	if f.g.HeldRoute(tank) == nil {
		t.Error("firing dropped the held route; it must stay for the next turn")
	}
}

// TestMoveFinishedDrivesOnPastAnEnemyOutOfRange checks that a spotted enemy beyond weapon range draws no shot and does
// not hold the unit back: it drives on.
func TestMoveFinishedDrivesOnPastAnEnemyOutOfRange(t *testing.T) {
	f, a, tank := haltedFixture(t)
	enemy := spotted(f, 90, 4, 20) // 8 cells off, range 4
	hitsBefore := enemy.Hits

	a.MoveFinished(f.g, tank)

	if enemy.Hits != hitsBefore {
		t.Error("the tank fired on an enemy out of range")
	}

	if !f.g.HasOrder(tank) {
		t.Error("a unit with no shot to take stood still; it should drive on")
	}
}

// TestMoveFinishedDoesNothingWhenItCannotAct checks every early return and the speed gate: no order is laid and the
// held route is left alone.
func TestMoveFinishedDoesNothingWhenItCannotAct(t *testing.T) {
	cases := []struct {
		name  string
		setup func(f *exploreFixture, a *AI, u *game.UnitInfo)
	}{
		{"no moves left", func(f *exploreFixture, a *AI, u *game.UnitInfo) { u.Speed = 0 }},
		{"human team", func(f *exploreFixture, a *AI, u *game.UnitInfo) { u.Team = 0 }},
		{"team above range", func(f *exploreFixture, a *AI, u *game.UnitInfo) { u.Team = game.TeamCount }},
		{"team below range", func(f *exploreFixture, a *AI, u *game.UnitInfo) { u.Team = -1 }},
		{"AI disabled", func(f *exploreFixture, a *AI, u *game.UnitInfo) { a.SetEnabled(false) }},
		{"AI not inited", func(f *exploreFixture, a *AI, u *game.UnitInfo) { a.inited = false }},
		{"dead unit", func(f *exploreFixture, a *AI, u *game.UnitInfo) { u.Hits = 0 }},
		{"unit building", func(f *exploreFixture, a *AI, u *game.UnitInfo) { u.Building = true }},
		{"unit disabled", func(f *exploreFixture, a *AI, u *game.UnitInfo) { f.g.RestoreDisabled(u, 2) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, a, tank := haltedFixture(t)
			c.setup(f, a, tank)

			a.MoveFinished(f.g, tank)

			if f.g.HasOrder(tank) {
				t.Error("the unit got an order")
			}

			if f.g.HeldRoute(tank) == nil {
				t.Error("the held route was consumed")
			}
		})
	}
}

// TestMoveFinishedWithoutAHeldRouteStands checks that a unit with no held route is not moved.
func TestMoveFinishedWithoutAHeldRouteStands(t *testing.T) {
	f, a, tank := haltedFixture(t)
	f.g.ClearHeldRoute(tank)

	a.MoveFinished(f.g, tank)

	if f.g.HasOrder(tank) {
		t.Error("a unit with no held route got an order")
	}
}

// TestMoveFinishedIgnoresANilUnit checks that a nil unit is a no-op.
func TestMoveFinishedIgnoresANilUnit(t *testing.T) {
	f, a, _ := haltedFixture(t)

	a.MoveFinished(f.g, nil)
}
