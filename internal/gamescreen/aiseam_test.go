// aiseam_test.go : tests the computer opponent through the live round order and the live unit factory.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/ai"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/testutil"
)

// newLiveAIScreen builds a test screen whose computer opponent uses the live unit factory (newUnit).
func newLiveAIScreen(t *testing.T, size int, computers ...int) (*Screen, *sim.Game) {
	t.Helper()
	s, g := newTestScreen(t, size, computers...)
	s.lang = assets.Strings{}
	s.turnFlow = turnFlowState{handover: -1}
	s.debugMenu = debugMenuState{endless: true} // no win check ends the rounds early
	s.ai = ai.New(s.newUnit)
	s.makeUnit = s.newUnit
	s.attachGameServices()
	return s, g
}

// TestAIAdvancesInSimultaneousRounds runs the computer opponent through the live simultaneous
// round (beginSimRound, the moves resolved, commitSimTurn): every team plans at round start and
// every team ends together. A mustered computer force must close on a distant enemy across the
// rounds, as it does in the turn based order.
func TestAIAdvancesInSimultaneousRounds(t *testing.T) {
	s, g := newLiveAIScreen(t, 24, 1)
	// The tier knobs are package globals: set the tier this test needs and do not leak it.
	s.ai.SetDifficulty(ai.OpponentAverage)
	t.Cleanup(func() { s.ai.SetDifficulty(ai.OpponentAverage) })
	const firstTurn = 201 // past the Average grace floor (turn 200), so a mustered force may assault
	g.State.TurnCounter = firstTurn

	armed := tankStats{attack: 12, rng: 4, rounds: 1, ammo: 6, speed: 12, hits: 24, flags: game.FlagTurretSprite}
	enemy := mkTank(g.Units, 0, 2, 16, 2, armed)
	armed.scan = 48 // the computer tanks see the whole map, so they spot the enemy
	aiTank := mkTank(g.Units, 1, 1, 2, 2, armed)
	mkTank(g.Units, 1, 3, 2, 3, armed) // three tanks: the Average muster
	mkTank(g.Units, 1, 4, 2, 4, armed)
	distance := func() int { return max(iabsInt(aiTank.GridX-enemy.GridX), iabsInt(aiTank.GridY-enemy.GridY)) }
	start := distance()

	for round := 0; round < 10; round++ {
		s.beginSimRound()
		g.ResolveMovement()
		s.commitSimTurn()
	}

	if g.State.TurnCounter != firstTurn+10 {
		t.Fatalf("turn counter = %d after 10 rounds, want %d", g.State.TurnCounter, firstTurn+10)
	}
	if aiTank.Hits > 0 && enemy.Hits > 0 && distance() >= start {
		t.Errorf("AI tank made no progress in simultaneous rounds: still %d cells away (start %d)", distance(), start)
	}
}

// TestAIBuildsWithRealAttribs runs one computer turn with the live unit factory on the retail
// ATTRIBS table: the structure the AI queues carries the table's values (hits, build turns),
// not the empty values of a test factory.
func TestAIBuildsWithRealAttribs(t *testing.T) {
	lib, err := resfile.OpenDir(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := lib.Read("ATTRIBS")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := assets.ParseAttribs(raw)
	if err != nil {
		t.Fatal(err)
	}

	s, g := newLiveAIScreen(t, 12, 1)
	s.SetAttribs(rows)

	con := s.newUnit(1, assets.Constrct, game.FlagConstructorUnit|game.FlagMobileLandUnit|game.FlagSelectable)
	if want := rows[assets.Constrct].HitPoints; con.Hits != want || want == 0 {
		t.Fatalf("constructor hits = %d, want the ATTRIBS value %d", con.Hits, want)
	}
	con.Storage = con.Values.Get(game.AttribStorage) // a real constructor is loaded with raw to spend
	g.Units.Deploy(con, 6, 6)

	g.BeginTeamTurn(1)

	orders := g.BuildOrders()
	if len(orders) == 0 {
		t.Fatal("the AI placed no build order with the real ATTRIBS table")
	}
	for _, o := range orders {
		p := o.Product
		if want := rows[p.Type].HitPoints; p.Values.Get(game.AttribHits) != want || want == 0 {
			t.Errorf("%v product hits = %d, want the ATTRIBS value %d", p.Type, p.Values.Get(game.AttribHits), want)
		}
		if want := rows[p.Type].TurnsToBuild; p.Values.Get(game.AttribTurns) != want {
			t.Errorf("%v product build turns = %d, want the ATTRIBS value %d", p.Type, p.Values.Get(game.AttribTurns), want)
		}
	}
}
