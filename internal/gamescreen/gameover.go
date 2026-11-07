// gameover.go : implements the game screen game over trigger.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/winloss"
)

// The GameOver data transfer objects now live in internal/screendata, shared with
// the menus renderer.

// GameOverData returns an end of game snapshot for the caller to pass to
// menus.RunGameOver after scr.Run() returns.
//
// C++ ported : menu.cpp - menu_wrap_up_game (status, turn_counter, mode)
func (s *Screen) GameOverData() screendata.GameOverData {
	st := s.game.Status()
	var teams []screendata.GameOverTeam

	for team := 0; team < game.TeamCount; team++ {

		if s.game.State.Teams[team].Type == game.TeamNone {
			continue
		}

		ts := s.game.TeamStats[team]
		teams = append(teams, screendata.GameOverTeam{
			Name:           s.teamName(team),
			Player:         team == s.game.State.PlayerTeam,
			Points:         s.game.TeamPoints[team],
			Rank:           st.TeamRank[team],
			FactoriesBuilt: ts.FactoriesBuilt,
			MinesBuilt:     ts.MinesBuilt,
			BuildingsBuilt: ts.BuildingsBuilt,
			BuildingsLost:  ts.BuildingsLost,
			UnitsBuilt:     ts.UnitsBuilt,
			UnitsLost:      ts.UnitsLost,
			GoldOnUpgrades: ts.GoldOnUpgrades,
		})
	}

	return screendata.GameOverData{
		TurnCounter: s.game.State.TurnCounter,
		Outcome:     s.outcome,
		Teams:       teams,
		// A mission with an objective is won or lost on THAT, so the player's column says
		// "Mission Success" / "Mission Failed" instead of a score and a placing.
		Scripted: s.missionRunsAnObjective(),
	}
}

// Win/lose music: the jingles played once when the game ends.
const (
	winMusicTag  = "WINR_MSC"
	loseMusicTag = "LOSE_MSC"
)

// enterGameOver shows the end of game screen in a dedicated top level window,
// plays the mission movie if applicable, switches music, and blocks until the
// player presses RETURN or ESC; the caller then exits the Run loop.
// C++ ported : menu.cpp - menu_wrap_up_game / menu_draw_game_over_screen.
func (s *Screen) enterGameOver(o winloss.Outcome) {
	if s.gameOver {
		return
	}

	s.gameOver = true
	s.outcome = o

	/* // note: unported C++ code (mission victory / defeat story screen)
	// Mission win/lose movie before the score screen (skirmish tags are empty).
	// C++ ported : menu.cpp - menu_draw_mission_story_screen / Movie_Play(video).
	if !s.noMovie && s.hooks.movieFn != nil {
		tag := s.loseMovieTag

		if o == winloss.PlayerWon {
			tag = s.winMovieTag
		}

		if tag != "" {
			if err := s.hooks.movieFn(tag); err != nil {
				dbg.Printf("game over movie %q: %v", tag, err)
			}
		}
	}
	*/

	if s.hooks.musicFn != nil {
		tag := loseMusicTag

		if o == winloss.PlayerWon {
			tag = winMusicTag
		}

		dbg.Printf("game over: %s music", tag)
		s.hooks.musicFn(tag, false)
	}

	// C++ ported : menu.cpp - PlayVoice(V_M283/F283 win, V_M010/F010 lose).
	if o == winloss.PlayerWon {
		s.playVoiceForced(vsMissionSuccess)
	} else {
		s.playVoiceForced(vsMissionFailed)
	}

	s.exitToMenu = true // signals Run() to exit; the caller shows game over standalone
}

// Outcome reports the local player's end of game result after Run returns:
// PlayerWon / PlayerLost, or Continue if the game was abandoned (ESC to menu).
// The mission/campaign launcher reads it to drive campaign progression.
func (s *Screen) Outcome() winloss.Outcome {
	return s.outcome
}
