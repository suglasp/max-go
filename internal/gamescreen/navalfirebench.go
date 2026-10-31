// navalfirebench.go : provides the naval line of fire bench.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/combat"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// navalFireBenchZoom frames the three lanes together.
const navalFireBenchZoom = 32

// navalFireBenchRange covers the longest lane (7 cells) with a cell to spare.
const navalFireBenchRange = 8

// navalLane is a straight row or column from a player ship (From) to an enemy ship (To).
type navalLane struct {
	From, To game.Point
}

// cells lists every cell of the lane, both ends included.
func (l navalLane) cells() []game.Point {
	stepX, stepY := sign(l.To.X-l.From.X), sign(l.To.Y-l.From.Y)
	var out []game.Point
	for p := l.From; ; p = (game.Point{X: p.X + stepX, Y: p.Y + stepY}) {
		out = append(out, p)
		if p == l.To {
			return out
		}
	}
}

// middle is the cell halfway along the lane.
func (l navalLane) middle() game.Point {
	return game.Point{X: (l.From.X + l.To.X) / 2, Y: (l.From.Y + l.To.Y) / 2}
}

// DeployNavalFireBench lays out the -emulate navalfire bench for the klei 23/08/26 naval line of fire
// (step 1440). Three lanes, each a player ship facing a static enemy Corvette (AI off) in range:
//   - BLOCKED: a player Corvette with land between it and the enemy, so no attack cursor and no shot;
//   - CLEAR: a player Submarine on open water, so it fires;
//   - FRIENDLY: a player Corvette with its own Sea Transport between, so it still fires (units never block).
//
// C++: n/a (debug helper).
func (s *Screen) DeployNavalFireBench() {
	const player, enemy = 0, 1
	seaFlags := game.FlagMobileSeaUnit | game.FlagSelectable

	originX, originY, ok := s.snowBlock(8, 8)
	if !ok {
		originX, originY = s.wmap.W/2, s.wmap.H/2
	}
	center := game.Point{X: originX + 4, Y: originY + 4}

	taken := map[game.Point]bool{}
	blocked, haveBlocked := s.findNavalLane(center, taken, true)
	if haveBlocked {
		center = blocked.From // keep the other lanes close to the island
	}
	clear, haveClear := s.findNavalLane(center, taken, false)
	friendly, haveFriendly := s.findNavalLane(center, taken, false)

	deployPair := func(lane navalLane, shooter assets.UnitType) *game.UnitInfo {
		u := s.deployUnit(shooter, seaFlags, player, 0, lane.From.X, lane.From.Y)
		u.Values.Set(game.AttribRange, navalFireBenchRange)
		u.Values.Set(game.AttribScan, navalFireBenchRange+2) // the enemy must be in sight to be targeted
		s.deployUnit(assets.Corvette, seaFlags, enemy, 4, lane.To.X, lane.To.Y)
		return u
	}

	var first *game.UnitInfo
	report := func(name string, lane navalLane, u *game.UnitInfo) {
		fires := combat.WithinAttackRange(u, lane.To.X, lane.To.Y, navalFireBenchRange, s.game.Grid.Surf)
		dbg.Printf("navalfire bench: %s lane (%d,%d) -> (%d,%d), line of fire clear=%v", name,
			lane.From.X, lane.From.Y, lane.To.X, lane.To.Y, fires)
		if first == nil {
			first = u
		}
	}
	if haveBlocked {
		report("BLOCKED (want false)", blocked, deployPair(blocked, assets.Corvette))
	} else {
		dbg.Printf("navalfire bench: no water lane across land found near (%d,%d)", center.X, center.Y)
	}
	if haveClear {
		report("CLEAR (want true)", clear, deployPair(clear, assets.Submarne))
	}
	if haveFriendly {
		u := deployPair(friendly, assets.Corvette)
		mid := friendly.middle()
		s.deployUnit(assets.SeaTrans, seaFlags, player, 0, mid.X, mid.Y)
		report("FRIENDLY between (want true)", friendly, u)
	}

	s.game.State.Teams[enemy].Type = game.TeamPlayer // LOCK the AI: enemies are static targets
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.selected = first
	s.showNames = true
	s.fogShow = false
	s.game.RefreshVisibility()

	s.r.SetZoom(navalFireBenchZoom)
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(center.X, center.Y, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
	s.showMessage("Naval fire bench (AI off): the Corvette with land between it and the enemy must NOT get an attack cursor or fire. The Submarine on open water fires. The Corvette behind its own Sea Transport fires too: units never block, only land does.")
}

// findNavalLane returns the nearest free row or column lane of 3 to 7 cells between two water ends (a
// shore is buffered by coast cells, so a short lane across land needs ends next to the coast). With
// acrossLand at least one cell between the ends is land; without, every cell of the lane is water.
// The lane and a one cell margin are marked taken, so the next lane keeps clear of it.
func (s *Screen) findNavalLane(center game.Point, taken map[game.Point]bool, acrossLand bool) (navalLane, bool) {
	var found navalLane
	free := func(p game.Point) bool { return !taken[p] && s.wmap.SurfaceAt(p.X, p.Y) == world.SurfaceWater }
	fits := func(lane navalLane) bool {
		cells := lane.cells()
		land := false
		for i, p := range cells {
			if taken[p] {
				return false
			}
			if i == 0 || i == len(cells)-1 {
				continue // the ends are checked by free
			}
			switch {
			case s.wmap.SurfaceAt(p.X, p.Y) == world.SurfaceLand:
				land = true
			case !acrossLand && s.wmap.SurfaceAt(p.X, p.Y) != world.SurfaceWater:
				return false
			}
		}
		return land == acrossLand
	}

	_, _, ok := s.nearestCell(center, 60, func(x, y int) bool {
		from := game.Point{X: x, Y: y}
		if !free(from) {
			return false
		}
		for length := 3; length <= 7; length++ {
			for _, dir := range []game.Point{{X: 1}, {Y: 1}} {
				lane := navalLane{From: from, To: game.Point{X: x + dir.X*length, Y: y + dir.Y*length}}
				if free(lane.To) && fits(lane) {
					found = lane
					return true
				}
			}
		}
		return false
	})
	if !ok {
		return navalLane{}, false
	}

	for _, p := range found.cells() {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				taken[game.Point{X: p.X + dx, Y: p.Y + dy}] = true
			}
		}
	}
	return found, true
}

// sign returns -1, 0 or 1 for the sign of n.
func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
