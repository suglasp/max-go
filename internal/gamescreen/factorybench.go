// factorybench.go : provides the factory emulation bench.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// factoryRunZoom shows the base + the units rolled out clearly.
const factoryRunZoom = 36

// DebugFactoryRun is the -emulate factoryrun sandbox: a powered base with a Heavy
// Vehicle Plant + Light Vehicle Plant (each given a unit to build, so a finished
// unit parks ON the plant build ready - select the plant to see its BLDMRK exit
// markers and click one to drive the unit out), a Depot for storing/repairing
// units, a Shipyard linked back to the base by a line of connectors ("collectors")
// and sited at the water's edge when one is near, and a Scout damaged to half HP parked
// by the Depot to repair/store/unload. C++: n/a (debug layout helper).
func (s *Screen) DebugFactoryRun() {
	const player = 0

	bx, by, ok := s.snowBlock(18, 12)

	if !ok {
		bx, by = 6, 6
	}

	// Base row (all abutting -> one complex), with the row ABOVE kept clear so the
	// produced units have somewhere to roll out to. Mining | PowGen | Heavy | Light | Depot.
	row := by + 5
	s.benchPlaceBuilding(assets.MiningSt, player, bx, row, true)  // cols bx, bx+1
	s.benchPlaceBuilding(assets.PowGen, player, bx+2, row, false) // col  bx+2

	heavy := s.benchPlaceBuilding(assets.LandPlt, player, bx+3, row, true)  // cols bx+3, bx+4
	light := s.benchPlaceBuilding(assets.LightPlt, player, bx+5, row, true) // cols bx+5, bx+6
	s.benchPlaceBuilding(assets.Depot, player, bx+7, row, true)             // cols bx+7, bx+8

	baseCX, baseCY := bx+4, row

	shipyard := s.benchFloatSeaBuilding(assets.Shipyard, player, baseCX, baseCY)
	shipPlaced := shipyard != nil

	// Fund + power the whole base so every factory / storage building can draw the pool.
	for _, cx := range s.game.Complexes {
		if cx.Team == player {
			cx.Material = 1000
			cx.Fuel = 300
		}
	}

	s.benchStorageUnits(player, bx, by, row, shipyard)

	// Queue one unit on each land factory and build it to completion, so a Tank and a
	// Scout sit parked build ready on the Heavy/Light plants (their BLDMRK exit
	// markers show when the plant is selected).
	s.game.SetFactoryQueue(heavy, []assets.UnitType{assets.Tank}, 1, false)
	s.game.SetFactoryQueue(light, []assets.UnitType{assets.Scout}, 1, false)

	for turn := 0; turn < 16 && (s.game.BuildOrderFor(heavy) != nil || s.game.BuildOrderFor(light) != nil); turn++ {
		s.game.AdvanceBuilds(player)
	}

	s.selected = heavy
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.fogShow = false
	s.r.SetZoom(factoryRunZoom)
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(bx+5, row+1, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)

	dbg.Printf("factoryrun: base (%d,%d) shipyard-at-water=%v units=%d", bx, by, shipPlaced, len(s.game.Units.Units()))
}

// benchPlaceBuilding places a powered, slabbed building (2x2 when big, else 1x1).
func (s *Screen) benchPlaceBuilding(t assets.UnitType, player, gx, gy int, big bool) *game.UnitInfo {
	flags := game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable

	if big {
		flags |= game.FlagBuilding
	}

	u := game.NewUnitInfo(t, player, s.spawnSeq+1, 0)
	u.Flags = flags
	u.Values = s.teamValues(player, t)
	u.InitState()

	u.Order = game.OrderPowerOn
	u.State = game.OrderStateExecutingOrder
	s.spawnSeq++
	s.game.PlaceBuilding(u, gx, gy)

	return u
}

// benchPlaceConnector places a 1x1 connector ("collector") - a flat special (no slab)
// that merges the structure it bridges into the base complex.
func (s *Screen) benchPlaceConnector(player, gx, gy int) {
	c := game.NewUnitInfo(assets.Cnct4W, player, s.spawnSeq+1, 0)
	c.Flags = game.FlagStationary | game.FlagSelectable
	c.Values = s.teamValues(player, assets.Cnct4W)
	c.InitState()

	c.Order = game.OrderPowerOn
	c.State = game.OrderStateExecutingOrder
	s.spawnSeq++
	s.game.PlaceBuilding(c, gx, gy)
}

// benchWaterAt reports whether the cell is water.
func (s *Screen) benchWaterAt(x, y int) bool {
	return s.wmap.SurfaceAt(x, y)&world.SurfaceWater != 0
}

// benchClearBlock reports whether the 2x2 block at (x,y) holds no unit.
func (s *Screen) benchClearBlock(x, y int) bool {
	return len(s.game.Units.Cells.At(x, y)) == 0 && len(s.game.Units.Cells.At(x+1, y)) == 0 && len(s.game.Units.Cells.At(x, y+1)) == 0 && len(s.game.Units.Cells.At(x+1, y+1)) == 0
}

