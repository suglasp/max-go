// gameover.go : carries the game over screen data transfer objects.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : menu.cpp - menu_wrap_up_game / menu_draw_game_over_screen
package screendata

import "github.com/suglasp/max-go/internal/winloss"

// GameOverTeam holds the results per team shown on the game over screen. The stat
// fields are sourced from sim.Game.TeamStats (tracked at the build / landing /
// death / upgrade sites; see internal/sim/teamstats.go).
type GameOverTeam struct {
	Name           string
	Player         bool // the human player's column: it is the one a mission verdict replaces
	Points         int
	Rank           int // 0 = eliminated, 1 = 1st place, 2 = 2nd, ...
	FactoriesBuilt int
	MinesBuilt     int
	BuildingsBuilt int
	BuildingsLost  int
	UnitsBuilt     int
	UnitsLost      int
	GoldOnUpgrades int
}

// GameOverData is the end of game snapshot the game screen passes to
// menus.RunGameOver after the run loop returns.
type GameOverData struct {
	TurnCounter int
	Outcome     winloss.Outcome
	Teams       []GameOverTeam
	// Scripted says a MISSION OBJECTIVE decided the outcome, not the points. The
	// player's column then reads "Mission Success" or "Mission Failed" where a
	// skirmish shows the score and the placing - because in a mission the score is
	// beside the point.
	Scripted bool
}
