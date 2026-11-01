// commando_test.go : tests commando.go.
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

// addCommando deploys a player team commando with one shot at (gx, gy).
func addCommando(g *Game, team, gx, gy int) *game.UnitInfo {
	c := game.NewUnitInfo(assets.Commando, team, 50, 0)
	c.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	c.Values = &game.UnitValues{}
	c.Values.Set(game.AttribRounds, 1)
	c.Shots = 1
	g.Units.Deploy(c, gx, gy)
	return c
}

// addElectronicEnemy deploys an enemy electronic unit (a tank) with the given
// build turns attribute (drives the disable chance/duration) and extra flags.
func addElectronicEnemy(g *Game, team, gx, gy, turns int, extra game.Flags) *game.UnitInfo {
	e := game.NewUnitInfo(assets.Tank, team, 60, 2)
	e.Flags = game.FlagMobileLandUnit | game.FlagSelectable | game.FlagElectronicUnit | extra
	e.Values = &game.UnitValues{}
	e.Values.Set(game.AttribTurns, turns)
	e.Values.Set(game.AttribHits, 30)
	e.Values.Set(game.AttribSpeed, 4)
	e.Hits = 30
	e.Speed = 4
	g.Units.Deploy(e, gx, gy)
	return e
}

// seedForRoll resets the global randomizer to a seed whose next Generate(101)
// roll is <= chance (want=true, a success) or > chance (want=false, a failure),
// leaving that seed installed so the next Disable/Steal consumes that roll.
func seedForRoll(t *testing.T, chance int, want bool) {
	t.Helper()
	for s := uint32(1); s < 100000; s++ {
		game.RandomizerSetSeed(s)
		if (game.RandomizerGenerate(101) <= chance) == want {
			game.RandomizerSetSeed(s) // reinstall: Disable's first roll repeats this value
			return
		}
	}
	t.Fatalf("no seed found for chance=%d want=%v", chance, want)
}

func TestStealthChanceFormula(t *testing.T) {
	g := newGame(t, 8, 8)
	c := addCommando(g, 0, 1, 1)              // Experience 0 -> XP 0
	e := addElectronicEnemy(g, 1, 2, 1, 8, 0) // ATTRIB_TURNS 8

	// disable = (96*0 + 640) / 8 = 80; steal = 80/4 = 20.
	if got := g.StealthChance(c, e, false); got != 80 {
		t.Errorf("disable chance = %d, want 80", got)
	}
	if got := g.StealthChance(c, e, true); got != 20 {
		t.Errorf("steal chance = %d, want 20", got)
	}
	// A low build turns target pushes the raw chance over the 90 cap.
	e2 := addElectronicEnemy(g, 1, 4, 4, 4, 0) // 640/4 = 160 -> capped 90
	if got := g.StealthChance(c, e2, false); got != 90 {
		t.Errorf("disable chance = %d, want 90 (capped)", got)
	}
}

func TestDisableShutsDownAndRecovers(t *testing.T) {
	g := newGame(t, 8, 8)
	g.State.Teams[1].Type = game.TeamComputer
	c := addCommando(g, 0, 1, 1)
	e := addElectronicEnemy(g, 1, 2, 1, 8, 0) // dur = (0+8)/8 = 1

	seedForRoll(t, g.StealthChance(c, e, false), true) // force a hit
	ok, _ := g.Disable(c, 2, 1)
	if !ok {
		t.Fatal("disable did not succeed with a forced hit")
	}
	if !g.IsDisabled(e) || e.Order != game.OrderDisable {
		t.Fatalf("target not disabled: IsDisabled=%v order=%v", g.IsDisabled(e), e.Order)
	}
	if c.Shots != 0 {
		t.Errorf("commando shots = %d, want 0 (one spent)", c.Shots)
	}
	if c.Experience != 1 {
		t.Errorf("commando experience = %d, want 1 (gained on a live hit)", c.Experience)
	}
	if g.OrderMove(e, 5, 5) {
		t.Error("disabled unit accepted a move order")
	}

	// First owner turn: still counted as blocked (no budget), then restored.
	g.BeginTeamTurn(1)
	if g.IsDisabled(e) {
		t.Error("target still disabled after its turn (dur 1)")
	}
	if e.Speed != 0 {
		t.Errorf("unit speed recovered this turn = %d, want 0 (blocked turn)", e.Speed)
	}
	// Next turn it has a full budget again.
	g.BeginTeamTurn(1)
	if e.Speed != 4 {
		t.Errorf("unit speed = %d, want 4 (recovered)", e.Speed)
	}
}

