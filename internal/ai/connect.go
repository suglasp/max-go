// connect.go : the M.A.X. connection logic for the AI.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// reconnectBuildings joins the team's DETACHED building complexes back into the
// main base by laying a line of 4 way connectors, one cell per turn. A base
// splits when the manager has to place a structure away from the complex - most
// often a mining station out on a distant deposit (findMiningSite does not
// require connection), which then forms its own pool with no nearby power. The
// player repairs this by dragging a connector line; this is the AI's equivalent.
//
// The chain grows FROM the stocked primary complex OUTWARD toward the island (the
// way the player drags a connector line out of the base, and the way the original
// seeds the connection search from the largest mining complex). That direction matters:
// each new connector borders the primary, so its build draws the primary's pooled
// material (buildPool's complexAdjacentTo) and completes even with an empty cargo
// engineer - whereas a connector laid bordering the EMPTY detached island would
// have no material to draw and stall at 0 forever, deadlocking the assistant
// (handling(Cnct4W) stays true). A finished connector is a flat special that joins
// the primary at once, extending it one cell toward the island each turn until a
// primary connector borders the island and Network.Add merges them - and the
// island's stalled builds can then draw the now shared pool.
//
// build.Network maintains connected components incrementally as adjacent buildings are added. The assistant reads that live complex list, assigns
// one connector destination to an idle engineer, and the following build-manager pass drives the engineer there and raises the connector.
//
// C++ ported : taskmanagebuildings.cpp - TaskManageBuildings::ReconnectBuildings /
// ConnectBuilding / GetConnectionDistance (reduced as above: live build.Network
// complex list instead of the AccessMap flood fill; greedy one connector per turn
// cardinal stepping instead of queueing the whole line at once).
func (t *taskManageBuildings) reconnectBuildings() {
	g := t.ctx.Game
	if g == nil {
		return
	}

	n := g.Networks[t.team]
	if n == nil || len(n.Complexes) < 2 {
		return // one base (or none): nothing to connect
	}

	if t.handling(assets.Cnct4W) {
		return // a connector effort is already pending or under construction (one at a time)
	}

	primary := largestComplex(n.Complexes)
	if primary == nil {
		return
	}

	eng := t.idleEngineer()
	if eng == nil {
		return // no free engineer this turn; try again next turn
	}

	cx, cy, ok := t.nextConnectorSite(n, primary, eng)
	if !ok {
		return // no buildable cell steps a detached complex toward the base
	}

	// Seed the drive intent only. taskManageBuildings.BeginTurn calls advancePending
	// for this engineer, which drives it onto the cell and placeBuilds the connector
	// on arrival. Seeding (rather than buildOrDrive here) avoids a duplicate move
	// being issued in the same turn.
	t.pending[eng] = pendingBuildSite{unitType: assets.Cnct4W, large: false, x: cx, y: cy}
}

// largestComplex picks the team's primary complex: the one with the most
// buildings, breaking ties by lowest id (the same bias build.Network uses when it
// merges, so the chain always grows toward the dominant base). C++:
// TaskManageBuildings::MarkBuildings seeds from the largest MININGST complex.
func largestComplex(cs []*economy.Complex) *economy.Complex {
	var best *economy.Complex

	for _, c := range cs {
		if best == nil || c.Buildings > best.Buildings ||
			(c.Buildings == best.Buildings && c.ID < best.ID) {
			best = c
		}
	}

	return best
}

// idleEngineer returns this team's idle Engineer (the 1x1 builder that raises
// connectors), chosen deterministically, or nil if none is free this turn. It is
// kept distinct from the constructor so the connection assistant never steals the
// 2x2 builder that is growing the base.
func (t *taskManageBuildings) idleEngineer() *game.UnitInfo {
	g := t.ctx.Game
	var best *game.UnitInfo

	for _, u := range g.Units.Units() {
		if u.Team != t.team || u.Type != assets.Engineer || !t.isIdleBuilder(u) {
			continue
		}
		if _, headed := t.pending[u]; headed {
			continue // already committed to a site
		}
		if best == nil || unitBefore(u, best) {
			best = u
		}
	}

	return best
}

