// task_escort.go : M.A.X. AI escorting weak units
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

// taskEscort pairs a spare combat unit with each of the team's fragile high value
// movers - a Scanner (enemy recon), a Missile Launcher (artillery), an AWAC or a
// Bomber - so the mover does not wander forward alone and die to a single intercept.
// Each turn it has every assigned guard shadow its target and, when a spotted enemy
// comes into the guard's weapon range, engage it. It does not reschedule itself;
// TaskManager queues it again each turn.
//
// It is active only on the experienced tiers (escortEnabled - Expert and up); the
// lower tiers do not micro escorts, so the default behaviour is unchanged. The
// guard to target pairing is a PURE function (escortAssignments): the attack and
// explore tasks consult the same pairing and leave an assigned guard alone, the same
// way reservedGuards decouples the home reserve. A guard is only ever drawn from the
// SPARE force (units beyond the assault muster), so escorting never blunts the attack.
//
// Reductions vs the original: no per target TaskEscort spawned at unit creation and
// no TaskObtainUnits request for a missing escort type (escorts come from the shipped
// roster); no TransporterMap or damage potential safe cell positioning (a simple
// adjacent follow, like the rest of the reduced AI); no reload, repair, or upgrade of
// the escort between fights (separate task families); the escort TYPE is reduced from
// the original's exact TANK / SP_FLAK / FIGHTER to the nearest spare DIRECT FIRE guard
// in the target's movement domain (air mover -> air guard, ground/sea mover -> land/sea
// guard), demoting long range artillery (poorEscortGuard) to a last resort guard so it
// stays on the gun line.
//
// C++ ported : taskescort.cpp - TaskEscort (Execute/EndTurn: engage an enemy in range
// else follow the protected unit).
type taskEscort struct {
	TaskBase
	ctx *aiContext
}

// escortHoldRadius is how close a guard must sit to its target to count as on station
// (it then holds rather than issuing a follow move again every turn). Small, so the guard
// hugs the unit it protects.
const escortHoldRadius = 2

// escortEnabled gates the escort behaviour to the experienced tiers (Expert and up),
// set by applyDifficulty. Default false keeps unconfigured games and tests unchanged, like aiKeepReserve.
var escortEnabled = false

func newTaskEscort(team int, ctx *aiContext) *taskEscort {
	return &taskEscort{
		TaskBase: newTaskBase(team, nil, 730), // between the defence reserve (720) and commando (750)
		ctx:      ctx,
	}
}

// BeginTurn has each assigned guard engage a threat in range or shadow its target.
// C++: TaskEscort::Execute (RetreatFromDanger / EvaluateAssault / IssueOrders) over
// the escort units, here driven from the pure guard<->target pairing.
func (t *taskEscort) BeginTurn() {
	if !escortEnabled {
		return
	}

	g := t.ctx.Game
	if g == nil {
		return
	}

	assignments := escortAssignments(g, t.team)
	if len(assignments) == 0 {
		return
	}

	// Deterministic order: by guard ID (the map iteration order is random).
	guards := make([]*game.UnitInfo, 0, len(assignments))
	for guard := range assignments {
		guards = append(guards, guard)
	}

	sort.Slice(guards, func(i, j int) bool { return guards[i].ID < guards[j].ID })
	for _, guard := range guards {
		if g.HasOrder(guard) || guard.Speed <= 0 {
			continue
		}

		if e := t.engageable(guard); e != nil {
			t.engage(guard, e)
			continue
		}

		t.follow(guard, assignments[guard])
	}
}

