// fixture_test.go : the unit builders and the test screen shared by the gamescreen tests.
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
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/world"
)

// allLandSurf is a surface that is land on every cell (for an access.Grid).
type allLandSurf struct{}

// SurfaceAt reports land for every cell.
func (allLandSurf) SurfaceAt(x, y int) uint8 { return world.SurfaceLand }

// flatWorld builds an all land map of w x h cells.
func flatWorld(w, h int) *world.Map {
	m := &world.Map{Surface: make([]uint8, w*h)}
	m.W, m.H = w, h
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	return m
}

// newTestScreen builds a headless Screen on an all land map of size x size cells, with team 0
// as the player (active and in view) and the listed teams as computer opponents.
func newTestScreen(t *testing.T, size int, computers ...int) (*Screen, *sim.Game) {
	t.Helper()
	m := flatWorld(size, size)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	for _, team := range computers {
		p.Teams[team].Type = game.TeamComputer
	}
	g := sim.New(p, m)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0
	return &Screen{game: g, wmap: m, r: drawmap.New(m)}, g
}

// mkUnit deploys a unit of a type and team with the given flags and empty values.
func mkUnit(g *sim.Game, t assets.UnitType, team, id, gx, gy int, flags game.Flags) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, id, 0)
	u.Flags = flags
	u.Values = &game.UnitValues{}
	g.Units.Deploy(u, gx, gy)
	return u
}

// tankStats holds the values a test tank is given; a zero value stays unset.
type tankStats struct {
	angle                                                      uint8
	attack, rng, rounds, ammo, speed, scan, armor, hits, turns int
	flags                                                      game.Flags // added to the mobile land and selectable flags
}

// mkTank deploys a selectable land tank with the given stats (InitState sets hits, ammo, shots and speed from them).
func mkTank(mgr *units.Manager, team, id, gx, gy int, st tankStats) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, st.angle)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable | st.flags
	u.Values = &game.UnitValues{}
	for _, a := range []struct {
		attr  game.Attrib
		value int
	}{
		{game.AttribAttack, st.attack}, {game.AttribRange, st.rng}, {game.AttribRounds, st.rounds},
		{game.AttribAmmo, st.ammo}, {game.AttribSpeed, st.speed}, {game.AttribScan, st.scan},
		{game.AttribArmor, st.armor}, {game.AttribHits, st.hits}, {game.AttribTurns, st.turns},
	} {
		if a.value != 0 {
			u.Values.Set(a.attr, a.value)
		}
	}
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}

// t4Truck deploys a mobile cargo unit with a storage capacity and a current load.
func t4Truck(g *sim.Game, typ assets.UnitType, id, gridX, gridY, capacity, stored int) *game.UnitInfo {
	u := mkUnit(g, typ, 0, id, gridX, gridY, game.FlagMobileLandUnit|game.FlagSelectable)
	u.Values.Set(game.AttribStorage, capacity)
	u.Storage = stored

	return u
}

// mkBuilding makes a building of a type and team with the given flags, order and storage
// capacity, at a cell it does not deploy to (the caller places it).
func mkBuilding(t assets.UnitType, team, id, gx, gy int, flags game.Flags, order game.Order, storage int) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, id, 0)
	u.Flags = flags
	u.GridX, u.GridY = gx, gy
	u.Order = order
	u.State = game.OrderStateExecutingOrder
	u.Values = &game.UnitValues{}
	if storage != 0 {
		u.Values.Set(game.AttribStorage, storage)
	}
	return u
}

// spoke reports whether tag is among the played voice tags.
func spoke(played []string, tag string) bool {
	for _, t := range played {
		if t == tag {
			return true
		}
	}
	return false
}
