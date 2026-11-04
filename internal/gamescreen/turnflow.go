// turnflow.go : implements the turn handover: end turn, the computer turn and the next team.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"runtime"
	"sync/atomic"
	"time"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/winloss"
)

// turnFlowState holds turn pacing, handoff state, the turn timer, and the atomics used by the AI turn watchdog.
type turnFlowState struct {
	aiTurn          bool         // an in progress computer turn the idle loop animates frame by frame
	aiTurnTeam      int          // which computer team is moving
	aiTurnStartMS   uint64       // when the current computer turn began animating (min dwell aiTurnPaceMS holds it open)
	endTurnSettling bool         // End Turn was pressed with a move still animating: hold the commit until it settles
	endTurnSettleMS uint64       // when the End Turn settle began (bounds it against a wedged move)
	handBackTeam    int          // human team owed a turn once opponents finish and the announcer drains (-1 = none)
	handBackStartMS uint64       // bounds the hand back wait
	handover        int          // RETURN turn loop: team a hot seat pass banner is announcing (-1 = none)
	simPlanned      bool         // current simultaneous round's plan phase is set up (false at start/after a load)
	simLoadResume   bool         // the next beginSimRound resumes a LOADED mid round game: replan only, no turn start re-run
	turnSeconds     int          // the RUNNING turn's budget in seconds (0 = no limit); the End Turn grace shortens it
	fullTurnSeconds int          // the configured per turn budget, restored at the start of every round
	endTurnSeconds  int          // End Turn grace: seconds left once the opponents are done (0 = off, simultaneous only)
	graceArmed      bool         // the grace countdown has already been applied this round
	prefsApplied    bool         // ApplyPrefs ran once (game creation); later calls only follow a changed setting
	prefTurnTimer   int          // the turn timer setting ApplyPrefs last saw
	prefEndTurn     int          // the End Turn Time setting ApplyPrefs last saw
	turnStart       uint64       // tick the current interactive turn began
	timeWarnedTurn  turnID       // the turn the "time low" voice last warned for (zero = none yet; a fresh turn re arms it)
	heartbeat       atomic.Int64 // unix nano start of the current Run() iteration (AI turn hang watchdog)
	wdInAITurn      atomic.Bool  // whether the current frame is animating a computer turn (watchdog)
	wdModalDepth    atomic.Int32 // open blocking modal count, so the stall watchdog ignores a parked modal loop
}

// endTurn ends the active team's turn and hands over to the next: resolves movement
// and checks win/loss, then rolls to the next team, refilling its budgets and
// raising the hot seat pass banner. C++: GameManager_TeamTurnTurnBased + menu_check_end_game_conditions.
func (s *Screen) endTurn() {
	if s.turnFlow.aiTurn {
		return // not the human's turn; the computer is still moving
	}

	// A unit still sliding when End Turn is pressed reaches its stopping point before commit. ResolveMovement
	// fast-forwards remaining movement, so the idle loop finishes the animation first.
	if s.game.ActiveOrders() > 0 {
		if !s.turnFlow.endTurnSettling {
			s.turnFlow.endTurnSettling = true
			s.turnFlow.endTurnSettleMS = platform.Ticks()
			dbg.Printf("endTurn: settling on %d active order(s)", s.game.ActiveOrders())
			s.announceWaitingForOpponents() // "Waiting for A.I. to finish turn." (simultaneous play)
		}

		return
	}

	s.commitTurn()
}

// announceWaitingForOpponents speaks "Waiting for A.I. to finish turn." when the player
// has pressed End Turn in SIMULTANEOUS play and a computer team's units are still
// driving, so the round cannot close yet. That wait is the line's whole subject, and it
// is the only one a single player game has: a turn based game never waits on anybody
// while the player holds the turn, and the player's own units settling is not the
// computer's doing.
func (s *Screen) announceWaitingForOpponents() {
	if !s.simMode() {
		return
	}

	human := s.humanViewTeam()

	for _, o := range s.game.MoveOrders() {
		if o.Unit != nil && o.Unit.Team != human {
			s.playVoice(vsWaitingForOpponents)

			return
		}
	}
}

