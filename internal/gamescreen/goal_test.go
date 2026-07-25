// goal_test.go : tests goal.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/winloss"
)

// TestChatLabelGoalVsChat checks the left panel caption flips to GOAL only when a
// goal text is set.
func TestChatLabelGoalVsChat(t *testing.T) {
	s, _ := simTestScreen()

	if got := s.chatLabel(); got != "CHAT" {
		t.Fatalf("no goal: chatLabel = %q, want CHAT", got)
	}

	s.SetGameGoal("Random game:\nType : Turn-based")
	if got := s.chatLabel(); got != "GOAL" {
		t.Fatalf("goal set: chatLabel = %q, want GOAL", got)
	}

	s.SetGameGoal("   ") // whitespace only is treated as empty
	if got := s.chatLabel(); got != "CHAT" {
		t.Fatalf("blank goal: chatLabel = %q, want CHAT", got)
	}
}

// TestBuildGameInfoText verifies the random game info block: team roles, the empty
// slots, the play mode, and the victory line.
func TestBuildGameInfoText(t *testing.T) {
	s, g := simTestScreen() // team0 Player, team1 Computer, team2/3 empty, simultaneous
	g.VictoryType = winloss.VictoryDuration
	g.VictoryLimit = 50

	text := s.buildGameInfoText()

	for _, want := range []string{
		"Random game:",
		"Red Team : Player",
		"Green Team : NPC",
		"Blue Team : -",
		"Gray Team : -",
		"Turns : 50",
		"Type : Simultaneous",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("info text missing %q\n--- got ---\n%s", want, text)
		}
	}
}

// TestSetRandomGameGoalEnablesGoal confirms building the info block from live
// state turns the button into GOAL mode.
func TestSetRandomGameGoalEnablesGoal(t *testing.T) {
	s, _ := simTestScreen()
	s.SetRandomGameGoal()

	if s.gameGoal == "" {
		t.Fatal("SetRandomGameGoal left the goal empty")
	}

	if got := s.chatLabel(); got != "GOAL" {
		t.Fatalf("chatLabel = %q, want GOAL after SetRandomGameGoal", got)
	}
}

// TestSetMissionGoal verifies the mission briefing goal: a title + description
// join with a blank line (C++: std::format), an empty description keeps CHAT,
// and a titleless briefing shows the briefing alone.
func TestSetMissionGoal(t *testing.T) {
	s, _ := simTestScreen()

	// Title + briefing: joined "title\n\ndescription", button lights GOAL.
	s.SetMissionGoal("Mission 1", "Objective: hold out 30 turns")
	if want := "Mission 1\n\nObjective: hold out 30 turns"; s.gameGoal != want {
		t.Fatalf("gameGoal = %q, want %q", s.gameGoal, want)
	}

	if got := s.chatLabel(); got != "GOAL" {
		t.Fatalf("chatLabel = %q, want GOAL for a mission with a briefing", got)
	}

	// No briefing -> stays the inert CHAT stub (C++: mission_description.empty()).
	s.SetMissionGoal("Mission 2", "   ")
	if s.gameGoal != "" {
		t.Fatalf("empty briefing left gameGoal = %q, want empty (CHAT)", s.gameGoal)
	}

	if got := s.chatLabel(); got != "CHAT" {
		t.Fatalf("chatLabel = %q, want CHAT for a mission with no briefing", got)
	}

	// Briefing but no title -> the briefing shows alone (no leading blank line).
	s.SetMissionGoal("", "Objective: survive")
	if want := "Objective: survive"; s.gameGoal != want {
		t.Fatalf("title-less gameGoal = %q, want %q", s.gameGoal, want)
	}
}
