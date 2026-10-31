// simultaneous.go : implements the simultaneous (real time) play mode turn loop.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : game_manager.cpp - GameManager_TeamTurnConcurrent
package gamescreen

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/winloss"
)

// SetPlayMode selects the turn loop the game runs: turn based (hot seat) or
// simultaneous (all teams act in real time within the turn). The launcher calls
// it from the chosen custom game / mission setting, or the -simultane /
// -turnbased CLI flag. C++: GameManager_PlayMode from the play_mode setting.
func (s *Screen) SetPlayMode(mode int) {
	if mode == int(game.PlaySimultaneous) {
		s.game.State.PlayMode = game.PlaySimultaneous
	} else {
		s.game.State.PlayMode = game.PlayTurnBased
	}
}

// simMode reports whether the game runs the simultaneous (real time) turn loop.
func (s *Screen) simMode() bool {
	return s.game != nil && s.game.State.PlayMode == game.PlaySimultaneous
}

// humanViewTeam returns the local player's team, the one whose view is on screen.
// For a single player simultaneous game that is the sole human team.
func (s *Screen) humanViewTeam() int {
	for t := s.game.State.FirstActiveTeam(); t != -1; t = s.game.State.NextActiveTeam(t) {
		if s.game.State.Teams[t].Type == game.TeamPlayer {
			return t
		}
	}

	return s.game.State.FirstActiveTeam()
}

// beginSimRound opens a simultaneous round: it runs the per team economy and a
// turn of construction for every active team, lets each computer team plan and
// QUEUE its move orders, then hands the live view to the human. The queued AI
// orders are NOT held back - the idle tick animates every team's orders together,
// so within the turn the game plays out in real time (the manual: "within that
// turn the game functions as a real time game"). The computer teams are then
// flagged finished, so only the human keeps the turn open until End Turn / the
// timer commits it. C++: GameManager_TeamTurnConcurrent (ManageEconomy + Ai_BeginTurn
// run up front for every team, then the human plays live).
func (s *Screen) beginSimRound() {
	s.turnFlow.simPlanned = true
	human := s.humanViewTeam()

	// A LOADED mid round game re-enters here: the round's turn start already ran
	// before the save (economy, construction, budget refresh - all state the save
	// carries), so re-running BeginTeamTurn would double run the round. Only the
	// computer teams' PLANS are rebuilt - their move orders are transient and never
	// saved. The restored FinishedTurn flags and the restored turn clock both stay.
	// C++ ported : game_manager.cpp - the GAME_STATE_10_LOAD_GAME guards on
	// ManageEconomy/ProgressBuildState (klei's saves carry the AI state too, so it
	// also skips Ai_BeginTurn; the port's do not, hence the replan half).
	if s.turnFlow.simLoadResume {
		s.turnFlow.simLoadResume = false

		for t := s.game.State.FirstActiveTeam(); t != -1; t = s.game.State.NextActiveTeam(t) {
			if t != human {
				s.game.ReplanTeamTurn(t)
			}
		}

		dbg.Printf("sim round: turn %d resumed from a load (AI plans rebuilt, no turn start re-run)", s.game.State.TurnCounter)

		return
	}

	for t := s.game.State.FirstActiveTeam(); t != -1; t = s.game.State.NextActiveTeam(t) {
		// The human's turn 1 economy already ran at landing: CustomStart, RandomStart and
		// MissionStart each open with BeginTeamTurn for the player, so running it here too
		// would advance the human's economy twice on the opening turn.
		if t == human && s.game.State.TurnCounter == 1 {
			continue
		}

		before := s.selectableIDs(t)   // diff against the units this turn start construction adds
		goldBefore := s.teamCredits(t) // detect a first gold refine across this turn's economy
		s.buyAITurnUpgrades(t)         // a computer team spends its banked refinery gold before it plans
		s.game.BeginTeamTurn(t)        // economy + a turn of construction; a computer team also plans + queues its orders
		s.announceTurnStart(t, before) // surfaces the human's turn start results (does nothing for a computer team)
		s.checkGoldFirstRefined(t, goldBefore)
	}

	// Every computer team has issued its orders for the turn: flag it finished so
	// only the human still holds the turn open (the human commits with End Turn).
	for t := s.game.State.FirstActiveTeam(); t != -1; t = s.game.State.NextActiveTeam(t) {
		if t != human {
			s.game.State.Teams[t].FinishedTurn = true
		}
	}

	s.game.State.ActiveTeam = human
	s.game.State.PlayerTeam = human
	s.clearSelectionForNewTurn(human)
	s.tickAutoSurvey(human)                 // drive any Auto Survey surveyor toward the nearest unsurveyed cell
	s.resetEndTurnGrace()                   // full turn budget back; the grace re arms once the opponents finish
	s.turnFlow.turnStart = platform.Ticks() // (re)start the real time turn clock
	dbg.Printf("sim round: turn %d real-time plan phase, view=%s", s.game.State.TurnCounter, s.teamName(human))
}

// commitSimTurn ends the human's real time turn: any in flight AI / human move is
// settled, every team's end of turn processing runs, the turn rolls over and the
// next round begins. The human pressing End Turn (or the turn timer expiring) is
// the only trigger; the computer teams already finished when they planned at round
// start. C++: the human closing GameManager_TeamTurnConcurrent.
func (s *Screen) commitSimTurn() {
	// No per team end turn voice: concurrent play has no per team end moment, and the
	// player's own line is suppressed anyway (the pressed End Turn button confirms it).
	if !s.finishSimRound() {
		return // game over during the round close
	}

	s.game.AdvanceTurn()
	s.maybeWarnGameEnd()     // Duration game: "Notice: Game will end in 10 turns!" at the start of turn limit-9
	s.announceLeaderChange() // "<colour> team has the most Ecospheres!" when the lead changes hands
	s.beginSimRound()
}

// finishSimRound runs end of turn processing for every active team after a
// simultaneous round: research, the movement / score settle (EndTeamTurn drains
// any move still in flight), the human autosave, and the win/loss check. It
// returns false when the game has ended. C++: the per team GameManager end of turn
// step run for all teams once the concurrent turn closes.
func (s *Screen) finishSimRound() bool {
	for t := s.game.State.FirstActiveTeam(); t != -1; t = s.game.State.NextActiveTeam(t) {
		s.syncResearchOrders(t) // idle research centres with no allocated topic so they draw no power
		s.advanceResearch(t)    // a turn of research; level ups upgrade the team's units
		s.game.EndTeamTurn(t)   // settle movement + eliminations + score, mark finished
	}

	// The human's silent autosave at the end of the player's turn.
	if s.game.State.Teams[s.game.State.PlayerTeam].Type == game.TeamPlayer {
		s.maybeAutoSave()
	}

	if !s.debugMenu.endless { // the F2 "Endless mode" flag suppresses the win/loss check

		if o := s.game.Outcome(); o != winloss.Continue {
			s.enterGameOver(o)
			return false
		}
	}

	return true
}
