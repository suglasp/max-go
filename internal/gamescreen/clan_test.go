// clan_test.go : tests clan selection wiring into the game screen startup flow.
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

// TestTeamValuesAppliesClanBonus checks the clan stat layer in teamValues: a "+N"
// mod is added to the base attribute and an absolute "N turns" sets the build time,
// while a team whose clan is still unresolved gets no bonus.
func TestTeamValuesAppliesClanBonus(t *testing.T) {
	s := newResearchScreen()
	// Synthetic Clan A: the AWAC gains +2 scan and the Tank's build time is set to 7.
	s.buildUI.clans = []assets.Clan{{

		Mods: map[assets.UnitType][]assets.ClanMod{
			assets.Awac: {{Attr: "scan", Delta: 2}},
			assets.Tank: {{Attr: "turns", Delta: 7, Abs: true}},
		},
	}}
	s.game.State.Teams[0].Clan = 1 // -> s.buildUI.clans[0]

	if got := s.teamValues(0, assets.Awac).Get(game.AttribScan); got != 2 {
		t.Errorf("AWAC scan with Clan A = %d, want 2 (+2 from clan)", got)
	}
	if got := s.teamValues(0, assets.Tank).Get(game.AttribTurns); got != 7 {
		t.Errorf("Tank turns with Clan A = %d, want 7 (absolute set)", got)
	}

	// A team still left ClanRandom picks up no clan bonus.
	s.game.State.Teams[1].Type = game.TeamComputer
	s.game.State.Teams[1].Clan = game.ClanRandom
	if got := s.teamValues(1, assets.Awac).Get(game.AttribScan); got != 0 {
		t.Errorf("unresolved-clan team AWAC scan = %d, want 0", got)
	}
}

// TestResolveTeamClansAndEmblem checks that a new game resolves every ClanRandom
// playable team to a concrete 1..8, preserves an explicit clan, leaves the alien
// slot alone, and that teamClan reads the resolved value.
func TestResolveTeamClansAndEmblem(t *testing.T) {
	s := newResearchScreen()
	s.game.State.Teams[0].Type = game.TeamPlayer
	s.game.State.Teams[1].Type = game.TeamComputer
	s.game.State.Teams[2].Type = game.TeamComputer
	s.game.State.Teams[2].Clan = 5 // an explicit choice that must be kept

	// No AI wired: every ClanRandom team gets a uniform pick.
	s.resolveTeamClans(game.NewRandomizer(12345))

	for team := 0; team < 2; team++ {
		if c := s.game.State.Teams[team].Clan; c < 1 || c > 8 {
			t.Errorf("team %d clan = %d, want a resolved 1..8", team, c)
		}
	}
	if s.game.State.Teams[2].Clan != 5 {
		t.Errorf("explicit clan 5 overwritten: %d", s.game.State.Teams[2].Clan)
	}
	if got := s.teamClan(2); got != 5 {
		t.Errorf("teamClan(2) = %d, want 5", got)
	}
	// The fixed alien slot is never resolved.
	if c := s.game.State.Teams[game.AlienTeam].Clan; c != game.ClanRandom {
		t.Errorf("alien slot clan = %d, want ClanRandom (untouched)", c)
	}
	// An unresolved team's emblem falls back to a stable team derived clan.
	bench := newResearchScreen()
	if got := bench.teamClan(1); got != 2 {
		t.Errorf("teamClan(1) fallback = %d, want 2 (team+1)", got)
	}
}
