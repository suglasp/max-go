// aiplayer.go : the M.A.X. AI player and teams
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// Strategy is the high level plan a computer team commits to. It biases which
// units the team builds and how aggressively it expands and attacks. C++:
// enums.hpp AI_STRATEGY_*.
type Strategy int

const (
	StrategyRandom Strategy = iota
	StrategyDefensive
	StrategyMissiles
	StrategyAir
	StrategySea
	StrategyScoutHorde
	StrategyTankHorde
	StrategyFastAttack
	StrategyCombinedArms
	StrategyEspionage
)

// Clan ids, matching game.Team.Clan (1..8) and the C++ TEAM_CLAN_* enum. Used
// to pick a default strategy when one was not chosen by analysis. C++:
// enums.hpp TEAM_CLAN_*.
const (
	clanTheChosen    = 1
	clanCrimsonPath  = 2
	clanVonGriffin   = 3
	clanAyersHand    = 4
	clanMusashi      = 5
	clanSacredEights = 6
	clanSevenKnights = 7
	clanAxisInc      = 8
)

// AiPlayer owns one computer team's strategy and the tasks that grow its base and control its units. C++: class AiPlayer (aiplayer.cpp / .hpp).
type AiPlayer struct {
	team                int
	clan                int
	needInit            bool
	strategy            Strategy
	goal                winGoal
	coastal             bool // base sits near water: field a shipyard + a navy
	endTurnRetryCounter int
	ctx                 *aiContext
	/* // note: ported but unused code in Go port - the tasks pending flag, see ChangeTasksPendingFlag below
	tasksPending bool
	*/
}

// Init resets the player for a new game. C++: AiPlayer::Init.
func (p *AiPlayer) Init(team, clan int, ctx *aiContext) {
	p.team = team
	p.clan = clan
	p.needInit = true
	p.strategy = StrategyRandom
	p.endTurnRetryCounter = 0
	p.ctx = ctx
}

// SelectStrategy chooses the team's strategy from the map's landmass layout scaled
// by the opponent difficulty: it scores the nine strategies (findContinents +
// scoreStrategies) and weighted rolls one, diversifying against the strategies the
// sibling computer teams already took (decided). A degenerate map (no viable
// landmass) leaves the strategy Random, which the first BeginTurn resolves to the
// clan default. C++: AiPlayer::SelectStrategy (the terrain scoring half; see
// strategy.go for the reductions).
func (p *AiPlayer) SelectStrategy(decided []Strategy) {
	p.strategy = StrategyRandom
	if p.ctx == nil || p.ctx.Game == nil {
		return
	}

	continents := findContinents(p.ctx.Game)
	if len(continents) == 0 {
		return // no viable landmass: BeginTurn's clan fallback resolves the Random
	}

	p.strategy = pickStrategy(scoreStrategies(continents, aiTier), decided)
	dbg.Printf("ai: team %d strategy = %s (continents=%d, tier=%d)", p.team, strategyName(p.strategy), len(continents), aiTier)
}

// GetStrategy returns the committed strategy. C++: AiPlayer::GetStrategy.
func (p *AiPlayer) GetStrategy() Strategy {
	return p.strategy
}

// The tasks pending flag has no home in this port and is commented out rather than
// wired. In the original, Ai_SetTasksPendingFlag (ai.cpp:141) raises it from about
// forty UNIT EVENT sites (a unit moves, fires, transforms, starts a build) because the
// C++ AI turn runs across frames while the units animate, so CheckEndTurn must not end
// a turn whose orders are still resolving. This port plans the whole turn synchronously
// inside BeginTurn and only then animates, so no unit event can fire while the flag
// would matter, and the "wait until the orders resolve" job is done by the live loop's
// ActiveOrders gate (gamescreen.go, aiTurn). Wiring it faithfully would need sim -> ai
// event callbacks, which the one way ai -> sim seam does not allow.
/* // note: ported but unused code in Go port
func (p *AiPlayer) ChangeTasksPendingFlag(value bool) {
	p.tasksPending = value
}
*/

