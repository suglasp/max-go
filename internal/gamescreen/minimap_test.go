// minimap_test.go : tests minimap.go.
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
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

func TestZoomSliderMath(t *testing.T) {
	// Knob: max zoom at the far left (knobW/2), min zoom at the right edge.
	if got := zoomKnobOffset(drawmap.MaxZoom); got != 14 {
		t.Errorf("knob at MaxZoom = %d, want 14 (knobW/2)", got)
	}
	if lo, hi := zoomKnobOffset(drawmap.MaxZoom), zoomKnobOffset(drawmap.MinZoom); hi <= lo {
		t.Errorf("knob should move right as zoom shrinks: max=%d min=%d", lo, hi)
	}

	// Click mapping: far left -> MaxZoom, far right -> MinZoom, monotonic.
	left := zoomFromSliderX(ZoomSlider.ULX, 16)
	right := zoomFromSliderX(ZoomSlider.LRX, 16)
	if left != drawmap.MaxZoom {
		t.Errorf("left edge zoom = %d, want %d", left, drawmap.MaxZoom)
	}
	if right != drawmap.MinZoom {
		t.Errorf("right edge zoom = %d, want %d", right, drawmap.MinZoom)
	}
	mid := zoomFromSliderX((ZoomSlider.ULX+ZoomSlider.LRX)/2, 16)
	if !(mid < left && mid > right) {
		t.Errorf("mid zoom %d should be between %d and %d", mid, right, left)
	}
}

func TestDotColor(t *testing.T) {
	s := &Screen{minimap: minimapState{minimapDots: [4]uint8{10, 20, 30, 40}}}
	if s.dotColor(0) != 10 || s.dotColor(3) != 40 {
		t.Error("dotColor should return the team's resolved colour")
	}
	if s.dotColor(99) != gfx.ColorYellow {
		t.Error("out-of-range team should be the alien/neutral yellow")
	}
	if (&Screen{}).dotColor(0) != gfx.ColorYellow {
		t.Error("unset (zero) dot colour should fall back to yellow")
	}
}

// TestMinimapHidesEasterEggProps verifies that the hidden Waldo / Harvster gag
// props (alien team, stationary) must NOT blip on the minimap - the surprise is
// finding them on the map itself. A normal alien unit of the same colour still
// blips, so the skip is specific to the gag props, not the team.
func TestMinimapHidesEasterEggProps(t *testing.T) {
	mmW, mmH := viewSize(Minimap)

	yellowDots := func(deploy func(*Screen)) int {
		s, _ := newClickScreen(t)
		s.fogTeam = 0
		s.fogShow = false // reveal every blip so visibility never hides the unit
		deploy(s)
		dst := gfx.New(mmW, mmH)
		s.drawMinimapUnits(dst, mmW, mmH)
		n := 0
		for _, px := range dst.Pix {
			if px == gfx.ColorYellow {
				n++
			}
		}
		return n
	}

	waldo := yellowDots(func(s *Screen) {
		u := game.NewUnitInfo(assets.Waldo, game.AlienTeam, 20, 0)
		u.Flags = game.FlagStationary
		u.Values = &game.UnitValues{}
		s.game.Units.Deploy(u, 5, 5)
	})
	if waldo != 0 {
		t.Errorf("a Waldo gag prop stamped %d minimap pixels, want 0 (it must stay hidden)", waldo)
	}

	tank := yellowDots(func(s *Screen) {
		u := game.NewUnitInfo(assets.Tank, game.AlienTeam, 21, 0)
		u.Flags = game.FlagMobileLandUnit
		u.Values = &game.UnitValues{}
		s.game.Units.Deploy(u, 5, 5)
	})
	if tank == 0 {
		t.Error("a normal alien unit should blip on the minimap (control check)")
	}
}

func TestIsCombatUnit(t *testing.T) {
	armed := &game.UnitInfo{Values: &game.UnitValues{}}
	armed.Values.Set(game.AttribAttack, 5)
	if !isCombatUnit(armed) {
		t.Error("unit with attack>0 should be a combat unit")
	}
	unarmed := &game.UnitInfo{Values: &game.UnitValues{}}
	if isCombatUnit(unarmed) {
		t.Error("unit with attack 0 should not be a combat unit")
	}
	if isCombatUnit(&game.UnitInfo{}) {
		t.Error("unit with nil Values should not be a combat unit")
	}
}

