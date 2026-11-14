// mission_test.go : tests mission.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/maxsave"
	"github.com/suglasp/max-go/internal/missionfile"
	"github.com/suglasp/max-go/internal/testutil"
)

// TestMissionRulesCarryTheFileOptions checks that a mission takes its play mode, both timers
// and its opponent from the file's options block (N13), not from the player's settings.
func TestMissionRulesCarryTheFileOptions(t *testing.T) {
	gs := &maxsave.GameState{Options: maxsave.Options{Timer: 0, EndTurn: 45, PlayMode: 1, Opponent: 3}}

	r := missionRules(gs)
	if r.TurnTimer != 0 || r.EndTurn != 45 || r.PlayMode != 1 || r.Opponent != 3 {
		t.Errorf("rules: turn time %d, end turn %d, play mode %d, opponent %d; want 0, 45, 1, 3",
			r.TurnTimer, r.EndTurn, r.PlayMode, r.Opponent)
	}
}

// TestRetailTrainingRules reads the first retail training mission: simultaneous play, a 3:00
// turn, a 0:45 End Turn and opponent 3, as its file fixes them.
func TestRetailTrainingRules(t *testing.T) {
	path := filepath.Join(testutil.DataDir(t), "SAVE1.TRA")

	_, _, r, err := loadMissionState(path, 112, 112, &missionfile.Info{})
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}

	if r.PlayMode != 1 || r.TurnTimer != 180 || r.EndTurn != 45 || r.Opponent != 3 {
		t.Errorf("SAVE1.TRA: play mode %d, turn time %d, end turn %d, opponent %d; want 1, 180, 45, 3",
			r.PlayMode, r.TurnTimer, r.EndTurn, r.Opponent)
	}
}

// TestMissionUpgradeOffsetsSeparateResearch checks that only the gold-upgrade remainder becomes an offset. Runtime research levels supply the
// research portion, so research-only mission data produces no offset.
func TestMissionUpgradeOffsetsSeparateResearch(t *testing.T) {
	gs := &maxsave.GameState{}
	gs.Research[0][game.ResearchAttack].Level = 1 // attack researched to level 1

	base := &maxsave.UnitValues{Attack: 10, Hits: 20, Turns: 8}
	cur := &maxsave.UnitValues{Attack: 14, Hits: 20, Turns: 8} // researched 11 + 3 gold
	gs.BaseValues[0][int(assets.Tank)] = base
	gs.CurrentValues[0][int(assets.Tank)] = cur

	offs := missionUpgradeOffsets(gs, 0)
	off, ok := offs[assets.Tank]
	if !ok {
		t.Fatal("expected a Tank offset entry")
	}
	if off[game.AttribAttack] != 3 {
		t.Errorf("attack offset = %d, want 3 (14 minus the researched 11)", off[game.AttribAttack])
	}
	if off[game.AttribHits] != 0 || off[game.AttribTurns] != 0 {
		t.Errorf("unchanged attributes must carry no offset: hits=%d turns=%d", off[game.AttribHits], off[game.AttribTurns])
	}

	// Research only: current equals the researched baseline -> no offsets at all.
	gs.CurrentValues[0][int(assets.Tank)] = &maxsave.UnitValues{Attack: 11, Hits: 20, Turns: 8}
	if offs := missionUpgradeOffsets(gs, 0); offs != nil {
		t.Errorf("research only current values must yield no offsets, got %v", offs)
	}
}
