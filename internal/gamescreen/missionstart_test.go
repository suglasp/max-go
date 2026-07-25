// missionstart_test.go : tests missionstart.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestSeedMissionCombat verifies that seedMissionCombat fills omitted ammunition and shots from ATTRIB values while preserving scripted Hits in V70 mission data.
func TestSeedMissionCombat(t *testing.T) {
	armed := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	armed.Values = &game.UnitValues{}
	armed.Values.Set(game.AttribAmmo, 6)
	armed.Values.Set(game.AttribRounds, 2)
	armed.Hits = 20 // scripted partial damage; must survive seeding

	unarmed := game.NewUnitInfo(assets.Engineer, 0, 2, 0)
	unarmed.Values = &game.UnitValues{} // ATTRIB_AMMO/ROUNDS == 0

	preloaded := game.NewUnitInfo(assets.Tank, 0, 3, 0)
	preloaded.Values = &game.UnitValues{}
	preloaded.Values.Set(game.AttribAmmo, 6)
	preloaded.Values.Set(game.AttribRounds, 2)
	preloaded.Ammo = 3 // already set: the guard must leave it
	preloaded.Shots = 1

	seedMissionCombat([]*game.UnitInfo{armed, unarmed, preloaded})

	if armed.Ammo != 6 || armed.Shots != 2 {
		t.Errorf("armed unit: Ammo=%d Shots=%d, want 6/2", armed.Ammo, armed.Shots)
	}
	if armed.Hits != 20 {
		t.Errorf("armed unit Hits = %d, want 20 (file value preserved, not clobbered)", armed.Hits)
	}
	if unarmed.Ammo != 0 || unarmed.Shots != 0 {
		t.Errorf("unarmed unit: Ammo=%d Shots=%d, want 0/0", unarmed.Ammo, unarmed.Shots)
	}
	if preloaded.Ammo != 3 || preloaded.Shots != 1 {
		t.Errorf("preloaded unit: Ammo=%d Shots=%d, want 3/1 (guard leaves existing load)", preloaded.Ammo, preloaded.Shots)
	}
}

// TestIdleEmptyFactories verifies the "Usage 1/1" on an idle mission
// factory: a plant that loaded on the retail ORDER_BUILD with an empty queue reads
// as active and drew power. It must be idled to OrderPowerOff; a producing factory
// (queue not empty), an already off one, and a unit that is not a factory are left untouched.
func TestIdleEmptyFactories(t *testing.T) {
	idle := game.NewUnitInfo(assets.LandPlt, 0, 1, 0)
	idle.Order = game.OrderBuild // stale retail "active" order, empty queue

	busy := game.NewUnitInfo(assets.LandPlt, 0, 2, 0)
	busy.Order = game.OrderBuild
	busy.BuildQueue = []assets.UnitType{assets.Tank} // has work: leave it

	off := game.NewUnitInfo(assets.LandPlt, 0, 3, 0)
	off.Order = game.OrderPowerOff // already idle: unchanged

	mobile := game.NewUnitInfo(assets.Tank, 0, 4, 0)
	mobile.Order = game.OrderBuild // a mobile unit mid order: not a factory, leave it

	idleEmptyFactories([]*game.UnitInfo{idle, busy, off, mobile})

	if idle.Order != game.OrderPowerOff {
		t.Errorf("idle empty factory Order = %#x, want OrderPowerOff (%#x)", idle.Order, game.OrderPowerOff)
	}
	if busy.Order != game.OrderBuild {
		t.Errorf("busy factory Order = %#x, want OrderBuild left alone", busy.Order)
	}
	if off.Order != game.OrderPowerOff {
		t.Errorf("already-off factory Order = %#x, want OrderPowerOff", off.Order)
	}
	if mobile.Order != game.OrderBuild {
		t.Errorf("non-factory Order = %#x, want OrderBuild left alone", mobile.Order)
	}
}
