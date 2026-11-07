// placement.go : lands a team's starting base and finds the ground a building may occupy.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : game_manager.cpp - GameManager_DeployUnit placement rules
package sim

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// PlaceBuilding deploys a finished building, joins it into its team's complex
// network (forming/merging by adjacency), refreshes the complex list + visibility,
// and returns the complex it joined. Mining stations get initial extraction from
// the resource map.
// C++: AttachToPrimaryComplex + UnitsManager_SetInitialMining (units_manager.cpp).
func (g *Game) PlaceBuilding(u *game.UnitInfo, gx, gy int) *economy.Complex {
	g.removeConnectorsUnder(u, gx, gy) // a building consumes any connector it is placed on
	g.Units.Deploy(u, gx, gy)
	g.recordBuildingBuilt(u) // score screen tally (factories / mines / buildings built)
	g.laySlab(u)
	if u.Type == assets.MiningSt && g.Resources != nil {
		g.Resources.SetInitialMining(u)
	}
	// The neutral alien faction has no economy, so an alien owned building deploys (and lays
	// its slab) but never joins a complex network - it just sits there, inert and salvageable.
	if game.IsAlien(u.Team) {
		g.RefreshVisibility()
		g.invalidateReach()
		return nil
	}
	c := g.network(u.Team).Add(u)
	g.syncComplexes()
	g.RefreshVisibility()
	g.invalidateReach() // a new building/bridge changes terrain connectivity
	return c
}

// removeConnectorsUnder deletes the team's own connectors (Cnct4W) under a new
// building's footprint, so the building takes their place instead of leaving a stale
// connector drawing over it. The building bridges the same complex gap (network.Add
// re merges by adjacency), so the connector is simply consumed.
// C++: a building deployed on a CONNECTOR_UNIT cell replaces it.
func (g *Game) removeConnectorsUnder(u *game.UnitInfo, gx, gy int) {
	if g.Units == nil || g.Units.Cells == nil {
		return
	}
	fp := build.Footprint(u.Flags)
	var doomed []*game.UnitInfo
	for dy := 0; dy < fp; dy++ {
		for dx := 0; dx < fp; dx++ {
			for _, o := range g.Units.Cells.At(gx+dx, gy+dy) {
				// Only a player team's own connector (never the alien slot: g.Networks is
				// sized [TeamCount] = 4, so the fifth team's index is out of range).
				if o.Type == assets.Cnct4W && o.Team == u.Team && !game.IsAlien(o.Team) {
					doomed = append(doomed, o)
				}
			}
		}
	}
	for _, c := range doomed {
		g.network(c.Team).Remove(c)
		g.Units.Remove(c)
	}
}

// laySlab deploys the foundation slab a building stands on: a slab requiring unit
// over land lays a large slab (building) or small slab (else) as ground cover.
// C++: REQUIRES_SLAB && Access_IsAnyLandPresent deploy in GameManager_DeployUnit.
func (g *Game) laySlab(u *game.UnitInfo) {
	if !u.Flags.Has(game.FlagRequiresSlab) {
		return
	}
	isBuilding := u.Flags.Has(game.FlagBuilding)
	if !g.Grid.IsAnyLandPresent(u.GridX, u.GridY, isBuilding) {
		return // over water a platform is used instead (deferred)
	}
	slabType := assets.SmlSlab
	slabFlags := game.FlagGroundCover
	if isBuilding {
		slabType = assets.LrgSlab
		// The large slab is a 2x2 foundation; FlagBuilding gives it the
		// footprint centre sprite anchor matching the building above it.
		slabFlags |= game.FlagBuilding
	}
	g.nextBuilt++
	slab := game.NewUnitInfo(slabType, u.Team, 10000+g.nextBuilt, 0)
	slab.Flags = slabFlags
	g.Units.Deploy(slab, u.GridX, u.GridY)
}

/* // note: unused code in Go port
// Land is only read by tests; the live landing is gamescreen deployStartBase (tested in startbase_land_test.go).
// Land deploys a team's starting colony at mining site (gx, gy): places the mine
// (first complex + large slab + initial mining), drops gold into it, adds powerGen
// on an adjacent site, and scatters the loadout onto nearby reachable cells. mine
// and powerGen are caller constructed; loadout units carry their own Storage.
// Returns the starting complex. C++: GameManager_ProcessTeamMissionSupplyUnits
// (mine + LRGSLAB + POWGEN + SMLSLAB + Access_FindReachableSpot loadout).
func (g *Game) Land(team, gx, gy, gold int, mine, powerGen *game.UnitInfo, loadout []*game.UnitInfo) *economy.Complex {
	mine.Team = team
	mine.Order = game.OrderPowerOn
	mine.State = game.OrderStateExecutingOrder
	c := g.PlaceBuilding(mine, gx, gy)
	c.Gold += gold
	g.Gold[team] += gold

	if powerGen != nil {
		px, py, ok := g.findBuildingSite(powerGen, team, gx, gy)
		if ok {
			powerGen.Team = team
			powerGen.Order = game.OrderPowerOn
			powerGen.State = game.OrderStateExecutingOrder
			g.PlaceBuilding(powerGen, px, py)
		}
	}

	for _, u := range loadout {
		u.Team = team
		if ux, uy, ok := g.findReachableSpot(u, gx, gy, build.Footprint(mine.Flags)); ok {
			g.Units.Deploy(u, ux, uy)
			g.recordUnitBuilt(team) // score screen tally (landed roster counts as units built)
		}
	}
	g.RefreshVisibility()
	return c
}
*/

