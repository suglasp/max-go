// sentry_test.go : tests sentry mode assignment and reaction fire integration on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// TestDefenseBuildingPopupHasSentry verifies that a defense building's popup
// includes a Sentry item (gated on isDefenseBuilding, not mobile).
func TestDefenseBuildingPopupHasSentry(t *testing.T) {
	s, g := newClickScreen(t)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0
	// A Gun Turret: stationary + attack > 0 = defense building.
	turret := mkUnit(g, assets.GunTurrt, 50, 4, 4, game.FlagBuilding|game.FlagStationary|game.FlagSelectable)
	turret.Values.Set(game.AttribAttack, 8)
	turret.Values.Set(game.AttribRange, 4)
	turret.Order = game.OrderSentry // mirrors the finishBuild default

	items := s.popupItemsFor(turret)

	// Find the Sentry item.
	var sentryItem *popupItem
	for i := range items {
		if items[i].label == "Sentry" {
			sentryItem = &items[i]
			break
		}
	}
	if sentryItem == nil {
		t.Fatal("popup for a defense building should contain a 'Sentry' item")
	}
	// Should be active (on guard by default).
	if !sentryItem.active {
		t.Error("Sentry item should be active when the turret is in OrderSentry")
	}

	// Invoke toggle: turret goes off guard.
	sentryItem.action()
	if turret.Order == game.OrderSentry {
		t.Error("invoking Sentry action should toggle off (remove OrderSentry)")
	}
	// Recheck active state after rebuilding items.
	items = s.popupItemsFor(turret)
	for _, it := range items {
		if it.label == "Sentry" {
			if it.active {
				t.Error("Sentry item should be inactive after toggling off")
			}
			break
		}
	}
}

// TestNonDefenseBuildingNoSentry confirms that a stationary building without an
// attack value (e.g. a Power Station) does NOT get a Sentry popup item.
func TestNonDefenseBuildingNoSentry(t *testing.T) {
	s, g := newClickScreen(t)
	g.State.ActiveTeam = 0
	pwr := mkUnit(g, assets.PowerStn, 60, 4, 4, game.FlagBuilding|game.FlagStationary|game.FlagSelectable)
	pwr.Order = game.OrderPowerOff
	for _, it := range s.popupItemsFor(pwr) {
		if it.label == "Sentry" {
			t.Error("Power Station popup should not include a Sentry item")
		}
	}
}

// TestToggleSentry checks that the popup controls ORDER_SENTRY and ORDER_AWAIT.
// Selection advances to the next waiting mobile unit and skips guarded units.
// A manual move replaces ORDER_SENTRY and drops the guard state.
func TestToggleSentry(t *testing.T) {
	m := &world.Map{Surface: make([]uint8, 16*16)}
	m.W, m.H = 16, 16
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0
	s := &Screen{game: g, wmap: m, r: drawmap.New(m)}

	mk := func(id, gx, gy int) *game.UnitInfo {
		u := game.NewUnitInfo(assets.Tank, 0, id, 0)
		u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
		u.Values = &game.UnitValues{}
		u.Values.Set(game.AttribSpeed, 10)
		u.Values.Set(game.AttribHits, 20)
		u.Hits = 20
		u.Speed = 10 // live per turn budget (selectNextWithMoves gates on this)
		g.Units.Deploy(u, gx, gy)
		return u
	}
	// A selectable BUILDING (mining station) with a high id: an extra unit for the
	// awaiting orders cycle checks below.
	ms := game.NewUnitInfo(assets.MiningSt, 0, 99, 0)
	ms.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	ms.Values = &game.UnitValues{}
	g.Units.Deploy(ms, 10, 10)

	a := mk(1, 4, 4)
	b := mk(2, 6, 4)

	// Sentry A while it is selected: it goes on guard and STAYS selected (no jump to
	// another unit - a found issue was that the automatic advance felt disorienting).
	s.selected = a
	s.toggleSentry(a)
	if a.Order != game.OrderSentry {
		t.Fatalf("toggleSentry: A order = %v, want OrderSentry", a.Order)
	}
	if s.selected != a {
		t.Errorf("sentry should keep A selected (no auto-jump), got %v", s.selected)
	}

	// A is out of the awaiting orders cycle: from the building, Next reaches B, not
	// the sentried A (without the skip it would wrap to A, the lower id).
	s.selected = ms
	s.cycleSelection(1)
	if s.selected != b {
		t.Errorf("Next should skip the sentried unit A and reach B, got %v", s.selected)
	}

	// Release A: back to awaiting orders and back into the cycle.
	s.toggleSentry(a)
	if a.Order != game.OrderAwait {
		t.Fatalf("toggleSentry release: A order = %v, want OrderAwait", a.Order)
	}
	s.selected = ms
	s.cycleSelection(1) // ids [A(1), B(2), ms(99)]: Next from ms wraps to A
	if s.selected != a {
		t.Error("a released unit should be back in the awaiting-orders cycle")
	}

	// Clearing on move: a fresh move order replaces ORDER_SENTRY (drops guard).
	s.selected = a
	s.toggleSentry(a)
	if a.Order != game.OrderSentry {
		t.Fatalf("re-sentry: A order = %v, want OrderSentry", a.Order)
	}
	if !g.OrderMove(a, 5, 4) {
		t.Fatal("OrderMove should succeed on open land")
	}
	if a.Order == game.OrderSentry {
		t.Error("ordering a sentried unit to move should drop guard (order no longer OrderSentry)")
	}
}
