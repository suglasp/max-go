// cursor_pick_test.go : pins the ONE rule that resolves a cell holding several units.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestStackedMobilesResolveTheSameForCursorAndClick verifies that hovering and clicking choose the same mobile unit from a shared cell, regardless of arrival order.
func TestStackedMobilesResolveTheSameForCursorAndClick(t *testing.T) {
	s, g := newTestScreen(t, 16)

	// Deployed high id first, so arrival order and id order disagree.
	late := mkUnit(g, assets.Tank, 0, 7, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	first := mkUnit(g, assets.Scout, 0, 3, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)

	if got := s.topUnitAt(5, 5); got != first {
		t.Fatalf("the cursor should name the lowest id unit (%d), got %v", first.ID, got)
	}

	s.clickCell(5, 5)

	if s.selected != first {
		t.Fatalf("the click should select the same unit the cursor named (%d), got %v", first.ID, s.selected)
	}

	// Drive the low id unit onto the cell again: it lands at the END of the cell bucket.
	// The answer must not move with it.
	g.Units.Move(first, 5, 5)

	if got := s.topUnitAt(5, 5); got != first {
		t.Errorf("re-arriving must not change the pick: want %d, got %v", first.ID, got)
	}

	// The click cycle still reaches the other unit; it just never wins the FIRST click.
	s.clickCell(5, 5)

	if s.selected != late {
		t.Errorf("a second click should cycle to the other stacked unit (%d), got %v", late.ID, s.selected)
	}
}

// TestOwnUnitWinsOverAnEnemyOnTheSameCell verifies the tie break prefers the unit the player
// can actually act on. An enemy sharing the cell (a surfaced submarine under your own boat)
// must never steal the click.
func TestOwnUnitWinsOverAnEnemyOnTheSameCell(t *testing.T) {
	s, g := newTestScreen(t, 16)

	enemy := mkUnit(g, assets.Tank, 0, 1, 6, 6, game.FlagMobileLandUnit|game.FlagSelectable)
	enemy.Team = 1
	g.Units.Move(enemy, 6, 6) // rebucket under its new team

	mine := mkUnit(g, assets.Scout, 0, 9, 6, 6, game.FlagMobileLandUnit|game.FlagSelectable)

	if got := s.topUnitAt(6, 6); got != mine {
		t.Errorf("your own unit must win the cell over an enemy's, even with the higher id; got %v", got)
	}

	s.clickCell(6, 6)

	if s.selected != mine {
		t.Errorf("the click should select your own unit, got %v", s.selected)
	}
}
