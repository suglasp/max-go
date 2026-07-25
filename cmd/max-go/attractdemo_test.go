// attractdemo_test.go : tests team controller mapping for attract mode.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/missionfile"
)

// TestDemoTeamTypesAllComputer locks the attract demo's team mapping: every
// active team in the header (player, computer or remote) plays AI driven, an
// absent slot stays TeamNone. A demo has no local player.
func TestDemoTeamTypesAllComputer(t *testing.T) {
	info := &missionfile.Info{}
	info.TeamType[0] = 1 // TEAM_TYPE_PLAYER in the file
	info.TeamType[1] = 2 // TEAM_TYPE_COMPUTER
	info.TeamType[2] = 3 // TEAM_TYPE_REMOTE
	info.TeamType[3] = 0 // TEAM_TYPE_NONE

	got := demoTeamTypes(info)

	want := []game.TeamType{game.TeamComputer, game.TeamComputer, game.TeamComputer, game.TeamNone}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("team %d = %v, want %v", i, got[i], w)
		}
	}
}
