// patharrows.go : implements the ground path arrow markers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Ground path arrow markers: 8-direction triangles drawn for a unit's move route,
// green for normal steps and blue at every per turn boundary (the speed frontier).
// Arrows are rendered once and cached per colour+heading+zoom; all routes are
// collected into a per cell map before blitting so each cell is drawn exactly once
// and the blue boundary markers always beat green path steps on a shared cell.
package gamescreen

import (
	"fmt"
	"sort"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/paths"
)

// The 8-direction ground path arrow marker: a triangle (three vertices) per
// heading (N=0, NE=1, E=2, SE=3, S=4, SW=5, W=6, NW=7), in 64px-cell pixels
// relative to the cell centre. Vertex A is the tip (it points the way the unit
// travels); B and C are the base corners. Drawn as a 3-line outline.
//
// C++ ported : paths.cpp - Paths_8DirPointsArrayMarkerA/B/C (Paths_DrawMarker
// draws the three lines A-B, B-C, C-A in the route colour).
var (
	pathArrowA = [8][2]int{{0, -10}, {8, -8}, {10, 0}, {8, 8}, {0, 10}, {-8, 8}, {-10, 0}, {-8, -8}}
	pathArrowB = [8][2]int{{-10, 10}, {-12, 0}, {-10, -10}, {0, -12}, {10, -10}, {12, 0}, {10, 10}, {0, 12}}
	pathArrowC = [8][2]int{{10, 10}, {0, 12}, {-10, 10}, {-12, 0}, {-10, -10}, {0, -12}, {10, -10}, {12, 0}}
)

// routeCell holds the winning color and direction for one map cell across all
// routes drawn this frame: blue boundary markers beat green path steps so the
// per turn landmarks remain legible when two routes share a cell.
type routeCell struct {
	color uint8
	dir   int
}

// pathArrow returns the cached arrow marker sprite for a heading (dir 0-7)
// in the given colour, sized for the current zoom z (the 64px-base vertices are
// scaled by z/64 so the arrow tracks the map like the resource overlay). Each
// colour+dir+zoom triangle is rendered once into a transparent buffer (index 0)
// and cached; later frames just blit it. Returns nil for an out of range heading.
//
// C++ ported : paths.cpp - Paths_DrawMarker (the per step direction triangle).
func (s *Screen) pathArrow(color uint8, dir, z int) *gfx.Buffer {
	if dir < 0 || dir > 7 {
		return nil
	}

	if s.mapArt.pathArrowCache == nil {
		s.mapArt.pathArrowCache = map[string]*gfx.Buffer{}
	}

	key := fmt.Sprintf("%d-%d@%d", color, dir, z)

	// Reuse the already rendered marker when one exists for this colour/dir/zoom.
	if b, ok := s.mapArt.pathArrowCache[key]; ok {
		return b
	}

	// Size a square buffer that holds the zoom scaled triangle centred on its
	// middle (the largest vertex offset is 12 in 64-base units).
	half := 12*z/64 + 2
	size := 2*half + 1
	buf := gfx.New(size, size)
	ax, ay := half+pathArrowA[dir][0]*z/64, half+pathArrowA[dir][1]*z/64
	bx, by := half+pathArrowB[dir][0]*z/64, half+pathArrowB[dir][1]*z/64
	cx, cy := half+pathArrowC[dir][0]*z/64, half+pathArrowC[dir][1]*z/64
	buf.DrawLine(ax, ay, bx, by, color)
	buf.DrawLine(bx, by, cx, cy, color)
	buf.DrawLine(cx, cy, ax, ay, color)
	s.mapArt.pathArrowCache[key] = buf
	return buf
}

// pathSeed returns the per turn accumulator seed that pins the blue boundary
// markers to fixed map cells: the first boundary falls at the unit's remaining
// move points this turn (so the markers do not slide along as the unit advances),
// then every perTurn cells after. A unit with no moves left (a queued route, or a
// fresh full budget unit) seeds 0, anchoring the first boundary a full turn out.
func pathSeed(perTurn, remaining int) int {
	if remaining < 0 {
		remaining = 0
	}

	if remaining > perTurn {
		remaining = perTurn
	}

	return (perTurn - remaining) % perTurn
}