// commitTurn runs the actual turn close once no move is still animating: the
// simultaneous round commit, or the turn based finish + roll to the next team.
func (s *Screen) commitTurn() {
	s.turnFlow.endTurnSettling = false
	s.announcer.openingTurnDone = true // allow storage full lines from the next turn start

	// A move still deferred behind the planning bulb belongs to the turn being
	// committed: drop it, or tickPlanningBulb would execute the stale order in the
	// NEXT turn and silently spend the fresh budget.
	s.cancelPlanningBulb()

	if s.simMode() { // simultaneous mode commits the whole round at once
		s.commitSimTurn()
		return
	}

	if !s.finishActiveTurn() {
		return // game over
	}

	s.beginNextTeam()
}

// maybeWarnGameEnd shows "Notice: Game will end in 10 turns!" once, at the start of turn limit-9 of a
// Duration game. openMenuScreen pauses the clock and swallows the closing click.
func (s *Screen) maybeWarnGameEnd() {
	if !s.gameEndWarningDue() || s.hooks.messageFn == nil {
		return
	}

	text := s.lang.Get("4556", "Notice: Game will end in 10 turns!")
	s.openMenuScreen(func() { s.hooks.messageFn(text) }, false)
}

// gameEndWarningDue reports whether this turn opens the last ten of a Duration game (turns limit-9 to limit)
// for a human player. It runs after AdvanceTurn, so the counter is already the new turn.
// C++ ported : game_manager.cpp - GameManager_UpdateGui, DialogMenu_Menu(_(4556)) at
// GameManager_TurnCounter == victory_limit-9 after the turn counter increment.
func (s *Screen) gameEndWarningDue() bool {
	if s.game.VictoryType != winloss.VictoryDuration || s.game.VictoryLimit <= 0 {
		return false
	}

	if s.game.State.TurnCounter != s.game.VictoryLimit-9 {
		return false
	}

	player := s.game.State.PlayerTeam

	// Attract demos and AI benches have no human team.
	return player >= 0 && player < game.TeamCount && s.game.State.Teams[player].Type == game.TeamPlayer
}

// endTurnSettleMaxMS caps the End Turn settle wait. ResolveMovement completes any remaining movement after the cap.
const endTurnSettleMaxMS = 8000

// beginNextTeam rolls the active slot to the next team and starts its turn. A
// human turn resumes interactively (hot seat banner, or at once solo). A computer
// team is animated: AI issues orders now, the idle loop ticks the moves, and
// tickComputerTurn finishes the turn once they play out. C++: GameManager_TeamTurnTurnBased.
func (s *Screen) beginNextTeam() {
	if s.game.State.AllFinished() {
		s.game.AdvanceTurn()
		s.maybeWarnGameEnd()     // Duration game: "Notice: Game will end in 10 turns!" at the start of turn limit-9
		s.announceLeaderChange() // "<colour> team has the most Ecospheres!" when the lead changes hands
	}

	// The attract demo ends by itself after its turn cap: back to the menu.
	// C++: menu_check_end_game_conditions (menu.cpp, demo_turns capped at 5).
	if s.demoMode && s.game.State.TurnCounter-s.demoTurnStart >= demoTurnLimit {
		s.exitToMenu = true

		return
	}

	next := s.game.State.ActiveTeam

	// A bench's frozen enemy (locked still as a human team) passes its turn at once: its turn start and end
	// still run, but there is no hot seat banner and no turn to play, so End Turn comes straight back.
	// C++: n/a (debug benches).
	if s.benchFrozen(next) {
		s.game.BeginTeamTurn(next)
		dbg.Printf("turn %d : %s passes (frozen bench team)", s.game.State.TurnCounter, s.teamName(next))

		if !s.finishActiveTurn() {
			return // game over
		}

		s.beginNextTeam()

		return
	}

	before := s.selectableIDs(next)   // diff against the units this team's turn start construction adds
	goldBefore := s.teamCredits(next) // detect a first gold refine across this turn's economy
	s.buyAITurnUpgrades(next)         // a computer team spends its banked refinery gold before it plans
	s.game.BeginTeamTurn(next)        // economy + a turn of construction (the original: ManageEconomy at turn start), then a computer team's AI issues its orders
	s.announceTurnStart(next, before)
	s.checkGoldFirstRefined(next, goldBefore)

	// The attract demo watches over each team's shoulder: view and fog follow
	// the team whose turn plays. Announcements stay quiet, isHumanTeam checks
	// the team TYPE (all computer in a demo), not PlayerTeam.
	// C++ ported : game_manager.cpp - GameManager_TeamTurnTurnBased (GameManager_PlayerTeam = team)
	if s.demoMode {
		s.game.State.PlayerTeam = next
		s.centerViewOnTeam(next)
	}

	if s.game.State.Teams[next].Type == game.TeamComputer {
		// Animate instead of resolving instantly; PlayerTeam stays human so the
		// board keeps its view. tickComputerTurn finishes the turn and rolls on.
		s.turnFlow.aiTurn = true
		s.turnFlow.aiTurnTeam = next
		s.turnFlow.aiTurnStartMS = platform.Ticks() // start the human pace dwell for this turn
		dbg.Printf("turn %d : %s begin (computer)", s.game.State.TurnCounter, s.teamName(next))
		return
	}

	// Control returns to the human. If the announcer is still speaking the
	// opponents' end of turn lines, hold the turn closed until the voice queue
	// drains (tickComputerTurn's settle path then goes live).
	if s.turnFlow.aiTurn && s.endTurnVoicesPending() {
		s.turnFlow.handBackTeam = next
		s.turnFlow.handBackStartMS = s.now()
		dbg.Printf("turn %d : %s hand-back held for NPC end-turn voices", s.game.State.TurnCounter, s.teamName(next))
		return
	}

	s.goLiveForHuman(next)
}

