// statpower_test.go : tests statpower.go.
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

// TestSumComplexPowerSplitsProductionAndConsumption checks the info panel power
// gauge math: a power station (produces 6) plus a powered comm tower (draws 1)
// in one complex report curProd/capProd 6 and curUse/capUse 1, so a selected
// generator's Total = available power and Usage = consumed power.
func TestSumComplexPowerSplitsProductionAndConsumption(t *testing.T) {
	m := &world.Map{Surface: make([]uint8, 16*16)}
	m.W, m.H = 16, 16
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	g.State.ActiveTeam, g.State.PlayerTeam = 0, 0
	s := &Screen{game: g, wmap: m, r: drawmap.New(m)}

	gen := building(assets.PowerStn, 0, 8, 8, game.OrderPowerOn, 0)
	tower := building(assets.CommTwr, 0, 10, 8, game.OrderPowerOn, 0)
	g.PlaceBuilding(gen, 8, 8)
	g.PlaceBuilding(tower, 10, 8) // merges into the generator's complex

	cs := s.sumComplexResource(gen, false)
	if cs.capProd != 6 || cs.curProd != 6 {
		t.Errorf("production sums = cur %d / cap %d, want 6 / 6", cs.curProd, cs.capProd)
	}
	if cs.capUse != 1 || cs.curUse != 1 {
		t.Errorf("consumption sums = cur %d / cap %d, want 1 / 1", cs.curUse, cs.capUse)
	}
}

// TestComplexCargoSumsPooledRaw checks the mining station "Total" row math: two
// raw storing mining stations in one complex report the pooled stock and the
// pooled capacity, so HITS / RAW (own) / TOTAL reads the whole base's raw.
func TestComplexCargoSumsPooledRaw(t *testing.T) {
	m := &world.Map{Surface: make([]uint8, 16*16)}
	m.W, m.H = 16, 16
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	g.State.ActiveTeam, g.State.PlayerTeam = 0, 0
	s := &Screen{game: g, wmap: m, r: drawmap.New(m)}

	mineA := building(assets.MiningSt, 0, 8, 8, game.OrderPowerOn, 20)
	mineB := building(assets.MiningSt, 0, 10, 8, game.OrderPowerOn, 20)
	mineA.Storage, mineB.Storage = 5, 7
	g.PlaceBuilding(mineA, 8, 8)
	g.PlaceBuilding(mineB, 10, 8) // merges into one complex

	cur, capacity := s.complexCargo(mineA)
	if cur != 12 || capacity != 40 {
		t.Errorf("complex raw = %d / %d, want 12 / 40", cur, capacity)
	}
	if key := s.cargoIconKey(assets.MiningSt); key != "CARGO" {
		t.Errorf("mining station cargo icon = %q, want CARGO (raw)", key)
	}
}

// TestSumComplexResourceNoComplex confirms a building with no complex (e.g. a
// bench unit) yields zero sums, so only its own contribution row draws.
func TestSumComplexResourceNoComplex(t *testing.T) {
	m := &world.Map{Surface: make([]uint8, 16*16)}
	m.W, m.H = 16, 16
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	g.State.ActiveTeam, g.State.PlayerTeam = 0, 0
	s := &Screen{game: g, wmap: m, r: drawmap.New(m)}

	loose := building(assets.PowerStn, 0, 5, 5, game.OrderPowerOn, 0) // not placed
	cs := s.sumComplexResource(loose, false)
	if cs != (complexResourceSums{}) {
		t.Errorf("a building with no complex must sum to zero, got %+v", cs)
	}
}