// drawAllGroundPaths is the combined ground path drawer for every active and
// queued route this frame. It works in two passes:
//
//  1. Collect: every route (active MoveOrders first, then pending PendingRoutes)
//     is processed in stable unit ID order; each step's cell->color is written
//     to a per cell map where a blue boundary marker always beats a green path
//     step (so the per turn landmarks stay legible when two routes share a cell).
//
//  2. Blit: each collected cell is drawn exactly once. Because each cell maps to
//     a distinct screen position (arrow triangles are centred on distinct cells),
//     the arbitrary map iteration order of the blit pass does not cause flicker.
//
// C++ ported : groundpath.cpp - GroundPath::Draw (late terrain pass).
func (s *Screen) drawAllGroundPaths(view *gfx.Buffer) {
	cells := map[game.Point]routeCell{}

	// Active routes (currently moving units): draw remaining steps from the
	// current MoveOrder index. Sort by unit ID for a stable blit order.
	orders := s.game.MoveOrders()
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].Unit.ID < orders[j].Unit.ID
	})

	for _, o := range orders {
		// Skip a nil or fully consumed order (all steps taken).
		if o == nil || o.Index >= len(o.Steps) {
			continue
		}

		// In simultaneous mode every team's units move during the player's own turn,
		// so restrict the route arrows to the player's units - otherwise an enemy's
		// queued path would draw straight through the fog. The showAIPaths debug flag
		// (F2) deliberately overrides this to reveal NPC routes, like fog off. Turn
		// based is unaffected (only one team has orders at a time).
		if s.simMode() && o.Unit.Team != s.game.State.PlayerTeam && !s.showAIPaths {
			continue
		}

		s.collectRoute(cells, o.Unit, o.Steps[o.Index:])
	}

	// Queued (over budget) routes: the full pending path the unit resumes next
	// turn. Sort by unit ID to keep the stable priority consistent.
	pending := s.game.PendingRoutes()
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Unit.ID < pending[j].Unit.ID
	})

	for _, pr := range pending {
		if s.simMode() && pr.Unit.Team != s.game.State.PlayerTeam && !s.showAIPaths {
			continue // sim mode: do not leak an enemy's queued route through the fog (unless showAIPaths debug reveal is on)
		}

		s.collectRoute(cells, pr.Unit, pr.Steps)
	}

	// Routes HELD by a halt on enemy stop. Drawn exactly like a queued one - the unit is
	// still going there and drives on at the next turn start (or at once on Continue).
	held := s.game.HeldRoutes()
	sort.Slice(held, func(i, j int) bool {
		return held[i].Unit.ID < held[j].Unit.ID
	})

	for _, hr := range held {
		if s.simMode() && hr.Unit.Team != s.game.State.PlayerTeam && !s.showAIPaths {
			continue
		}

		s.collectRoute(cells, hr.Unit, hr.Steps)
	}

	// Blit each collected cell exactly once. Each cell writes to its own
	// screen position (centred on its cell), so blit order here does not matter.
	z := s.r.Zoom()

	for pt, e := range cells {
		arrow := s.pathArrow(e.color, e.dir, z)

		if arrow == nil {
			continue
		}

		px := pt.X*z + z/2 - s.camera.scrollX
		py := pt.Y*z + z/2 - s.camera.scrollY
		blitTransClipped(view, arrow, px-arrow.W/2, py-arrow.H/2)
	}
}

