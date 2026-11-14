// setup_carry_test.go : tests transferring custom-game and mission choices into a running game.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// TestSetTeamClansKeepsExplicit checks that a clan chosen on the setup screen is
// written into the team and then kept by resolveTeamClans, while a ClanRandom slot
// is filled in and the alien slot is left alone.
func TestSetTeamClansKeepsExplicit(t *testing.T) {
	s := newResearchScreen()
	s.game.State.Teams[0].Type = game.TeamPlayer
	s.game.State.Teams[1].Type = game.TeamComputer

	s.SetTeamClans([]int{4, game.ClanRandom, 0, 0}) // the player picked clan 4 for team 0
	if s.game.State.Teams[0].Clan != 4 {
		t.Fatalf("SetTeamClans: team 0 clan = %d, want 4", s.game.State.Teams[0].Clan)
	}

	s.resolveTeamClans(game.NewRandomizer(999))
	if s.game.State.Teams[0].Clan != 4 {
		t.Errorf("explicit clan 4 overwritten by resolve: %d", s.game.State.Teams[0].Clan)
	}
	if c := s.game.State.Teams[1].Clan; c < 1 || c > 8 {
		t.Errorf("ClanRandom team not resolved: %d", c)
	}
	if s.game.State.Teams[game.AlienTeam].Clan != game.ClanRandom {
		t.Errorf("alien slot clan changed: %d", s.game.State.Teams[game.AlienTeam].Clan)
	}
}

// TestSetTeamUpgradesReachTheUnits checks that an upgrade bought on the purchase screen
// lands on the team: teamValues reports it, so every unit deployed or built afterwards
// carries it. Before this the bought upgrade was dropped and only the gold was gone.
func TestSetTeamUpgradesReachTheUnits(t *testing.T) {
	s, g := newTestScreen(t, 16)

	before := s.teamValues(0, assets.Tank).Get(game.AttribArmor)

	s.SetTeamUpgrades(0, []game.UnitUpgrade{
		{Type: assets.Tank, Attrib: game.AttribArmor, Offset: 6},
		{Type: assets.Scout, Attrib: game.AttribScan, Offset: 0}, // a zero offset is not an upgrade
	}, 24)

	if got := s.teamValues(0, assets.Tank).Get(game.AttribArmor); got != before+6 {
		t.Errorf("Tank armor after the purchased upgrade = %d, want %d", got, before+6)
	}

	if got := g.State.Teams[0].UpgradeOffset(assets.Scout, game.AttribScan); got != 0 {
		t.Errorf("zero offset recorded as an upgrade: %d", got)
	}

	// The other team is untouched.
	if got := g.State.Teams[1].UpgradeOffset(assets.Tank, game.AttribArmor); got != 0 {
		t.Errorf("team 1 picked up team 0's upgrade: %d", got)
	}
}

// TestSetVictory applies the win condition the setup screen chose (was ignored, so a
// custom game ran on the engine default).
func TestSetVictory(t *testing.T) {
	s := newResearchScreen()
	s.SetVictory(winloss.VictoryScore, 500)
	if s.game.VictoryType != winloss.VictoryScore || s.game.VictoryLimit != 500 {
		t.Errorf("SetVictory = %v/%d, want score/500", s.game.VictoryType, s.game.VictoryLimit)
	}
}

// TestCustomStartOpponentCount checks that the custom game lands one player plus the
// chosen number of computer opponents.
func TestCustomStartOpponentCount(t *testing.T) {
	s, g := newTestScreen(t, 16)
	s.SetCustomOpponents(2) // two computer opponents -> three teams
	s.CustomStart(nil, nil, 100, 42)

	if g.State.Teams[0].Type != game.TeamPlayer {
		t.Errorf("team 0 type = %v, want Player", g.State.Teams[0].Type)
	}
	for team := 1; team <= 2; team++ {
		if g.State.Teams[team].Type != game.TeamComputer {
			t.Errorf("team %d type = %v, want Computer", team, g.State.Teams[team].Type)
		}
	}
	if g.State.Teams[3].Type != game.TeamNone {
		t.Errorf("team 3 type = %v, want None (only two opponents requested)", g.State.Teams[3].Type)
	}
}

// TestSetResourceLevels checks the setup screen raw/fuel/gold density is recorded
// and flagged, so CustomStart seeds the map at that richness; an untouched screen
// (the -random benches never call it) leaves the flag clear for the default params.
func TestSetResourceLevels(t *testing.T) {
	s := newResearchScreen()
	if s.resourceLevelsSet {
		t.Fatalf("resourceLevelsSet should start false")
	}

	s.SetResourceLevels(0, 1, 2) // Poor raw, Average fuel, Rich gold
	if !s.resourceLevelsSet {
		t.Errorf("SetResourceLevels did not flag the levels as set")
	}
	if s.resourceLevels != [3]int{0, 1, 2} {
		t.Errorf("resourceLevels = %v, want [0 1 2]", s.resourceLevels)
	}
}

// TestMissionStartAppliesTheFileTimers checks that a mission runs on its own timers: an untimed
// mission is untimed even when the player has a turn timer set, and its End Turn Time replaces
// the player's (N13).
func TestMissionStartAppliesTheFileTimers(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	s.SetTurnTimer(120) // the player's settings, as the screen build sets them
	s.SetEndTurnTimer(30)

	s.MissionStart(nil, []game.TeamType{game.TeamPlayer, game.TeamComputer}, nil, MissionRules{TurnTimer: 0, EndTurn: 45}, 7)
	if s.turnFlow.turnSeconds != 0 || s.turnFlow.endTurnSeconds != 45 {
		t.Errorf("untimed mission: turn %ds, end turn %ds; want 0 and 45", s.turnFlow.turnSeconds, s.turnFlow.endTurnSeconds)
	}

	s2, _ := newTestScreen(t, 16)
	s2.MissionStart(nil, []game.TeamType{game.TeamPlayer, game.TeamComputer}, nil, MissionRules{TurnTimer: 180, EndTurn: 45}, 7)
	if s2.turnFlow.turnSeconds != 180 || s2.turnFlow.fullTurnSeconds != 180 {
		t.Errorf("timed mission: turn %ds (full %ds), want 180", s2.turnFlow.turnSeconds, s2.turnFlow.fullTurnSeconds)
	}
}

// TestMissionStartAppliesClan checks that a mission's scripted clan is honoured (a 0
// stays random for resolveTeamClans to fill).
func TestMissionStartAppliesClan(t *testing.T) {
	s, g := newTestScreen(t, 16)
	var rules MissionRules
	rules.TeamClan[1] = 5 // the mission scripts clan 5 for team 1

	s.MissionStart(nil, []game.TeamType{game.TeamPlayer, game.TeamComputer}, nil, rules, 7)
	if g.State.Teams[1].Clan != 5 {
		t.Errorf("mission clan not applied: team 1 clan = %d, want 5", g.State.Teams[1].Clan)
	}
}
