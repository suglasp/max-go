// voiceturn.go : speaks the per turn and per team announcements: Begin, ended turn, the
// points leader, finished construction, full storage and the first refined gold.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : game_manager.cpp - GameManager_AnnounceWinner and the turn advance voices
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// leaderVoiceTag is the "<colour> team has the most Ecospheres!" announce for team:
// F118 red, F122 green, F126 blue, F130 grey. The lines sit four apart because the
// original's resource table pairs a male and a female sample per team.
// C++ ported : game_manager.cpp - GameManager_AnnounceWinner (V_M118 + winner * 2).
func leaderVoiceTag(team int) string {
	if team < 0 || team >= game.TeamCount {
		return ""
	}

	return fmt.Sprintf("V_F%03d", 118+team*4)
}

// announceLeaderChange speaks "<colour> team has the most Ecospheres!" when the points lead changes at round close in either play mode.
// The first round seeds the leader without an announcement. Loads do the same, and a zero-point leader remains silent.
// Eco Sphere points use the same announcement rule for every victory type.
// C++ ported : game_manager.cpp:594 - GameManager_AnnounceWinner(team_winner) at ++TurnCounter.
func (s *Screen) announceLeaderChange() {
	leader := s.pointsLeader()

	if leader == s.announcer.pointsLeader {
		return
	}

	previous := s.announcer.pointsLeader
	s.announcer.pointsLeader = leader

	if previous == announcerNoLeader || leader < 0 || s.game.TeamPoints[leader] <= 0 {
		return // the opening seed, an empty board, or a lead inherited at zero points
	}

	s.playVoiceTag(leaderVoiceTag(leader), 40, "team leads on Eco Spheres")
}

// pointsLeader is the active team with the most points, ties going to the lower team
// index (the original scans the teams in order and only a STRICTLY higher score takes
// the lead). Returns -1 when no team is left.
// C++ ported : game_manager.cpp - GameManager_EvaluateWinner (VICTORY_TYPE_SCORE branch).
func (s *Screen) pointsLeader() int {
	leader, best := -1, 0

	for t := s.game.State.FirstActiveTeam(); t != -1; t = s.game.State.NextActiveTeam(t) {
		if points := s.game.TeamPoints[t]; leader == -1 || points > best {
			leader, best = t, points
		}
	}

	return leader
}

// teamEndedTurnVoice queues the "<colour> team has ended turn!" line for teams 0 through 3 (F279 through F282). tickVoiceQueue drains these lines one at a time.
// Single-player games skip the local player's line because the End Turn button already confirms that action. Multiplayer announcements are not implemented.
func (s *Screen) teamEndedTurnVoice(team int) {
	if team < 0 || team >= game.TeamCount {
		return
	}

	if s.game != nil && s.isHumanTeam(team) {
		return // the pressed End Turn button confirms the player's own click; see the multiplayer note above
	}

	s.enqueueVoiceTag(fmt.Sprintf("V_F%03d", 279+team), 20, "team ended turn")
}

// isHumanTeam reports whether a team is human controlled. Turn processing voices
// (construction, research, storage) use this rather than PlayerTeam, which during
// end of turn processing tracks whichever team is currently being resolved.
func (s *Screen) isHumanTeam(team int) bool {
	return team >= 0 && team < game.TeamCount && s.game.State.Teams[team].Type == game.TeamPlayer
}

// announceTurnBegin plays "Begin!" once when the human player's interactive turn goes
// live. Waits out a landing sequence or hot seat handover banner, keyed on turn+team
// so it fires exactly once per turn.
// C++ : the V_F053 "Begin!" line at the start of the player's turn (game_manager.cpp).
func (s *Screen) announceTurnBegin() {
	if s.gameOver || s.turnFlow.handover != -1 || s.camera.landingStep > 0 {
		return
	}

	team := s.game.State.ActiveTeam

	if !s.isHumanTeam(team) {
		return
	}

	key := s.game.State.TurnCounter*game.TeamCount + team

	if key == s.announcer.lastBeginKey {
		return
	}

	s.announcer.lastBeginKey = key
	s.playVoice(vsTurnBegin)
}

