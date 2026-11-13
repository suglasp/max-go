// power_generator_test.go : tests the engine managed generator power toggle.
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
)

/* // note: unused code in Go port
// replaced by the shared newTestScreen(t, 16) in fixture_test.go
// landGameForPower builds a 16x16 all land game with team 0 as the player.
func landGameForPower(t *testing.T) (*sim.Game, *world.Map) {
	t.Helper()
	m := &world.Map{Surface: make([]uint8, 16*16)}
	m.W, m.H = 16, 16
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	g.State.ActiveTeam, g.State.PlayerTeam = 0, 0
	return g, m
}
*/

// TestGeneratorPowerOffStaysOnlineWhenNeeded verifies a found gap:
// powering a generator off seemed to do nothing (factories kept running, no
// message). Generators are engine managed, so the optimizer repowers a generator
// the base still needs - the toggle now reconciles immediately and the message
// explains it, instead of the generator silently coming back next turn.
func TestGeneratorPowerOffStaysOnlineWhenNeeded(t *testing.T) {
	s, g := newTestScreen(t, 16)

	// One mine powered by one generator (one complex): the generator is needed.
	gen := mkBuilding(assets.PowGen, 0, 1, 6, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 0)
	mine := mkBuilding(assets.MiningSt, 0, 1, 8, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 0)
	g.PlaceBuilding(gen, 6, 8)
	g.PlaceBuilding(mine, 8, 8)
	for _, cx := range g.Complexes {
		if cx.Team == 0 {
			cx.Fuel = 50
		}
	}

	s.setPower(gen, false)

	if gen.Order != game.OrderPowerOn {
		t.Errorf("a needed generator must come straight back on, got %v", gen.Order)
	}
	if !strings.Contains(s.banner.msg, "needed") {
		t.Errorf("the message should explain the generator is needed, got %q", s.banner.msg)
	}
}

// TestGeneratorPowerOffSpareStaysOff: with a redundant generator already running, the
// player CAN power one off - it stays off and the other keeps the base supplied.
func TestGeneratorPowerOffSpareStaysOff(t *testing.T) {
	s, g := newTestScreen(t, 16)

	// One mine (demand 1) with TWO generators both running (a transient surplus).
	genA := mkBuilding(assets.PowGen, 0, 1, 4, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 0)
	genB := mkBuilding(assets.PowGen, 0, 1, 6, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 0)
	mine := mkBuilding(assets.MiningSt, 0, 1, 8, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 0)
	g.PlaceBuilding(genA, 4, 8)
	g.PlaceBuilding(genB, 6, 8)
	g.PlaceBuilding(mine, 8, 8)
	for _, cx := range g.Complexes {
		if cx.Team == 0 {
			cx.Fuel = 50
		}
	}

	s.setPower(genA, false)

	if genA.Order != game.OrderPowerOff {
		t.Errorf("a spare generator the player turns off should stay off, got %v", genA.Order)
	}
	if genB.Order != game.OrderPowerOn {
		t.Errorf("the other generator must keep the mine supplied, got %v", genB.Order)
	}
}
