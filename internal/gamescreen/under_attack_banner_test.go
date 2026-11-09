// under_attack_banner_test.go : tests the banner posted when a player unit is hit.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// newDamageScreen builds a headless Screen on a flat map with the player as team 0.
func newDamageScreen(t *testing.T) *Screen {
	t.Helper()
	m := flatWorld(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamPlayer
	g := sim.New(p, m)
	g.State.PlayerTeam = 0
	return &Screen{game: g, wmap: m, turnFlow: turnFlowState{handover: -1}}
}

// mkDamagedTank returns a tank of the given team with the given current and max hits.
func mkDamagedTank(team, hits, max int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, 1, 0)
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, max)
	u.Hits = hits
	return u
}

// TestUnitUnderAttackBanner: a surviving hit on the player's own unit raises the type 1
// "under attack" banner (vanilla GameManager_NotifyEvent case 1). An enemy unit being hit
// posts nothing, and a unit already at zero hits is left to the casualty drain.
func TestUnitUnderAttackBanner(t *testing.T) {
	// A player unit that survives a hit gets the banner (world tint, type 1).
	s := newDamageScreen(t)
	s.notifyDamage(mkDamagedTank(0, 15, 30))
	text, ok, tint := s.bannerText()
	if !ok || tint != bannerTintWorld || text != "Tank under attack!" {
		t.Errorf("player hit banner = (%q, ok=%v, tint=%v), want (\"Tank under attack!\", true, world)", text, ok, tint)
	}

	// An enemy unit being hit is not the player's concern: no banner.
	s = newDamageScreen(t)
	s.notifyDamage(mkDamagedTank(1, 15, 30))
	if _, ok, _ := s.bannerText(); ok {
		t.Error("an enemy unit hit should post no under-attack banner")
	}

	// A unit reduced to zero hits is a casualty, handled by the death drain, not here.
	s = newDamageScreen(t)
	s.notifyDamage(mkDamagedTank(0, 0, 30))
	if _, ok, _ := s.bannerText(); ok {
		t.Error("a destroyed unit should post no under-attack banner (casualties handle it)")
	}
}