// goLiveForHuman opens the human's interactive turn: input unlocks, End Turn
// releases, selection carries over, turn timer (re)starts. Hot seat raises the
// pass banner instead. "Begin!" is announced separately, once live.
func (s *Screen) goLiveForHuman(next int) {
	s.turnFlow.aiTurn = false
	s.game.State.PlayerTeam = next
	s.clearSelectionForNewTurn(next)

	// A new turn opens on the full time budget. Loading may resume a partially spent turn, but later turns use
	// the configured budget.
	if s.turnFlow.fullTurnSeconds > 0 {
		s.turnFlow.turnSeconds = s.turnFlow.fullTurnSeconds
	}
	s.tickAutoSurvey(next) // drive any Surveyor with Auto Survey on: base first, then the nearest unsurveyed cell

	if s.soleHuman() { // single player: resume at once instead of a pass banner every turn
		s.turnFlow.handover = -1
		s.turnFlow.turnStart = platform.Ticks()
	} else {
		s.turnFlow.handover = next
	}

	dbg.Printf("turn %d : %s begin", s.game.State.TurnCounter, s.teamName(next))
}

// endTurnVoicesPending reports whether the announcer is still speaking the
// opponents' end of turn lines; the hand back to the human waits on this.
func (s *Screen) endTurnVoicesPending() bool {
	if len(s.announcer.voiceQueue) > 0 {
		return true
	}

	if s.announcer.voicePlayingFn != nil {
		return s.announcer.voicePlayingFn()
	}

	return false
}

// tickAutoSurvey drives each ready Surveyor with AutoSurvey on over the ground around
// the base first, then toward the nearest unsurveyed cell (same scan the AI uses);
// turns itself off once nothing is left.
// C++ ported : units_manager.cpp - UnitsManager_Popup_OnClick_Auto.
func (s *Screen) tickAutoSurvey(team int) {
	for _, u := range s.game.Units.Units() {
		if u.Team != team || !u.AutoSurvey {
			continue
		}

		s.driveAutoSurvey(u, team)
	}
}

// driveAutoSurvey sends one Surveyor to the next unsurveyed cell, base first, clearing
// AutoSurvey once the map is fully revealed. Shared by the per turn tick and the
// popup Auto toggle (drives at once on enable).
func (s *Screen) driveAutoSurvey(u *game.UnitInfo, team int) {
	if u.Speed <= 0 || s.game.HasOrder(u) {
		return // no budget or already ordered this turn
	}

	if !s.game.NearestUnsurveyedMove(u, team) {
		u.AutoSurvey = false // map fully surveyed - turn off automatically
	}
}