/* // note: unused code in Go port
// findBuildingSite was only called by Land; the live landing places buildings with adjacentSite.
// findBuildingSite finds a valid placement for a building next to the origin
// footprint at (gx, gy). It first tries the four sites flush against each side
// (sliding along that side), which abut the origin so the new building joins
// its complex; failing that it falls back to a widening ring search.
func (g *Game) findBuildingSite(b *game.UnitInfo, team, gx, gy int) (int, int, bool) {
	d := access.UnitDesc{Type: b.Type, Flags: b.Flags, LandType: access.LandTypeForUnit(b.Type, b.Flags)}
	n := build.Footprint(b.Flags) // origin is a building too (same size)

	// Flush against a side, then offset by one along it (still adjacent).
	var flush [][2]int
	for _, off := range []int{0, n - 1, 1 - n} {
		flush = append(flush,
			[2]int{gx + n, gy + off}, // east
			[2]int{gx - n, gy + off}, // west
			[2]int{gx + off, gy + n}, // south
			[2]int{gx + off, gy - n}, // north
		)
	}
	for _, c := range flush {
		if build.SiteValid(g.Grid, d, team, c[0], c[1]) {
			return c[0], c[1], true
		}
	}

	for r := 1; r <= 8; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				x, y := gx+dx, gy+dy
				if build.SiteValid(g.Grid, d, team, x, y) {
					return x, y, true
				}
			}
		}
	}
	return 0, 0, false
}
*/

// findReachableSpot searches outward from (gx, gy) for the nearest free,
// accessible single cell for a mobile unit.
//
// C++: Access_FindReachableSpot (access.cpp), reduced to a ring search.
func (g *Game) findReachableSpot(u *game.UnitInfo, gx, gy, fp int) (int, int, bool) {
	d := access.UnitDesc{Type: u.Type, Flags: u.Flags, LandType: access.LandTypeForUnit(u.Type, u.Flags)}
	blocked := FootprintCells(gx, gy, fp) // the finished building the exiting unit leaves behind
	const exteriorReach = 16              // an exit to open ground within this radius is plenty; keep the scan bounded
	fbX, fbY, haveFb := 0, 0, false
	for r := 1; r <= g.W+g.H; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx > -r && dx < r && dy > -r && dy < r {
					continue // only the ring at radius r
				}
				x, y := gx+dx, gy+dy
				if x < 0 || y < 0 || x >= g.W || y >= g.H {
					continue
				}
				if len(g.Units.Cells.At(x, y)) != 0 ||
					!g.Grid.IsAccessible(d, u.Team, x, y, access.SameClassBlocks).Passable() {
					continue
				}
				// An empty passable cell. Prefer one that leads to the OUTSIDE (not a
				// pocket the base has sealed), so the builder never exits into a trap.
				if r <= exteriorReach && g.connectsToOpen(u, x, y, blocked) {
					return x, y, true
				}
				if !haveFb {
					fbX, fbY, haveFb = x, y, true // nearest empty cell, the last resort
				}
			}
		}
		if r >= exteriorReach && haveFb {
			return fbX, fbY, true // no exterior exit within reach: best effort (the AI rescue frees a pocket)
		}
	}
	if haveFb {
		return fbX, fbY, true
	}
	return 0, 0, false
}

// connectsToOpen reports whether a unit standing at (sx, sy) can drive to open ground -
// a flood over cells it can enter (treating blocked, a finished footprint, as walls)
// that reaches at least openThreshold cells. A pocket sealed by buildings floods to only
// a handful; open ground reaches the cap at once. It tells an exterior exit from a trap.
func (g *Game) connectsToOpen(u *game.UnitInfo, sx, sy int, blocked map[[2]int]bool) bool {
	const openThreshold = 12
	d := access.UnitDesc{Type: u.Type, Flags: u.Flags, LandType: access.LandTypeForUnit(u.Type, u.Flags)}
	drivable := func(x, y int) bool {
		if x < 0 || y < 0 || x >= g.W || y >= g.H {
			return false
		}
		if blocked[[2]int{x, y}] {
			return false
		}
		return g.Grid.IsAccessible(d, u.Team, x, y, access.SameClassBlocks).Passable()
	}
	if !drivable(sx, sy) {
		return false
	}
	seen := map[[2]int]bool{{sx, sy}: true}
	queue := [][2]int{{sx, sy}}
	count := 0
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		count++
		if count >= openThreshold {
			return true
		}
		for _, step := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := [2]int{c[0] + step[0], c[1] + step[1]}
			if seen[n] || !drivable(n[0], n[1]) {
				continue
			}
			seen[n] = true
			queue = append(queue, n)
		}
	}
	return count >= openThreshold
}

// FootprintCells returns the cells a fp x fp structure at (x, y) occupies, as a set.
func FootprintCells(x, y, fp int) map[[2]int]bool {
	cells := make(map[[2]int]bool, fp*fp)
	for cy := y; cy < y+fp; cy++ {
		for cx := x; cx < x+fp; cx++ {
			cells[[2]int{cx, cy}] = true
		}
	}
	return cells
}
