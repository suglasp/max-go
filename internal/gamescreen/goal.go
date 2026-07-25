// goal.go : implements the single-player GOAL mode behind the left-panel CHAT button.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : game_manager.cpp - GameManager_MenuInitButtons / GameManager_MenuClickChatGoalButton
package gamescreen

import (
	"fmt"
	"strings"

	"github.com/suglasp/max-go/internal/ai"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// SetMessageFunc installs the modal message hook for GOAL and Alt+P. Without it, the GOAL button remains inert.
func (s *Screen) SetMessageFunc(fn func(text string)) {
	s.hooks.messageFn = fn
}

// SetGameGoal changes the left-panel CHAT slot into GOAL when text is present. Missions supply title and description; random and custom games use SetRandomGameGoal.
func (s *Screen) SetGameGoal(text string) {
	s.gameGoal = strings.TrimSpace(text)
}

// SetRandomGameGoal builds the random or custom game info block from the live game state.
func (s *Screen) SetRandomGameGoal() {
	s.SetGameGoal(s.buildGameInfoText())
}

// SetMissionGoal lights the GOAL button for a scripted mission: it shows the
// mission title followed by its briefing (the retail DESCR<N> file, e.g.
// "Objective: ... / Planet: ... / Clan: ... / Notes: ..."). An empty briefing
// leaves the inert CHAT stub - the original keeps CHAT when a mission carries no
// description. The host calls it after MissionStart for a campaign / scenario /
// training mission.
// C++ ported : game_manager.cpp - GameManager_MenuClickChatGoalButton (title + "\n\n" + description)
func (s *Screen) SetMissionGoal(title, description string) {
	description = strings.TrimSpace(description)
	// Keep the two halves: a save carries them (v22) so a reloaded mission puts its own
	// objective back on the GOAL button instead of the "Random game:" block a skirmish gets.
	s.missionTitle, s.missionBriefing = strings.TrimSpace(title), description

	if description == "" {
		s.SetGameGoal("") // no briefing -> stay CHAT (the original: mission_description.empty())

		return
	}

	if title = strings.TrimSpace(title); title == "" {
		s.SetGameGoal(description) // briefing only, no title header

		return
	}

	s.SetGameGoal(title + "\n\n" + description)
}

// chatLabel is the left panel button caption: GOAL while a goal is set, else CHAT.
func (s *Screen) chatLabel() string {
	if s.gameGoal != "" {
		return "GOAL"
	}

	return "CHAT"
}

// buildGameInfoText assembles the random/custom game info block shown by the GOAL dialog: AI difficulty, one line per team colour (Player / NPC / "-" plus the resolved faction),
// the victory condition, and the play mode. It reads the live resolved state so it is correct for every start path (random, random2, custom).
func (s *Screen) buildGameInfoText() string {
	var b strings.Builder
	b.WriteString("Random game:\n\n")

	if s.ai != nil {
		b.WriteString(goalInfoLine("AI Difficulty", ai.DifficultyName(s.ai.Difficulty())) + "\n")
	}

	for team := 0; team < game.TeamCount; team++ {
		b.WriteString(s.teamGoalLine(team) + "\n")
	}

	b.WriteString(s.victoryGoalLine() + "\n")
	mode := "Turn based"

	if s.game.State.PlayMode == game.PlaySimultaneous {
		mode = "Simultaneous"
	}

	b.WriteString(goalInfoLine("Type", mode))

	return b.String()
}

// teamGoalLine formats one "<Colour> Team : <Player|NPC> [<faction>]" row, or "<Colour> Team : -" for a slot not in play.
func (s *Screen) teamGoalLine(team int) string {
	label := s.teamName(team) + " Team"
	var who string

	switch s.game.State.Teams[team].Type {
	case game.TeamPlayer:
		who = "Player"
	case game.TeamComputer:
		who = "NPC"
	default:
		return goalInfoLine(label, "-") // empty slot
	}

	if faction := s.factionName(team); faction != "" {
		who += " [" + faction + "]"
	}

	return goalInfoLine(label, who)
}

// factionName returns a team's resolved clan (faction) display name, or "" when the clan is unresolved/out of range so the caller omits the bracket.
func (s *Screen) factionName(team int) string {
	clan := s.game.State.Teams[team].Clan

	if clan >= 1 && clan <= len(s.buildUI.clans) {
		return s.buildUI.clans[clan-1].Name
	}

	return ""
}

// victoryGoalLine formats the win condition row: a turn cap, a score target, or '-' when neither is set.
func (s *Screen) victoryGoalLine() string {
	switch {
	case s.game.VictoryType == winloss.VictoryDuration && s.game.VictoryLimit > 0:
		return goalInfoLine("Turns", fmt.Sprintf("%d", s.game.VictoryLimit))
	case s.game.VictoryType == winloss.VictoryScore && s.game.VictoryLimit > 0:
		return goalInfoLine("Score", fmt.Sprintf("%d", s.game.VictoryLimit))
	default:
		return goalInfoLine("Turns", "-")
	}
}

// goalInfoLine formats one "label : value" info row. The dialog renderer wraps
// words and collapses runs of spaces, so column padding would not survive - a single separator keeps each row on its own line and readable.
func goalInfoLine(label, value string) string {
	return label + " : " + value
}