// BeginTurn performs the team's start-of-turn planning. The first call resolves a Random strategy and registers the team's root tasks. C++: AiPlayer::BeginTurn.
func (p *AiPlayer) BeginTurn(mgr *TaskManager) {
	p.endTurnRetryCounter = 0

	if p.needInit {
		if p.strategy == StrategyRandom {
			p.strategy = strategyForClan(p.clan)
		}
		// Append the team's root economy task (C2). C3 adds the movement/combat
		// and defense reserve roots alongside it. C++: AiPlayer::BeginTurn's
		// need_init block appending TaskCheckAssaults + the managers.
		if p.ctx != nil {
			// Commit to a strategy: keep the one a loaded save carries, else persist the
			// one SelectStrategy already rolled (now clan resolved, never Random) so a
			// save captures it and a reload does not re-roll a different posture.
			p.strategy = resolveStrategy(p.ctx.Game, p.team, p.strategy)

			// Commit to a win goal: keep the one a loaded save carries, else roll one now
			// and write it back so a save captures it (resolveWinGoal; the same persist
			// through the game state pattern the difficulty tier uses, so a reload keeps
			// the AI's kill vs colonise plan instead of re-rolling it).
			p.goal = resolveWinGoal(p.ctx.Game, p.team, aiTier)

			// Read the terrain once: a coastal base goes sea oriented (shipyard + boats),
			// a landlocked one stays land only (and so a Sea clan landlocked team does not
			// sink effort into an unusable dock).
			p.coastal = baseIsCoastal(p.ctx.Game, p.team)
			dbg.Printf("ai: team %d win goal = %s, terrain = %s (difficulty tier %d)", p.team, goalName(p.goal), terrainName(p.coastal), aiTier)

			// The connection assistant is registered before the manager so it seeds a
			// connector site (when the base is split) ahead of the manager advancing its
			// builders the same turn. C++: TaskConnectionAssistant alongside the building
			// manager's other assistants.
			mb := newTaskManageBuildings(p.team, p.strategy, p.goal, p.coastal, p.ctx)
			mgr.AppendTask(newTaskConnectionAssistant(mb))
			mgr.AppendTask(mb)
			mgr.AppendTask(newTaskProduceUnits(p.team, p.strategy, p.ctx))
			mgr.AppendTask(newTaskSurvey(p.team, p.ctx))

			// Reclaim base area rubble (a destroyed building/unit's debris) so the base
			// can rebuild on the footprint; it obtains a bulldozer if the team has none.
			mgr.AppendTask(newTaskBulldozer(p.team, p.ctx))

			// Tasks run in registration order; their priority tier (priority.go) only
			// decides who keeps a unit in the unit ledger. Retreat runs before attack, so
			// a damaged unit is claimed and pulled back before the attack task would send
			// it in to die. The defence reserve then claims its home guards and the escort
			// a spare guard for each fragile mover; the special unit tasks (commando,
			// mines) run between them and attack. All of them come before the gun line
			// pass, so each claims its units first, and the explore task after the attack
			// only gets what is left.
			mgr.AppendTask(newTaskRetreat(p.team, p.ctx))
			mgr.AppendTask(newTaskDefenseReserve(p.team, p.ctx))
			mgr.AppendTask(newTaskEscort(p.team, p.ctx))
			mgr.AppendTask(newTaskCommando(p.team, p.ctx))
			mgr.AppendTask(newTaskMines(p.team, p.ctx))
			ferry := newTaskTransport(p.team, p.ctx)
			attack := newTaskAttackEnemies(p.team, p.ctx)
			attack.ferry = ferry
			mgr.AppendTask(attack)
			mgr.AppendTask(newTaskFireTurrets(p.team, p.ctx))
			mgr.AppendTask(newTaskExplore(p.team, p.ctx))

			// The ferry comes last: the manager runs transport tasks after the others, so
			// it sees every ride asked for in this turn.
			mgr.AppendTask(ferry)
		}

		p.needInit = false
	}

	// Switch on whatever construction handed over powered off, before the tasks plan
	// against it: a dark plant can take a production order but will never work it off.
	// C++ ported : aiplayer.cpp - the ORDER_POWER_ON sweep over the team's buildings.
	if p.ctx != nil {
		powerOnBuildings(p.ctx.Game, p.team)

		// Then put the centres that sweep just switched on to work. The order matches the
		// original's turn block: power the buildings, then choose the research projects.
		p.determineResearchProjects()
	}
}

