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

// navalFireBenchZoom frames the lanes together.
const navalFireBenchZoom = 32

// navalFireBenchRange covers the longest lane (7 cells, or a 5 cell diagonal) with room to spare.
const navalFireBenchRange = 8

// navalFireBenchTurnSeconds is the bench's turn timer: the "about to run out of time" line comes at 0:20 (H3).
const navalFireBenchTurnSeconds = 40

// navalLane is a straight row, column or diagonal from a player ship (From) to an enemy ship (To).
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

// navalLaneKind is what lies between the two ships of a lane.
type navalLaneKind int

const (
	laneLand     navalLaneKind = iota // at least one land cell between: no line of fire
	laneOpen                          // only water: fires
	laneCoast                         // coast between, no land: fires (coast never blocks)
	laneDiagonal                      // a diagonal whose cells only touch land at a corner: fires
)

// navalFireScenario is one lane of the bench: what lies between the ships, the shooter, and the banner.
type navalFireScenario struct {
	kind    navalLaneKind
	shooter assets.UnitType
	title   string
	text    string
	extra   func(lane navalLane) // the friendly ship or bridge on the middle cell, or nil
}

// DeployNavalFireBench lays out the -emulate navalfire bench for the naval line of fire
// (step 1440, hand check H9) and the turn timer warning (H3). Six lanes, each a player ship facing a still
// enemy Corvette (AI off) in range, each near its own compass anchor so only one is on screen at a time
// (Tab moves on):
//   - BLOCKED: land between, so no attack cursor and no shot;
//   - COAST: coast cells between, so it fires;
//   - DIAGONAL: land only touching the line at a corner, so it fires;
//   - CLEAR: a Submarine on open water, so it fires;
//   - FRIENDLY: its own Sea Transport between, so it still fires (units never block);
//   - BRIDGE: a Bridge laid between, so a Submarine still fires under it.
//
// The scarcest lane kinds claim their anchor first. The turn timer runs at 40 s, so the "about to run out of
// time" line comes once at 0:20.
//
// C++: n/a (debug helper).
func (s *Screen) DeployNavalFireBench() {
	const player, enemy = 0, 1
	seaFlags := game.FlagMobileSeaUnit | game.FlagSelectable

	scenarios := []navalFireScenario{
		{kind: laneLand, shooter: assets.Corvette, title: "H9 BLOCKED",
			text: "land between: select the Corvette, the enemy gets the yellow cursor (out of reach), no shot"},
		{kind: laneCoast, shooter: assets.Corvette, title: "H9 COAST",
			text: "coast between: red attack cursor, it fires"},
		{kind: laneDiagonal, shooter: assets.Corvette, title: "H9 DIAGONAL",
			text: "land only touches the line at a corner: red attack cursor, it fires"},
		{kind: laneOpen, shooter: assets.Submarne, title: "H9 CLEAR",
			text: "open water: the Submarine gets the red attack cursor and fires"},
		{kind: laneOpen, shooter: assets.Corvette, title: "H9 FRIENDLY",
			text: "your own Sea Transport between: red attack cursor, it still fires",
			extra: func(lane navalLane) {
				mid := lane.middle()
				s.deployUnit(assets.SeaTrans, seaFlags, player, 0, mid.X, mid.Y)
			}},
		{kind: laneOpen, shooter: assets.Submarne, title: "H9 BRIDGE",
			text: "a bridge between: the Submarine gets the red attack cursor and fires",
			extra: func(lane navalLane) {
				mid := lane.middle()
				s.deployProp(player, assets.Bridge, game.FlagStationary, mid.X, mid.Y)
			}},
	}

	taken := map[game.Point]bool{}

	for _, sc := range scenarios {
		var lane navalLane

		compass, ok := s.claimAnchor(func(anchor game.Point) bool {
			found, fits := s.findNavalLane(anchor, taken, sc.kind, benchAnchorRadius)
			lane = found
			return fits
		})
		if !ok {
			dbg.Printf("navalfire bench: no free compass anchor has a %s lane", sc.title)
			continue
		}

		u := s.deployUnit(sc.shooter, seaFlags, player, 0, lane.From.X, lane.From.Y)
		u.Values.Set(game.AttribRange, navalFireBenchRange)
		u.Values.Set(game.AttribScan, navalFireBenchRange+2) // the enemy must be in sight to be targeted
		s.deployUnit(assets.Corvette, seaFlags, enemy, 4, lane.To.X, lane.To.Y)

		if sc.extra != nil {
			sc.extra(lane)
		}

		fires := combat.WithinAttackRange(u, lane.To.X, lane.To.Y, navalFireBenchRange, s.game.Grid.Surf)
		dbg.Printf("navalfire bench: %s %s lane (%d,%d) -> (%d,%d), line of fire clear=%v", compass, sc.title,
			lane.From.X, lane.From.Y, lane.To.X, lane.To.Y, fires)

		s.addBenchStop(benchStop{Compass: compass, Title: sc.title, Text: sc.text, Cell: lane.middle(), Select: u})
	}

	s.freezeBenchTeam(enemy) // LOCK the AI: enemies are static targets
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.showNames = true
	s.fogShow = false
	s.game.RefreshVisibility()
	s.SetTurnTimer(navalFireBenchTurnSeconds) // H3: the time warning line plays once at 0:20

	s.r.SetZoom(navalFireBenchZoom)
	s.startBench("Naval fire bench (AI off). H3: 40 s turns, the time warning plays once at 0:20, not again after a menu; End Turn re arms it. H4: two voices never cut each other off.")
}