// announceTurnStart surfaces the results of a team's turn start economy + build
// step: the "needs material" alert, completion voices (diffed against before),
// the build ready prompt, colony started, and storage full voices. Human team
// gated, so it is a no op for a computer team. Runs at turn START, matching
// GameManager_ManageEconomy's messages.
func (s *Screen) announceTurnStart(team int, before map[uint16]bool) {
	s.checkStorageFull(team) // pooled storage filled by this turn's economy

	if s.isHumanTeam(team) {
		for _, b := range s.game.LastStalledBuilders() { // out of material: tell the player to refill the builder
			s.showAlert(displayName(b.Type) + " needs material.") // red ALERT: insufficient material
		}

		// Buildings shut for lack of power (shed order: research, Eco Sphere, gold
		// refinery, factories; mining stays up): one line per type, world tint banner.
		if lost := s.game.LastPowerLost(); len(lost) > 0 {
			seen := make(map[assets.UnitType]bool)
			var lines []string

			for _, u := range lost {
				if seen[u.Type] {
					continue
				}

				seen[u.Type] = true
				lines = append(lines, displayName(u.Type)+" shut down - not enough power.")
			}

			s.showInfoLines(lines)
		}

		// Personnel buildings shut for lack of WORKERS (a habitat was destroyed).
		if lost := s.game.LastLifeLost(); len(lost) > 0 {
			seen := make(map[assets.UnitType]bool)
			var lines []string

			for _, u := range lost {
				if seen[u.Type] {
					continue
				}

				seen[u.Type] = true
				lines = append(lines, displayName(u.Type)+" shut down - not enough workers.")
			}

			s.showInfoLines(lines)
		}

		// Generators the optimizer brought online on its own this turn because power
		// demand rose (queuing production or powering a consumer on can pull an idle
		// generator back in between turns). The transient on/off SFX already play on the
		// player's OWN toggle; this surfaces the otherwise silent AUTO management so a
		// generator coming back "because power is needed" is not a surprise. Message only:
		// the spin up hum is a long clip that must not fire and forget on an unselected,
		// possibly off screen generator (it would ring on), so it stays silent like the
		// shed banners above.
		if on := s.game.LastGeneratorsPoweredOn(); len(on) > 0 {
			seen := make(map[assets.UnitType]bool)
			var lines []string

			for _, u := range on {
				if seen[u.Type] {
					continue
				}

				seen[u.Type] = true
				lines = append(lines, displayName(u.Type)+" powered on - power demand increased.")
			}

			s.showInfoLines(lines)
		}
	}

	s.announceBuildsCompleted(team, before)
	s.announceBuildReady(team) // "Finished building X - click a marker to move the builder out"

	// A path build stopped before a cell its builder cannot afford. The warning comes
	// after the ready prompt so it is the banner left on screen.
	if s.isHumanTeam(team) && len(s.game.LastPathShortBuilders()) > 0 {
		s.showAlert(s.lang.Get("c529", "WARNING: insufficient material in storage to start construction."))
	}

	s.checkColonyStarted() // a team's first working Eco Sphere -> colony started announce
}

// clearSelectionForNewTurn keeps the human's selection alive across End Turn and
// the opponents' turns, dropping it only if nothing selected, it's not yours, or
// it died meanwhile (a stale pointer). C++: the active unit stays selected across
// turn advance; only order input is gated while another team moves.
func (s *Screen) clearSelectionForNewTurn(next int) {
	if s.selected == nil || s.selected.Team != next { // nothing selected, or another team's
		s.clearSelected()

		return
	}

	if s.game.Units.ByID.Get(s.selected.ID) != s.selected { // destroyed meanwhile: stale pointer
		s.clearSelected()

		return
	}
}

// tickComputerTurn advances an in progress computer turn one frame: it animates
// the AI's move orders through the same per frame stepping a human's moves use, so
// units drive on screen instead of teleporting. Once every move plays out, it
// finishes the turn and rolls control on. C++: the AI acting during GameManager_ProgressTurn.
func (s *Screen) tickComputerTurn() {
	// Settle: we owe the human a turn but the announcer is still speaking the
	// opponents' end of turn lines. Hold, bounded by handBackMaxMS.
	if s.turnFlow.handBackTeam != -1 {
		if s.endTurnVoicesPending() && s.now()-s.turnFlow.handBackStartMS < handBackMaxMS {
			return // NPCs still wrapping up: keep the turn closed
		}

		s.goLiveForHuman(s.turnFlow.handBackTeam)
		s.turnFlow.handBackTeam = -1

		return
	}

	s.tickMovementSfx() // step the sim one frame: AI units drive (collision checked)

	if s.game.ActiveOrders() > 0 {
		return // still moving
	}

	// Moves resolved, but hold for a minimum dwell so a fast computer turn does not
	// snap past instantly; reads as the NPC "thinking" at a human like pace.
	if platform.Ticks()-s.turnFlow.aiTurnStartMS < aiTurnPaceMS {
		return
	}

	if !s.finishActiveTurn() { // dwell elapsed, AI idle: finish the turn and roll on
		s.turnFlow.aiTurn = false
		return // game over during the computer turn
	}

	s.beginNextTeam()
}

