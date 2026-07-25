// fog_save_test.go : fog of war memory save/load tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/vis"
)

// scoutAt deploys a scanning unit for team 0 and refreshes the fog from it.
func scoutAt(t *testing.T, g *sim.Game, x, y int) *game.UnitInfo {
	t.Helper()
	scout := game.NewUnitInfo(assets.Scout, 0, 7, 0)
	scout.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	scout.Values = &game.UnitValues{}
	scout.Values.Set(game.AttribScan, 3)
	scout.InitState()
	g.Units.Deploy(scout, x, y)
	g.RefreshVisibility()

	return scout
}

// TestSaveRestoreFogMemory: the scan coverage is re derived from the units on load, but
// the EXPLORED memory is not, so a load used to black out every cell the player had
// walked and driven away from. A cell explored before the save must come back explored.
func TestSaveRestoreFogMemory(t *testing.T) {
	g := newTestGame(t)
	scout := scoutAt(t, g, 12, 12)

	// Drive the scout home: (12,12) is now explored but no longer scanned.
	g.Units.Move(scout, 2, 2)
	g.RefreshVisibility()

	if got := g.Vis.Team(0).FogAt(12, 12); got != vis.FogExplored {
		t.Fatalf("before the save, fog at (12,12) = %v, want FogExplored (the test itself is wrong)", got)
	}

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	if got := g2.Vis.Team(0).FogAt(12, 12); got != vis.FogExplored {
		t.Errorf("after the load, fog at (12,12) = %v, want FogExplored: the map exploration was wiped", got)
	}
	if got := g2.Vis.Team(0).FogAt(2, 2); got != vis.FogVisible {
		t.Errorf("the scout's own cell after the load = %v, want FogVisible", got)
	}
	if got := g2.Vis.Team(0).FogAt(15, 0); got != vis.FogUnseen {
		t.Errorf("a never visited corner after the load = %v, want FogUnseen", got)
	}
	if got := g2.Vis.Team(1).FogAt(12, 12); got != vis.FogUnseen {
		t.Errorf("team 1 sees team 0's exploration at (12,12) = %v, want FogUnseen", got)
	}
}

// TestRestoreWithoutFogMemory checks the pre v22 path: a save with no bitmap restores the
// way it always did (the exploration is lost), rather than failing or reviving stale cells.
func TestRestoreWithoutFogMemory(t *testing.T) {
	g := newTestGame(t)
	scout := scoutAt(t, g, 12, 12)
	g.Units.Move(scout, 2, 2)
	g.RefreshVisibility()

	snap := Capture(g, "GREEN_1.WRL")
	snap.Explored = nil // an older save carries no fog memory

	g2 := snap.Restore(landMap(16, 16))
	if got := g2.Vis.Team(0).FogAt(12, 12); got != vis.FogUnseen {
		t.Errorf("a pre v22 save restored fog at (12,12) = %v, want FogUnseen (it never held the memory)", got)
	}
	if got := g2.Vis.Team(0).FogAt(2, 2); got != vis.FogVisible {
		t.Errorf("the scout's own cell = %v, want FogVisible", got)
	}
}
