// task_defensereserve.go : M.A.X. AI home defence reserve.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"sort"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// taskDefenseReserve keeps a fraction of the team's army home as a positioned guard:
// each turn it parks every reserve unit near a key building (mining, Eco Sphere, or
// plant) and, when a spotted enemy comes near a guarded building, sends the nearest
// guard to intercept it. It is the POSITIONING and INTERCEPT half of the original's
// TaskDefenseReserve; the production half (sizing the reserve against the enemy's
// assets) overlaps the shipped economy and the keepReserve sortie holdback. It does
// not reschedule itself; TaskManager queues it again each turn.
//
// It is active only on the tiers that keep a reserve (aiKeepReserve - Master/God);
// the easier tiers overcommit, going all in, so the default behaviour is unchanged.
// The reserve membership is a PURE set (reservedGuards): the force/4 armed units
// closest to base. The task claims the whole set through the unit ledger, so the
// attack and explore tasks leave a parked guard alone (no pull into the assault or
// off to scout).
//
// Reductions vs the original: no eight category DefenseManager production planning
// (PlanDefenses / EvaluateNeeds, which overlaps the shipped factoryProducts and
// keepReserve); no FollowAttacker escort chain (the next backlog item); the intercept
// is tethered by reacting only to a threat already inside guardEngageRadius of the
// guarded building (the original's TaskMoveHome leash), so a guard does not chase
// across the map.
//
// C++ ported : taskdefensereserve.cpp - TaskDefenseReserve::Execute (engage in range
// else return to the defended site).
type taskDefenseReserve struct {
	TaskBase
	ctx *aiContext
}

const (
	// reserveDenominator sets the reserve size: 1/4 of the armed force, the 25% the
	// sortie holdback used to withhold (so the count is unchanged, only positioned).
	reserveDenominator = 4

	// guardPostRadius is how close to its guarded building a guard must sit to count
	// as on post (it then holds without an order). guardEngageRadius is how near that
	// building a spotted enemy must come for the guard to leave the post and
	// intercept. Interpreted values (the original reads a defended site and assault
	// range): a guard hugs its building, and reacts a little before an attacker
	// reaches it.
	guardPostRadius   = 3
	guardEngageRadius = 10
)

func newTaskDefenseReserve(team int, ctx *aiContext) *taskDefenseReserve {
	return &taskDefenseReserve{
		TaskBase: newTaskBase(team, nil, taskPriorityDefenseReserve),
		ctx:      ctx,
	}
}

// BeginTurn positions each reserve guard: it intercepts a spotted enemy near its
// guarded building, otherwise stations it on post. C++: TaskDefenseReserve::Execute
// over the reserve's units.
func (t *taskDefenseReserve) BeginTurn() {
	g := t.ctx.Game

	if g == nil {
		return
	}

	reserve := reservedGuards(g, t.team)

	if len(reserve) == 0 {
		return // this tier keeps no reserve, or the force is too small / has no base
	}

	for _, u := range g.Units.Units() {
		// Every guard is this task's, even one that cannot act this turn.
		if !reserve[u] || !t.claim(u) {
			continue
		}

		if g.HasOrder(u) || u.Speed <= 0 {
			continue
		}

		sx, sy := t.guardSite(u)
		if e := t.threatNear(sx, sy); e != nil {
			t.intercept(u, e)
			continue
		}

		t.holdPost(u, sx, sy)
	}
}

// guardSite returns the building this guard defends: the nearest own key facility
// (mining, Eco Sphere, or plant - the things worth guarding), or the base centre when
// the team owns no key facility. reservedGuards has already guaranteed a base exists.
func (t *taskDefenseReserve) guardSite(u *game.UnitInfo) (int, int) {
	g := t.ctx.Game
	var best *game.UnitInfo
	bestD := 1 << 30

	for _, b := range g.Units.Units() {
		if b.Team != t.team || !isKeyFacility(b.Type) {
			continue
		}

		if d := sim.SquaredDist(u.GridX, u.GridY, b.GridX, b.GridY); best == nil || d < bestD {
			best, bestD = b, d
		}
	}

	if best != nil {
		return best.GridX, best.GridY
	}

	bx, by, _ := teamBaseCenter(g, t.team)

	return bx, by
}

// threatNear returns the spotted enemy nearest the guarded site whose believed cell
// lies within guardEngageRadius of it, or nil when no enemy is pressing the site.
func (t *taskDefenseReserve) threatNear(sx, sy int) *game.UnitInfo {
	g := t.ctx.Game
	var best *game.UnitInfo
	bestD := guardEngageRadius*guardEngageRadius + 1

	for _, e := range g.Units.Units() {
		if e.Team == t.team || e.Team < 0 || e.Team >= game.TeamCount {
			continue
		}

		if e.GridX < 0 || e.GridY < 0 || !g.SpottedByTeam(e, t.team) {
			continue
		}

		ex, ey := believedCell(g, t.team, e)
		if d := sim.SquaredDist(sx, sy, ex, ey); d <= guardEngageRadius*guardEngageRadius && d < bestD {
			best, bestD = e, d
		}
	}

	return best
}

