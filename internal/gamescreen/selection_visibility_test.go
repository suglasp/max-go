// selection_visibility_test.go : tests selected unit visibility rules under fog of war.
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

// TestSelectionVisibilityGate verifies the rule: a cell occupant is selectable only if
// the player can SEE it. A visible enemy is selectable (for inspection), a fogged
// enemy is not, the F2 omniscient reveal (fogShow off) lifts the gate, and the
// player's own units are always selectable.
func TestSelectionVisibilityGate(t *testing.T) {
	s, g := newTestScreen(t, 16)
	s.fogTeam = 0
	s.fogShow = true

	// A player scout reveals a small area around (5,5).
	seer := mkUnit(g, assets.Scout, 0, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	seer.Values.Set(game.AttribScan, 3)
	seer.Order = game.OrderAwait

	// A visible enemy (one tile from the scout) and a fogged enemy far away.
	// enemy tanks in an awaiting (not idle) order, so the fog model evaluates their visibility by scan
	// coverage rather than the idle early out
	nearEnemy := mkTank(g.Units, 1, 2, 6, 5, tankStats{})
	nearEnemy.Order = game.OrderAwait
	farEnemy := mkTank(g.Units, 1, 3, 13, 13, tankStats{})
	farEnemy.Order = game.OrderAwait
	g.RefreshVisibility()

	if !s.selectableOccupant(nearEnemy) {
		t.Error("a VISIBLE enemy should be selectable (for inspection)")
	}
	if s.selectableOccupant(farEnemy) {
		t.Error("a FOGGED enemy must NOT be selectable")
	}

	// The omniscient debug reveal (fog off) makes everything selectable.
	s.fogShow = false
	if !s.selectableOccupant(farEnemy) {
		t.Error("with fog off a fogged enemy is selectable (omniscient)")
	}
	s.fogShow = true

	// The player always sees its own units.
	if !s.selectableOccupant(seer) {
		t.Error("own units are always selectable")
	}

	// A click on a fogged enemy's cell must not select that invisible unit.
	s.selected = nil
	s.clickCell(13, 13)
	if s.selected == farEnemy {
		t.Error("clickCell must not select a fogged enemy unit")
	}
}