func TestMinimapControlClicks(t *testing.T) {
	rel := func(x, y int) platform.MouseState {
		return platform.MouseState{X: x, Y: y, Buttons: platform.MouseReleaseLeft}
	}
	// X2 click toggles on, then off.
	s := &Screen{turnFlow: turnFlowState{handover: -1}}
	if !s.handleMinimapZoomControls(rel(MinimapX2.ULX+2, MinimapX2.ULY+2)) || !s.minimap.minimap2x {
		t.Fatalf("X2 click did not toggle on (on=%v)", s.minimap.minimap2x)
	}
	if !s.handleMinimapZoomControls(rel(MinimapX2.ULX+2, MinimapX2.ULY+2)) || s.minimap.minimap2x {
		t.Fatal("second X2 click should toggle off")
	}
	// TNT click toggles.
	s2 := &Screen{turnFlow: turnFlowState{handover: -1}}
	if !s2.handleMinimapZoomControls(rel(MinimapTnt.ULX+2, MinimapTnt.ULY+2)) || !s2.minimap.minimapTnt {
		t.Fatal("TNT click did not toggle on")
	}
	// A click away from the controls is not consumed.
	s3 := &Screen{turnFlow: turnFlowState{handover: -1}}
	if s3.handleMinimapZoomControls(rel(300, 300)) {
		t.Fatal("click away from controls should not be consumed")
	}
	// Controls are inert during a hot seat handover banner.
	s4 := &Screen{turnFlow: turnFlowState{handover: 0}}
	if s4.handleMinimapZoomControls(rel(MinimapX2.ULX+2, MinimapX2.ULY+2)) || s4.minimap.minimap2x {
		t.Fatal("controls should be disabled during handover")
	}
}

