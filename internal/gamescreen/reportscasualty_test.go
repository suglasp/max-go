// reportscasualty_test.go : tests the Reports Casualties tab data.
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

// TestCasualtyRowsListOnlyLostTypes verifies that the Casualties tab has one row for each lost unit type, with team counts in separate columns and no rows before a loss.
func TestCasualtyRowsListOnlyLostTypes(t *testing.T) {
	s, g := newClickScreen(t)
	g.State.Teams[1].Type = game.TeamComputer

	teamOrder := []int{0, 1}

	if rows := s.casualtyRows(teamOrder); len(rows) != 0 {
		t.Fatalf("a game with no losses lists %d casualty rows, want none", len(rows))
	}

	// The player lost two tanks and a scout; the opponent lost one tank.
	g.TeamStats[0].Casualties = map[assets.UnitType]int{assets.Tank: 2, assets.Scout: 1}
	g.TeamStats[1].Casualties = map[assets.UnitType]int{assets.Tank: 1}

	rows := s.casualtyRows(teamOrder)

	if len(rows) != 2 {
		t.Fatalf("listed %d types, want 2 (Tank + Scout)", len(rows))
	}

	byName := map[string][]int{}
	for _, r := range rows {
		byName[r.Name] = r.Counts
	}

	tank := byName[displayName(assets.Tank)]
	if len(tank) != 2 || tank[0] != 2 || tank[1] != 1 {
		t.Errorf("Tank counts = %v, want [2 1] (player lost 2, opponent 1)", tank)
	}

	scout := byName[displayName(assets.Scout)]
	if len(scout) != 2 || scout[0] != 1 || scout[1] != 0 {
		t.Errorf("Scout counts = %v, want [1 0]", scout)
	}
}
