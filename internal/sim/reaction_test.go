// reaction_test.go : tests reaction.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

const reactBuilding game.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable

// addReactor deploys an armed unit set up to reaction fire: attack 12, ammo 4,
// one shot, the given weapon range + scan range, and the given flags (a stationary
// turret or a parked mobile unit).
func addReactor(g *Game, typ assets.UnitType, id, team, gx, gy, rng, scan int, flags game.Flags) *game.UnitInfo {
	u := game.NewUnitInfo(typ, team, id, 0)
	u.Flags = flags
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribAttack, 12)
	u.Values.Set(game.AttribAmmo, 4)
	u.Values.Set(game.AttribRange, rng)
	u.Values.Set(game.AttribScan, scan)
	u.Values.Set(game.AttribHits, 20)
	u.Hits = 20
	u.Ammo = 4
	u.Shots = 1
	g.Units.Deploy(u, gx, gy)
	return u
}

// addAircraftTarget deploys a hovering enemy aircraft (18 hits, no armour).
func addAircraftTarget(g *Game, id, gx, gy, team int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Fighter, team, id, 0)
	u.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, 18)
	u.Hits = 18
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestReactionFireDefendsBase: an enemy unit settling within a player turret's
// range draws an automatic reaction shot - the "defensive buildings actually
// defend" behaviour, with no order needed on the turret.
func TestReactionFireDefendsBase(t *testing.T) {
	g := newGame(t, 20, 12)
	turret := addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 8, reactBuilding)
	tank := addTankTarget(g, 2, 7, 5, 1) // enemy, 2 tiles east -> within range 4
	g.RefreshVisibility()

	g.checkReactions(tank)

	if tank.Hits >= 30 {
		t.Errorf("the turret should have reaction-fired at the enemy tank (hits %d)", tank.Hits)
	}
	if turret.Shots != 0 {
		t.Error("the turret should have spent its shot on the reaction")
	}
	if got := len(g.DrainReactions()); got != 1 {
		t.Errorf("reaction events = %d, want 1", got)
	}
}

// TestReactionRespectsTargetClass: anti air reacts to aircraft but not ground, and
// a gun turret reacts to ground but not aircraft.
func TestReactionRespectsTargetClass(t *testing.T) {
	g := newGame(t, 24, 12)
	aa := addReactor(g, assets.AntiAir, 1, 0, 5, 5, 4, 10, game.FlagMobileLandUnit|game.FlagSelectable)
	gun := addReactor(g, assets.GunTurrt, 2, 0, 6, 5, 4, 10, reactBuilding)
	plane := addAircraftTarget(g, 3, 5, 7, 1) // enemy air, in range of the AA
	tank := addTankTarget(g, 4, 7, 5, 1)      // enemy ground, in range of the gun
	g.RefreshVisibility()

	// Anti air reacts to the aircraft; the gun turret cannot target air.
	g.checkReactions(plane)
	if plane.Hits >= 18 {
		t.Errorf("anti-air should have reacted to the enemy aircraft (hits %d)", plane.Hits)
	}
	if gun.Shots != 1 {
		t.Error("a gun turret must not react to an aircraft (wrong target class)")
	}

	// The gun turret reacts to the tank; anti air cannot target ground.
	aa.Shots = 1 // refill (it spent its shot on the plane)
	g.checkReactions(tank)
	if tank.Hits >= 30 {
		t.Error("the gun turret should have reacted to the enemy tank")
	}
	if aa.Shots != 1 {
		t.Error("anti-air must not react to a ground unit (wrong target class)")
	}
}

// TestDisabledReactionFireStaysSilent: the per unit reaction fire toggle suppresses
// the shot.
func TestDisabledReactionFireStaysSilent(t *testing.T) {
	g := newGame(t, 20, 12)
	turret := addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 8, reactBuilding)
	turret.DisabledReactionFire = true
	tank := addTankTarget(g, 2, 7, 5, 1)
	g.RefreshVisibility()

	g.checkReactions(tank)

	if tank.Hits != 30 {
		t.Error("a unit with reaction fire disabled must not fire")
	}
	if turret.Shots != 1 {
		t.Error("a disabled reactor keeps its shot")
	}
}

// TestNoReactionWhenOutOfShots: a reactor that has spent its shots this turn holds.
func TestNoReactionWhenOutOfShots(t *testing.T) {
	g := newGame(t, 20, 12)
	turret := addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 8, reactBuilding)
	turret.Shots = 0
	tank := addTankTarget(g, 2, 7, 5, 1)
	g.RefreshVisibility()

	g.checkReactions(tank)

	if tank.Hits != 30 {
		t.Error("a reactor with no shots left must not fire")
	}
}

// TestReactionRequiresVisibility: a reactor that cannot SEE the mover (fog) holds,
// even when the mover is inside its weapon range.
func TestReactionRequiresVisibility(t *testing.T) {
	g := newGame(t, 20, 12)
	// scan 1: the turret sees only its own neighbourhood, NOT the tank 3 cells away,
	// although the tank is inside weapon range 4 - so the fog gate suppresses it.
	addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 1, reactBuilding)

	tank := addTankTarget(g, 2, 8, 5, 1) // distance 3: in range 4, outside scan 1
	g.RefreshVisibility()

	if g.Vis.VisibleToTeam(tank, 0) {
		t.Fatal("test setup: the tank should be outside the turret's scan coverage")
	}
	g.checkReactions(tank)

	if tank.Hits != 30 {
		t.Error("a reactor that cannot see the mover (fog) must not react")
	}
}

