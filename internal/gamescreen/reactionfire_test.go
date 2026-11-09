// reactionfire_test.go : tests opportunity/reaction fire triggering on the game screen.
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

// TestArmedUnitPopupHasManual verifies that an armed unit/building (base ammo > 0,
// not a Commando) gets the "Manual" reaction fire toggle, that it is inactive while
// reaction fire is on (DisabledReactionFire false), and that invoking it flips the
// flag and lights the highlight. A Gun Turret has ammo, so it gets Manual alongside
// Sentry - matching vanilla. C++ ported : units_manager.cpp - Popup_OnClick_Manual.
func TestArmedUnitPopupHasManual(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0
	// A Gun Turret: stationary, armed (attack + ammo).
	turret := mkUnit(g, assets.GunTurrt, 0, 50, 4, 4, game.FlagBuilding|game.FlagStationary|game.FlagSelectable)
	turret.Values.Set(game.AttribAttack, 8)
	turret.Values.Set(game.AttribAmmo, 4)

	items := s.popupItemsFor(turret)

	// Find the Manual item.
	var manual *popupItem
	for i := range items {
		if items[i].label == "Manual" {
			manual = &items[i]
			break
		}
	}
	if manual == nil {
		t.Fatal("popup for an armed turret should contain a 'Manual' item")
	}
	// Reaction fire defaults ON, so the toggle is inactive.
	if manual.active {
		t.Error("Manual should be inactive while reaction fire is enabled")
	}
	if turret.DisabledReactionFire {
		t.Error("DisabledReactionFire should default to false")
	}

	// Invoke: switch to manual fire.
	manual.action()
	if !turret.DisabledReactionFire {
		t.Error("invoking Manual should set DisabledReactionFire to true")
	}
	// Rebuild: the highlight now follows the manual state.
	for _, it := range s.popupItemsFor(turret) {
		if it.label == "Manual" && !it.active {
			t.Error("Manual should be active once reaction fire is off")
		}
	}
}

// TestManualReleasesSentry confirms that switching a turret to manual fire while it
// is on guard releases the order back to awaiting (the original drops ORDER_SENTRY here).
func TestManualReleasesSentry(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0
	turret := mkUnit(g, assets.GunTurrt, 0, 51, 4, 4, game.FlagBuilding|game.FlagStationary|game.FlagSelectable)
	turret.Values.Set(game.AttribAttack, 8)
	turret.Values.Set(game.AttribAmmo, 4)
	turret.Order = game.OrderSentry

	items := s.popupItemsFor(turret)
	var manual *popupItem
	for i := range items {
		if items[i].label == "Manual" {
			manual = &items[i]
			break
		}
	}
	if manual == nil {
		t.Fatal("armed turret popup should contain a Manual item")
	}
	manual.action()
	if turret.Order == game.OrderSentry {
		t.Error("switching to manual fire should release an on-guard turret (drop OrderSentry)")
	}
}

// TestSelectBlipAlwaysPlays confirms the default in world select blip (KBUY0) fires
// on the SFX channel for an own unit AND an own building (incl. an otherwise silent
// non defense building), independent of the announcer voice.
func TestSelectBlipAlwaysPlays(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.State.PlayerTeam = 0
	var sfx []string
	s.sound.sfxFn = func(primary, _ string, _ float64) { sfx = append(sfx, primary) }

	// An own mobile unit: blip plays.
	tank := mkUnit(g, assets.Tank, 0, 1, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	s.unitStatusVoice(tank)
	// An own (silent) building: blip still plays.
	pwr := mkUnit(g, assets.PowerStn, 0, 2, 6, 6, game.FlagBuilding|game.FlagStationary|game.FlagSelectable)
	s.unitStatusVoice(pwr)

	if len(sfx) != 2 {
		t.Fatalf("select blip should play once per select, got %d plays", len(sfx))
	}
	for _, tag := range sfx {
		if tag != "KBUY0" {
			t.Errorf("select blip = %q, want KBUY0", tag)
		}
	}
}

// TestSelectBlipSkipsEnemy confirms the blip does not play for another team's unit.
func TestSelectBlipSkipsEnemy(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.State.PlayerTeam = 0
	var sfx []string
	s.sound.sfxFn = func(primary, _ string, _ float64) { sfx = append(sfx, primary) }

	enemy := mkUnit(g, assets.Tank, 0, 1, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	enemy.Team = 1
	s.unitStatusVoice(enemy)
	if len(sfx) != 0 {
		t.Errorf("no select blip should play for an enemy unit, got %d", len(sfx))
	}
}

// TestUnarmedBuildingNoManual confirms an unarmed building (no ammo) does NOT get
// the Manual reaction fire toggle.
func TestUnarmedBuildingNoManual(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.State.ActiveTeam = 0
	pwr := mkUnit(g, assets.PowerStn, 0, 60, 4, 4, game.FlagBuilding|game.FlagStationary|game.FlagSelectable)
	for _, it := range s.popupItemsFor(pwr) {
		if it.label == "Manual" {
			t.Error("an unarmed Power Station should not get a Manual item")
		}
	}
}
