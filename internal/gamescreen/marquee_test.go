// marquee_test.go : tests drag box multi selection, shift toggle and group orders.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// boxScreenX / boxScreenY return the screen pixels of a cell centre at scroll 0
// (the inverse of mapCell) so a test can aim the drag box at a known cell range.
// The two axes use their own viewport origin (ULX vs ULY).
func boxScreenX(s *Screen, cell int) int { return cell*s.r.Zoom() + MainMap.ULX + s.r.Zoom()/2 }
func boxScreenY(s *Screen, cell int) int { return cell*s.r.Zoom() + MainMap.ULY + s.r.Zoom()/2 }

// setMarqueeBox aims the drag box from cell (ax,ay) to (bx,by) and commits it.
func setMarqueeBox(s *Screen, ax, ay, bx, by int) {
	s.dragBox.marqueeAX, s.dragBox.marqueeAY = boxScreenX(s, ax), boxScreenY(s, ay)
	s.dragBox.marqueeX, s.dragBox.marqueeY = boxScreenX(s, bx), boxScreenY(s, by)
	s.commitMarquee()
}

func mkMobile(g *sim.Game, team, id, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	g.Units.Deploy(u, gx, gy)

	return u
}

// TestMarqueeSelectsOwnMobileUnitsInBox: a drag box takes the player's mobile units
// inside it and leaves out an enemy, a building and units outside the box.
func TestMarqueeSelectsOwnMobileUnitsInBox(t *testing.T) {
	s, g := newClickScreen(t)

	a := mkMobile(g, 0, 1, 3, 3) // three own mobile units inside the box
	b := mkMobile(g, 0, 2, 4, 4)
	c := mkMobile(g, 0, 3, 5, 5)
	mkMobile(g, 0, 4, 12, 12)                                                                      // own, but outside the box
	mkMobile(g, 1, 5, 4, 5)                                                                        // enemy inside: excluded
	mkUnit(g, assets.PowerStn, 6, 5, 4, game.FlagStationary|game.FlagBuilding|game.FlagSelectable) // building: excluded

	setMarqueeBox(s, 2, 2, 6, 6)

	got := map[*game.UnitInfo]bool{}
	for _, u := range s.selectionSet() {
		got[u] = true
	}

	if len(s.selectionSet()) != 3 || !got[a] || !got[b] || !got[c] {
		t.Fatalf("box should select the 3 own mobile units inside; got %d units", len(s.selectionSet()))
	}

	if s.selected == nil || !got[s.selected] {
		t.Errorf("the leader must be one of the boxed units, got %v", s.selected)
	}
}

// TestMarqueeCapsAtTen: a box over 12 units yields at most maxGroupSize (10).
func TestMarqueeCapsAtTen(t *testing.T) {
	s, g := newClickScreen(t)

	for i := 0; i < 12; i++ {
		mkMobile(g, 0, i+1, 2+i%4, 2+i/4)
	}

	setMarqueeBox(s, 1, 1, 7, 7)

	if n := len(s.selectionSet()); n != maxGroupSize {
		t.Fatalf("selection should cap at %d, got %d", maxGroupSize, n)
	}
}

// TestMarqueeEmptyBoxClears: dragging an empty box deselects (Access_MultiSelect
// clears when nothing is caught).
func TestMarqueeEmptyBoxClears(t *testing.T) {
	s, g := newClickScreen(t)

	sel := mkMobile(g, 0, 1, 3, 3)
	s.selected = sel

	setMarqueeBox(s, 9, 9, 12, 12) // empty region

	if s.selected != nil || len(s.selection) != 0 {
		t.Fatalf("an empty box should clear the selection, got selected=%v selection=%d", s.selected, len(s.selection))
	}
}

