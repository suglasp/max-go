// deploy_overlap_test.go : tests that starting units occupy distinct cells.
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
	"github.com/suglasp/max-go/internal/sim"
)

// founderRoster is the founder mix plus a small combat tail a random / custom base fields.
func founderRoster() []unitSpec {
	return []unitSpec{
		{assets.Constrct, flagsLand | game.FlagConstructorUnit},
		{assets.Engineer, flagsLand | game.FlagConstructorUnit},
		{assets.Surveyor, flagsLand},
		{assets.Scout, flagsLand},
		{assets.Scout, flagsLand},
		{assets.Tank, flagsLand},
	}
}

// mobileCellsOf groups every mobile land unit on the game by its cell.
func mobileCellsOf(g *sim.Game) map[game.Point][]assets.UnitType {
	occupied := map[game.Point][]assets.UnitType{}
	for _, u := range g.Units.Units() {
		if !u.Flags.Has(game.FlagMobileLandUnit) {
			continue
		}
		p := game.Point{X: u.GridX, Y: u.GridY}
		occupied[p] = append(occupied[p], u.Type)
	}
	return occupied
}

// TestDeployStartBaseNoMobileOverlap: a single base never stacks two mobile founders
// (the per team taken map guards it). A full founder roster on a flat map, each unit
// on its own cell.
func TestDeployStartBaseNoMobileOverlap(t *testing.T) {
	s, g := newClickScreen(t)
	rng := rand.New(rand.NewSource(7))

	s.deployStartBase(1, game.Point{X: 8, Y: 8}, founderRoster(), nil, false, rng)

	for p, types := range mobileCellsOf(g) {
		if len(types) > 1 {
			t.Errorf("cell (%d,%d) holds %d mobile units %v; each must have its own cell", p.X, p.Y, len(types), types)
		}
	}
}

// TestDeployStartBaseNoCrossTeamOverlap verifies that nearby starting bases cannot stack units from different teams.
// Each deployStartBase call owns a separate taken map, so cross-team protection comes from the shared grid occupancy check.
func TestDeployStartBaseNoCrossTeamOverlap(t *testing.T) {
	s, g := newClickScreen(t)
	rng := rand.New(rand.NewSource(7))

	// Two NPC bases centred two cells apart, so their outward search rings overlap.
	s.deployStartBase(1, game.Point{X: 6, Y: 8}, founderRoster(), nil, false, rng)
	s.deployStartBase(2, game.Point{X: 8, Y: 8}, founderRoster(), nil, false, rng)

	for p, types := range mobileCellsOf(g) {
		if len(types) > 1 {
			t.Errorf("cell (%d,%d) holds %d mobile units %v across teams; each must have its own cell", p.X, p.Y, len(types), types)
		}
	}
}
