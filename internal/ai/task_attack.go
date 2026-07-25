// task_attack.go : implements AI target selection, attacks, and pursuit.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"sort"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// taskAttackEnemies is a team's combat brain: each turn it sends every ready armed
// mobile unit against the nearest enemy, firing if in range else advancing to a stand
// off firing position at weapon range. It does not reschedule itself (TaskManager.
// BeginTurn requeues it). The team only pursues enemies it has SPOTTED (SpottedUnit
// memory: an enemy stays remembered with its position frozen at the last sighting once
// fogged) and musters a minimum strike force before committing. Target choice weighs
// raid priority, distance and the threat map. Simplification: one action per unit per
// turn (fire OR move, not both). C++: TaskAttack + TaskMove + AiAttack.
type taskAttackEnemies struct {
	TaskBase
	ctx          *aiContext
	committed    bool                    // latches once the team spots an enemy and musters minStrikeForce attackers; until then it holds/scouts. C++: AiPlayer commits after recon+buildup
	searchMode   map[*game.UnitInfo]int  // per searching unit, HOW it hunts a lost target: searchZone sweep or searchExplore wander; rolled once per lost contact, cleared on reacquire
	tm           *threatMap              // team threat map for THIS turn (rebuilt in BeginTurn once committed); sortApproachSafest prefers the SAFEST in range cell. nil = rank by distance
	reserve      map[*game.UnitInfo]bool // this turn's home defence reserve (reservedGuards), excluded from the assault so the two tasks do not fight over them; nil on no reserve tiers
	escort       map[*game.UnitInfo]bool // this turn's escort guards (escortGuards) shadowing a fragile mover; excluded from the assault; nil on no escort tiers
	abandoned    map[*game.UnitInfo]int  // lost target -> turn shelved; nearestEnemy skips it while fogged until abandonCooldown passes or it is seen again. C++: AiPlayer aging out a stale SpottedUnit
	searchSince  map[*game.UnitInfo]int  // per searching unit, the turn it began hunting its CURRENT lost target, so it gives up after searchPatience; cleared on reacquire/retarget
	searchClaims map[int]bool            // reserves a lost contact cell for ONE searcher per turn (no stack all searching the same stale cell); rebuilt each BeginTurn
	focus        *game.UnitInfo          // this turn's single shared objective the committed force converges on (teamFocusTarget); nil before commit. C++: the shared AttackTask target
}

// minStrikeForce is how many armed mobile units a team must field before committing to
// an assault; below it the team builds up (idle scan units scout via taskExplore). A
// reduced stand in for AiPlayer's muster gate (aiplayer.cpp - DetermineAttack); a
// package var so tests can isolate geometry from the muster decision.
var minStrikeForce = 3

// aiDefensiveOnly, set by a low difficulty tier (applyDifficulty), keeps the team from
// ever marching out: it holds near base and only fires on enemies that come into range
// (reaction and in range defence are not gated). Default false (AVERAGE+ hunt once
// mustered). C++ ported : the low tier AI_STRATEGY_DEFENSIVE bias (aiplayer.cpp).
var aiDefensiveOnly = false

// The attack timeline knobs, set per difficulty tier by applyDifficulty: a buildup
// before a full assault, then a window of light harassment probes, then the assault.
// Defaults are a "no arc" state (no grace, no harassment, commit everything) so the
// combat tests (turn 1, never calling applyDifficulty) keep their behaviour.
var (
	aiHarassStart = neverAttack // turn light harassment probes begin (default: never)
	aiGraceTurns  = 0           // turn the full assault may begin (default: 0 = no grace floor)
	aiKeepReserve = false       // keep a home defence reserve during the assault (smarter tiers)
)

func newTaskAttackEnemies(team int, ctx *aiContext) *taskAttackEnemies {
	return &taskAttackEnemies{
		TaskBase: newTaskBase(team, nil, 800), // below the economy/production roots
		ctx:      ctx,
	}
}