// TestTrackMarqueeThreshold: a press that moves past the threshold becomes a drag
// and commits on release; a press that never moves stays a click (no marquee).
func TestTrackMarqueeThreshold(t *testing.T) {
	s, g := newClickScreen(t)
	s.turnFlow.handover = -1 // a live human turn: marquee eligible
	a := mkMobile(g, 0, 1, 3, 3)

	ax, ay := boxScreenX(s, 2), boxScreenY(s, 2)
	bx, by := boxScreenX(s, 6), boxScreenY(s, 6)

	// Press, drag past the threshold, release: the box commits.
	s.trackMarquee(platform.MouseState{X: ax, Y: ay, Buttons: platform.MousePressLeft})
	if s.dragBox.marqueeState != 1 {
		t.Fatalf("press should enter state 1, got %d", s.dragBox.marqueeState)
	}

	s.trackMarquee(platform.MouseState{X: bx, Y: by, Buttons: platform.MousePressLeft})
	if s.dragBox.marqueeState != 2 {
		t.Fatalf("a drag past the threshold should enter state 2, got %d", s.dragBox.marqueeState)
	}

	consumed := s.trackMarquee(platform.MouseState{X: bx, Y: by, Buttons: platform.MouseReleaseLeft})
	if !consumed || s.dragBox.marqueeState != 0 {
		t.Fatalf("release should commit and consume; consumed=%v state=%d", consumed, s.dragBox.marqueeState)
	}

	if len(s.selectionSet()) == 0 || s.selected != a {
		t.Errorf("the committed drag should select the boxed unit, got %v", s.selected)
	}

	// A press with no movement is a plain click: no marquee, release not consumed.
	s.clearSelected()
	s.trackMarquee(platform.MouseState{X: ax, Y: ay, Buttons: platform.MousePressLeft})
	consumed = s.trackMarquee(platform.MouseState{X: ax + 2, Y: ay, Buttons: platform.MouseReleaseLeft})
	if consumed || s.dragBox.marqueeState != 0 {
		t.Errorf("a sub threshold click must not marquee; consumed=%v state=%d", consumed, s.dragBox.marqueeState)
	}
}

// TestShiftClickTogglesSelection: shift click adds a friendly to the group and a
// second shift click removes it again.
func TestShiftClickTogglesSelection(t *testing.T) {
	s, g := newClickScreen(t)
	a := mkMobile(g, 0, 1, 3, 3)
	b := mkMobile(g, 0, 2, 6, 6)

	s.selected = a
	s.orderInput.shiftMod = true

	s.clickCell(6, 6) // shift click B: add
	if len(s.selectionSet()) != 2 {
		t.Fatalf("shift click should add B to the selection, got %d units", len(s.selectionSet()))
	}

	s.clickCell(6, 6) // shift click B again: drop
	set := s.selectionSet()
	if len(set) != 1 || set[0] != a {
		t.Fatalf("a second shift click should drop B, leaving only A; got %d units leader=%v", len(set), s.selected)
	}

	_ = b
}

// TestGroupMoveFormation: a group move translates every member by the leader's
// move vector (member_dest = member_origin + (leader_dest - leader_origin)).
func TestGroupMoveFormation(t *testing.T) {
	s, g := newClickScreen(t)
	a := mkMobile(g, 0, 1, 5, 5) // leader
	b := mkMobile(g, 0, 2, 7, 8) // member, offset (+2,+3) from the leader
	a.Speed, b.Speed = 10, 10

	s.setSelectionSet([]*game.UnitInfo{a, b}, a)
	s.orderGroupMoveTo(10, 5) // leader vector (+5, 0)

	if d, ok := moveDestGS(g, a); !ok || d.X != 10 || d.Y != 5 {
		t.Fatalf("leader should move to (10,5), got %v ok=%v", d, ok)
	}

	if d, ok := moveDestGS(g, b); !ok || d.X != 12 || d.Y != 8 {
		t.Fatalf("member should keep its offset -> (12,8), got %v ok=%v", d, ok)
	}
}