// engageable returns the spotted enemy nearest guard that it can attack from where it
// stands (so the escort shoots a threat that wanders into range instead of only hugging
// its charge), or nil. C++: AiAttack_EvaluateAssault on the escort unit.
func (t *taskEscort) engageable(guard *game.UnitInfo) *game.UnitInfo {
	g := t.ctx.Game
	var best *game.UnitInfo
	bestD := 1 << 30

	for _, e := range g.Units.Units() {
		if e.Team == t.team || e.Team < 0 || e.Team >= game.TeamCount {
			continue
		}

		if e.GridX < 0 || e.GridY < 0 || !g.SpottedByTeam(e, t.team) {
			continue
		}

		if !g.CanTargetUnit(guard, e) {
			continue
		}

		ex, ey := believedCell(g, t.team, e)
		if !g.CanAttack(guard, ex, ey) {
			continue
		}

		if d := sim.SquaredDist(guard.GridX, guard.GridY, ex, ey); d < bestD {
			best, bestD = e, d
		}
	}

	return best
}

// engage fires guard on enemy e from its current cell (air units hover to attack,
// they do not overfly). C++: AiAttack_EvaluateAssault -> the attack order.
func (t *taskEscort) engage(guard, e *game.UnitInfo) {
	g := t.ctx.Game
	ex, ey := believedCell(g, t.team, e)

	g.Attack(guard, ex, ey)

	dbg.Printf("ai: team %d ESCORT %s engages %s at (%d,%d)", t.team, guard.Type.Tag(), e.Type.Tag(), ex, ey)
}

// follow keeps guard on station beside its target: a guard already within
// escortHoldRadius holds, otherwise it drives to a cell adjacent to the target.
// C++: TaskEscort::IssueOrders moving the escort to a position by the protected unit.
func (t *taskEscort) follow(guard, target *game.UnitInfo) {
	g := t.ctx.Game

	if chebyshev(guard.GridX, guard.GridY, target.GridX, target.GridY) <= escortHoldRadius {
		return // on station
	}

	for _, c := range g.AdjacentApproachCells(guard, target.GridX, target.GridY) {
		if g.OrderMove(guard, c.X, c.Y) {
			dbg.Printf("ai: team %d ESCORT %s follows %s to (%d,%d)", t.team, guard.Type.Tag(), target.Type.Tag(), target.GridX, target.GridY)

			return
		}
	}
}

// escortGuards is the set of units assigned as escorts this turn - the keys of
// escortAssignments. The attack and explore tasks consult it to leave an escort alone,
// the same decoupling reservedGuards provides for the home reserve.
func escortGuards(g *sim.Game, team int) map[*game.UnitInfo]bool {
	a := escortAssignments(g, team)
	if len(a) == 0 {
		return nil
	}

	set := make(map[*game.UnitInfo]bool, len(a))
	for guard := range a {
		set[guard] = true
	}

	return set
}

// escortAssignments pairs each escortable mover with a spare armed guard, as a pure,
// deterministic function of the current positions (so the escort, attack and explore
// tasks all derive the same pairing). It returns nil when the tier keeps no escorts
// (escortEnabled). Candidate guards are the team's armed mobile, undamaged units that
// are neither home reserve guards nor escortable themselves; at most
// (guards - minStrikeForce) are spared as escorts (floored at 0) so the assault always
// keeps its muster. Each target (in ID order) takes the nearest unassigned guard in its
// movement domain (an air mover gets an air guard; a ground/sea mover a land/sea guard),
// preferring a direct fire guard over a long range artillery unit (poorEscortGuard).
// C++: the TaskEscort instances aiplayer.cpp creates per AWAC/BOMBER/MISSLLCH/SCANNER,
// reduced to one shared assignment pass.
func escortAssignments(g *sim.Game, team int) map[*game.UnitInfo]*game.UnitInfo {
	if !escortEnabled {
		return nil
	}

	reserved := reservedGuards(g, team)
	var targets, guards []*game.UnitInfo

	for _, u := range g.Units.Units() {
		switch {
		case escortableTarget(u, team):
			targets = append(targets, u)
		case armedMobileUnit(u, team) && !unitDamaged(u) && !reserved[u]:
			guards = append(guards, u)
		}
	}

	if len(targets) == 0 {
		return nil
	}

	// Spare force only: never take the assault below its muster.
	maxEscorts := len(guards) - minStrikeForce
	if maxEscorts <= 0 {
		return nil
	}

	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	sort.Slice(guards, func(i, j int) bool { return guards[i].ID < guards[j].ID })
	assigned := make(map[*game.UnitInfo]*game.UnitInfo)
	used := make(map[*game.UnitInfo]bool)

	for _, target := range targets {
		if len(assigned) >= maxEscorts {
			break
		}

		// Pick the best same domain guard by (escort suitability, distance): a direct fire
		// guard (rank 0) is always preferred over a long range artillery unit (rank 1), so a
		// tank/scout/SP flak/fighter screens the mover while the artillery stays on the gun line
		// to fire from range; an artillery unit is taken only when nothing better is free.
		var best *game.UnitInfo
		bestRank, bestD := 2, 1<<30

		for _, guard := range guards {
			if used[guard] || isAirUnit(guard) != isAirUnit(target) {
				continue
			}

			rank := 0
			if poorEscortGuard(guard) {
				rank = 1
			}

			d := sim.SquaredDist(guard.GridX, guard.GridY, target.GridX, target.GridY)
			if rank < bestRank || (rank == bestRank && d < bestD) {
				best, bestRank, bestD = guard, rank, d
			}
		}

		if best != nil {
			assigned[best] = target
			used[best] = true
		}
	}

	if len(assigned) == 0 {
		return nil
	}

	return assigned
}

