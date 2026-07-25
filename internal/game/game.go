// game.go : defines teams and the settings used to start a match.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : game_manager.cpp - GameManager state types + turn loop
package game

import "github.com/suglasp/max-go/internal/settings"

// TeamType says who controls a team (remote/network is out of scope).
//
// C++ ported : enums.hpp - TEAM_TYPE_*
type TeamType uint8

const (
	TeamNone TeamType = iota
	TeamPlayer
	TeamComputer
	// TeamAlien owns neutral alien derelicts. It is not playable or included in turn rotation, but provides their stable owner ID.
	TeamAlien
)

// String returns the menu label of the team type.
func (t TeamType) String() string {
	switch t {
	case TeamPlayer:
		return "Human"
	case TeamComputer:
		return "Computer"
	case TeamAlien:
		return "Alien"
	default:
		return "None"
	}
}

// TeamCount is the number of playable teams: red, green, blue, and gray. It excludes the alien faction.
//
// C++ ported : enums.hpp - PLAYER_TEAM_MAX - 1
const TeamCount = 4

// AlienTeam is the neutral alien slot. TeamSlots includes it so GameState.Teams[u.Team] remains in bounds; playable-team loops stop at TeamCount.
//
// C++ ported : enums.hpp - PLAYER_TEAM_ALIEN / PLAYER_TEAM_MAX
const (
	AlienTeam = TeamCount
	TeamSlots = TeamCount + 1
)

// IsAlien reports whether team is the fixed alien derelict faction.
func IsAlien(team int) bool {
	return team == AlienTeam
}

// ClanRandom in a Team.Clan means pick a random clan at game start.
//
// C++ ported : enums.hpp - TEAM_CLAN_RANDOM
const ClanRandom = 0

// Team is one player slot of a new game.
type Team struct {
	Type TeamType
	Clan int // ClanRandom or 1..8 (assets.Clans index + 1)
	Name string
}

// StartParams holds the menu choices consumed by game setup.
//
// C++ ported : game_manager.cpp - GameManager_GameLoop (GAME_STATE_6_GAME_SETUP)
type StartParams struct {
	World   int // 0..23
	Teams   [TeamCount]Team
	Options settings.Game // play mode, timers, credit, resources, victory
}