// handBackMaxMS bounds the hand back wait for the opponents' end of turn voices,
// a safety net so a wedged voice channel can never freeze the turn.
const handBackMaxMS = 12000

// aiTurnPaceMS is the minimum wall clock duration of a computer turn before it
// hands on: a "thinking" dwell so a fast AI plays at a human like pace without
// slowing the sim itself (moves still animate at full speed).
const aiTurnPaceMS = 1500

// Stall tracer thresholds (debug gated): aiFrameSlowMS logs a slow frame census;
// aiStallDumpMS treats a stuck frame as a hard lock and dumps every goroutine
// stack; stallPollMS is the watchdog's sample interval.
const (
	aiFrameSlowMS = 250
	aiStallDumpMS = 3000
	stallPollMS   = 500
)

// runStallWatchdog polls the Run loop heartbeat and, when it has not advanced for
// aiStallDumpMS, dumps every goroutine stack via dbg (a frozen loop cannot log its
// own diagnostics). Dumps once per distinct stall; exits when done closes. The
// stack walk is skipped unless debugging is enabled (idle cost: one atomic load).
func (s *Screen) runStallWatchdog(done <-chan struct{}) {
	ticker := time.NewTicker(stallPollMS * time.Millisecond)
	defer ticker.Stop()

	var lastBeat int64
	dumped := false

	for {
		select {
		case <-done:
			return

		case <-ticker.C:
			// A blocking modal legitimately parks the Run loop (heartbeat frozen), not a
			// hang. Skip and re arm; a fresh beat lands the instant the modal closes.
			if s.turnFlow.wdModalDepth.Load() > 0 {
				dumped = false

				continue
			}

			beat := s.turnFlow.heartbeat.Load()

			if beat != lastBeat {
				lastBeat = beat
				dumped = false // the loop advanced: re arm for the next stall
			}

			// Fires on any frame wedged this long, not only computer turns (a freeze
			// can hit the player's own turn or right after a load).
			if dumped || !dbg.Enabled() {
				continue
			}

			if stuckMS := (time.Now().UnixNano() - beat) / 1e6; stuckMS >= aiStallDumpMS {
				buf := make([]byte, 1<<20)
				n := runtime.Stack(buf, true)
				dbg.Printf("STALL-DRAIN: Run loop frozen %dms (aiTurn=%v) - goroutine dump follows:\n%s", stuckMS, s.turnFlow.wdInAITurn.Load(), buf[:n])
				dbg.Flush() // the loop is wedged; force the batch to disk now
				dumped = true
			}
		}
	}
}

// soleHuman reports whether exactly one team is human controlled (a single player
// skirmish); the hot seat banner is only meaningful with two or more humans. A
// bench's frozen enemy plays as a human but never takes a turn, so it does not count.
func (s *Screen) soleHuman() bool {
	humans := 0

	for i := 0; i < game.TeamCount; i++ {
		if s.game.State.Teams[i].Type == game.TeamPlayer && !s.benchFrozen(i) {
			humans++
		}
	}

	return humans == 1
}

// finishActiveTurn runs the active team's end of turn processing (economy, a turn
// of construction, research, and the win/loss check). It returns false when the
// game has ended. C++: the per team step of GameManager_TeamTurnTurnBased.
func (s *Screen) finishActiveTurn() bool {
	active := s.game.State.ActiveTeam
	var elapsed int

	if now := platform.Ticks(); now > s.turnFlow.turnStart {
		elapsed = int((now - s.turnFlow.turnStart) / 1000)
	}

	dbg.Printf("turn %d : %s end %s", s.game.State.TurnCounter, s.teamName(active), formatClock(elapsed))
	s.syncResearchOrders(active) // idle research centres with no allocated topic so they draw no power

	// Economy and construction run once per team at turn start (BeginTeamTurn / announceTurnStart). They do not
	// run again here.
	s.advanceResearch(active) // a turn of research; level ups upgrade the team's units
	s.game.EndTeamTurn(active)

	// Announce whoever just finished - vanilla speaks "<colour> team has ended turn!"
	// (F279..F282) for EVERY team's end of turn, including the player's own, so pressing
	// End Turn always makes a sound.
	s.teamEndedTurnVoice(active)

	if s.game.State.Teams[active].Type == game.TeamPlayer && !s.benchFrozen(active) {
		s.maybeAutoSave() // Auto-Save: silent save at the player's end of turn
	}

	if !s.debugMenu.endless { // the F2 "Endless mode" flag suppresses the win/loss check

		if o := s.game.Outcome(); o != winloss.Continue {
			s.enterGameOver(o)
			return false
		}
	}

	return true
}