// BeginTurn issues a fire or an advance for each ready attacker. Defence is always
// honoured (a ready attacker fires on a spotted in range enemy even before mustering),
// but the offensive advance waits until the team is committed (spotted + a minimum
// strike force). C++: the attack pass gated by AiPlayer's recon and muster.
func (t *taskAttackEnemies) BeginTurn() {
	g := t.ctx.Game
	if g == nil {
		return
	}

	ready, sortie := t.planAssault(g)

	sortied := 0
	for _, u := range ready {
		target := t.chooseTarget(u)
		if target == nil {
			t.endSearch(u) // nothing to hunt: end any search
			continue
		}

		// Act on where the team BELIEVES the enemy is: its true cell while visible,
		// else the frozen last seen cell - never the true position of a fogged unit.
		tx, ty := t.targetCell(target)

		if g.CanAttack(u, tx, ty) {
			t.endSearch(u)
			// Every unit, air or ground, fires from its current cell and holds
			// position; M.A.X. air units hover to attack, they do not run a strafing
			// pass over the target. The drive INTO range happens earlier (move to
			// attack); this is only the shot.
			g.Attack(u, tx, ty)

			continue
		}

		// Out of range: advance only within this turn's sortie budget; the rest hold
		// near base (still firing in range above) as the buildup or defence reserve.
		if sortied >= sortie {
			continue
		}

		sortied++
		if !t.fogged(target) {
			t.endSearch(u) // in contact: close straight in

			if t.advanceToward(u, tx, ty) {
				// The drive is a MOVE TO ATTACK: the shot goes off when this unit PARKS in
				// range, at the end of its animated drive, instead of arriving out of nowhere
				// at the next turn's planning pass. It also stops the assault throwing away a
				// turn of fire just getting into position. Only ever armed on a target the
				// team can SEE - a last seen ghost is marched at, never shot at.
				g.AttackOnArrival(u, target)
			}

			continue
		}

		// The enemy is out of sight. If already searching, keep at it - but give up (and
		// shelve the last seen cell) once this unit has hunted too long or the search area
		// is exhausted. Otherwise drive toward the last sighting, then begin a search once
		// it can get no closer.
		if mode, searching := t.searchMode[u]; searching {
			if g.State.TurnCounter-t.searchSince[u] > searchPatience || !t.runSearch(u, tx, ty, mode) {
				t.abandon(target)
				t.endSearch(u)
			}

			continue
		}

		if t.advanceToward(u, tx, ty) {
			dbg.Printf("ai: team %d %s pursues last seen %s at (%d,%d)", t.team, u.Type.Tag(), target.Type.Tag(), tx, ty)

			continue
		}

		// Can get no closer and not yet searching. Dedupe: only one unit begins a search
		// for a given stale cell per turn; the rest stay idle for the explore task to
		// spread them out (reacquiring from new angles instead of all crowding one cell).
		if t.claimSearchCell(tx, ty) {
			t.beginSearch(u, target, tx, ty)
		} else {
			t.endSearch(u)
		}
	}
}

// planAssault settles this turn's plan before any unit acts: who is held back, whether
// the team is committed, the threat map and shared objective that follow from that, how
// many units may advance, and the ready attackers in the order they get their turn.
func (t *taskAttackEnemies) planAssault(g *sim.Game) (ready []*game.UnitInfo, sortie int) {
	// The home defence reserve and the escort guards for this turn (empty unless a
	// reserve keeping or escort tier): these units are positioned by taskDefenseReserve
	// and taskEscort and skipped below so the assault does not steal them.
	t.reserve = reservedGuards(g, t.team)
	t.escort = escortGuards(g, t.team)
	t.searchClaims = nil // one searcher per stale cell, reset each turn
	committed := t.isCommitted()

	// Once committed, build this turn's threat map so advancing units pick the SAFEST
	// in range firing cell (the caution weighting), then pick the single objective the
	// whole force converges on (teamFocusTarget); both rebuilt every turn.
	if committed {
		t.tm = buildThreatMap(g, t.team)
		t.focus = t.teamFocusTarget()
		if t.focus != nil {
			fx, fy := t.targetCell(t.focus)
			dbg.Printf("ai: team %d focus = %s at (%d,%d)", t.team, t.focus.Type.Tag(), fx, fy)
		}
	} else {
		t.tm = nil
		t.focus = nil
	}

	// sortie is how many units may advance OFFENSIVELY this turn: 0 in the build phase or
	// a defensive tier, a small escalating probe in the harassment window, the whole force
	// in the full assault. Units beyond the budget hold near base (still firing in range).
	sortie = t.sortieBudget(g.State.TurnCounter, committed)

	// Take ready attackers in a stable order so the limited sortie budget is handed out
	// deterministically (not by random map order). Before the team commits, a long range
	// launcher leads the harassment probe (it pings from range and falls back); once
	// committed the whole force sorties, so order is moot.
	ready = t.readyAttackers(g)
	if !committed {
		t.launchersFirst(ready)
	}

	return ready, sortie
}