// nextConnectorSite chooses the next connector cell: the empty, buildable cell
// bordering the PRIMARY (stocked) complex that steps closest to a detached island.
// Because the cell borders the primary, a connector raised there draws the
// primary's pooled material (buildPool's complexAdjacentTo) - so it is funded even
// when the engineer is empty - and merges INTO the primary (Network.Add),
// extending it one cell toward the island; repeating shrinks the gap by one each
// turn until a primary connector borders the island and they merge. Two distinct
// complexes are never closer than two cells apart (one cell would already be
// edge adjacent, hence one complex), so the final step is always a single bridging
// connector. It targets the nearest island over all detached complexes, so the
// closest detached base is hooked up first.
//
// C++ ported : taskmanagebuildings.cpp - TaskManageBuildings::ConnectBuilding (the
// cardinal step toward the nearest connected square), reduced to one cell per turn.
func (t *taskManageBuildings) nextConnectorSite(n *build.Network, primary *economy.Complex, eng *game.UnitInfo) (int, int, bool) {
	g := t.ctx.Game
	primaryCells := complexFootprintCells(primary)

	if len(primaryCells) == 0 {
		return 0, 0, false
	}

	// The cells of every DETACHED complex (the targets the chain grows toward).
	var islandCells [][2]int
	for _, c := range n.Complexes {
		if c == primary {
			continue
		}
		islandCells = append(islandCells, complexFootprintCells(c)...)
	}

	if len(islandCells) == 0 {
		return 0, 0, false // nothing detached to reach
	}

	flags := buildFlagsFor(assets.Cnct4W, false) // a 1x1 connector: a terrain special, lays no slab
	d := access.UnitDesc{Type: assets.Cnct4W, Flags: flags, LandType: access.LandTypeForUnit(assets.Cnct4W, flags)}
	g.Units.Cells.Remove(eng) // ignore the builder so it does not block a site it stands on
	defer g.Units.Cells.Add(eng)

	gap := minPairManhattan(primaryCells, islandCells) // current nearest primary->island gap
	bestX, bestY, bestDist, found := 0, 0, 0, false

	// Step the primary's border one cell toward the nearest island.
	for _, cand := range borderCells(primaryCells, g.W, g.H) {
		cx, cy := cand[0], cand[1]
		dist := minManhattanTo(cx, cy, islandCells)
		if dist >= gap {
			continue // not a step toward the island
		}
		if !build.SiteValid(g.Grid, d, t.team, cx, cy) {
			continue
		}
		if !found || dist < bestDist || (dist == bestDist && tieBefore(cx, cy, bestX, bestY)) {
			bestX, bestY, bestDist, found = cx, cy, dist, true
		}
	}

	return bestX, bestY, found
}

// complexFootprintCells returns every cell occupied by a complex's member
// buildings (each member expanded over its footprint).
func complexFootprintCells(c *economy.Complex) [][2]int {
	var out [][2]int

	for _, m := range c.Members {
		fp := build.Footprint(m.Flags)
		for y := m.GridY; y < m.GridY+fp; y++ {
			for x := m.GridX; x < m.GridX+fp; x++ {
				out = append(out, [2]int{x, y})
			}
		}
	}

	return out
}

// borderCells returns the distinct in bounds cells edge adjacent to the cell set
// but not in it - the candidate sites for the next connector stepping outward. The
// result follows the input order (then first seen), so selection downstream is
// deterministic despite the random unit iteration order elsewhere.
func borderCells(cells [][2]int, w, h int) [][2]int {
	in := make(map[[2]int]bool, len(cells))

	for _, c := range cells {
		in[c] = true
	}

	seen := map[[2]int]bool{}
	var out [][2]int
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	for _, c := range cells {
		for _, dxy := range dirs {
			nx, ny := c[0]+dxy[0], c[1]+dxy[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}
			k := [2]int{nx, ny}
			if in[k] || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, k)
		}
	}

	return out
}

// minPairManhattan is the smallest Manhattan distance between any cell of a and
// any cell of b - the gap between two complexes.
func minPairManhattan(a, b [][2]int) int {
	best := 1 << 30

	for _, p := range a {
		for _, q := range b {
			if dd := manhattan(p[0], p[1], q[0], q[1]); dd < best {
				best = dd
			}
		}
	}

	return best
}

// minManhattanTo is the smallest Manhattan distance from (x, y) to any cell in the set.
func minManhattanTo(x, y int, cells [][2]int) int {
	best := 1 << 30

	for _, c := range cells {
		if dd := manhattan(x, y, c[0], c[1]); dd < best {
			best = dd
		}
	}

	return best
}

// manhattan is the grid (taxicab) distance between two cells.
func manhattan(ax, ay, bx, by int) int {
	dx := ax - bx
	if dx < 0 {
		dx = -dx
	}

	dy := ay - by
	if dy < 0 {
		dy = -dy
	}

	return dx + dy
}

// tieBefore gives a stable ordering for connector cells at equal distance (top
// row first, then leftmost), so the chosen site does not flicker between turns.
func tieBefore(x, y, bx, by int) bool {
	if y != by {
		return y < by
	}

	return x < bx
}

// unitBefore gives a stable ordering for units (row, then column, then id) so a
// choice among equally eligible builders does not depend on the random
// map iteration order Units() returns.
func unitBefore(a, b *game.UnitInfo) bool {
	if a.GridY != b.GridY {
		return a.GridY < b.GridY
	}

	if a.GridX != b.GridX {
		return a.GridX < b.GridX
	}

	return a.ID < b.ID
}