// TestShiftDragAppendsToSelection: SHIFT + drag box adds the boxed units to the
// current selection instead of replacing it.
func TestShiftDragAppendsToSelection(t *testing.T) {
	s, g := newClickScreen(t)
	a := mkMobile(g, 0, 1, 2, 2)
	b := mkMobile(g, 0, 2, 5, 5) // both within the visible viewport at scroll 0

	s.selected = a               // a single unit selected
	s.orderInput.shiftMod = true // shift held during the drag
	setMarqueeBox(s, 4, 4, 6, 6) // box over B only (A is outside it)

	got := map[*game.UnitInfo]bool{}
	for _, u := range s.selectionSet() {
		got[u] = true
	}

	if len(s.selectionSet()) != 2 || !got[a] || !got[b] {
		t.Fatalf("shift+drag should add B to the A selection, got %d units", len(s.selectionSet()))
	}

	// An empty shift box leaves the selection alone.
	setMarqueeBox(s, 0, 0, 1, 1)
	if len(s.selectionSet()) != 2 {
		t.Errorf("an empty shift box must keep the selection, got %d units", len(s.selectionSet()))
	}
}

// TestInRangeGroupFire: a click on an in range enemy with a group selected makes
// every attack capable member fire (the target takes more than a single shot).
func TestInRangeGroupFire(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	a := clickTestUnit(0, 1, 5, 5, mgr) // two player attackers, both in range 4
	b := clickTestUnit(0, 2, 6, 5, mgr)
	enemy := clickTestUnit(1, 3, 8, 5, mgr)

	s := &Screen{game: g}
	s.setSelectionSet([]*game.UnitInfo{a, b}, a)

	start := enemy.Hits
	s.clickCell(8, 5)

	if enemy.Hits > start-9 { // one 8-damage shot alone could not drop it this far
		t.Fatalf("group fire should land two shots on the enemy; hits %d, start %d", enemy.Hits, start)
	}
}

// moveDestGS returns the final cell of u's issued move order, or false if none.
func moveDestGS(g *sim.Game, u *game.UnitInfo) (game.Point, bool) {
	for _, mo := range g.MoveOrders() {
		if mo.Unit == u && len(mo.Steps) > 0 {
			return mo.Steps[len(mo.Steps)-1].To, true
		}
	}

	return game.Point{}, false
}

// TestGroupMoveStaggersTheDeparture pins the pull out order: a group used to lurch into motion
// on a single frame, every unit at once, which reads as a blob sliding across the map. The
// unit nearest the destination now leads and the rest follow a few frames apart, so the
// column visibly leaves in order. The formation itself is unchanged: each unit still keeps
// its offset to the leader.
func TestGroupMoveStaggersTheDeparture(t *testing.T) {
	s, g := newClickScreen(t)

	// Three units in a line, all bound east. The rear one is furthest from the destination.
	front := mkUnit(g, assets.Tank, 1, 6, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	middle := mkUnit(g, assets.Tank, 2, 4, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	rear := mkUnit(g, assets.Tank, 3, 2, 5, game.FlagMobileLandUnit|game.FlagSelectable)

	for _, u := range []*game.UnitInfo{front, middle, rear} {
		u.Values.Set(game.AttribSpeed, 12)
		u.Values.Set(game.AttribHits, 30)
		u.InitState()
	}

	s.selected = front
	s.selection = []*game.UnitInfo{front, middle, rear}

	s.orderGroupMoveTo(12, 5)

	delays := map[*game.UnitInfo]int{}

	for _, o := range g.MoveOrders() {
		delays[o.Unit] = o.Delay
	}

	if len(delays) != 3 {
		t.Fatalf("the group did not all take a move order: %d of 3 moving", len(delays))
	}

	if delays[front] != 0 {
		t.Errorf("the unit nearest the destination should lead with no delay, got %d frames", delays[front])
	}

	if !(delays[front] < delays[middle] && delays[middle] < delays[rear]) {
		t.Errorf("the column did not pull out front first: front=%d middle=%d rear=%d",
			delays[front], delays[middle], delays[rear])
	}
}
