// clan.go : implements team identity: clan assignment, emblems, colours and names.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// clanAttrWord maps a CLANATRB stat word to its UnitValues attribute. assets.ParseClanMods discards words without a value, such as "disable".
var clanAttrWord = map[string]game.Attrib{
	"attack":  game.AttribAttack,
	"shots":   game.AttribRounds,
	"range":   game.AttribRange,
	"armor":   game.AttribArmor,
	"hits":    game.AttribHits,
	"speed":   game.AttribSpeed,
	"scan":    game.AttribScan,
	"turns":   game.AttribTurns,
	"ammo":    game.AttribAmmo,
	"storage": game.AttribStorage,
}

// applyClanMods applies a team's clan modifiers for unit type t to v. Relative modifiers add to a value; absolute modifiers replace it. C++: ResourceManager_SetClanUpgradesFromClans.
func (s *Screen) applyClanMods(v *game.UnitValues, team int, t assets.UnitType) {
	if team < 0 || team >= game.TeamCount {
		return
	}

	clan := s.game.State.Teams[team].Clan
	if clan < 1 || clan > len(s.buildUI.clans) {
		return
	}

	for _, m := range s.buildUI.clans[clan-1].Mods[t] {
		a, ok := clanAttrWord[m.Attr]
		if !ok {
			continue
		}

		if m.Abs {
			v.Set(a, m.Delta)
		} else {
			v.Add(a, m.Delta)
		}
	}
}

// teamColor pairs a team's lang_english.ini clan colour name key with an
// English fallback used when the string table lacks it.
type teamColor struct {
	key      string // clan colour string key
	fallback string // English colour name
}

// teamColorKeys maps a team index to its colour name (ba19/d486/ec7d/0d5a).
var teamColorKeys = [game.TeamCount]teamColor{
	{"ba19", "Red"}, {"d486", "Green"}, {"ec7d", "Blue"}, {"0d5a", "Gray"},
}

// teamName returns a team's M.A.X. clan colour name from the language strings.
func (s *Screen) teamName(team int) string {
	if team < 0 || team >= len(teamColorKeys) {
		return "Neutral"
	}

	c := teamColorKeys[team]
	return s.lang.Get(c.key, c.fallback)
}

// resolveTeamClans assigns concrete clans to playable teams that still use ClanRandom. The AI assigns its own teams before the remaining teams receive a random clan.
func (s *Screen) resolveTeamClans(rng *game.Randomizer) {
	if s.ai != nil {
		s.ai.ResolveTeamClans(s.game, rng)
	}

	for team := 0; team < game.TeamCount; team++ {
		if s.game.State.Teams[team].Clan == game.ClanRandom {
			s.game.State.Teams[team].Clan = rng.Generate(8) + 1
		}
	}

	if dbg.Enabled() { // one line per team so resolved clans can be hand verified
		for team := 0; team < game.TeamCount; team++ {
			c := s.game.State.Teams[team].Clan
			name := "?"

			if c >= 1 && c <= len(s.buildUI.clans) {
				name = s.buildUI.clans[c-1].Name
			}

			dbg.Printf("clan: team %d (%v) -> clan %d %q", team, s.game.State.Teams[team].Type, c, name)
		}
	}
}

// teamClan returns the clan whose emblem a team's buildings display. An unresolved clan uses a stable team-derived fallback.
// C++: TeamInfo::team_clan.
func (s *Screen) teamClan(team int) int {
	if team >= 0 && team < game.TeamCount {
		if c := s.game.State.Teams[team].Clan; c >= 1 && c <= 8 {
			return c
		}
	}

	clan := team + 1

	if clan < 1 || clan > 8 {
		clan = 1
	}

	return clan
}

// clanEmblemFrame maps a clan to its mining-station emblem frame. The sheet stores emblems in even frames; odd frames are magenta team-colour placeholders.
func clanEmblemFrame(clan int) int {
	return (clan - 1) * 2
}
