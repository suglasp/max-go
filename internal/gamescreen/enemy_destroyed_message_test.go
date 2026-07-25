// enemy_destroyed_message_test.go : tests the message posted after destroying a visible enemy.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// TestEnemyDestroyedMessage checks the vanilla kill banner: destroying a VISIBLE
// enemy unit posts "Enemy <type> at [x,y] destroyed." with 1-based coordinates (the
// message comes from tickCasualties, so a reaction / mine / AI kill triggers it too).
// C++: unitinfo.cpp - the e6d7 path.
func TestEnemyDestroyedMessage(t *testing.T) {
	const w, h = 16, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.State.PlayerTeam = 0

	// A player tank that can see and kill the adjacent enemy in a single shot.
	tank := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribAttack, 24)
	tank.Values.Set(game.AttribRange, 4)
	tank.Values.Set(game.AttribRounds, 1)
	tank.Values.Set(game.AttribScan, 6)
	tank.Values.Set(game.AttribHits, 30)
	tank.Hits = 30
	tank.Shots = 1
	tank.Ammo = 1
	tank.Order = game.OrderAwait // not idle, so it projects scan coverage (sees the enemy)
	mgr.Deploy(tank, 5, 4)

	// A weak, active (not idle, so it is spotted) enemy tank two cells away.
	enemy := game.NewUnitInfo(assets.Tank, 1, 2, 0)
	enemy.Flags = game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite
	enemy.Values = &game.UnitValues{}
	enemy.Values.Set(game.AttribHits, 1)
	enemy.Hits = 1
	enemy.Order = game.OrderAwait
	mgr.Deploy(enemy, 7, 4)

	g.RefreshVisibility()
	s := &Screen{game: g, fogShow: true, fogTeam: 0, turnFlow: turnFlowState{handover: -1}} // -1: no turn banner masking the message

	if _, ok := g.Attack(tank, 7, 4); !ok {
		t.Fatal("the player tank failed to attack the adjacent enemy")
	}

	if g.Units.ByID.Get(enemy.ID) != nil {
		t.Fatal("the enemy should have been destroyed")
	}

	s.tickCasualties() // drains the kill -> posts the message

	text, ok, tint := s.bannerText()
	if !ok || tint != bannerTintDark {
		t.Fatalf("killing a visible enemy: banner=(%q, ok=%v, tint=%v), want a normal dark message", text, ok, tint)
	}

	if want := "Enemy Tank at [8,5] destroyed."; text != want {
		t.Errorf("enemy-destroyed message = %q, want %q", text, want)
	}
}