// findNavalLane returns the free lane of the given kind nearest to center, starting at most radius cells from
// it, between two water ends: a row or column of 3 to 7 cells, or for laneDiagonal a diagonal of 3 to 5 cells
// (a shore is buffered by coast cells, so a short lane across land needs ends next to the coast). The lane and
// a one cell margin are marked taken, so the next lane keeps clear of it.
func (s *Screen) findNavalLane(center game.Point, taken map[game.Point]bool, kind navalLaneKind, radius int) (navalLane, bool) {
	var found navalLane
	free := func(p game.Point) bool { return !taken[p] && s.wmap.SurfaceAt(p.X, p.Y) == world.SurfaceWater }
	isLand := func(p game.Point) bool { return s.wmap.SurfaceAt(p.X, p.Y) == world.SurfaceLand }
	fits := func(lane navalLane) bool {
		cells := lane.cells()
		land, coast, corner := false, false, false
		for i, p := range cells {
			if taken[p] {
				return false
			}
			if i == 0 || i == len(cells)-1 {
				continue // the ends are checked by free
			}
			switch s.wmap.SurfaceAt(p.X, p.Y) {
			case world.SurfaceLand:
				land = true
			case world.SurfaceCoast:
				coast = true
			}
			for _, d := range []game.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
				if isLand(game.Point{X: p.X + d.X, Y: p.Y + d.Y}) {
					corner = true
				}
			}
		}
		switch kind {
		case laneLand:
			return land
		case laneCoast:
			return coast && !land
		case laneDiagonal:
			return corner && !land
		default:
			return !land && !coast
		}
	}

	dirs := []game.Point{{X: 1}, {Y: 1}}
	maxLength := 7
	if kind == laneDiagonal {
		dirs = []game.Point{{X: 1, Y: 1}, {X: 1, Y: -1}}
		maxLength = 5 // a 5 cell diagonal is about 7.1 cells away, inside the bench range of 8
	}

	_, _, ok := s.nearestCell(center, radius, func(x, y int) bool {
		from := game.Point{X: x, Y: y}
		if !free(from) {
			return false
		}
		for length := 3; length <= maxLength; length++ {
			for _, dir := range dirs {
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
