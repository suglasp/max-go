// ai.go : the M.A.X. unit AI: task manager, per team players, and the turn hook.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// AI is the computer opponent for a game: one shared TaskManager plus a per team
// AiPlayer. It implements sim.TurnAI, so a game wired with `game.AI = ai.New()`
// hands each computer team's turn here. C++: the Ai_* free functions in ai.cpp
// over the global TaskManager and AiPlayer_Teams[] array.
type AI struct {
	mgr           *TaskManager
	players       [game.TeamCount]AiPlayer
	ctx           *aiContext
	makeUnit      sim.UnitFactory
	inited        bool
	disabled      bool
	difficulty    int       // current opponent tier (0..5); see difficulty.go
	difficultySet bool      // whether setup or the F2 menu selected the tier
	initedGame    *sim.Game // game instance used to initialise the players
}

// New builds an uninitialised AI. makeUnit mints fully statted units for the
// tasks (injected from the game layer; see sim.UnitFactory). The per team players
// are set up lazily on the first turn so the AI sees the final team/clan
// configuration. The difficulty starts at AVERAGE (the default tier); the launch
// path / F2 debug menu adjust it via SetDifficulty. C++: the AI is brought up by
// Ai_Init at game setup.
func New(makeUnit sim.UnitFactory) *AI {
	return &AI{mgr: NewTaskManager(), makeUnit: makeUnit, difficulty: OpponentAverage}
}

// SetDifficulty sets the computer opponent tier (0..5 Clueless..God) and applies it
// to the AI knobs. A single global tier drives every computer team, matching
// vanilla M.A.X.'s one "opponent" setting. Called from the launch path (custom game
// setting / -opponent flag) and the F2 debug menu's difficulty row.
func (a *AI) SetDifficulty(d int) {
	if d < 0 {
		d = OpponentAverage
	}

	if d >= OpponentCount {
		d = OpponentCount - 1
	}

	a.difficulty = d
	a.difficultySet = true
	applyDifficulty(d)

	// Record the tier on the persisted game state so a save carries it and the F2
	// debug change survives a reload. init reads it back the same way, so the game
	// state stays the single source of truth for the AI's strength. At launch the AI
	// has no game handle yet (ctx is nil), so init does this capture instead.
	if a.ctx != nil && a.ctx.Game != nil && a.ctx.Game.State != nil {
		a.ctx.Game.State.Difficulty = d
	}
}

// Difficulty returns the current opponent tier (0..5).
func (a *AI) Difficulty() int {
	return a.difficulty
}

// SetEnabled turns the AI planner on or off (the F2 debug "AI reasoning" toggle).
// When off, computer teams issue no new orders (BeginTurn returns early), so the AI
// stops thinking. The battlefield is NOT fully frozen, though: the sim still resumes
// each team's leftover routes and pursuits at turn start (BeginTeamTurn) and reaction
// fire still triggers, so NPC units already on the move keep going. Planner only by
// design. There is no C++ equivalent; it is a dev aid.
func (a *AI) SetEnabled(on bool) {
	a.disabled = !on
}

// IsEnabled reports whether the AI is currently planning turns.
func (a *AI) IsEnabled() bool {
	return !a.disabled
}

// Ensure the AI satisfies the sim hook at compile time.
var _ sim.TurnAI = (*AI)(nil)

// init brings up the task manager and every computer player once per game,
// reading each team's clan from the live game state. C++: Ai_Init.
func (a *AI) init(g *sim.Game) {
	a.mgr.Clear()

	a.ctx = &aiContext{Game: g, MakeUnit: a.makeUnit}

	// Reconcile the tier with the game state. A game that carries one (a loaded save,
	// or a mid game F2 change already written through) is adopted, which applies the
	// difficulty knobs. A game with none yet, but where the launch path already chose a
	// tier through SetDifficulty (before the AI had a game handle), captures that tier
	// onto the state so a save carries it. A never configured game stays unset, so its
	// default play (the launch default knobs) is not frozen into a save.
	switch {
	case g.State.Difficulty != game.DifficultyUnset:
		a.SetDifficulty(g.State.Difficulty)
	case a.difficultySet:
		g.State.Difficulty = a.difficulty
	}

	// Resolve each computer team's strategy in order, passing the strategies already
	// chosen so the scoring diversifies the AI teams (the original only diversifies
	// against other TEAM_TYPE_COMPUTER teams). A StrategyRandom (degenerate map) is not
	// a real choice, so it does not block one.
	decided := make([]Strategy, 0, game.TeamCount)

	for team := 0; team < game.TeamCount; team++ {
		clan := resolveClan(g.State.Teams[team].Clan)
		a.players[team].Init(team, clan, a.ctx)

		if g.State.Teams[team].Type != game.TeamComputer {
			continue
		}

		a.players[team].SelectStrategy(decided)

		if s := a.players[team].GetStrategy(); s != StrategyRandom {
			decided = append(decided, s)
		}
	}

	a.inited = true
	a.initedGame = g
}

