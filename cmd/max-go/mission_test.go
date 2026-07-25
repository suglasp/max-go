// mission_test.go : tests mission.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/maxsave"
)

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