// benchFloatSeaBuilding sites a sea building on the nearest FREE all water 2x2 to
// (fromX,fromY), floats it (no slab) and runs the connector chain. Nil if no water.
// Sea buildings (Shipyard FACTORY + Dock STORAGE) float on water and join the base
// by a connector ("collector") line. NOTE the manual pairs them: Air Units Plant <->
// Shipyard are the FACTORIES, Hangar <-> Dock are the STORAGE - a Shipyard is NOT a
// Dock.
func (s *Screen) benchFloatSeaBuilding(t assets.UnitType, player, fromX, fromY int) *game.UnitInfo {
	water2x2 := func(x, y int) bool {
		return s.benchWaterAt(x, y) && s.benchWaterAt(x+1, y) && s.benchWaterAt(x, y+1) && s.benchWaterAt(x+1, y+1)
	}

	abs := func(n int) int {
		if n < 0 {
			return -n
		}

		return n
	}

	sign := func(n int) int {
		switch {
		case n < 0:
			return -1

		case n > 0:
			return 1

		default:
			return 0
		}
	}

	sx, sy, bestD := -1, -1, 1<<30

	for y := 1; y+1 < s.wmap.H; y++ {
		for x := 1; x+1 < s.wmap.W; x++ {
			if water2x2(x, y) && s.benchClearBlock(x, y) {
				if d := abs(x-fromX) + abs(y-fromY); d < bestD {
					bestD, sx, sy = d, x, y
				}
			}
		}
	}

	if sx < 0 || bestD > 80 {
		return nil
	}

	b := game.NewUnitInfo(t, player, s.spawnSeq+1, 0)
	b.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable // sea building: floats, no slab
	b.Values = s.teamValues(player, t)
	b.InitState()
	b.Order = game.OrderPowerOn
	b.State = game.OrderStateExecutingOrder
	s.spawnSeq++
	s.game.PlaceBuilding(b, sx, sy)

	// Greedy connector chain from (fromX,fromY) out to the building footprint.
	ccx, ccy := fromX, fromY

	for steps := 0; steps < 80; steps++ {
		dx, dy := sx-ccx, sy-ccy

		if abs(dx) <= 1 && abs(dy) <= 1 {
			break
		}

		if abs(dx) >= abs(dy) {
			ccx += sign(dx)
		} else {
			ccy += sign(dy)
		}

		if ccx >= sx && ccx < sx+2 && ccy >= sy && ccy < sy+2 {
			break
		}

		if len(s.game.Units.Cells.At(ccx, ccy)) > 0 {
			continue // skip a building already on this cell
		}

		s.benchPlaceConnector(player, ccx, ccy)
	}

	return b
}

// benchStorageUnits gives the bench one unit per storage class, damaged to half HP,
// to repair / store / unload, plus the Hangar and Dock that store them.
func (s *Screen) benchStorageUnits(player, bx, by, row int, shipyard *game.UnitInfo) {
	damage := func(u *game.UnitInfo) {
		if u != nil && u.Values != nil {
			if half := u.Values.Get(game.AttribHits) / 2; half > 0 {
				u.Hits = half
			}
		}
	}

	// LAND: a Scout parked by the Depot.
	damage(s.deployUnit(assets.Scout, game.FlagMobileLandUnit|game.FlagSelectable, player, 0, bx+7, row-1))

	// AIR: a Hangar (air unit storage) on the cleared land below the base, with a
	// damaged Fighter beside it to store / repair / unload.
	hangarX, hangarY := -1, -1

	for x := bx; x+1 <= bx+16 && hangarX < 0; x += 2 {
		if s.benchClearBlock(x, by+8) {
			hangarX, hangarY = x, by+8
		}
	}

	if hangarX >= 0 {
		s.benchPlaceBuilding(assets.Hangar, player, hangarX, hangarY, true)
		damage(s.deployUnit(assets.Fighter, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, player, 0, hangarX+2, hangarY))
	}

	// SEA: a Dock (sea unit storage; the water counterpart of the Hangar, NOT the
	// Shipyard factory) floated near the shipyard, with a damaged Escort beside it.
	var dock *game.UnitInfo

	if shipyard != nil {
		dock = s.benchFloatSeaBuilding(assets.Dock, player, shipyard.GridX, shipyard.GridY)
	}

	if dock != nil {
		dx, dy := dock.GridX, dock.GridY

		for _, c := range [][2]int{{dx - 1, dy}, {dx + 2, dy}, {dx - 1, dy + 1}, {dx + 2, dy + 1}, {dx, dy - 1}, {dx, dy + 2}} {
			if s.benchWaterAt(c[0], c[1]) && len(s.game.Units.Cells.At(c[0], c[1])) == 0 {
				damage(s.deployUnit(assets.FastBoat, game.FlagMobileSeaUnit|game.FlagSelectable, player, 0, c[0], c[1]))

				break
			}
		}
	}
}
