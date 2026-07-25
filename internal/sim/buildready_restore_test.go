// buildready_restore_test.go : tests build ready state restoration after save/load.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestRestoredNPCBuilderAutoExits verifies that BeginTeamTurn derives a new exit for a restored computer builder parked on its completed structure.
// buildExits is not persisted, and computer builders have no player to select a BLDMRK marker.
func TestRestoredNPCBuilderAutoExits(t *testing.T) {
	g := newGame(t, 16, 16)
	g.State.Teams[1].Type = game.TeamComputer

	// A computer build ready engineer parked on a just finished structure, as a load
	// leaves it: the build ready record is present, but buildExits is empty.
	eng := game.NewUnitInfo(assets.Engineer, 1, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.InitState()
	g.Units.Deploy(eng, 7, 7)
	structure := game.NewUnitInfo(assets.PowGen, 1, 2, 0)
	structure.Flags = game.FlagStationary
	structure.Values = &game.UnitValues{}
	g.Units.Deploy(structure, 7, 7)
	structure.HiddenUntilExpand = true
	g.RestoreBuildReady(eng, structure, assets.PowGen, 7, 7, 1)

	if !g.IsBuildReady(eng) {
		t.Fatal("setup: engineer should be build ready")
	}

	g.BeginTeamTurn(1) // the computer team's turn must automatically drive the parked builder off

	if g.IsBuildReady(eng) {
		t.Error("restored NPC builder still build ready after its turn began - no automatic exit was re-derived")
	}
}
