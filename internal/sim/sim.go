// sim.go : orchestrates the headless turn loop, unit registry, pathfinding, and movement.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : game_manager.cpp - turn machinery (non UI parts)
package sim

import (
	"sort"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/vis"
	"github.com/suglasp/max-go/internal/winloss"
	"github.com/suglasp/max-go/internal/world"
)

// Game is one in progress match: the turn state, the unit registry, the
// passability grid used for routing, and the active movement orders. It has
// no goroutine and no I/O; the caller (the in game screen, a test, or the
// AI in M6) drives it through BeginTeamTurn / OrderMove / EndTeamTurn.
type Game struct {
	State     *game.GameState      // turn and team state
	Units     *units.Manager       // unit registry
	Grid      *access.Grid         // movement and placement grid
	Vis       *vis.View            // team visibility maps
	Resources *economy.ResourceMap // map resource deposits
	Complexes []*economy.Complex   // connected building complexes
	W, H      int                  // map dimensions
	Fast      bool                 // skips movement animation delays

	MiningDifficulty int                            // mining output divisor
	Networks         [game.TeamCount]*build.Network // team build networks and orders
	Gold             [game.TeamCount]int            // team gold reserves
	AI               TurnAI                         // computer turn driver
	MakeUnit         UnitFactory                    // unit factory used by production and AI

	Mission           game.MissionCategory      // which KIND of mission is running (MissionNone = a free game)
	MissionIndex      int                       // WHICH mission of that kind (the retail header's mission_index; 0 = no scripted objective)
	VictoryType       winloss.VictoryType       // selected victory condition
	VictoryLimit      int                       // victory condition target
	TeamPoints        [game.TeamCount]int       // current team scores
	ScoreHistory      [game.TeamCount][50]int16 // recent team score snapshots
	WinLossConditions winloss.ConditionFunc     // optional outcome override

	TeamStats [game.TeamCount]TeamStats // end game team statistics

	orders    []*units.MoveOrder  // active move orders
	builds    []*build.BuildOrder // active build orders
	nextBuilt int                 // next built unit ID

	spotted spottedMemory // last seen enemy positions

	pending map[*game.UnitInfo][]paths.Step // unfinished unit routes
	held    map[*game.UnitInfo][]paths.Step // routes HELD by a halt on enemy stop: shown, but never auto resumed (heldroute.go)

	arrivalShots map[*game.UnitInfo]*game.UnitInfo // shots queued to fire when a mover parks

	reachLabels map[uint8][]int32 // cached terrain components by movement mask

	buildExits map[*game.UnitInfo][2]int // exit cells for computer builders

	buildReady map[*game.UnitInfo]*BuildReady // completed builds awaiting acknowledgement

	buildPaths map[*game.UnitInfo]*buildPath // active engineer path builds

	pendingExpand  map[*game.UnitInfo]*game.UnitInfo // buildings waiting for their builder to move
	expandReleased []*game.UnitInfo                  // buildings released to unfold

	mineBlasts []MineBlast // queued mine explosions
	minesFound []MineFound // enemy mines a detector just uncovered, for the screen to announce

	casualties     []Casualty      // queued instant attack deaths
	damage         []Damage        // queued instant attack hits the target survived
	reactions      []ReactionEvent // queued reaction fire
	commandoEvents []CommandoEvent // queued commando captures/disables/exposures

	lossSites map[int][]LossSite // recent mobile unit losses by team

	reactOnReveal  bool             // enables reaction fire on newly seen units
	pendingReveals []revealReaction // newly visible enemy pairs

	disabled map[*game.UnitInfo]int // remaining disabled turns by unit

	rubble   map[[2]int]rubbleSpot        // rubble by anchor cell
	clearing map[*game.UnitInfo]*clearJob // active bulldozer clearing jobs

	projectiles       []*Projectile      // missiles in flight
	projectileImpacts []ProjectileImpact // queued missile impacts

	turnStalled []*game.UnitInfo // builders stalled this turn

	turnPowerLost []*game.UnitInfo // units shut down for power this turn

	turnLifeLost []*game.UnitInfo // units shut down for workers this turn

	turnGeneratorsOn []*game.UnitInfo // generators enabled this turn

	prodEvents []ProdEvent // queued production state changes

	colonyActive [game.TeamCount]bool // teams with an operating Eco Sphere
}

// TurnAI is the seam the sim uses to hand a TeamComputer team to the AI at its
// turn start, mirroring game_manager.cpp Ai_BeginTurn. Defined here (not in
// internal/ai) so the sim never imports the AI package: the AI calls down into
// the game core, not the reverse.
type TurnAI interface {
	// BeginTurn lets the AI plan and issue the team's orders at turn start;
	// they resolve at turn end like a human's. C++: AiPlayer::BeginTurn.
	BeginTurn(g *Game, team int)
}

