// manualfire_fog_test.go : tests the fog gate for manual fire.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// TestClickDoesNotBlindFireIntoFog: with fog shown, a left click on a cell hiding an
// in range but unrevealed enemy must NOT fire (no blind fire into fog); once the enemy is
// revealed by the player's own scan coverage the same click fires.
func TestClickDoesNotBlindFireIntoFog(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	tank := clickTestUnit(0, 1, 5, 5, mgr)  // player attacker (clickTestUnit sets no scan)
	enemy := clickTestUnit(1, 2, 7, 5, mgr) // enemy 2 cells east, in range 4
	enemy.Order = game.OrderAwait           // not idle, so visibility keys on coverage
	g.RefreshVisibility()

	s := &Screen{game: g}
	s.fogShow = true // the player is looking at fog
	s.selected = tank
	startHits := enemy.Hits

	if s.unitVisibleToPlayer(enemy) {
		t.Fatal("setup: the enemy should be fogged (the attacker has no scan coverage)")
	}
	s.clickCell(7, 5)
	if enemy.Hits < startHits {
		t.Errorf("clickCell blind-fired into fog at an unrevealed enemy (hits %d -> %d)", startHits, enemy.Hits)
	}

	// Reveal the enemy via the attacker's own scan coverage; the same click now fires.
	tank.Values.Set(game.AttribScan, 8)
	g.RefreshVisibility()
	if !s.unitVisibleToPlayer(enemy) {
		t.Fatal("setup: the enemy should now be revealed to the player")
	}
	s.selected = tank // isolate the visible case assertion from the prior click
	s.clickCell(7, 5)
	if enemy.Hits >= startHits {
		t.Errorf("clickCell did not fire on a revealed in-range enemy (hits still %d)", enemy.Hits)
	}
}
