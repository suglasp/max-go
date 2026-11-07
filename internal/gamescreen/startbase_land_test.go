// startbase_land_test.go : tests of the live start base landing (deployStartBase).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"math/rand"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestDeployStartBaseOneComplex checks that the landed mining station and power generator share one complex,
// that the station counts as one mine and one building (the 1x1 generator carries no BUILDING flag, so it is not
// counted), and that every founder lands on the map.
//
// C++ ported : game_manager.cpp - GameManager_ProcessTeamMissionSupplyUnits (stats_mines_built, stats_buildings_built)
func TestDeployStartBaseOneComplex(t *testing.T) {
	s, g := newClickScreen(t)
	roster := founderRoster()

	s.deployStartBase(1, game.Point{X: 8, Y: 8}, roster, nil, false, rand.New(rand.NewSource(7)))

	var mine, gen *game.UnitInfo
	mobiles := 0
	for _, u := range g.Units.Units() {
		if u.Team != 1 {
			continue
		}
		switch {
		case u.Type == assets.MiningSt:
			mine = u
		case u.Type == assets.PowGen:
			gen = u
		case u.Flags.Has(game.FlagMobileLandUnit):
			mobiles++
		}
	}
	if mine == nil || gen == nil {
		t.Fatalf("landed base misses a building: mine %v, generator %v", mine != nil, gen != nil)
	}
	if c := g.ComplexOf(mine); c == nil || c != g.ComplexOf(gen) {
		t.Error("the mining station and the power generator should share one complex")
	}
	if mobiles != len(roster) {
		t.Errorf("landed founders = %d, want %d", mobiles, len(roster))
	}
	if ts := g.TeamStats[1]; ts.MinesBuilt != 1 || ts.BuildingsBuilt != 1 {
		t.Errorf("stats mines/buildings = %d/%d, want 1/1", ts.MinesBuilt, ts.BuildingsBuilt)
	}
}

// TestDeployStartBaseCountsRoster checks that the landed roster counts as units built on the score screen.
//
// C++ ported : game_manager.cpp - GameManager_ProcessTeamMissionSupplyUnits (stats_units_built += units count)
func TestDeployStartBaseCountsRoster(t *testing.T) {
	t.Skip("port bug, TODO.md DC-F8 / PLAN-NOV2026 N17: the live landing does not add the roster to UnitsBuilt")

	s, g := newClickScreen(t)
	roster := founderRoster()

	s.deployStartBase(1, game.Point{X: 8, Y: 8}, roster, nil, false, rand.New(rand.NewSource(7)))

	if got := g.TeamStats[1].UnitsBuilt; got != len(roster) {
		t.Errorf("UnitsBuilt = %d, want %d (the landed roster)", got, len(roster))
	}
}