// UnitFactory mints a fresh, unplaced, fully statted unit of a type for a team.
// Game.MakeUnit holds the same closure given to the AI, so the sim can turn out
// a factory's queued units itself.
type UnitFactory func(team int, t assets.UnitType, flags game.Flags) *game.UnitInfo

// PendingRoute is a unit and the remaining cells of a move it could not finish
// this turn (queued for next turn); the in game screen draws these as arrows.
type PendingRoute struct {
	Unit  *game.UnitInfo // unit following the route
	Steps []paths.Step   // remaining route cells
}

// New builds a game over a loaded map and the pre game StartParams. The
// passability grid reads the map surface and the live unit hash.
func New(p *game.StartParams, m *world.Map) *Game {
	mgr := units.NewManager()
	v := vis.NewView(m.W, m.H)
	v.Surf = m
	return &Game{
		State: game.NewGameState(p),
		Units: mgr,
		Grid:  &access.Grid{W: m.W, H: m.H, Surf: m, Units: mgr.Cells},
		Vis:   v,
		W:     m.W,
		H:     m.H,
	}
}

// NewWithGrid builds a game over an explicit grid and dimensions, for tests
// and the AI harness that supply a synthetic map.
func NewWithGrid(state *game.GameState, mgr *units.Manager, grid *access.Grid, w, h int) *Game {
	v := vis.NewView(w, h)
	v.Surf = grid.Surf // access.SurfaceSource also satisfies vis.SurfaceSource
	return &Game{State: state, Units: mgr, Grid: grid, Vis: v, W: w, H: h}
}

/* // note: unused code in Go port
// unitCost builds the routing cost function for one unit (terrain + unit
// blocking through the access grid).
func (g *Game) unitCost(u *game.UnitInfo) paths.CostFunc {
	return paths.GridCost(g.Grid, g.unitDesc(u), u.Team, access.SameClassBlocks)
}
*/

// BeginTeamTurn refreshes a team for a fresh turn: clears finished_turn and
// refills every team unit's movement budget (ResetMovement).
// C++: unitinfo.cpp speed = ATTRIB_SPEED reset + finished_turn = false at turn
// start (GameManager_TeamTurnTurnBased).
func (g *Game) BeginTeamTurn(team int) {
	g.State.Teams[team].FinishedTurn = false
	g.State.Phase = game.PhaseInGame
	g.reactOnReveal = true // turns have begun: a fog lift on an in range enemy now reveal fires (setup did not)
	for _, u := range g.Units.Units() {
		if u.Team == team {
			u.ResetMovement()
			u.ResetCombat()
			refreshCarried(u)
		}
	}
	g.tickDisabled(team) // re zero still disabled units' budget and count them down
	g.tickClearing(team) // advance any bulldozer clearing rubble (occupies it this turn)
	g.RefreshVisibility()
	g.RunEconomy(team)
	g.turnStalled = g.AdvanceBuilds(team)
	g.issueBuildExits(team)
	g.resumePending(team)
	g.resumePursuit(team)     // continue any 2+-turn move to attack chase now budgets refilled
	g.clearArrivalShots(team) // a shot queued last turn and never taken belongs to last turn
	// Hand a computer team to the AI once its economy and construction have run,
	// so it plans against current resources. Its orders resolve at turn end like
	// a human's. nil AI (or a non computer team) just proceeds. C++: Ai_BeginTurn.
	if g.AI != nil && g.State.Teams[team].Type == game.TeamComputer {
		g.AI.BeginTurn(g, team)
	}
}

// ReplanTeamTurn gives a computer team a new plan after loading mid-round. It skips turn-start economy, construction, and budget refresh because
// those already ran before the save. Move orders are not saved, so human and empty teams need no work.
func (g *Game) ReplanTeamTurn(team int) {
	if g.AI != nil && team >= 0 && team < game.TeamCount && g.State.Teams[team].Type == game.TeamComputer {
		g.AI.BeginTurn(g, team)
	}
}

// refreshCarried refills the movement and combat budgets of the units stored
// inside u, recursing into a carried transport. Carried units live OFF the unit
// registry, so the BeginTeamTurn sweep cannot reach them; the original keeps
// stored units in its main lists and refreshes them like any other, which is
// what lets an unload keep "the turn's remaining speed" (unitinfo.cpp speed
// reset at turn start + ProcessUnloading/ActivateUnit not touching it).
func refreshCarried(u *game.UnitInfo) {
	for _, c := range u.Carried {
		c.ResetMovement()
		c.ResetCombat()
		refreshCarried(c)
	}
}

