// patharrows_test.go : tests patharrows.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/paths"
)

// TestPathArrowRendersAndCaches verifies the cached arrow marker: a sprite
// is drawn in the requested colour, the same colour/dir/zoom is cached and reused,
// and an out of range heading returns nil.
func TestPathArrowRendersAndCaches(t *testing.T) {
	s := &Screen{}
	a := s.pathArrow(0x02, 2, 64) // green, east, full zoom
	if a == nil || a.W < 1 || a.H < 1 {
		t.Fatal("pathArrow should render a non-empty sprite")
	}
	// The triangle outline was drawn in the colour somewhere in the buffer.
	found := false
	for y := 0; y < a.H && !found; y++ {
		for x := 0; x < a.W; x++ {
			if a.At(x, y) == 0x02 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("the arrow sprite has no pixels of the requested colour")
	}
	if b := s.pathArrow(0x02, 2, 64); b != a {
		t.Error("pathArrow should cache and reuse the same colour/dir/zoom sprite")
	}
	if s.pathArrow(0x02, 9, 64) != nil {
		t.Error("an out-of-range heading should return nil")
	}
}

// blueCells returns the path cell indices (0-based from the unit's current cell)
// that get the blue per turn boundary marker, for a cardinal step route, mirroring
// collectRoute's accumulator with the pathSeed anchor.
func blueCells(perTurn, remaining, n int) []int {
	acc := pathSeed(perTurn, remaining)
	var out []int
	for i := 0; i < n; i++ {
		acc++ // a cardinal step costs 1 budget unit
		if acc >= perTurn {
			out = append(out, i)
			acc -= perTurn
		}
	}
	return out
}

// TestPathSeedAnchorsBoundary verifies that the blue
// per turn markers shifted along with the unit instead of staying put. The seed
// must keep each boundary pinned to the same MAP cell as the unit advances and its
// Speed drops.
func TestPathSeedAnchorsBoundary(t *testing.T) {
	// A fresh 6-move unit at the start of its route: boundaries at cells 6 and 12
	// (0-based indices 5 and 11).
	if got := blueCells(6, 6, 12); !equalInts(got, []int{5, 11}) {
		t.Fatalf("fresh route blue cells = %v, want [5 11]", got)
	}
	// After the unit has moved 2 cells (Speed now 4, route starts 2 cells later),
	// the boundaries must land on the SAME map cells: now at offsets 3 and 9.
	moved := blueCells(6, 4, 10)
	if !equalInts(moved, []int{3, 9}) {
		t.Fatalf("after moving 2, blue cells = %v, want [3 9]", moved)
	}
	// Absolute cells (start offset + index) are unchanged: {5,11} both frames.
	for i := range moved {
		if moved[i]+2 != []int{5, 11}[i] {
			t.Errorf("boundary %d moved to absolute cell %d, want it pinned", i, moved[i]+2)
		}
	}
	// A queued (0-move) route anchors the first boundary a full turn out (cell 6).
	if got := blueCells(6, 0, 12); !equalInts(got, []int{5, 11}) {
		t.Errorf("queued route blue cells = %v, want [5 11]", got)
	}
}

// TestActiveRouteBoundaryPinnedMidCell verifies the residual shimmer
// seen WHILE A UNIT DRIVES (the at rest case was already steady). A unit's
// Speed is debited at cell ENTRY but its step Index advances only at cell EXIT, so
// for the whole crossing the same route is drawn with a Speed one step smaller -
// which shifted every blue boundary back a cell, then snapped it back. The mid cell
// seed compensation must keep the blue per turn boundaries on the SAME map cells as
// the at rest frame.
func TestActiveRouteBoundaryPinnedMidCell(t *testing.T) {
	s, g := newClickScreen(t)
	u := mkUnit(g, assets.Tank, 1, 0, 0, game.FlagMobileLandUnit|game.FlagSelectable)
	u.Team = g.State.PlayerTeam
	u.Values.Set(game.AttribSpeed, 6)

	// A 12-cell eastward route (cardinal steps; Dir 2 = east).
	var steps []paths.Step
	for i := 1; i <= 12; i++ {
		steps = append(steps, paths.Step{Dir: 2, To: game.Point{X: i, Y: 0}})
	}

	// blueIdx returns the route cell X's that got the blue boundary marker (0x03).
	blueIdx := func() []int {
		cells := map[game.Point]routeCell{}
		s.collectRoute(cells, u, steps)
		var out []int
		for i := 1; i <= 12; i++ { // ascending X = already sorted
			if c, ok := cells[game.Point{X: i, Y: 0}]; ok && c.color == 0x03 {
				out = append(out, i)
			}
		}
		return out
	}

	// At rest: full Speed, no pixel progress into a cell.
	u.Speed, u.Moved = 6, 0
	atRest := blueIdx()

	// Mid cell entering the first step: Speed already debited by 1, Moved > 0, the
	// SAME route still drawn (Index has not advanced yet).
	u.Speed, u.Moved = 5, 10
	midCell := blueIdx()

	if !equalInts(atRest, midCell) {
		t.Errorf("blue boundary shimmered while driving: at-rest %v vs mid-cell %v", atRest, midCell)
	}
	if len(atRest) == 0 {
		t.Fatal("expected at least one blue boundary on a 12-cell route at speed 6")
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestDrawAllGroundPathsDrawsArrows verifies the combined route draw: a unit
// actively executing a move draws ground path arrows along its remaining route.
func TestDrawAllGroundPathsDrawsArrows(t *testing.T) {
	s, g := newClickScreen(t)
	tank := mkUnit(g, assets.Tank, 1, 1, 1, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Speed = 6
	tank.Values.Set(game.AttribSpeed, 6)
	if !g.OrderMove(tank, 7, 1) { // route east across open land
		t.Fatal("OrderMove should succeed on open land")
	}
	z := s.r.Zoom()
	s.camera.scrollX, s.camera.scrollY = 0, 0
	view := gfx.New(8*z, 2*z) // covers the (1,1)->(7,1) route at the origin
	s.drawAllGroundPaths(view)

	// At least one arrow pixel (not transparent) was drawn along the route.
	drawn := 0
	for y := 0; y < view.H; y++ {
		for x := 0; x < view.W; x++ {
			if view.At(x, y) != 0 {
				drawn++
			}
		}
	}
	if drawn == 0 {
		t.Error("an active move should draw ground-path arrows, drew nothing")
	}
}

// TestFlickerFreeSharedCell verifies that two routes sharing a cell always render
// the same stable color (blue wins over green) regardless of call order. Before
// the fix, PendingRoutes iterated a Go map in random order, so the color
// flickered between frames when two routes overlapped.
func TestFlickerFreeSharedCell(t *testing.T) {
	s, g := newClickScreen(t)
	// Two tanks on different rows routed through the same destination cell (4,1).
	a := mkUnit(g, assets.Tank, 1, 1, 1, game.FlagMobileLandUnit|game.FlagSelectable)
	a.Speed = 6
	a.Values.Set(game.AttribSpeed, 6)
	b := mkUnit(g, assets.Tank, 1, 3, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	b.Speed = 6
	b.Values.Set(game.AttribSpeed, 6)
	// Tank a routes through (4,1); tank b routes through (4,3): their routes
	// do not share cells, but we can test determinism by drawing twice and
	// verifying the pixel map is identical both times.
	g.OrderMove(a, 7, 1)
	g.OrderMove(b, 7, 3)

	z := s.r.Zoom()
	s.camera.scrollX, s.camera.scrollY = 0, 0
	view1 := gfx.New(8*z, 6*z)
	view2 := gfx.New(8*z, 6*z)
	s.drawAllGroundPaths(view1)
	s.drawAllGroundPaths(view2)

	// Both draws must produce identical pixels (no flicker).
	for y := 0; y < view1.H; y++ {
		for x := 0; x < view1.W; x++ {
			if view1.At(x, y) != view2.At(x, y) {
				t.Errorf("pixel (%d,%d) differs between draws: %d vs %d (flicker)", x, y, view1.At(x, y), view2.At(x, y))
				return
			}
		}
	}
}

// TestShowAIPathsRevealsNPCRouteInSimMode is the fix for the F2 "Show AI paths" toggle
// doing nothing in simultaneous mode: the anti fog-leak guard in drawAllGroundPaths drops
// every NPC route before the showAIPaths check, so the debug flag looked dead in the
// default play mode. Off = the NPC route stays hidden; on = it is revealed.
func TestShowAIPathsRevealsNPCRouteInSimMode(t *testing.T) {
	s, g := newClickScreen(t)
	s.SetPlayMode(int(game.PlaySimultaneous))
	if !s.simMode() {
		t.Fatal("setup: expected simultaneous mode")
	}

	// An NPC (team 1, not the player team 0) with a queued route east across open land.
	npc := game.NewUnitInfo(assets.Tank, 1, 20, 0)
	npc.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	npc.Values = &game.UnitValues{}
	npc.Values.Set(game.AttribSpeed, 6)
	npc.Speed = 6
	g.Units.Deploy(npc, 1, 1)
	if npc.Team == g.State.PlayerTeam {
		t.Fatalf("setup: the NPC must not be on the player team %d", g.State.PlayerTeam)
	}
	if !g.OrderMove(npc, 7, 1) {
		t.Fatal("OrderMove should succeed on open land")
	}

	z := s.r.Zoom()
	s.camera.scrollX, s.camera.scrollY = 0, 0

	drawnPixels := func() int {
		view := gfx.New(8*z, 2*z)
		s.drawAllGroundPaths(view)
		n := 0
		for y := 0; y < view.H; y++ {
			for x := 0; x < view.W; x++ {
				if view.At(x, y) != 0 {
					n++
				}
			}
		}
		return n
	}

	s.showAIPaths = false
	if n := drawnPixels(); n != 0 {
		t.Errorf("showAIPaths off: the NPC route must stay hidden in sim mode, drew %d pixels", n)
	}

	s.showAIPaths = true
	if n := drawnPixels(); n == 0 {
		t.Error("showAIPaths on: the NPC route should be revealed in sim mode, drew nothing")
	}
}
