// random2_test.go : tests random2.go.
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
	"github.com/suglasp/max-go/internal/world"
)

// TestBuildBigBaseFormsConnectedComplex checks that the -emulate random2 base lays
// out as ONE connected complex: every land building (the mining stations, power
// stations, factories, dumps, gold vault, radar and the connectors) shares the
// mining station's complex, so the pooled raw is reachable from any of them and a
// supply truck parked next to any base building can use X-Fer to reload. This covers
// the old fixed grid placement that fragmented the base into stranded complexes.
func TestBuildBigBaseFormsConnectedComplex(t *testing.T) {
	const w, h = 48, 48
	m := &world.Map{Preview: world.Preview{W: w, H: h}, Surface: make([]uint8, w*h)}
	// All land, so site validity never bumps a building off its connected slot.
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.New(p, m)
	s := &Screen{game: g, wmap: m}

	s.buildBigBase(0, game.Point{X: 24, Y: 24}, rand.New(rand.NewSource(7)))

	// Collect team-0 land buildings: skip ground cover (roads/slabs) and the naval
	// Shipyard/Bridge, which stand on water and form their own complex.
	var mine *game.UnitInfo
	var land []*game.UnitInfo
	for _, u := range g.Units.Units() {
		if u.Team != 0 || !u.Flags.Has(game.FlagStationary) || u.Flags.Has(game.FlagGroundCover) {
			continue
		}
		if u.Type == assets.Shipyard || u.Type == assets.Bridge {
			continue
		}
		land = append(land, u)
		if u.Type == assets.MiningSt && mine == nil {
			mine = u
		}
	}
	if mine == nil {
		t.Fatal("no mining station was placed")
	}
	cmine := g.ComplexOf(mine)
	if cmine == nil {
		t.Fatal("the mining station is not in a complex")
	}
	if len(land) < 10 {
		t.Fatalf("only %d land buildings placed, expected the full base", len(land))
	}
	for _, u := range land {
		if g.ComplexOf(u) != cmine {
			t.Errorf("%v at (%d,%d) is NOT in the mine's complex - the base fragmented", u.Type, u.GridX, u.GridY)
		}
	}
}

// TestBuildBigBaseConnectorLaysNoSlab verifies the sweep fix: a Cnct4W connector is a
// terrain special, so it must lay NO slab (the bench wrongly seeded it with
// FlagRequiresSlab, dropping a SmlSlab under each). Real 1x1 buildings (dumps /
// radar / generator) still lay their slab; only the connectors must not.
func TestBuildBigBaseConnectorLaysNoSlab(t *testing.T) {
	const w, h = 48, 48
	m := &world.Map{Preview: world.Preview{W: w, H: h}, Surface: make([]uint8, w*h)}
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.New(p, m)
	s := &Screen{game: g, wmap: m}

	s.buildBigBase(0, game.Point{X: 24, Y: 24}, rand.New(rand.NewSource(7)))

	connectors := map[game.Point]bool{}
	slabs := map[game.Point]bool{}
	for _, u := range g.Units.Units() {
		if u.Team != 0 {
			continue
		}
		switch u.Type {
		case assets.Cnct4W:
			connectors[game.Point{X: u.GridX, Y: u.GridY}] = true
		case assets.SmlSlab:
			slabs[game.Point{X: u.GridX, Y: u.GridY}] = true
		}
	}
	if len(connectors) == 0 {
		t.Fatal("no connectors were placed in the base")
	}
	for c := range connectors {
		if slabs[c] {
			t.Errorf("connector at (%d,%d) laid a slab - a terrain special must lay none", c.X, c.Y)
		}
	}
}