func TestDisableBuildingSetsOrderDisable(t *testing.T) {
	g := newGame(t, 8, 8)
	c := addCommando(g, 0, 1, 1)
	// A stationary electronic building next to the commando.
	b := addElectronicEnemy(g, 1, 2, 1, 8, game.FlagStationary|game.FlagBuilding)
	if !g.CanDisable(c, b) {
		t.Fatal("commando should be able to disable an adjacent enemy building")
	}
	if g.CanSteal(c, b) {
		t.Error("a building must not be stealable")
	}
	seedForRoll(t, g.StealthChance(c, b, false), true)
	if ok, _ := g.Disable(c, 2, 1); !ok {
		t.Fatal("building disable did not succeed")
	}
	// economy.IsActive treats ORDER_DISABLE as inactive (complex.go), so this
	// alone drops the building out of production.
	if b.Order != game.OrderDisable {
		t.Errorf("building order = %v, want OrderDisable", b.Order)
	}
}

func TestStealCapturesEnemyUnit(t *testing.T) {
	g := newGame(t, 8, 8)
	c := addCommando(g, 0, 1, 1)
	e := addElectronicEnemy(g, 1, 2, 1, 8, 0)

	seedForRoll(t, g.StealthChance(c, e, true), true)
	ok, _ := g.Steal(c, 2, 1)
	if !ok {
		t.Fatal("steal did not succeed with a forced hit")
	}
	if e.Team != c.Team {
		t.Errorf("stolen unit team = %d, want %d (commando's)", e.Team, c.Team)
	}
}

func TestInfiltrateEligibility(t *testing.T) {
	g := newGame(t, 12, 12)
	c := addCommando(g, 0, 1, 1)

	// Airborne (hovering air) target: never targetable.
	air := addElectronicEnemy(g, 1, 2, 1, 8, game.FlagMobileAirUnit|game.FlagHovering)
	if g.CanDisable(c, air) {
		t.Error("an airborne unit must not be able to be disabled")
	}
	// Target that is not adjacent.
	far := addElectronicEnemy(g, 1, 8, 8, 8, 0)
	if g.CanDisable(c, far) {
		t.Error("a unit that is not adjacent must not be able to be disabled")
	}
	// Organic (not electronic) target.
	inf := game.NewUnitInfo(assets.Infantry, 1, 70, 0)
	inf.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	inf.Values = &game.UnitValues{}
	g.Units.Deploy(inf, 2, 1)
	if g.CanDisable(c, inf) {
		t.Error("a unit that is not electronic must not be able to be disabled")
	}
	// Out of shots.
	e := addElectronicEnemy(g, 1, 1, 2, 8, 0)
	c.Shots = 0
	if g.CanDisable(c, e) {
		t.Error("a commando with no shots must not disable")
	}
}

// TestInfiltrateRejectsBusyOrDyingTarget checks the shared disable/steal rejections: a target that is
// transforming, firing, exploding or being destroyed cannot be infiltrated.
func TestInfiltrateRejectsBusyOrDyingTarget(t *testing.T) {
	cases := []struct {
		name  string
		order game.Order
		state game.OrderState
	}{
		{"transforming", game.OrderTransform, game.OrderStateInit},
		{"firing", game.OrderFire, game.OrderStateInit},
		{"exploding", game.OrderExplode, game.OrderStateInit},
		{"being destroyed", game.OrderAwait, game.OrderStateDestroy},
	}
	for _, tc := range cases {
		g := newGame(t, 12, 12)
		c := addCommando(g, 0, 1, 1)
		target := addElectronicEnemy(g, 1, 2, 1, 8, 0)
		if !g.CanDisable(c, target) || !g.CanSteal(c, target) {
			t.Fatalf("%s: the idle baseline target must be disable and steal eligible", tc.name)
		}
		target.Order, target.State = tc.order, tc.state
		if g.CanDisable(c, target) {
			t.Errorf("%s: a %s target must not be disabled", tc.name, tc.name)
		}
		if g.CanSteal(c, target) {
			t.Errorf("%s: a %s target must not be stolen", tc.name, tc.name)
		}
	}
}