// ResolveTeamClans assigns a concrete clan (1..8) to every playable team that was
// left ClanRandom, writing it into the live game state at setup - before the bases
// deploy - so the unit value layer and the building emblems read a real clan.
// Computer teams get a strategy flavored clan (SelectClan) so the AI fields a
// varied, thematic clan; a human or unconfigured team gets a uniform pick. Teams
// with an explicit clan (the clan select menu, a mission file) and the fixed alien
// slot are left untouched. Strategies are decided first (init) so a computer clan
// reflects the team's plan. C++: the TEAM_CLAN_RANDOM resolution in
// ResourceManager_InitClanUnitValues plus AiPlayer::SelectTeamClan at game setup.
func (a *AI) ResolveTeamClans(g *sim.Game, rng *game.Randomizer) {
	if !a.inited {
		a.init(g) // decide each computer team's strategy first
	}

	for team := 0; team < game.TeamCount; team++ {
		if g.State.Teams[team].Clan != game.ClanRandom {
			continue
		}

		clan := rng.Generate(8) + 1 // human / unconfigured: uniform pick
		if g.State.Teams[team].Type == game.TeamComputer {
			clan = a.players[team].SelectClan(rng)
		}

		g.State.Teams[team].Clan = clan
		a.players[team].clan = clan // keep the strategy fallback clan consistent
	}
}

// BeginTurn plans one computer team's turn and drains its task reminders until the player can finish. C++: Ai_BeginTurn and GameManager_ProgressTurn.
func (a *AI) BeginTurn(g *sim.Game, team int) {
	if g.State.Teams[team].Type != game.TeamComputer || a.disabled {
		return
	}

	// A load swaps in a fresh game instance behind the same AI; reinitialise against
	// it so the players replan from the restored board and the loaded opponent tier
	// is applied (init reads g.State.Difficulty).
	if a.initedGame != g {
		a.inited = false
	}

	if !a.inited {
		a.init(g)
	}

	a.ctx.Game = g // keep the tasks pointed at the live game each turn

	// The ledger takes a held unit only when it can take orders in the live game.
	a.mgr.readyForOrders = func(u *game.UnitInfo) bool { return unitReadyForOrders(g, u) }

	p := &a.players[team]
	p.BeginTurn(a.mgr)
	a.mgr.BeginTurn(team)

	// Hand the free units to the most urgent unit requests before the tasks plan.
	a.mgr.dispatchUnits(g, team)
	a.mgr.ExecuteReminders()

	// Drain the team's work: rerun reminders until the player is content to end
	// its turn, with a hard cap mirroring the engine's retry ceiling so an
	// unsatisfiable task can never spin forever. C++: AiPlayer::CheckEndTurn's
	// end_turn_retry_counter ceiling (~120).
	guard := 0
	for ; guard < 120; guard++ {
		if p.CheckEndTurn(a.mgr) {
			break
		}

		if !a.mgr.ExecuteReminders() {
			break
		}
	}

	// Report a team that cannot settle within the engine's retry limit.
	if guard >= 120 {
		dbg.Logf(dbg.AI, "team %d turn drain HIT the 120-pass cap (CheckEndTurn never content)", team)
	}

	// Close the turn. The end of turn pass currently dispatches to no ops: no task
	// overrides EndTurn, because this port folded the original's end of turn evaluation
	// (TaskManageBuildings::EndTurn's stock and power checks) into the start of turn
	// planning it drains above. The pass stays wired, and is cheap (one reminder per
	// task), so a task that does grow an EndTurn body is run without rediscovering this.
	a.mgr.EndTurn(team)
	a.mgr.ExecuteReminders()
}