// tickIdlePhase runs the once per frame work for whichever phase the screen is in:
// mouse handling, movement, animation and the redraw. During the landing sequence only
// the descent animates, with input and the sim held, and the shared draw at the end of
// the caller's loop still runs.
func (s *Screen) tickIdlePhase() {
	// Snapshot the force attack modifier once for this frame, so the click and
	// cursor paths agree on a stable value (the same once per frame pattern as the
	// mouse snapshot).
	s.orderInput.forceMod = s.plat != nil && s.plat.ShiftOrCtrlHeld()
	s.orderInput.shiftMod = s.plat != nil && s.plat.ShiftHeld()

	// Attract demo: a fresh mouse click leaves the demo in every phase
	// (the key gate above covers the keyboard).
	if s.demoMode {
		s.demoMouseExit()
	}

	switch s.phase() {
	case phaseLeaving:
		s.tickLeaving() // panels slide closed, then exit to the menu

	case phaseSiteSelect:
		if s.siteSelect.introStart != 0 {
			s.tickSiteIntro() // confirmed: grow the rings, then deploy + descend
		} else {
			s.handleSiteSelectMouse() // pan + click to land
		}

	case phaseLanding:
		s.tickLanding()

	default:
		// Simultaneous (real time) mode: open the round's plan phase the first
		// time the board is live (game start, or after a load). From then on the
		// human plays live in the branch below while every team's queued orders
		// (the AIs planned at round start) animate together in real time.
		if s.simMode() && !s.turnFlow.simPlanned {
			s.beginSimRound()
		}

		// Attract demo: no human ever opens the turn cycle, so arm the
		// active computer team's animation the moment the board is live.
		if s.demoMode {
			s.demoTickIdle()
		}

		if s.turnFlow.aiTurn {
			// Computer turn: it drives like the player; orders are blocked but the
			// view still pans/zooms and read only controls stay usable.
			mouse := s.plat.Mouse()

			if !s.handleViewControls(mouse, s.plat.ConsumeWheelY()) {
				s.handleSpectatorInput(mouse)
			}

			s.tickComputerTurn()
			s.surveyTick()             // an AI surveyor still reveals resources as it moves
			s.checkEnemyDetected()     // alert the human as AI units come into view
			s.checkSubmarineDetected() // "Submarine detected!" as a hostile sub enters the player's sonar
			s.updateSubmarines()
			s.updateBridges() // a bridge raises as an AI boat passes under, lowers after
		} else if s.turnFlow.endTurnSettling {
			// End Turn was pressed with a move still sliding: animate it to its stopping
			// point before the commit snaps it there. Orders are locked (like a computer
			// turn), but the view still pans/zooms. Commit once every mover has settled,
			// or after the safety cap so a re pathing move cannot freeze the turn.
			mouse := s.plat.Mouse()

			if !s.handleViewControls(mouse, s.plat.ConsumeWheelY()) {
				s.handleSpectatorInput(mouse)
			}

			s.tickMovementSfx() // slide the mover on and fire its drive/stop SFX
			s.surveyTick()
			s.updateSubmarines()
			s.updateBridges()

			if active := s.game.ActiveOrders(); active == 0 {
				s.commitTurn()
			} else if platform.Ticks()-s.turnFlow.endTurnSettleMS >= endTurnSettleMaxMS {
				dbg.Printf("endTurn: settle CAPPED at %dms with %d order(s) still active - forcing the commit", endTurnSettleMaxMS, active)
				s.commitTurn()
			}
		} else {
			s.handleMouse()
			s.tickMovementSfx()        // advances sim one frame and fires drive/stop SFX
			s.surveyTick()             // surveyors reveal the resources around them as they move
			s.announceTurnBegin()      // "Begin!" once the human player's interactive turn is live
			s.checkEnemyDetected()     // "Enemy detected!" when a hostile unit first comes into view
			s.checkSubmarineDetected() // "Submarine detected!" when the player's sonar reveals a hostile sub
			s.updateSubmarines()       // submarines surface when spotted by an enemy / re submerge otherwise
			s.updateBridges()          // a bridge raises as a sea unit passes under, lowers after
		}
	}
}