// TestMinimapOrderRightClickMoves verifies the minimap "move" gesture: a RIGHT click
// on the minimap orders the selected unit to that map cell (C++: WINDOW_MINIMAP
// right release = ORDER_MOVE), while a LEFT click recenters the view (elsewhere).
func TestMinimapOrderRightClickMoves(t *testing.T) {
	s, g := newClickScreen(t)
	tank := mkUnit(g, assets.Tank, 1, 1, 1, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Speed = 0 // out of moves -> the order queues (deterministic), like a 0-move map click
	s.selected = tank

	// A point outside the minimap is not consumed and orders nothing.
	if s.minimapOrder(MainMap.ULX+4, MainMap.ULY+4) {
		t.Fatal("a point off the minimap must not be consumed by minimapOrder")
	}

	// A right click on the minimap centre orders the tank to that (distant) cell:
	// consumed, the selection unchanged, and the move queued for next turn.
	mx := (Minimap.ULX + Minimap.LRX) / 2
	my := (Minimap.ULY + Minimap.LRY) / 2
	if !s.minimapOrder(mx, my) {
		t.Fatal("a right-click inside the minimap should be consumed")
	}
	s.effects.bulbDue = 0
	s.tickPlanningBulb() // flush the deferred move behind the planning bulb
	if s.selected != tank {
		t.Errorf("a minimap move must not change the selection, got %v", s.selected)
	}
	queued := false
	for _, pr := range g.PendingRoutes() {
		if pr.Unit == tank {
			queued = true
		}
	}
	if !queued {
		t.Errorf("the minimap right-click should queue the tank's move, PendingRoutes=%d", len(g.PendingRoutes()))
	}
}

// TestMinimapCellAt2xInvertsZoom verifies the QOL fix: while the 2X overview is
// active, a minimap click must map back through the zoom transform. Centring the
// view on a known cell and clicking the minimap centre returns that same cell;
// and a left edge click in 2X maps to a cell nearer the centre than the plain 1X
// mapping would (proving the zoom is inverted, not ignored).
func TestMinimapCellAt2xInvertsZoom(t *testing.T) {
	s, _ := newClickScreen(t)
	vw, vh := viewSize(MainMap)
	// Centre the main view on map cell (8,8) of the 16x16 test map.
	s.camera.scrollX, s.camera.scrollY = centerScroll(8, 8, s.r.Zoom(), vw, vh)
	s.minimap.minimap2x = true

	cx, cy := (Minimap.ULX+Minimap.LRX)/2, (Minimap.ULY+Minimap.LRY)/2
	gotX, gotY, ok := s.minimapCellAt(cx, cy)
	if !ok {
		t.Fatal("the minimap centre should be inside the minimap")
	}
	if gotX != 8 || gotY != 8 {
		t.Errorf("2X centre click should return the view-centre cell (8,8), got (%d,%d)", gotX, gotY)
	}

	// A left edge click: 2X (zoomed in around the centre) lands RIGHT of the 1X
	// mapping, which would read the far left of the whole map.
	twoX, _, _ := s.minimapCellAt(Minimap.ULX, cy)
	oneX, _, _ := minimapCell(Minimap.ULX, cy, s.wmap.W, s.wmap.H)
	if !(twoX > oneX) {
		t.Errorf("2X left-edge cell (%d) should be nearer the centre than 1X (%d)", twoX, oneX)
	}
}

// TestMinimap2xDragDoesNotDrift verifies the 2X click to scroll sensitivity fix: a
// held left drag recenter freezes the overview origin on the press edge, so a
// second held frame at the SAME cursor point lands on the SAME view (no drift).
// The contrast check confirms that an unfrozen origin would move between frames.
func TestMinimap2xDragDoesNotDrift(t *testing.T) {
	s, _ := newClickScreen(t)
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(8, 8, s.r.Zoom(), vw, vh)
	s.minimap.minimap2x = true

	// An off centre point so the first recenter actually moves the view.
	mx := Minimap.ULX + 3
	my := (Minimap.ULY + Minimap.LRY) / 2

	// First held frame: captures the origin and recenters.
	s.recenterFromMinimap(mx, my)
	if !s.minimap.minimapDrag {
		t.Fatal("a held minimap recenter should start a drag (freeze the origin)")
	}
	sx1, sy1 := s.camera.scrollX, s.camera.scrollY

	// The live origin has now moved (the view shifted); a unfrozen mapping would
	// resolve the same cursor point to a different cell -> the drift bug.
	mmW, mmH := viewSize(Minimap)
	liveOX, liveOY := s.minimap2xOrigin(mmW, mmH)
	if liveOX == s.minimap.minimapDragOX && liveOY == s.minimap.minimapDragOY {
		t.Fatal("setup: the view did not move, so the drag cannot show a drift")
	}

	// Second held frame at the same point: the frozen origin keeps the cell stable,
	// so the view must not move again.
	s.recenterFromMinimap(mx, my)
	if s.camera.scrollX != sx1 || s.camera.scrollY != sy1 {
		t.Errorf("held 2X drag drifted: scroll (%d,%d) -> (%d,%d), want stable", sx1, sy1, s.camera.scrollX, s.camera.scrollY)
	}

	// Releasing the button ends the drag and drops the frozen origin.
	s.minimap.minimapDrag = false
}

// TestMinimapOrderConsumesWithoutSelection verifies a minimap right click is
// consumed (nothing happens) even with nothing selected, so it never falls through
// to the map's right click = Status / deselect handler.
func TestMinimapOrderConsumesWithoutSelection(t *testing.T) {
	s, _ := newClickScreen(t)
	s.selected = nil
	mx := (Minimap.ULX + Minimap.LRX) / 2
	my := (Minimap.ULY + Minimap.LRY) / 2
	if !s.minimapOrder(mx, my) {
		t.Error("a minimap right-click with no selection should still be consumed")
	}
}

// TestMinimapBlipSize checks DrawMap_RenderMiniMapUnitList (and the DOS minimap): every unit is
// a 2x2 cell blip, a unit that is not a building is 1 cell in the 2X view, and an edge blip shifts inward.
func TestMinimapBlipSize(t *testing.T) {
	s, _ := newClickScreen(t)
	w, h := s.wmap.W, s.wmap.H // one minimap pixel per cell

	stamp := func(flags game.Flags, gridX, gridY int, zoom2x bool) (int, int, int) {
		s.minimap.minimap2x = zoom2x
		u := &game.UnitInfo{Team: 0, GridX: gridX, GridY: gridY, Flags: flags}
		dst := gfx.New(w, h)
		s.stampUnit(dst, u, w, h)
		n, minX, minY := 0, w, h
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if dst.At(x, y) != 0 {
					n++
					minX, minY = min(minX, x), min(minY, y)
				}
			}
		}
		return n, minX, minY
	}

	if n, _, _ := stamp(game.FlagMobileLandUnit, 5, 5, false); n != 4 {
		t.Errorf("mobile unit blip = %d pixels, want 4 (2x2)", n)
	}
	if n, _, _ := stamp(game.FlagMobileLandUnit, 5, 5, true); n != 1 {
		t.Errorf("mobile unit blip in the 2X view = %d pixels, want 1", n)
	}
	if n, _, _ := stamp(game.FlagBuilding|game.FlagStationary, 5, 5, true); n != 4 {
		t.Errorf("building blip in the 2X view = %d pixels, want 4", n)
	}
	if n, x, y := stamp(game.FlagMobileLandUnit, w-1, h-1, false); n != 4 || x != w-2 || y != h-2 {
		t.Errorf("corner blip = %d pixels from (%d,%d), want 4 from (%d,%d)", n, x, y, w-2, h-2)
	}
}
