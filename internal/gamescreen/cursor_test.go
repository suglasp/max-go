// cursor_test.go : tests cursor.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/game"
)

func TestPickCursor(t *testing.T) {
	const active = 0
	me := &game.UnitInfo{Team: active}
	mate := &game.UnitInfo{Team: active}
	foe := &game.UnitInfo{Team: 1}
	ownStruct := &game.UnitInfo{Team: active, Flags: game.FlagStationary}
	foeStruct := &game.UnitInfo{Team: 1, Flags: game.FlagStationary}

	tests := []struct {
		name string
		tgt  cellCursorTarget
		sel  *game.UnitInfo
		has  bool
		want cursors.ID
	}{
		// Nothing selected: friend over a friendly unit, no go everywhere else
		// (mirrors the engine's null selection branch, NOT hand).
		{"noSel/empty", cellCursorTarget{}, nil, false, cursors.UnitNoGo},
		{"noSel/friendly", cellCursorTarget{occupant: mate}, nil, false, cursors.Friend},
		{"noSel/enemy", cellCursorTarget{occupant: foe}, nil, false, cursors.UnitNoGo},
		// A controllable unit selected: action under the pointer wins.
		{"sel/attack", cellCursorTarget{canAttack: true, occupant: foe}, me, true, cursors.Enemy},
		{"sel/friendly", cellCursorTarget{occupant: mate}, me, true, cursors.Friend},
		{"sel/self", cellCursorTarget{occupant: me}, me, true, cursors.UnitNoGo},
		{"sel/enemy-out-of-range", cellCursorTarget{occupant: foe}, me, true, cursors.UnitNoGo},
		{"sel/move-go", cellCursorTarget{canEnter: true}, me, true, cursors.UnitGo},
		{"sel/move-nogo", cellCursorTarget{canEnter: false}, me, true, cursors.UnitNoGo},
		// A FRIENDLY passable structure the selected unit can enter (own connector /
		// landing pad, or any building an AIR unit can fly over) reads as drivable: the
		// move cursor. Drivable neutral terrain (road / bridge / slab) is ground cover,
		// excluded upstream, so it never arrives here as an occupant.
		{"sel/passable-structure", cellCursorTarget{occupant: ownStruct, canEnter: true}, me, true, cursors.UnitGo},
		// An ENEMY building an air unit could fly over (canEnter true) must NOT show the
		// green move cursor: out of range it is the far target (move to attack) cursor,
		// and with no shot to spare it is no go - never "fly onto it". This is the bomber
		// over enemy buildings.
		{"sel/enemy-structure-far", cellCursorTarget{occupant: foeStruct, canEnter: true, canFarTarget: true}, me, true, cursors.FarTarget},
		{"sel/enemy-structure-nogo", cellCursorTarget{occupant: foeStruct, canEnter: true}, me, true, cursors.UnitNoGo},
		// An impassable own building (canEnter false) is not drivable: friend cursor.
		{"sel/impassable-building", cellCursorTarget{occupant: ownStruct, canEnter: false}, me, true, cursors.Friend},
	}

	for _, tc := range tests {
		if got := pickCursor(tc.tgt, tc.sel, active, tc.has); got != tc.want {
			t.Errorf("%s: pickCursor = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestScrollArrow(t *testing.T) {
	cases := []struct {
		dx, dy int
		want   cursors.ID
	}{
		{0, -1, cursors.ArrowN}, {1, -1, cursors.ArrowNE}, {1, 0, cursors.ArrowE}, {1, 1, cursors.ArrowSE},
		{0, 1, cursors.ArrowS}, {-1, 1, cursors.ArrowSW}, {-1, 0, cursors.ArrowW}, {-1, -1, cursors.ArrowNW},
	}

	for _, c := range cases {
		if got := scrollArrow(c.dx, c.dy); got != c.want {
			t.Errorf("scrollArrow(%d,%d) = %d, want %d", c.dx, c.dy, got, c.want)
		}
	}
}

// TestSpectatorCursor checks the opponent turn cursor: the friend pointer over a
// unit the player can see, the no go pointer over an empty cell. An own team unit
// is always visible (no fog lookup needed), so it exercises the Friend branch.
func TestSpectatorCursor(t *testing.T) {
	s, g := newTestScreen(t, 16)
	s.fogTeam = 0 // the human's view; mkUnit deploys on team 0 (the player)
	mkUnit(g, assets.Tank, 0, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)

	// Over the player's own unit -> the friend (inspect) pointer.
	if got := s.spectatorCursor(5, 5); got != cursors.Friend {
		t.Errorf("over own unit = %d, want Friend %d", got, cursors.Friend)
	}

	// Over an empty cell -> the no go pointer (no order can be issued).
	if got := s.spectatorCursor(8, 8); got != cursors.UnitNoGo {
		t.Errorf("over empty cell = %d, want UnitNoGo %d", got, cursors.UnitNoGo)
	}
}
