// baselayout.go : the M.A.X. AI base layout guard - keep a builder from sealing itself
// in behind its own structures, so it can always drive out to raise the next one.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// A builder that can reach at least minRoamCells drivable cells is not boxed in. A
// pocket sealed by buildings is tiny (a handful of cells); the base and the open ground
// beyond give far more, so this cleanly tells a trapped builder from a free one.
//
// C++ ported : the AI keeping its builders mobile around the base
// (taskmanagebuildings.cpp - the AreaMap access checks), reduced to a reach test.
const minRoamCells = 12

// preservesEscape reports whether the builder still has a drivable way out after a
// fp x fp structure is raised at (x, y). It treats the footprint as blocked, seeds a
// flood from the enterable border cells (the builder's exits) and asks whether the flood
// reaches enough open ground (roamReaches). A site that would ring the builder in fails,
// so findSite never seals its own builder inside the base. The builder must already be
// out of the cell hash (findSite removes it for its whole run).
func (t *taskManageBuildings) preservesEscape(builder *game.UnitInfo, x, y, fp int) bool {
	blocked := sim.FootprintCells(x, y, fp)
	seeds := t.borderSeeds(x, y, fp, blocked)

	return t.roamReaches(builder, seeds, blocked, minRoamCells)
}

// canRoam reports whether the builder can drive to a reasonable amount of open ground
// from where it stands. A false result is the "locked between buildings" signature: the
// builder is sealed in a pocket and the rescue pass must free it. The builder is lifted
// out of the cell hash for the query so it never blocks its own start cell.
func (t *taskManageBuildings) canRoam(builder *game.UnitInfo) bool {
	g := t.ctx.Game
	g.Units.Cells.Remove(builder)
	defer g.Units.Cells.Add(builder)

	seeds := [][2]int{{builder.GridX, builder.GridY}}

	return t.roamReaches(builder, seeds, nil, minRoamCells)
}

// roamReaches runs a bounded flood from the seed cells over ground the builder can
// drive, counting distinct reachable cells and stopping as soon as it hits threshold.
// blocked cells (a hypothetical footprint) are treated as walls. The builder must
// already be out of the cell hash (findSite removes it for its whole run; canRoam wraps
// the call), so it never blocks its own seeds.
func (t *taskManageBuildings) roamReaches(builder *game.UnitInfo, seeds [][2]int, blocked map[[2]int]bool, threshold int) bool {
	g := t.ctx.Game
	d := access.UnitDesc{Type: builder.Type, Flags: builder.Flags, LandType: access.LandTypeForUnit(builder.Type, builder.Flags)}

	drivable := func(x, y int) bool {
		if x < 0 || y < 0 || x >= g.W || y >= g.H {
			return false
		}
		if blocked[[2]int{x, y}] {
			return false
		}
		return g.Grid.IsAccessible(d, builder.Team, x, y, access.SameClassBlocks).Passable()
	}

	seen := map[[2]int]bool{}
	var queue [][2]int
	for _, s := range seeds {
		if seen[s] || !drivable(s[0], s[1]) {
			continue
		}
		seen[s] = true
		queue = append(queue, s)
	}

	count := 0
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		count++
		if count >= threshold {
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

	return count >= threshold
}

/* // note: unused code in Go port
// footprintSet duplicated sim.FootprintCells byte for byte; baselayout now calls sim.FootprintCells.
// footprintSet returns the cells a fp x fp structure at (x, y) occupies, as a set.
func footprintSet(x, y, fp int) map[[2]int]bool {
	cells := make(map[[2]int]bool, fp*fp)
	for cy := y; cy < y+fp; cy++ {
		for cx := x; cx < x+fp; cx++ {
			cells[[2]int{cx, cy}] = true
		}
	}
	return cells
}
*/

// borderSeeds returns the in bounds border cells around a footprint that are not part
// of it - the cells a builder would step out onto, the flood seeds for preservesEscape.
func (t *taskManageBuildings) borderSeeds(x, y, fp int, blocked map[[2]int]bool) [][2]int {
	g := t.ctx.Game
	var seeds [][2]int

	for cy := y - 1; cy <= y+fp; cy++ {
		for cx := x - 1; cx <= x+fp; cx++ {
			if cx >= x && cx < x+fp && cy >= y && cy < y+fp {
				continue
			}
			if cx < 0 || cy < 0 || cx >= g.W || cy >= g.H {
				continue
			}
			if blocked[[2]int{cx, cy}] {
				continue
			}
			seeds = append(seeds, [2]int{cx, cy})
		}
	}

	return seeds
}
