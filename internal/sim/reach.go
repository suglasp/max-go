// reach.go : labels connected terrain so unreachable routes can bypass A* search.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
)

// Reachability components reject terrain-impossible routes before A* searches the whole reachable map. Cached labels answer whether two cells can
// connect in O(1), using access.IsAccessible with transient mobile blockers ignored. The real route search remains stricter, so a disconnected pair
// is genuinely unreachable while a connected pair still needs normal pathfinding.

// unitDesc builds the access descriptor for a unit's routing (its movement class
// and the surface mask it may occupy), shared by unitCost and the reachability
// components so both judge passability identically.
func (g *Game) unitDesc(u *game.UnitInfo) access.UnitDesc {
	return access.UnitDesc{Type: u.Type, Flags: u.Flags, LandType: access.LandTypeForUnit(u.Type, u.Flags)}
}

// invalidateReach drops the cached component labels. Called when a structure that
// changes terrain passability (a building, bridge or water platform) is placed or
// destroyed, so the next route check rebuilds against the current map.
func (g *Game) invalidateReach() {
	g.reachLabels = nil
}

// reachLabelsFor returns the connected component label of every cell (index
// y*W+x) for a unit's land type mask: cells sharing a label are mutually
// reachable on terrain; -1 marks a cell the unit cannot occupy at all. The result
// is cached per mask until invalidateReach clears it. The mask alone keys the
// cache because terrain passability for a mobile mover is team independent (only
// stationary movers being placed have team specific blocking).
func (g *Game) reachLabelsFor(d access.UnitDesc, team int) []int32 {
	if g.reachLabels == nil {
		g.reachLabels = make(map[uint8][]int32)
	}
	if lab, ok := g.reachLabels[d.LandType]; ok {
		return lab
	}
	lab := buildReachLabels(g.Grid, d, team, g.W, g.H)
	g.reachLabels[d.LandType] = lab
	return lab
}

// buildReachLabels flood fills the grid into connected components for one unit
// descriptor. A cell is passable when access.IsAccessible (NoModifiers: terrain +
// permanent structures, ignoring transient mobile blockers) is not Blocked. Each
// component gets a distinct label of zero or more; impassable cells stay -1.
func buildReachLabels(grid *access.Grid, d access.UnitDesc, team, w, h int) []int32 {
	lab := make([]int32, w*h)
	for i := range lab {
		lab[i] = -1
	}
	passable := func(x, y int) bool {
		return grid.IsAccessible(d, team, x, y, access.NoModifiers).Passable()
	}
	var stack []int
	var next int32
	for sy := 0; sy < h; sy++ {
		for sx := 0; sx < w; sx++ {
			si := sy*w + sx
			if lab[si] != -1 || !passable(sx, sy) {
				continue
			}
			// Flood this component (iterative DFS to bound the stack to live cells).
			lab[si] = next
			stack = append(stack[:0], si)
			for len(stack) > 0 {
				ci := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				cx, cy := ci%w, ci/w
				for dir := 0; dir < 8; dir++ {
					off := paths.StepOffset(dir)
					nx, ny := cx+off.X, cy+off.Y
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					ni := ny*w + nx
					if lab[ni] != -1 || !passable(nx, ny) {
						continue
					}
					lab[ni] = next
					stack = append(stack, ni)
				}
			}
			next++
		}
	}
	return lab
}

// reachable reports whether dest might be reachable from start for unit u on
// terrain. It returns false only when the cached components PROVE the two cells
// are in different terrain reachable components - the caller then skips the A*
// flood. It is deliberately permissive: an out of range cell, or a start/dest the
// component map marks impassable (label -1), returns true so paths.Find makes the
// final (and, for an impassable dest, already O(1)) decision.
func (g *Game) reachable(d access.UnitDesc, team, startX, startY, destX, destY int) bool {
	lab := g.reachLabelsFor(d, team)
	si := startY*g.W + startX
	di := destY*g.W + destX
	if si < 0 || si >= len(lab) || di < 0 || di >= len(lab) {
		return true
	}
	ls, ld := lab[si], lab[di]
	if ls < 0 || ld < 0 {
		return true // not both in the passable set: let paths.Find decide
	}
	return ls == ld
}

// findRoute plans a route to (destX, destY). It rejects terrain-unreachable destinations from the cached components before running A*, then returns
// the steps, the move executor's cost function, and whether a route was found.
func (g *Game) findRoute(u *game.UnitInfo, destX, destY int) ([]paths.Step, paths.CostFunc, bool) {
	return g.findRouteAvoiding(u, destX, destY, nil)
}

// findRouteAvoiding plans a route like findRoute but, when avoid is non nil, adds
// its per cell penalty to each enterable cell's cost while the A* CHOOSES the route,
// so the search prefers a longer path around high penalty ground. This is the AI
// danger weighting: a unit routes around cells under enemy fire instead of driving
// through them. The penalty steers route CHOICE only. The cost function handed back
// to the move executor is the plain terrain cost, so a safer detour never inflates
// the unit's movement point spend for the turn. An impassable cell stays impassable
// (a penalty never revives a zero cost cell), and the admissible heuristic still
// holds (penalties only raise real costs), so the search stays optimal. A nil avoid
// is identical to findRoute.
//
// C++ ported : the AI folding a unit's damage potential into the AccessMap the
// searcher reads, so a computed path bends around defended ground (paths_manager.cpp).
func (g *Game) findRouteAvoiding(u *game.UnitInfo, destX, destY int, avoid func(x, y int) int) ([]paths.Step, paths.CostFunc, bool) {
	d := g.unitDesc(u)
	start := u.Position()
	if !g.reachable(d, u.Team, start.X, start.Y, destX, destY) {
		return nil, nil, false // terrain disconnected: skip the A* flood
	}
	base := paths.GridCost(g.Grid, d, u.Team, access.SameClassBlocks)
	search := base
	if avoid != nil {
		search = func(x, y int) int {
			c := base(x, y)
			if c == 0 {
				return 0 // keep an impassable cell impassable
			}
			return c + avoid(x, y)
		}
	}
	res, ok := paths.Find(search, g.W, g.H, start, game.Point{X: destX, Y: destY}, 0)
	if !ok || len(res.Steps) == 0 {
		return nil, base, false
	}
	return res.Steps, base, true
}
