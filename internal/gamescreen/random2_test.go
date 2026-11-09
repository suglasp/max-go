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
)

// TestBuildBigBaseFormsConnectedComplex checks that the -emulate random2 base lays
// out as ONE connected complex: every land building (the mining stations, power
// stations, factories, dumps, gold vault, radar and the connectors) shares the
// mining station's complex, so the pooled raw is reachable from any of them and a
// supply truck parked next to any base building can use X-Fer to reload. This covers
// the old fixed grid placement that fragmented the base into stranded complexes.
func TestBuildBigBaseFormsConnectedComplex(t *testing.T) {
	const w = 48
	s, g := newTestScreen(t, w, 1)

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
	const w = 48
	s, g := newTestScreen(t, w, 1)

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

// landPlantReach counts the cells a Tank of the plant's team can drive to from the plant's exit ring, stopping at
// limit.
func landPlantReach(s *Screen, plant *game.UnitInfo, limit int) int {
	tank := game.NewUnitInfo(assets.Tank, plant.Team, 0, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	seen := map[game.Point]bool{}
	var queue []game.Point

	for y := plant.GridY - 1; y <= plant.GridY+2; y++ {
		for x := plant.GridX - 1; x <= plant.GridX+2; x++ {
			inside := x >= plant.GridX && x <= plant.GridX+1 && y >= plant.GridY && y <= plant.GridY+1

			if !inside && s.game.CanEnter(tank, x, y) {
				seen[game.Point{X: x, Y: y}] = true
				queue = append(queue, game.Point{X: x, Y: y})
			}
		}
	}

	for len(queue) > 0 && len(seen) < limit {
		cur := queue[0]
		queue = queue[1:]

		for _, d := range [4]game.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
			next := game.Point{X: cur.X + d.X, Y: cur.Y + d.Y}

			if !seen[next] && s.game.CanEnter(tank, next.X, next.Y) {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}

	return len(seen)
}

// TestRandom2PlantsCanBuild runs the bench on SNOW_1 for a few seeds: every unit factory starts powered off (it
// only runs with something queued), and a unit leaving a land plant can drive well away from it (the base used
// to box the Light Vehicle Plant in on every one of these seeds).
func TestRandom2PlantsCanBuild(t *testing.T) {
	const reach = 20

	for seed := int64(1); seed <= 4; seed++ {
		s := benchScreen(t, "SNOW_1.WRL")
		s.RandomStart2(seed, 2)
		plants := 0

		for _, u := range s.game.Units.Units() {
			if isFactory(u.Type) && u.Order != game.OrderPowerOff {
				t.Errorf("seed %d: team %d %v starts with order %v, want OrderPowerOff", seed, u.Team, u.Type, u.Order)
			}

			if u.Type != assets.LightPlt && u.Type != assets.LandPlt {
				continue
			}

			plants++

			if got := landPlantReach(s, u, reach); got < reach {
				t.Errorf("seed %d: team %d %v at (%d,%d) reaches %d cells from its exit ring, want %d", seed, u.Team, u.Type, u.GridX, u.GridY, got, reach)
			}
		}

		if plants == 0 {
			t.Errorf("seed %d: no land plant placed", seed)
		}
	}
}
