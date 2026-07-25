// loadselect_test.go : tests initial builder selection after loading a game.
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

// TestSelectFirstBuilderOnLoad: selectFirstBuilder picks the player's lowest ID builder
// (the founder), preferring a builder over any other unit regardless of deploy order, and
// selects nothing when the team has no builder.
func TestSelectFirstBuilderOnLoad(t *testing.T) {
	s, g := newClickScreen(t)

	mkUnit(g, assets.Tank, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable) // not a builder
	eng := mkUnit(g, assets.Engineer, 3, 6, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	mkUnit(g, assets.Constrct, 9, 7, 5, game.FlagMobileLandUnit|game.FlagSelectable) // higher id

	s.selected = nil
	s.selectFirstBuilder()
	if s.selected != eng {
		got := "nil"
		if s.selected != nil {
			got = displayName(s.selected.Type)
		}
		t.Errorf("load selected %s; want the lowest-ID builder (Engineer id 3)", got)
	}

	// A team with no builder ends with no selection.
	s2, g2 := newClickScreen(t)
	mkUnit(g2, assets.Tank, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	s2.selected = nil
	s2.selectFirstBuilder()
	if s2.selected != nil {
		t.Errorf("a no-builder team selected %v; want nil", s2.selected.Type)
	}
}
