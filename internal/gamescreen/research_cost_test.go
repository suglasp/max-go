// research_cost_test.go : tests the Cost research topic and HITS current HP scaling.
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

// costTurns mirrors the original's ResearchMenu_ApplyUpgrades COST result val(level): the
// build turns a unit with the given base ATTRIB_TURNS has at a Cost research level.
func costTurns(base, level int) int {
	t := ((level+10)/2 + base*10) / (level + 10)
	if t < 1 {
		t = 1
	}
	return t
}

// researchScreenWithAttribs returns a research screen whose ATTRIBS table sets the
// Tank (mobile) and Light Vehicle Plant (building) build turns + hit points, so
// teamValues has a real base to reduce.
func researchScreenWithAttribs(tankTurns, tankHits, pltTurns, pltHits int) *Screen {
	s := newResearchScreen()
	rows := make([]assets.UnitAttributes, assets.UnitCount)
	rows[assets.Tank] = assets.UnitAttributes{TurnsToBuild: tankTurns, HitPoints: tankHits}
	rows[assets.LightPlt] = assets.UnitAttributes{TurnsToBuild: pltTurns, HitPoints: pltHits}
	s.SetAttribs(rows)
	return s
}

// TestResearchCostReducesBuildTurns checks the Cost topic reduces ATTRIB_TURNS for a
// mobile unit AND a building per the original's formula, and that level 0 leaves the base.
func TestResearchCostReducesBuildTurns(t *testing.T) {
	s := researchScreenWithAttribs(12, 30, 8, 40)

	// Level 0 keeps the base turns (no Cost research yet).
	if got := s.teamValues(0, assets.Tank).Get(game.AttribTurns); got != 12 {
		t.Errorf("Tank turns at Cost level 0 = %d, want 12 (base)", got)
	}

	// A spot value pins the formula: base 12 at level 1 -> (11/2 + 120)/11 = 11.
	s.game.State.Teams[0].Research[game.ResearchCost].Level = 1
	if got := s.teamValues(0, assets.Tank).Get(game.AttribTurns); got != 11 {
		t.Errorf("Tank turns at Cost level 1 = %d, want 11", got)
	}

	// Higher levels keep shaving turns and match the formula for both a unit and a building.
	for _, level := range []int{2, 4, 8, 20} {
		s.game.State.Teams[0].Research[game.ResearchCost].Level = level
		if got, want := s.teamValues(0, assets.Tank).Get(game.AttribTurns), costTurns(12, level); got != want {
			t.Errorf("Tank turns at Cost level %d = %d, want %d", level, got, want)
		}
		if got, want := s.teamValues(0, assets.LightPlt).Get(game.AttribTurns), costTurns(8, level); got != want {
			t.Errorf("Light Vehicle Plant turns at Cost level %d = %d, want %d", level, got, want)
		}
	}

	// The combat topics are untouched by Cost research (a sanity guard against cross talk).
	if got := s.teamValues(0, assets.Tank).Get(game.AttribHits); got != 30 {
		t.Errorf("Tank hits with only Cost research = %d, want 30 (unchanged)", got)
	}
}

// TestResearchCostNeverBelowOne checks the floor: even a huge Cost level cannot drop a
// unit's build turns below 1, matching the original's std::max(new_value, 1).
func TestResearchCostNeverBelowOne(t *testing.T) {
	s := researchScreenWithAttribs(2, 30, 8, 40)
	s.game.State.Teams[0].Research[game.ResearchCost].Level = 100
	if got := s.teamValues(0, assets.Tank).Get(game.AttribTurns); got != 1 {
		t.Errorf("Tank turns at Cost level 100 = %d, want 1 (floor)", got)
	}
}

// TestUpgradeUnitValuesScalesHP checks HITS current HP scaling: a full unit whose max
// HITS rises stays full, and a damaged unit keeps its absolute missing HP gap.
func TestUpgradeUnitValuesScalesHP(t *testing.T) {
	s := researchScreenWithAttribs(12, 30, 8, 40)

	// A full HP Tank: Values + Hits both at the base max 30.
	full := &game.UnitInfo{Type: assets.Tank, Team: 0, Values: s.teamValues(0, assets.Tank)}
	full.Hits = full.Values.Get(game.AttribHits)
	// A damaged Tank: 7 HP below the base max.
	damaged := &game.UnitInfo{Type: assets.Tank, Team: 0, Values: s.teamValues(0, assets.Tank)}
	damaged.Hits = damaged.Values.Get(game.AttribHits) - 7

	// Bank two HITS research levels -> max rises to 30 + 2*3 = 36 (delta 6).
	s.game.State.Teams[0].Research[game.ResearchHits].Level = 2
	newMax := s.teamValues(0, assets.Tank).Get(game.AttribHits)
	if newMax != 36 {
		t.Fatalf("Tank max HITS at research level 2 = %d, want 36", newMax)
	}

	s.upgradeUnitValues(0, full)
	if full.Hits != newMax {
		t.Errorf("full unit Hits after upgrade = %d, want %d (stays full)", full.Hits, newMax)
	}

	s.upgradeUnitValues(0, damaged)
	if want := newMax - 7; damaged.Hits != want {
		t.Errorf("damaged unit Hits after upgrade = %d, want %d (gap preserved)", damaged.Hits, want)
	}
	if damaged.Hits > newMax {
		t.Errorf("damaged unit Hits %d exceeds new max %d", damaged.Hits, newMax)
	}
}
