// marquee.go : implements drag-box multi-unit selection and group orders.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

// marquee.go - the drag box that selects several units at once, ported from
// Access_MultiSelect / GameManager_RenderMultiSelectIndicator. A left drag
// over the open map rubber bands a yellow box; on release every own team mobile
// unit inside it (with a half tile inset, capped at maxGroupSize) becomes the
// selection, the prior leader staying in front when it fell in the box. A move or
// attack order issued to a group fans out in formation (each member keeps its
// offset to the leader), matching the original and MaxR.

import (
	"sort"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// dragBoxState tracks rubber band selection from the initial press through the active drag.
type dragBoxState struct {
	marqueeState         uint8 // drag box state: 0 idle, 1 pressed (no drag yet), 2 dragging (see marquee.go)
	marqueeAX, marqueeAY int   // the drag box anchor in screen (logical) pixels
	marqueeX, marqueeY   int   // the current drag box corner in screen (logical) pixels
}

// maxGroupSize caps a multi selection / control group, matching the 10 unit
// limit in Access_MultiSelect. marqueeThresholdPx is how far the pointer must
// travel before a press turns into a drag box (the original uses 16 on a 640 wide screen;
// the port uses a smaller value so a small drag still reads as a box, while a
// plain click stays a click). The half tile inset (16 of the 64 pixel tile) means
// a unit must sit substantially inside the box to be caught.
const (
	maxGroupSize       = 10
	marqueeThresholdPx = 8
	tileMapPx          = 64
	marqueeInsetMapPx  = 16
)

// trackMarquee runs the whole press/drag/release rubber band gesture each frame.
// It engages only on the open map with no build or armed order pick active; a
// committed drag returns true so handleMouse skips the release click. A press that
// never passes the drag threshold returns false, letting the normal click fire.
func (s *Screen) trackMarquee(mouse platform.MouseState) bool {
	eligible := s.turnFlow.handover == -1 && !s.gameOver && !s.buildUI.buildMenuOpen && !s.buildUI.buildPath &&
		s.buildUI.buildIdx == 0 && !s.debugPlaceArmed() && !s.orderInput.xferArmed && !s.orderInput.attackArmed &&
		!s.orderInput.infiltrateArmed && !s.orderInput.reloadArmed && !s.orderInput.repairArmed &&
		!(s.selected != nil && s.game.IsBuildReady(s.selected))

	if !eligible {
		s.dragBox.marqueeState = 0

		return false
	}

	held := mouse.Buttons&platform.MousePressLeft != 0
	release := mouse.Buttons&platform.MouseReleaseLeft != 0

	switch s.dragBox.marqueeState {
	case 0: // idle: start tracking on a fresh press inside the map
		if held {
			if _, _, ok := s.mapCell(mouse.X, mouse.Y); ok {
				s.dragBox.marqueeState = 1
				s.dragBox.marqueeAX, s.dragBox.marqueeAY = mouse.X, mouse.Y
				s.dragBox.marqueeX, s.dragBox.marqueeY = mouse.X, mouse.Y
			}
		}

	case 1: // pressed, waiting to pass the drag threshold
		if !held {
			s.dragBox.marqueeState = 0 // a plain click: let clickCell handle the release

			return false
		}

		s.dragBox.marqueeX, s.dragBox.marqueeY = mouse.X, mouse.Y

		if iabsInt(mouse.X-s.dragBox.marqueeAX) > marqueeThresholdPx || iabsInt(mouse.Y-s.dragBox.marqueeAY) > marqueeThresholdPx {
			s.dragBox.marqueeState = 2
		}

	case 2: // dragging the box
		s.dragBox.marqueeX, s.dragBox.marqueeY = mouse.X, mouse.Y

		if !held || release {
			s.commitMarquee()
			s.dragBox.marqueeState = 0

			return true // consume the release so clickCell does not also fire
		}

		return true // consume the drag frames
	}

	return false
}

// commitMarquee selects every own team mobile unit inside the finished box. It
// ports Access_MultiSelect: convert the box to world pixels, inset a half tile on
// each side, then take the units whose grid cell lands inside. An empty box
// deselects (the original clears when nothing is caught). C++ ported : access.cpp - Access_MultiSelect.
func (s *Screen) commitMarquee() {
	team := s.game.State.ActiveTeam

	ax, ay := s.mapPixel(s.dragBox.marqueeAX, s.dragBox.marqueeAY)
	bx, by := s.mapPixel(s.dragBox.marqueeX, s.dragBox.marqueeY)
	ulx := (min(ax, bx) + marqueeInsetMapPx) / tileMapPx
	uly := (min(ay, by) + marqueeInsetMapPx) / tileMapPx
	lrx := (max(ax, bx) - marqueeInsetMapPx) / tileMapPx
	lry := (max(ay, by) - marqueeInsetMapPx) / tileMapPx

	inBox := func(u *game.UnitInfo) bool {
		return u.GridX >= ulx && u.GridX <= lrx && u.GridY >= uly && u.GridY <= lry
	}

	var set []*game.UnitInfo

	for _, u := range s.game.Units.Units() {
		// Own mobile units only, not a build posed constructor and not a unit set to
		// guard (Sentry), and only what the player can actually see and select.
		if u.Team != team || !isMobileUnit(u) || u.Building || u.Order == game.OrderSentry {
			continue
		}

		if !s.selectableOccupant(u) || !inBox(u) {
			continue
		}

		set = append(set, u)
	}

	// Units() returns random map order; sort so the leader fallback and group order
	// are deterministic.
	sort.Slice(set, func(i, j int) bool { return set[i].ID < set[j].ID })

	// SHIFT + drag ADDS the boxed units to the current selection (an empty shift box
	// leaves the selection alone), matching the shift click add. C++: GameManager_
	// UpdateSelection with the modifier held.
	if s.orderInput.shiftMod {
		if len(set) == 0 {
			return
		}

		leader := s.selected
		if leader == nil {
			leader = set[0]
		}

		s.setSelectionSet(append(append([]*game.UnitInfo{}, s.selectionSet()...), set...), leader)

		return
	}

	if len(set) == 0 {
		s.clearSelected() // an empty box deselects, like Access_MultiSelect with nothing caught

		return
	}

	leader := s.selected
	if leader == nil || leader.Team != team || !inBox(leader) {
		leader = set[0] // keep the prior selection as leader only if it fell in the box
	}

	s.setSelectionSet(set, leader)
}

// mapPixel converts a screen (logical) pixel in the map viewport to a world pixel
// (tileMapPx per tile), the space Access_MultiSelect boxes in. The point is clamped
// to the viewport so a drag past the edge still gives sane bounds, and the result
// to the world extent.
func (s *Screen) mapPixel(px, py int) (int, int) {
	z := s.r.Zoom()
	px = clampInt(px, MainMap.ULX, MainMap.LRX)
	py = clampInt(py, MainMap.ULY, MainMap.LRY)
	wx := (s.camera.scrollX + px - MainMap.ULX) * tileMapPx / z
	wy := (s.camera.scrollY + py - MainMap.ULY) * tileMapPx / z

	return clampInt(wx, 0, s.wmap.W*tileMapPx-1), clampInt(wy, 0, s.wmap.H*tileMapPx-1)
}

// drawMarquee outlines the active drag box in yellow while dragging. The view sub
// buffer is already offset by MainMap.ULX/ULY, so the box is drawn in viewport
// local pixels, clipped to the view.
// C++ ported : game_manager.cpp - GameManager_RenderMultiSelectIndicator (4 yellow lines).
func (s *Screen) drawMarquee(view *gfx.Buffer) {
	if s.dragBox.marqueeState != 2 {
		return
	}

	x0 := clampInt(min(s.dragBox.marqueeAX, s.dragBox.marqueeX)-MainMap.ULX, 0, view.W-1)
	y0 := clampInt(min(s.dragBox.marqueeAY, s.dragBox.marqueeY)-MainMap.ULY, 0, view.H-1)
	x1 := clampInt(max(s.dragBox.marqueeAX, s.dragBox.marqueeX)-MainMap.ULX, 0, view.W-1)
	y1 := clampInt(max(s.dragBox.marqueeAY, s.dragBox.marqueeY)-MainMap.ULY, 0, view.H-1)

	boxClipped(view, x0, y0, x1-x0+1, y1-y0+1, gfx.ColorYellow)
}

// orderGroupMoveTo fans a move order out across the selection in formation: every
// member keeps its offset to the leader, so the group translates rigidly to the
// destination (clamped to the world). Issued immediately, without the single unit
// planning bulb. C++ ported : access.cpp - Access_GroupAttackOrder (the move branch);
// MaxR sendStartGroupMoveAction uses the same destination minus leader vector.
func (s *Screen) orderGroupMoveTo(destX, destY int) {
	sel := s.selectionSet()
	if len(sel) == 0 {
		return
	}

	leader := s.selected
	if leader == nil {
		leader = sel[0]
	}

	vecX, vecY := destX-leader.GridX, destY-leader.GridY

	// Pull out FRONT FIRST: the unit nearest the destination leads and the rest follow in
	// turn, each held a few frames longer than the one ahead of it. Without the stagger the
	// whole formation lurched into motion on a single frame, which reads as a blob sliding
	// across the map rather than a column moving off.
	order := append([]*game.UnitInfo(nil), sel...)
	sort.SliceStable(order, func(i, j int) bool {
		di := sqGridDist(order[i].GridX, order[i].GridY, destX, destY)
		dj := sqGridDist(order[j].GridX, order[j].GridY, destX, destY)
		if di != dj {
			return di < dj
		}

		return order[i].ID < order[j].ID // a stable order for units the same distance out
	})

	moved := 0

	for _, u := range order {
		if !canTakeMoveOrder(u) || u.Team != s.game.State.ActiveTeam {
			continue
		}

		tx := clampInt(u.GridX+vecX, 0, s.wmap.W-1)
		ty := clampInt(u.GridY+vecY, 0, s.wmap.H-1)
		s.executeMove(u, tx, ty)
		s.game.SetMoveDelay(u, moved*groupDepartStagger)
		moved++
	}
}

// groupDepartStagger is how many animation frames each unit of a group waits beyond the one
// in front of it before setting off. Small on purpose: enough that a column visibly leaves in
// order, not so much that a big selection takes a noticeable while to get going.
const groupDepartStagger = 4

// sqGridDist is the squared cell distance, used to rank a group's units by how close they
// already are to where the group is going.
func sqGridDist(ax, ay, bx, by int) int {
	dx, dy := ax-bx, ay-by

	return dx*dx + dy*dy
}

// orderGroupAttack sends the whole group at the enemy cell: a member in range and
// with line of sight FIRES in place, a member out of range but able to close moves
// to attack (pursues), and any remaining mover advances toward the cell in formation.
// Used for both an in range and a far target click on an enemy when a group is
// selected. C++ ported : access.cpp - Access_GroupAttackOrder.
func (s *Screen) orderGroupAttack(cellX, cellY int) {
	sel := s.selectionSet()
	if len(sel) == 0 {
		return
	}

	leader := s.selected
	if leader == nil {
		leader = sel[0]
	}

	vecX, vecY := cellX-leader.GridX, cellY-leader.GridY
	fired, pursued := false, false

	for _, u := range sel {
		if s.gameOver { // a kill in this loop can end the match
			return
		}

		if u.Team != s.game.State.ActiveTeam {
			continue
		}

		switch {
		case s.game.CanAttack(u, cellX, cellY) && s.canSeeTargetAt(u, cellX, cellY):
			s.attackWith(u, cellX, cellY, false)
			fired = true

		case s.game.CanFarTarget(u, cellX, cellY):
			s.game.OrderMoveToAttack(u, cellX, cellY)
			pursued = true

		case canTakeMoveOrder(u): // a non attacker still advances with the group
			tx := clampInt(u.GridX+vecX, 0, s.wmap.W-1)
			ty := clampInt(u.GridY+vecY, 0, s.wmap.H-1)
			s.executeMove(u, tx, ty)
		}
	}

	if pursued && !fired {
		s.showMessage("Moving to attack.")
	}
}

// friendlySelectableAt returns an own team mobile unit the player can select at
// the cell (nil if none), the target of a SHIFT/CTRL add/remove click.
func (s *Screen) friendlySelectableAt(cellX, cellY int) *game.UnitInfo {
	for _, u := range s.game.Units.Cells.At(cellX, cellY) {
		if u.Team == s.game.State.ActiveTeam && isMobileUnit(u) && !u.Building && s.selectableOccupant(u) {
			return u
		}
	}

	return nil
}

// toggleSelection adds u to the selection if it is not in it, or drops it if it is
// (SHIFT/CTRL click). Removing the leader promotes the next member; removing the
// last member clears the selection. C++: GameManager_UpdateSelection.
func (s *Screen) toggleSelection(u *game.UnitInfo) {
	cur := s.selectionSet()

	for i, e := range cur {
		if e != u {
			continue
		}

		rest := append(append([]*game.UnitInfo{}, cur[:i]...), cur[i+1:]...)
		leader := s.selected

		if leader == u { // promote a new leader (or clear when nothing is left)
			if len(rest) > 0 {
				leader = rest[0]
			} else {
				leader = nil
			}
		}

		s.setSelectionSet(rest, leader)

		return
	}

	leader := s.selected
	if leader == nil {
		leader = u
	}

	s.setSelectionSet(append(append([]*game.UnitInfo{}, cur...), u), leader)
}