func TestInfiltrateMissExposesCommando(t *testing.T) {
	g := newGame(t, 8, 8)
	c := addCommando(g, 0, 1, 1)
	e := addElectronicEnemy(g, 1, 2, 1, 8, 0)

	seedForRoll(t, g.StealthChance(c, e, false), false) // force a miss
	ok, exposed := g.Disable(c, 2, 1)
	if ok {
		t.Fatal("disable succeeded despite a forced miss")
	}
	if !exposed {
		t.Error("a failed attempt against a live target should expose the commando")
	}
	if g.IsDisabled(e) {
		t.Error("target should not be disabled after a miss")
	}
	if c.Shots != 0 {
		t.Errorf("commando shots = %d, want 0 (a miss still spends a shot)", c.Shots)
	}
	if c.Experience != 0 {
		t.Errorf("commando experience = %d, want 0 (no XP on a miss)", c.Experience)
	}
}

// TestCommandoEventCarriesTheVictimTeam pins the queue the VICTIM's side is told from:
// a capture rewrites the target's team in place, so the event has to remember who owned
// it when the raid landed - otherwise the screen looks at a unit that already belongs to
// the raider and reports the loss to nobody.
func TestCommandoEventCarriesTheVictimTeam(t *testing.T) {
	g := newGame(t, 8, 8)
	c := addCommando(g, 0, 1, 1)
	e := addElectronicEnemy(g, 1, 2, 1, 8, 0)

	seedForRoll(t, g.StealthChance(c, e, true), true) // force a successful steal
	if ok, _ := g.Steal(c, 2, 1); !ok {
		t.Fatal("steal failed despite a forced hit")
	}

	events := g.DrainCommandoEvents()
	if len(events) != 1 {
		t.Fatalf("drained %d commando events, want 1", len(events))
	}
	if events[0].Kind != CommandoCapture {
		t.Errorf("event kind = %v, want a capture", events[0].Kind)
	}
	if events[0].VictimTeam != 1 {
		t.Errorf("victim team = %d, want 1 (the team that LOST the unit)", events[0].VictimTeam)
	}
	if e.Team != 0 {
		t.Errorf("captured unit team = %d, want 0 (the raider's)", e.Team)
	}
	if events[0].Victim != e || events[0].Commando != c {
		t.Errorf("event = %+v, want the stolen tank and the commando", events[0])
	}
}

// TestCommandoDisableAndMissQueueEvents covers the other two kinds: a disable carries
// its duration, and a caught infiltrator is queued for the team it was caught by.
func TestCommandoDisableAndMissQueueEvents(t *testing.T) {
	g := newGame(t, 8, 8)
	c := addCommando(g, 0, 1, 1)
	e := addElectronicEnemy(g, 1, 2, 1, 8, 0)

	seedForRoll(t, g.StealthChance(c, e, false), true) // force a successful disable
	if ok, _ := g.Disable(c, 2, 1); !ok {
		t.Fatal("disable failed despite a forced hit")
	}

	events := g.DrainCommandoEvents()
	if len(events) != 1 || events[0].Kind != CommandoDisable {
		t.Fatalf("drained %+v, want one disable event", events)
	}
	if events[0].Turns != g.DisabledTurns(e) || events[0].Turns < 1 {
		t.Errorf("event turns = %d, want the live disable duration %d", events[0].Turns, g.DisabledTurns(e))
	}
	if events[0].VictimTeam != 1 {
		t.Errorf("victim team = %d, want 1", events[0].VictimTeam)
	}

	// A miss queues the exposure, flagged with what was being attempted.
	c.Shots = 1
	other := addElectronicEnemy(g, 1, 1, 2, 8, 0)
	seedForRoll(t, g.StealthChance(c, other, true), false) // force a missed steal
	if ok, exposed := g.Steal(c, 1, 2); ok || !exposed {
		t.Fatalf("steal ok=%v exposed=%v, want a miss that exposes", ok, exposed)
	}

	events = g.DrainCommandoEvents()
	if len(events) != 1 || events[0].Kind != CommandoExposed {
		t.Fatalf("drained %+v, want one exposed event", events)
	}
	if !events[0].Steal {
		t.Error("the exposed event should record that a CAPTURE was attempted")
	}
	if events[0].VictimTeam != 1 {
		t.Errorf("victim team = %d, want 1 (the team that caught the infiltrator)", events[0].VictimTeam)
	}
}
