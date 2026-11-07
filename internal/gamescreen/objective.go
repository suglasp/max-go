// objective.go : hands the running game the objective of the mission it is playing.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : missionmanager.cpp - MissionManager::SetupMission (the win/loss handler)
package gamescreen

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/objectives"
	"github.com/suglasp/max-go/internal/sim"
)

// SetMissionIndex records the mission_index from the save header (SAVE3.CAM is campaign mission 3). Mission victory and loss conditions use this identity, while 0 selects generic rules.
// Call it before MissionStart. Saved games retain the index and its objective.
//
// C++ ported : saveload.cpp - the mission_index field of the V70 header
func (s *Screen) SetMissionIndex(index int) {
	s.game.MissionIndex = index
}

// applyMissionObjective gives the sim the running mission's win/loss rule, or leaves generic rules in place when no mission objective exists.
func (s *Screen) applyMissionObjective() {
	o := objectives.For(s.game.Mission, s.game.MissionIndex)
	if o == nil {
		s.game.WinLossConditions = nil // generic rules: elimination, score, turn limit

		return
	}

	s.game.WinLossConditions = objectives.Bind(o, s.game)

	dbg.Printf("mission: %s %d runs its scripted objective", s.game.Mission, s.game.MissionIndex)
}

// The sim must implement every query used by mission objectives. This compile-time check prevents objectives from silently falling back to generic rules.
var _ objectives.Query = (*sim.Game)(nil)

// missionRunsAnObjective reports whether the running game is a mission whose outcome a
// scripted objective decides. The game over screen reads it: a mission the objective won
// or lost says so in words, where a skirmish shows the points and the placings.
//
// C++ ported : menu.cpp - the victory_status passed to menu_draw_game_over_screen
func (s *Screen) missionRunsAnObjective() bool {
	return s.game.Mission.IsMission() && s.game.WinLossConditions != nil
}