// TestEnemyMoveTriggersReaction: the end to end Tick hook - an enemy unit MOVING
// into a player turret's range is reaction fired when its move resolves.
func TestEnemyMoveTriggersReaction(t *testing.T) {
	g := newGame(t, 24, 10)
	addReactor(g, assets.GunTurrt, 1, 0, 12, 4, 4, 10, reactBuilding)
	tank := addMoveAttacker(g, 2, 1, 4, 4, 32, 3, 1) // enemy at (4,4), big speed budget
	g.State.ActiveTeam = 1
	g.RefreshVisibility()

	if !g.OrderMove(tank, 9, 4) { // ends 3 cells from the turret -> within range 4
		t.Fatal("OrderMove failed")
	}
	g.ResolveMovement()

	if tank.GridX != 9 || tank.GridY != 4 {
		t.Fatalf("tank ended at (%d,%d), want (9,4)", tank.GridX, tank.GridY)
	}
	if tank.Hits >= 30 {
		t.Error("the turret should have reaction-fired as the enemy moved into range")
	}
}

// TestSimModeHumanReactionSentryGate: the concurrent mode rule for HUMAN
// reactors (units_manager.cpp:4329-4348) - during live simultaneous movement a
// player unit auto fires only ON SENTRY, once its team committed the turn, or
// when the mover could shoot a friendly from where it stands. A plain awaiting
// unit holds against a harmless passer by.
func TestSimModeHumanReactionSentryGate(t *testing.T) {
	g := newGame(t, 20, 12)
	g.State.PlayMode = game.PlaySimultaneous

	turret := addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 8, reactBuilding)
	harmless := addTankTarget(g, 2, 7, 5, 1) // an enemy with NO weapon: not a threat
	g.RefreshVisibility()

	// A plain awaiting human reactor holds fire against the harmless mover.
	g.checkReactions(harmless)
	if harmless.Hits != 30 {
		t.Error("an awaiting human unit must NOT auto fire in simultaneous mode")
	}

	// ON SENTRY it answers.
	turret.Order = game.OrderSentry
	g.checkReactions(harmless)
	if harmless.Hits >= 30 {
		t.Error("a unit ON SENTRY reacts in simultaneous mode")
	}
	turret.Order = game.OrderAwait
	turret.Shots, harmless.Hits = 1, 30

	// Once the team committed its turn (the drain), it answers too.
	g.State.Teams[0].FinishedTurn = true
	g.checkReactions(harmless)
	if harmless.Hits >= 30 {
		t.Error("a committed team's unit reacts during the drain")
	}
	g.State.Teams[0].FinishedTurn = false
	turret.Shots, harmless.Hits = 1, 30

	// An ARMED mover that could shoot the turret from where it stands is a real
	// threat: the awaiting reactor answers it.
	threat := addMoveAttacker(g, 3, 1, 8, 5, 8, 4, 1) // enemy, range 4 covers the turret
	g.RefreshVisibility()
	g.checkReactions(threat)
	if threat.Hits >= 30 {
		t.Error("an awaiting human unit reacts when the mover threatens a friendly")
	}
}

// TestSortByDistToBreaksTiesOnID: equidistant reactors order by unit ID, so the
// reaction sequence stays deterministic whatever order the random Units() map
// iteration yielded them in.
func TestSortByDistToBreaksTiesOnID(t *testing.T) {
	mover := game.NewUnitInfo(assets.Tank, 0, 99, 0)
	mover.GridX, mover.GridY = 5, 5

	reactor := func(id, x, y int) *game.UnitInfo {
		u := game.NewUnitInfo(assets.Tank, 1, id, 0)
		u.GridX, u.GridY = x, y
		return u
	}
	near := reactor(7, 5, 6)     // squared distance 1
	tieLow := reactor(3, 7, 5)   // squared distance 4, the lower ID of the tie
	tieHigh := reactor(12, 5, 3) // squared distance 4

	list := []*game.UnitInfo{tieHigh, near, tieLow} // a worst-case input order
	sortByDistTo(list, mover)

	want := []*game.UnitInfo{near, tieLow, tieHigh}
	for i := range want {
		if list[i] != want[i] {
			t.Fatalf("order[%d] = unit %d, want unit %d (nearest first, ID on a tie)", i, list[i].ID, want[i].ID)
		}
	}
}

// TestParkedUnitDoesNotReact checks that a unit still hidden in its factory takes no reaction shot.
func TestParkedUnitDoesNotReact(t *testing.T) {
	g := newGame(t, 20, 12)
	parked := addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 8, reactBuilding)
	parked.HiddenUntilExpand = true
	tank := addTankTarget(g, 2, 7, 5, 1)
	g.RefreshVisibility()

	g.checkReactions(tank)

	if tank.Hits != 30 || parked.Shots != 1 {
		t.Errorf("a parked unit reacted (target hits %d, shots %d)", tank.Hits, parked.Shots)
	}
}