// isCommitted latches true once the team has BOTH spotted an enemy and assembled
// minStrikeForce armed mobile units. The latch never drops, so a temporary dip in
// strength does not make a fielded army march home. C++: AiPlayer::DetermineTargetTeam /
// DetermineAttack commit once recon finds the enemy and the team can assault.
func (t *taskAttackEnemies) isCommitted() bool {
	// A defensive tier never marches out to hunt; it only fires on enemies
	// already in range (defence and reaction fire are not gated by commitment).
	if aiDefensiveOnly {
		return false
	}

	if t.committed {
		return true
	}

	// Grace floor: no full assault before the buildup grace passes, even if the team
	// has already mustered - this guarantees the player an opening (only the lighter
	// harassment probes may sortie before then). The latch below can only be set, and
	// stay set, once the grace has elapsed.
	if t.ctx.Game.State.TurnCounter < aiGraceTurns {
		return false
	}

	g := t.ctx.Game
	spotted := false
	force := 0
	for _, u := range g.Units.Units() {
		if u.Team == t.team {
			if t.isArmedMobile(u) {
				force++
			}

			continue
		}

		if u.Team < 0 || u.Team >= game.TeamCount || u.GridX < 0 || u.GridY < 0 {
			continue
		}

		if t.spottedByTeam(u) {
			spotted = true
		}
	}

	if spotted && force >= minStrikeForce {
		t.committed = true
	}

	return t.committed
}

// sortieBudget is how many units may advance offensively this turn. Defensive tiers
// never sortie. Before commit it PROBES: nothing in the build phase, then a light ping
// (1 unit, rising to 2 past the buildup midpoint). Once committed it is the full armed
// force. A reserve keeping tier already withholds its guards upstream (taskDefenseReserve
// + isReadyAttacker), so the budget is not cut again here (that would double count).
// C++ ported : the DetermineAttack sortie plus the defense_reserve_task holdback.
func (t *taskAttackEnemies) sortieBudget(turn int, committed bool) int {
	if aiDefensiveOnly {
		return 0
	}

	if committed {
		return t.armedForce() // a loose cap; reserved guards are already excluded above
	}

	if turn < aiHarassStart {
		return 0 // build phase: no offence, only in range defence
	}

	if turn >= (aiHarassStart+aiGraceTurns)/2 {
		return 2 // late buildup: a heavier ping
	}

	return 1 // early buildup: a single light ping
}

// armedForce counts the team's armed mobile units (its military strength), the unit
// of account for the assault sortie size.
func (t *taskAttackEnemies) armedForce() int {
	n := 0

	for _, u := range t.ctx.Game.Units.Units() {
		if t.isArmedMobile(u) {
			n++
		}
	}

	return n
}

// readyAttackers collects the team's armed mobile units free to act this turn
// (isReadyAttacker), sorted by ID so the sortie rationing done each turn is
// deterministic rather than dependent on the random order from iterating Units().
func (t *taskAttackEnemies) readyAttackers(g *sim.Game) []*game.UnitInfo {
	var out []*game.UnitInfo

	for _, u := range g.Units.Units() {
		if t.isReadyAttacker(u) {
			out = append(out, u)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	return out
}

// launchersFirst stable sorts the team's long range launchers to the front of the ready
// list so they win the limited harassment sortie before commit (a launcher pings from
// range and survives, the preferred early raider). Only called before commit.
func (t *taskAttackEnemies) launchersFirst(ready []*game.UnitInfo) {
	sort.SliceStable(ready, func(i, j int) bool {
		return isHarassLauncher(ready[i].Type) && !isHarassLauncher(ready[j].Type)
	})
}

// isHarassLauncher reports whether ut is a long range standoff launcher preferred for an
// early HARASSMENT probe: a missile or rocket launcher, or the sea missile boat. They
// strike from range without closing to melee; with none fielded the probe falls back to
// any ready armed unit. C++ ported : the AI favouring long range artillery for early raids.
func isHarassLauncher(ut assets.UnitType) bool {
	switch ut {
	case assets.MisslLch, assets.RocktLch, assets.MsslBoat:
		return true
	}

	return false
}

// isReadyAttacker reports whether u is an armed mobile unit free to act this turn: not
// already ordered, not damaged, not a reserve guard, not an escort. Those are claimed by
// taskRetreat / taskDefenseReserve / taskEscort, so the attack task leaves them alone.
func (t *taskAttackEnemies) isReadyAttacker(u *game.UnitInfo) bool {
	return t.isArmedMobile(u) && !unitDamaged(u) && !t.reserve[u] && !t.escort[u] && !t.ctx.Game.HasOrder(u)
}

// isArmedMobile reports whether u is one of this team's armed mobile units (the
// team's military strength), regardless of whether it is free to act this turn.
// It is the unit of account for the muster threshold. Delegates to the free
// armedMobileUnit (shared with reservedGuards).
func (t *taskAttackEnemies) isArmedMobile(u *game.UnitInfo) bool {
	return armedMobileUnit(u, t.team)
}

// isMobileUnit reports whether the flags mark a mobile (movable) unit.
func isMobileUnit(flags game.Flags) bool {
	return flags.Has(game.FlagMobileLandUnit) || flags.Has(game.FlagMobileAirUnit) || flags.Has(game.FlagMobileSeaUnit)
}