// intercept engages enemy e: fire if it is already in range, otherwise drive toward
// its believed cell to meet it near the guarded building. C++: AiAttack_EvaluateAssault
// on a reserve unit.
func (t *taskDefenseReserve) intercept(u, e *game.UnitInfo) {
	g := t.ctx.Game
	ex, ey := believedCell(g, t.team, e)

	if g.CanAttack(u, ex, ey) {
		g.Attack(u, ex, ey)

		dbg.Printf("ai: team %d GUARD %s intercepts %s at (%d,%d)", t.team, u.Type.Tag(), e.Type.Tag(), ex, ey)

		return
	}

	for _, c := range g.AdjacentApproachCells(u, ex, ey) {
		if g.OrderMove(u, c.X, c.Y) {
			dbg.Printf("ai: team %d GUARD %s moves to intercept %s at (%d,%d)", t.team, u.Type.Tag(), e.Type.Tag(), ex, ey)

			return
		}
	}
}

// holdPost stations u near its guarded building: a guard already within
// guardPostRadius holds (no order); otherwise it drives to a cell beside the building.
// C++: TaskMoveHome returning the reserve unit to the defended site.
func (t *taskDefenseReserve) holdPost(u *game.UnitInfo, sx, sy int) {
	g := t.ctx.Game

	if chebyshev(u.GridX, u.GridY, sx, sy) <= guardPostRadius {
		return // on post: hold and watch
	}

	for _, c := range g.AdjacentApproachCells(u, sx, sy) {
		if g.OrderMove(u, c.X, c.Y) {
			dbg.Printf("ai: team %d GUARD %s moves to post near (%d,%d)", t.team, u.Type.Tag(), sx, sy)

			return
		}
	}
}

// reservedGuards returns the team's home defence reserve: the armed mobile, undamaged
// units closest to the base centre, up to a quarter of the armed force. It returns nil
// unless the team is on a reserve keeping tier (aiKeepReserve) and owns a base, and nil
// when the force is too small to spare anyone (force < reserveDenominator). The set is
// a pure function of the current positions, so the defense reserve, attack and explore
// tasks all derive the same membership and stay in agreement. C++: the units
// TaskDefenseReserve owns via AddUnit (reduced to a closest to base selection).
func reservedGuards(g *sim.Game, team int) map[*game.UnitInfo]bool {
	if !aiKeepReserve {
		return nil
	}

	bx, by, ok := teamBaseCenter(g, team)
	if !ok {
		return nil
	}

	var armed []*game.UnitInfo
	for _, u := range g.Units.Units() {
		if armedMobileUnit(u, team) && !unitDamaged(u) {
			armed = append(armed, u)
		}
	}

	k := len(armed) / reserveDenominator
	if k == 0 {
		return nil
	}

	// Closest to base first; ID breaks ties (the unit list comes back in random map
	// order, so breaking ties keeps the set deterministic across the three readers).
	sort.Slice(armed, func(i, j int) bool {
		di := sim.SquaredDist(armed[i].GridX, armed[i].GridY, bx, by)
		dj := sim.SquaredDist(armed[j].GridX, armed[j].GridY, bx, by)
		if di != dj {
			return di < dj
		}

		return armed[i].ID < armed[j].ID
	})

	set := make(map[*game.UnitInfo]bool, k)
	for i := 0; i < k; i++ {
		set[armed[i]] = true
	}

	return set
}

// armedMobileUnit reports whether u is one of team's armed mobile units (its military
// strength): mobile, not a constructor, not a commando / mine layer (those act via
// their own tasks), not a weaponless support / recon / cargo unit, and armed
// (AttribAttack > 0). The free form of taskAttackEnemies.isArmedMobile, shared with
// reservedGuards and the escort.
func armedMobileUnit(u *game.UnitInfo, team int) bool {
	if u.Team != team || u.Values == nil {
		return false
	}

	if !isMobileUnit(u.Flags) || u.Flags.Has(game.FlagConstructorUnit) {
		return false
	}

	if sim.IsCommando(u) || sim.IsMineLayer(u) {
		return false
	}

	if unarmedSupportType(u.Type) {
		return false // a soft support unit is never military, even if a debug bench stamped it with combat stats
	}

	return u.Values.Get(game.AttribAttack) > 0
}