// CheckEndTurn reports whether the team has finished its queued work. C++: AiPlayer::CheckEndTurn.
func (p *AiPlayer) CheckEndTurn(mgr *TaskManager) bool {
	if p.needInit {
		return false
	}

	/* // note: ported but unused code in Go port - nothing raises tasksPending, see ChangeTasksPendingFlag
	if p.tasksPending {
		p.endTurnRetryCounter = 0
		return false
	}
	*/

	if p.endTurnRetryCounter == 0 {
		mgr.MarkTasksForProcessing(p.team)
	}
	p.endTurnRetryCounter++

	// No queued reminders means no work remains: end the turn.
	return mgr.RemindersCount() == 0
}

// strategyForClan maps a team's clan to its default strategy, used when the
// strategy was left Random. C++: AiPlayer::BeginTurn (the AI_STRATEGY_RANDOM
// clan switch).
func strategyForClan(clan int) Strategy {
	switch clan {
	case clanTheChosen:
		return StrategyAir
	case clanCrimsonPath:
		return StrategySea
	case clanAyersHand:
		return StrategyMissiles
	case clanSacredEights:
		return StrategyFastAttack
	case clanSevenKnights:
		return StrategyDefensive
	default:
		return StrategyCombinedArms
	}
}

// resolveClan turns a team's configured clan (which may be ClanRandom) into a
// concrete clan id for strategy selection. A random clan defaults to the
// combined arms yielding branch.
func resolveClan(clan int) int {
	if clan == game.ClanRandom {
		return clanAxisInc // -> StrategyCombinedArms (the default branch)
	}

	return clan
}

// SelectClan resolves the team's clan from its committed strategy, so a computer
// team that left its clan Random gets a thematic, varied clan instead of a fixed
// default. Each strategy favours one clan with a 1 in 7 chance of the generalist
// Axis Inc. instead; combined arms spreads across Von Griffin / Ayer's Hand /
// Musashi. A Random or unknown strategy (e.g. a degenerate map) falls back to a
// uniform pick. C++ ported : aiplayer.cpp - AiPlayer::SelectTeamClan.
func (p *AiPlayer) SelectClan(rng *game.Randomizer) int {
	// orAxis returns clan 6 times in 7, else the generalist Axis Inc., matching
	// the original's `if (Randomizer_Generate(7)) clan; else AXIS_INC`.
	orAxis := func(clan int) int {
		if rng.Generate(7) != 0 {
			return clan
		}
		return clanAxisInc
	}

	switch p.strategy {
	case StrategyDefensive:
		return orAxis(clanSevenKnights)
	case StrategyMissiles:
		return orAxis(clanAyersHand)
	case StrategyAir:
		return orAxis(clanTheChosen)
	case StrategySea:
		return orAxis(clanCrimsonPath)
	case StrategyScoutHorde, StrategyFastAttack:
		return clanSacredEights
	case StrategyTankHorde:
		return clanMusashi
	case StrategyCombinedArms:
		if rng.Generate(7) != 0 {
			return clanVonGriffin + rng.Generate(3) // Von Griffin / Ayer's Hand / Musashi
		}
		return clanAxisInc
	case StrategyEspionage:
		if rng.Generate(7) != 0 {
			return clanCrimsonPath
		}
		return clanVonGriffin
	default: // StrategyRandom or unknown: uniform 1..8
		return rng.Generate(8) + 1
	}
}