// collectRoute accumulates one route's steps into the cells map, applying the
// boundary accumulator (blue every perTurn cells; green otherwise).
// When two routes share a cell, a blue boundary marker beats a green path step;
// among entries of equal priority, the first route in stable ID order wins (so direction
// stays consistent). An NPC route is skipped unless the showAIPaths debug flag is on.
//
// C++ ported : groundpath.cpp - GroundPath::Draw (boundary accumulator logic).
func (s *Screen) collectRoute(cells map[game.Point]routeCell, unit *game.UnitInfo, steps []paths.Step) {
	if unit == nil {
		return
	}

	// Vanilla shows only the local player's own routes; an NPC route needs the
	// debug flag and uses the gray/red palette to distinguish it.
	npc := unit.Team != s.game.State.PlayerTeam

	if npc && !s.showAIPaths {
		return
	}

	segColor, boundColor := uint8(0x02), uint8(0x03) // player: green / blue

	if npc {
		segColor, boundColor = s.effects.colRubble, uint8(0x01) // NPC debug: gray / red
	}

	// A per turn boundary (the blue marker) falls every AttribSpeed cells worth
	// of movement.
	perTurn := 1

	if unit.Values != nil {
		perTurn = unit.Values.Get(game.AttribSpeed)
	}

	if perTurn < 1 {
		perTurn = 1
	}

	// Seed the accumulator from the unit's remaining move points so the first
	// blue boundary lands on the fixed cell where the unit stops this turn; as
	// the unit advances its Speed drops by what it travelled, so the boundaries
	// stay pinned to the same map cells instead of shifting. A queued route
	// (Speed 0) anchors the first boundary a full turn out. C++: GroundPath::Draw
	// starts limited_speed at the unit's current speed.
	//
	// WHILE ACTIVELY DRIVING a unit's Speed is debited at cell ENTRY (move.go
	// calculateStep, when Moved==0) but the step's Index only advances at cell
	// EXIT - so for the whole crossing the cell is still drawn (Steps[Index:])
	// yet its cost is already gone from Speed, which shifts every blue boundary
	// back one cell and snaps it back at the boundary = a per cell shimmer. Add
	// the cost of the step in progress back to the seed baseline (1 cardinal / 2
	// diagonal, matching the accumulator below) so the boundary stays pinned during
	// the crossing. A unit at rest (Moved==0) and a queued route are unaffected.
	remaining := unit.Speed

	if unit.Moved > 0 && len(steps) > 0 {
		if steps[0].Dir&1 != 0 {
			remaining += 2 // a diagonal step is in progress
		} else {
			remaining++ // a cardinal step is in progress
		}
	}

	acc := pathSeed(perTurn, remaining)

	for _, st := range steps {
		// Approximate the per cell move cost: 1 for a cardinal step, 2 for a
		// diagonal (the odd Dir values).
		if st.Dir&1 != 0 {
			acc += 2
		} else {
			acc++
		}

		color := segColor

		if acc >= perTurn {
			// Crossed a per turn boundary: use the blue marker.
			color = boundColor
			acc -= perTurn
		}

		// Blue beats green for a shared cell; for equal priority the first
		// route in stable ID order wins, preserving a consistent direction.
		if e, ok := cells[st.To]; !ok || color > e.color {
			cells[st.To] = routeCell{color: color, dir: st.Dir}
		}
	}
}

// drawRoutes draws the remaining route of each active MoveOrder as small white
// markers (the A* path); a debug visualisation gated by -set-a-star. The over
// budget queued route (drawAllGroundPaths) shows regardless.
// C++: Paths_DrawMarker over the GroundPath steps.
func (s *Screen) drawRoutes(view *gfx.Buffer) {
	if !s.showAStarPath {
		return
	}

	z := s.r.Zoom()

	for _, o := range s.game.MoveOrders() {
		if o.Unit != nil && o.Unit.Team != s.game.State.PlayerTeam && !s.showAIPaths { // never leak NPC A* paths
			continue
		}

		for i := o.Index; i < len(o.Steps); i++ {
			c := o.Steps[i].To
			px := c.X*z + z/2 - s.camera.scrollX
			py := c.Y*z + z/2 - s.camera.scrollY

			if px < 0 || py < 0 || px >= view.W || py >= view.H {
				continue
			}

			startX := max(0, min(px-1, view.W-1))
			startY := max(0, min(py-1, view.H-1))
			endX := max(0, min(px+1, view.W-1))
			endY := max(0, min(py+1, view.H-1))
			view.DrawBox(startX, startY, endX, endY, 0xFF)
		}
	}
}

// SetShowAStarPath toggles the -set-a-star debug overlay (the white A* route of
// active move orders). Off by default.
func (s *Screen) SetShowAStarPath(on bool) {
	s.showAStarPath = on
}