// selectableIDs is the set of a team's real (non ground cover) selectable unit ids,
// used to diff which units a turn of construction added.
func (s *Screen) selectableIDs(team int) map[uint16]bool {
	ids := map[uint16]bool{}

	for _, u := range s.game.Units.Units() {
		if u.Team == team && u.Flags.Has(game.FlagSelectable) && !u.Flags.Has(game.FlagGroundCover) {
			ids[u.ID] = true
		}
	}

	return ids
}

// announceBuildsCompleted plays the completion line for units finished this turn. One building says "Construction complete", one unit says "Unit completed", and several say "New units available".
// It runs for the player team at turn start and bypasses the cross-voice window so it is not suppressed by end-turn or "Begin!" lines.
func (s *Screen) announceBuildsCompleted(team int, before map[uint16]bool) {
	if !s.isHumanTeam(team) {
		return
	}

	newBuildings, newUnits := 0, 0

	for _, u := range s.game.Units.Units() {
		if u.Team != team || !u.Flags.Has(game.FlagSelectable) || u.Flags.Has(game.FlagGroundCover) {
			continue
		}

		if before[u.ID] {
			continue
		}

		if u.Building || u.Flags.Has(game.FlagStationary) {
			newBuildings++
		} else {
			newUnits++
		}
	}

	switch total := newBuildings + newUnits; {
	case total == 0:
		return
	case total == 1 && newBuildings == 1:
		s.playVoiceForced(vsBuildComplete)
	case total == 1:
		s.playVoiceForced(vsUnitComplete)
	default:
		s.playVoiceForced(vsNewUnits)
	}
}

// checkStorageFull announces when the player's pooled raw/fuel/gold storage first
// fills up, once per resource until it drops below full again. C++: the storage full
// voices F057/F061/F066.
func (s *Screen) checkStorageFull(team int) {
	if !s.isHumanTeam(team) {
		return
	}

	var stored, capacity economy.Cargo

	for _, c := range s.game.Complexes {
		if c.Team != team {
			continue
		}

		m, cp := c.CargoInfo()
		stored.Add(m)
		capacity.Add(cp)
	}

	dbg.Logf(dbg.ECON, "team %d storage: raw=%d/%d fuel=%d/%d gold=%d/%d (announced raw=%v fuel=%v gold=%v)",
		team, stored.Raw, capacity.Raw, stored.Fuel, capacity.Fuel, stored.Gold, capacity.Gold,
		s.announcer.rawFull, s.announcer.fuelFull, s.announcer.goldFull)

	s.storageFullVoice(&s.announcer.fuelFull, stored.Fuel, capacity.Fuel, vsStorageFuelFull)
	s.storageFullVoice(&s.announcer.rawFull, stored.Raw, capacity.Raw, vsStorageRawFull)
	s.storageFullVoice(&s.announcer.goldFull, stored.Gold, capacity.Gold, vsStorageGoldFull)
}

// storageFullVoice fires once when stored material first reaches positive capacity, then rearms when it drops below full. It bypasses the cross-voice window so queued end-turn lines do not suppress it.
func (s *Screen) storageFullVoice(announced *bool, stored, capacity int, state voiceState) {
	full := capacity > 0 && stored >= capacity

	if full && !*announced {
		s.playVoiceForced(state)
	}

	*announced = full
}

// teamCredits reads a team's spendable credit balance (bounds safe), used to detect
// a gold refine across a turn's economy.
func (s *Screen) teamCredits(team int) int {
	if team < 0 || team >= len(s.game.Gold) {
		return 0
	}

	return s.game.Gold[team]
}

// checkGoldFirstRefined plays the one time "first gold refined" voice the first time
// the human team's spendable credits rise from a Gold Refinery's output (g.Gold grew
// across this turn's economy, goldBefore -> now). Latched per team, player team only.
// C++: unitinfo.cpp UpdateProduction COMMTWR (!gold_reserves && PlayerTeam && GetGold()>0).
func (s *Screen) checkGoldFirstRefined(team, goldBefore int) {
	if team < 0 || team >= len(s.announcer.goldRefinedVoiced) || s.announcer.goldRefinedVoiced[team] {
		return
	}

	if !s.isHumanTeam(team) || s.teamCredits(team) <= goldBefore {
		return
	}

	s.announcer.goldRefinedVoiced[team] = true
	s.playVoice(vsGoldRefined)
}