// LastStalledBuilders returns the builders that ran out of material on the most
// recent BeginTeamTurn's construction pass. The in game screen reads it right
// after BeginTeamTurn to raise the "needs material" alert (the build step now
// runs once, at turn start, matching the original's GameManager_ManageEconomy).
func (g *Game) LastStalledBuilders() []*game.UnitInfo {
	return g.turnStalled
}

// resumePending re issues a move order for each of team's units holding a
// leftover route, re pathfinding (terrain/units may have shifted) to the
// route's final cell. An unreachable destination drops the queued route.
func (g *Game) resumePending(team int) {
	if len(g.pending) == 0 {
		return
	}
	for u, steps := range g.pending {
		if u.Team != team || len(steps) == 0 {
			continue
		}
		// A builder in its build pose or parked build ready must never be auto driven
		// by a leftover route: that skips ExitBuildSite/clearBuildReady and strands it
		// under its own structure. Drop the stale route so it cannot fire later either.
		if u.Building || g.buildReady[u] != nil {
			delete(g.pending, u)
			dbg.Printf("resume: skip build-ready/build-pose %s at (%d,%d); dropped stale queued route", u.Type.Tag(), u.GridX, u.GridY)
			continue
		}
		dest := steps[len(steps)-1].To
		delete(g.pending, u) // OrderMove starts a fresh, current route
		g.OrderMove(u, dest.X, dest.Y)
	}
}

// Restore rebuilds live state from a saved unit set: clears units/orders/
// complexes, deploys the saved units as is (no slab re lay), rejoins buildings
// into their team network, and refreshes visibility.
// C++: post load fixups after SaveLoadMenu reads the unit lists (saveload.cpp).
func (g *Game) Restore(units []*game.UnitInfo) {
	for _, u := range g.Units.Units() {
		g.Units.Remove(u)
	}
	for i := range g.Networks {
		g.Networks[i] = nil
	}
	g.Complexes = nil
	g.orders = nil
	g.builds = nil

	for _, u := range units {
		g.Units.Deploy(u, u.GridX, u.GridY)
	}
	for _, u := range units {
		// Alien derelict buildings carry no economy, so - like PlaceBuilding's live
		// path - they never join a building network (g.Networks is sized [TeamCount],
		// indexing it by the alien team would panic). Match that skip on restore.
		if u.Flags.Has(game.FlagStationary) && !u.Flags.Has(game.FlagGroundCover) && !game.IsAlien(u.Team) {
			g.network(u.Team).Add(u)
		}
	}
	g.syncComplexes()
	g.RefreshVisibility()
	g.invalidateReach() // restored buildings define fresh terrain connectivity
}

// RefreshVisibility recomputes every team's scan coverage and fog from the
// current unit positions. Call it after units move or spawn.
//
// C++: the per unit Access_UpdateMapStatus sweep (access.cpp).
func (g *Game) RefreshVisibility() {
	if g.Vis != nil {
		g.Vis.Rebuild(g.Units.Units())
	}
	g.sweepMineDetectors() // a surveyor / mine layer / commando uncovers the enemy mines it drives past
	g.updateSpotted()      // refresh each team's spotted memory from the fresh fog
}

// EndTeamTurn finishes team's turn: it plays out any pending movement and
// marks the team done (handing the active slot on in turn based play).
//
// C++: GameManager_RefreshOrders setting finished_turn (game_manager.cpp).
func (g *Game) EndTeamTurn(team int) {
	g.ResolveMovement()
	g.UpdateEliminations()
	g.accrueScore(team) // each operating Eco Sphere earns the team a point this turn
	g.State.Phase = game.PhaseEndTurn
	g.State.FinishTurn(team)
}

// RunRounds plays n full turns of every active team. Before each team acts, perTeam
// (if set) is called to issue that team's orders; movement is then resolved and the
// team finished. After all active teams act, the turn rolls over.
// C++: GameManager_TeamTurnTurnBased looped by GameManager_GameLoop.
func (g *Game) RunRounds(n int, perTeam func(g *Game, team int)) {
	for r := 0; r < n; r++ {
		for _, team := range g.activeTeams() {
			g.BeginTeamTurn(team)
			if perTeam != nil {
				perTeam(g, team)
			}
			g.EndTeamTurn(team)
		}
		g.AdvanceTurn()
	}
}

// activeTeams lists the active team indices in ascending order.
func (g *Game) activeTeams() []int {
	var out []int
	for t := g.State.FirstActiveTeam(); t != -1; t = g.State.NextActiveTeam(t) {
		out = append(out, t)
	}
	sort.Ints(out)
	return out
}
