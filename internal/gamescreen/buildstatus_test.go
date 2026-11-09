// buildstatus_test.go : tests the factory build status indicator on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// TestBuildStatusMessage checks that selecting a builder mid construction reports
// what it is raising and how many turns remain, and that a unit that is not
// building says nothing.
func TestBuildStatusMessage(t *testing.T) {
	s, g := newTestScreen(t, 16)

	// A constructor raising a 12-turn Mining Station.
	con := mkUnit(g, assets.Constrct, 0, 1, 5, 5, game.FlagMobileLandUnit|game.FlagConstructorUnit|game.FlagSelectable)
	prod := game.NewUnitInfo(assets.MiningSt, 0, 99, 0)
	prod.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	prod.Values = &game.UnitValues{}
	prod.Values.Set(game.AttribTurns, 12)
	g.StartBuild(con, prod, 5, 5, 1)

	s.buildStatusMessage(con)
	if !strings.Contains(s.banner.msg, "Mining Station") || !strings.Contains(s.banner.msg, "12") {
		t.Errorf("build status message = %q, want it to name Mining Station + 12 turns", s.banner.msg)
	}

	// A plain mobile unit (not building) reports nothing.
	s.banner.msg = ""
	tank := mkUnit(g, assets.Tank, 0, 2, 8, 8, game.FlagMobileLandUnit|game.FlagSelectable)
	s.buildStatusMessage(tank)
	if s.banner.msg != "" {
		t.Errorf("a unit that is not building must not post a build status message, got %q", s.banner.msg)
	}
}

// producingFactory wires a factory owned by the player, mid production of a 6-turn Tank and
// returns it: queue + rate set (so FactoryProducing is true) and a live build order.
func producingFactory(t *testing.T, g *sim.Game) *game.UnitInfo {
	t.Helper()
	f := mkUnit(g, assets.LandPlt, 0, 1, 4, 4, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	f.Order = game.OrderPowerOn
	f.State = game.OrderStateExecutingOrder
	prod := game.NewUnitInfo(assets.Tank, 0, 99, 0)
	prod.Flags = game.FlagMobileLandUnit
	prod.Values = &game.UnitValues{}
	prod.Values.Set(game.AttribTurns, 6)
	g.StartBuild(f, prod, 4, 4, 1)
	f.BuildQueue = []assets.UnitType{assets.Tank}
	f.BuildRate = 1
	return f
}

// TestFactoryStatusMessageProducing covers a reported gap: selecting your own
// producing factory now shows the vanilla "Currently building: X. Turns to
// completion: N." line (factories are not u.Building, so the old gate skipped them).
func TestFactoryStatusMessageProducing(t *testing.T) {
	s, g := newTestScreen(t, 16)
	f := producingFactory(t, g)

	s.buildStatusMessage(f)
	if !strings.Contains(s.banner.msg, "Currently building") || !strings.Contains(s.banner.msg, "Tank") || !strings.Contains(s.banner.msg, "6") {
		t.Errorf("producing-factory status = %q, want 'Currently building: Tank. Turns to completion: 6.'", s.banner.msg)
	}
}

// TestFactoryStatusMessagePaused: a STOPped factory keeps its order in progress and
// reports the paused "Was building X, with N turns to completion." line.
func TestFactoryStatusMessagePaused(t *testing.T) {
	s, g := newTestScreen(t, 16)
	f := producingFactory(t, g)
	f.BuildRate = 0 // STOP: keeps the queue + order, paused

	s.buildStatusMessage(f)
	if !strings.Contains(s.banner.msg, "Was building") || !strings.Contains(s.banner.msg, "Tank") {
		t.Errorf("paused-factory status = %q, want the 'Was building Tank, with N turns to completion.' form", s.banner.msg)
	}
}

// TestBulldozerStatusMessage: selecting a bulldozer clearing rubble reports the turns
// to clear the site (vanilla af4a).
func TestBulldozerStatusMessage(t *testing.T) {
	s, g := newTestScreen(t, 16)
	dozer := mkUnit(g, assets.Bulldozr, 0, 1, 7, 7, game.FlagMobileLandUnit|game.FlagSelectable)
	g.RestoreClearJob(dozer, 7, 7, 1, 4, nil)

	s.buildStatusMessage(dozer)
	if !strings.Contains(s.banner.msg, "clear site") || !strings.Contains(s.banner.msg, "4") {
		t.Errorf("bulldozer status = %q, want 'Number of turns to clear site: 4.'", s.banner.msg)
	}
}
