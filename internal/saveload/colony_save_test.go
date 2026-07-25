// colony_save_test.go : colony edge (Eco Sphere) save/load tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// colonyStarted reports whether a drained event batch announces a colony start.
func colonyStarted(events []sim.ProdEvent) bool {
	for _, e := range events {
		if e.Kind == sim.ColonyStarted && e.Team == 0 {
			return true
		}
	}

	return false
}

// TestReloadDoesNotReAnnounceTheColony: the colony announcement is edge triggered off a
// latch the sim holds, and a load rebuilt the game with that latch cleared, so the first
// score pass announced a colony that had been running for turns. It must announce once,
// on the turn it starts, and stay quiet across a reload.
func TestReloadDoesNotReAnnounceTheColony(t *testing.T) {
	g := newTestGame(t)

	sphere := game.NewUnitInfo(assets.Greenhse, 0, 9, 0)
	sphere.Flags = game.FlagStationary
	sphere.Values = &game.UnitValues{}
	sphere.Values.Set(game.AttribHits, 30)
	sphere.InitState()
	sphere.Order = game.OrderPowerOn
	g.Units.Deploy(sphere, 9, 8) // flush against the test game's power generator at (8,8), so they rebuild as one complex

	g.EndTeamTurn(0)
	if !colonyStarted(g.DrainProductionEvents()) {
		t.Fatal("a colony must announce itself on the turn it starts (the test itself is wrong)")
	}

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	if n := g2.OperatingEcoSpheres(0); n != 1 {
		t.Fatalf("the reloaded Eco Sphere is not operating (%d), so this test cannot see the bug", n)
	}
	g2.DrainProductionEvents() // drop anything the restore itself queued

	g2.EndTeamTurn(0)
	if colonyStarted(g2.DrainProductionEvents()) {
		t.Error("a reloaded game re-announced a colony that never stopped: the phantom ColonyStarted is back")
	}
}
