// setup_carry.go : transfers clans, win conditions, and opponent counts into the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// SetTeamClans writes the clans chosen on the setup screen into the teams, so a
// player's picked clan (and its stat bonuses) is honoured instead of being
// resolved at random. A ClanRandom (0) entry is left alone for resolveTeamClans
// to fill in; the alien slot is never touched. Call before CustomStart, whose
// resolveTeamClans keeps any explicit clan. C++: GameManager_GameSetup carrying
// the menu_team_clan settings into each team.
func (s *Screen) SetTeamClans(clans []int) {
	for i := 0; i < len(clans) && i < game.TeamCount; i++ {
		if game.IsAlien(i) {
			continue
		}

		s.game.State.Teams[i].Clan = clans[i]
	}
}

// SetVictory applies the win condition (duration or score, and its limit) the
// player picked on the setup screen. A custom game otherwise ran on the engine
// default and ignored the choice. C++: GameManager_GameSetup - victory_type /
// victory_limit into the win/loss check.
func (s *Screen) SetVictory(vtype winloss.VictoryType, limit int) {
	s.game.VictoryType = vtype
	s.game.VictoryLimit = limit
}

// SetCustomOpponents records how many computer opponents a custom game lands (1
// to 3 on the setup screen). Zero keeps the default single opponent, so an
// unset value preserves the original two team game. CustomStart reads it.
func (s *Screen) SetCustomOpponents(n int) {
	s.customOpponents = n
}

// SetMissionCategory records WHICH KIND of mission is running (training, scenario,
// campaign, multiplayer scenario), so the rules that key off it can see it: a training
// mission holds the computer down to its most passive tier, trims the buildable roster,
// and is the only place the "position the tape" build prompt is spoken. A custom game
// and every dev bench leave it at MissionNone. It rides the save, so a reloaded training
// mission is still a training mission.
//
// C++ ported : mission.hpp - MissionCategory, read through GetMissionManager()->GetMission()
func (s *Screen) SetMissionCategory(c game.MissionCategory) {
	s.game.Mission = c
}

/* // note: unused code in Go port
// MissionCategory was only read by a test; the test reads s.game.Mission.
// MissionCategory reports the running mission's kind (MissionNone in a free game).
func (s *Screen) MissionCategory() game.MissionCategory {
	return s.game.Mission
}
*/

// SetTeamUpgrades applies the attribute upgrades a team bought on the PURCHASE screen
// before it lands, plus the gold they cost (for the score screen tally). Every unit the
// team deploys and everything it builds later reads these through teamValues, so a
// player who spends their starting credit on tougher tanks actually gets them. Call
// BEFORE CustomStart deploys the base. The in game Upgrade menu writes the same offsets
// itself (openUpgrade), so it does not go through here.
//
// C++ ported : cargomenu.cpp - CargoMenu::CommitUpgradeChanges
func (s *Screen) SetTeamUpgrades(team int, ups []game.UnitUpgrade, goldSpent int) {
	if team < 0 || team >= game.TeamCount {
		return
	}

	for _, u := range ups {
		if u.Offset == 0 {
			continue
		}

		s.game.State.Teams[team].SetUpgradeOffset(u.Type, u.Attrib, u.Offset)
	}

	if goldSpent != 0 {
		s.game.RecordUpgradeGold(team, goldSpent)
	}
}

// SetResourceLevels records the raw/fuel/gold density the player chose on the
// setup screen (0 Poor, 1 Average, 2 Rich), so CustomStart seeds the map with
// the matching deposit richness instead of the fixed engine default. Leaving it
// unset (the -random / -random2 benches never call this) keeps those on the
// default params. C++: menu_update_resource_levels feeding the allocator.
func (s *Screen) SetResourceLevels(raw, fuel, gold int) {
	s.resourceLevels = [3]int{raw, fuel, gold}
	s.resourceLevelsSet = true
}

// SetStartGold records the game's landing credit: what a team has to spend before it
// lands. It does two jobs. On the quick start benches it is the human's gold reserve
// (an explicit -startcredit, consumed by RandomStart/RandomStart2; the menu flows pass
// the human's LEFTOVER credits through CustomStart instead, since their purchase screen
// already ran). And for every game it is the budget each COMPUTER team lands with, which
// it pays its landing force out of and spends the rest of on upgrades (applyAIStartUpgrades).
// Unset means no reserve and no computer upgrades. Zero is a real value (-startcredit None).
//
// C++ ported : the "start_gold" setting seeding UnitsManager_TeamMissionSupplies[team].team_gold
func (s *Screen) SetStartGold(gold int) {
	s.startCredit = gold
	s.startCreditSet = true
}

// rawResourceLevel is the RAW richness the game runs at (0 poor, 1 average, 2 rich).
// A game that never set the levels (both random benches) reads as average, which is
// the density their allocator uses.
func (s *Screen) rawResourceLevel() int {
	if !s.resourceLevelsSet {
		return 1 // average
	}

	return s.resourceLevels[0]
}