// escortableTarget reports whether u is one of the team's fragile high value movers
// that warrants a combat escort: a Scanner (enemy recon), a Missile Launcher
// (artillery), or an AWAC / Bomber (air). SP_FLAK and FASTBOAT defend themselves, so they do not receive escorts.
// C++: the unit types aiplayer.cpp wraps in a TaskEscort.
func escortableTarget(u *game.UnitInfo, team int) bool {
	if u.Team != team || !isMobileUnit(u.Flags) {
		return false
	}

	switch u.Type {
	case assets.Scanner, assets.MisslLch, assets.Awac, assets.Bomber:
		return true
	}

	return false
}

// isAirUnit reports whether u moves through the air (so an air mover is escorted by an
// air guard, not a tank that cannot keep up).
func isAirUnit(u *game.UnitInfo) bool {
	return u.Flags.Has(game.FlagMobileAirUnit)
}

// unarmedSupportType reports whether t is a mobile unit that carries no weapon in the real
// ATTRIBS - a recon, cargo or utility unit: a Surveyor, Scanner, Supply / Gold truck,
// Bulldozer, Repair unit, transport (clan / sea / air) or AWAC. Such a unit is never combat
// strength, so the AI must not muster, reserve or escort with it. The check is by TYPE (not
// the runtime AttribAttack) because a debug bench deploy stamps every unit with placeholder
// combat stats (gamescreen deployUnit's flat Attack=12), which would otherwise make a soft
// supply truck look armed to armedMobileUnit. In a real game these types have AttribAttack 0
// and are excluded anyway, so this only hardens the bench / debug paths.
func unarmedSupportType(t assets.UnitType) bool {
	switch t {
	case assets.Surveyor, assets.Scanner, assets.SplyTrck, assets.GoldTrck,
		assets.Bulldozr, assets.Repair, assets.ClnTrans, assets.SeaTrans,
		assets.AirTrans, assets.Awac:
		return true
	}

	return false
}

// poorEscortGuard reports whether u is a long range, indirect fire artillery unit
// (Artillery, Rocket Launcher, Missile Cruiser) whose value is firing from the back line,
// not screening a mover out front. Such a unit is taken as an escort only as a last resort,
// so a direct fire guard is preferred and the artillery stays with the gun line. This keeps
// the escort type close to the original's choices (TANK / SP_FLAK / FIGHTER are all direct
// fire).
func poorEscortGuard(u *game.UnitInfo) bool {
	switch u.Type {
	case assets.Artillry, assets.RocktLch, assets.MsslBoat:
		return true
	}

	return false
}
