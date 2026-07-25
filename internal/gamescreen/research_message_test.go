// research_message_test.go : tests the human player's research-level banner.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestResearchLevelUpPostsMessage: when a human team's research topic gains a level,
// advanceResearch posts an on screen message naming the topic + new level (vanilla
// ResearchMenu_NewTurn), beyond the single "Research complete" voice.
func TestResearchLevelUpPostsMessage(t *testing.T) {
	s := newResearchScreen() // team 0 = player
	// One turn from a level up: the allocation pushes turns to complete to <= 0.
	s.game.State.Teams[0].Research[game.ResearchAttack] = game.ResearchTopic{Level: 0, TurnsToComplete: 1, Allocation: 3}

	s.advanceResearch(0)

	if got := s.game.State.Teams[0].Research[game.ResearchAttack].Level; got != 1 {
		t.Fatalf("topic did not level up: level=%d", got)
	}
	if !strings.Contains(s.banner.msg, "Attack") || !strings.Contains(s.banner.msg, "level 1") {
		t.Errorf("research message = %q; want it to name the topic Attack + level 1", s.banner.msg)
	}
}

// TestResearchLevelUpSilentForEnemy: a non human team's level up posts no message
// (the player is not the audience).
func TestResearchLevelUpSilentForEnemy(t *testing.T) {
	s := newResearchScreen() // only team 0 is the player
	if s.isHumanTeam(1) {
		t.Skip("team 1 is unexpectedly human in this fixture")
	}
	s.game.State.Teams[1].Research[game.ResearchScan] = game.ResearchTopic{Level: 0, TurnsToComplete: 1, Allocation: 3}

	s.advanceResearch(1)

	if s.banner.msg != "" {
		t.Errorf("enemy research level-up posted a message: %q", s.banner.msg)
	}
}

// TestResearchCenterStatusShowsTurns: selecting a RUNNING research centre reports the
// team's soonest research advance - the next topic to level up and its turns (turns at
// the current allocation = ceil(TurnsToComplete / Allocation)).
func TestResearchCenterStatusShowsTurns(t *testing.T) {
	s := newResearchScreen()
	s.game.State.PlayerTeam = 0
	center := researchBld(0, 1, game.OrderPowerOn)

	// Two topics allocated; Scan completes soonest (8/2 = 4 turns vs Attack 30/1 = 30).
	s.game.State.Teams[0].Research[game.ResearchAttack] = game.ResearchTopic{TurnsToComplete: 30, Allocation: 1}
	s.game.State.Teams[0].Research[game.ResearchScan] = game.ResearchTopic{TurnsToComplete: 8, Allocation: 2}

	s.buildStatusMessage(center)

	if !strings.Contains(s.banner.msg, "Scan") || !strings.Contains(s.banner.msg, "4 turns") {
		t.Errorf("research centre status = %q; want it to name Scan + 4 turns", s.banner.msg)
	}
}

// TestResearchCenterStatusSilentWhenIdle: an idled centre (powered, no topic running)
// reports nothing - only a running (OrderPowerOn) centre shows the status.
func TestResearchCenterStatusSilentWhenIdle(t *testing.T) {
	s := newResearchScreen()
	s.game.State.PlayerTeam = 0
	idle := researchBld(0, 1, game.OrderIdle)
	s.game.State.Teams[0].Research[game.ResearchScan] = game.ResearchTopic{TurnsToComplete: 8, Allocation: 2}

	s.buildStatusMessage(idle)

	if s.banner.msg != "" {
		t.Errorf("an idle research centre posted a status: %q", s.banner.msg)
	}
}
