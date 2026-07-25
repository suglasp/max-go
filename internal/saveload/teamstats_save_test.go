// teamstats_save_test.go : team statistics save/load tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/sim"
)

// TestSaveRestoreTeamStats checks that v10 end-game statistics survive a round trip,
// including the per unit type casualty tally the Reports screen lists. Older saves
// default these fields to zero (and to a nil casualty map).
func TestSaveRestoreTeamStats(t *testing.T) {
	g := newTestGame(t)
	g.TeamStats[0] = sim.TeamStats{
		FactoriesBuilt: 3, MinesBuilt: 2, BuildingsBuilt: 9,
		BuildingsLost: 1, UnitsBuilt: 14, UnitsLost: 5, GoldOnUpgrades: 120,
		Casualties: map[assets.UnitType]int{assets.Tank: 3, assets.Scout: 2},
	}
	g.TeamStats[1] = sim.TeamStats{UnitsBuilt: 7, UnitsLost: 2}

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	// TeamStats holds a map now, so compare deeply rather than with ==.
	if !reflect.DeepEqual(g2.TeamStats[0], g.TeamStats[0]) {
		t.Errorf("team-0 stats = %+v, want %+v", g2.TeamStats[0], g.TeamStats[0])
	}
	if !reflect.DeepEqual(g2.TeamStats[1], g.TeamStats[1]) {
		t.Errorf("team-1 stats = %+v, want %+v", g2.TeamStats[1], g.TeamStats[1])
	}

	if got := g2.TeamStats[0].Casualties[assets.Tank]; got != 3 {
		t.Errorf("restored Tank casualties = %d, want 3", got)
	}
}
