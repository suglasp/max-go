// task_attack_target.go : picks what the AI shoots at: which enemy team, which unit,
// and where the team believes it stands.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : aiplayer.cpp - DetermineTargetTeam + DetermineAttack target ranking
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// isKeyFacility reports whether ut is an enemy economy/score building the AI prioritises
// as a raid target: mining, the Eco Sphere (colonization score), the three production
// plants and the shipyard. Hitting these denies economy and score; with none reachable
// the AI falls back to the nearest target. C++ ported : aiplayer.cpp - IsKeyFacility.
func isKeyFacility(ut assets.UnitType) bool {
	switch ut {
	case assets.MiningSt, assets.Greenhse, assets.Shipyard, assets.LightPlt, assets.LandPlt, assets.AirPlt:
		return true
	}

	return false
}

// keyFacilityRevealTier is the difficulty at/above which the team is handed the location
// of every enemy MINING STATION and ECO SPHERE through the fog, so higher AIs beeline for
// your economy/score buildings. Difficulty tier alone controls this reveal.
const keyFacilityRevealTier = OpponentExpert

// revealedKeyFacility reports whether enemy e is an economy facility revealed through the
// fog at EXPERT+: a Mining Station or Eco Sphere. Only these two (the original's cheat
// spots exactly MININGST + GREENHSE); the wider isKeyFacility set feeds only TARGET
// PRIORITY. Both stationary, so targetCell yields their true position.
// C++ ported : aiplayer.cpp - the EXPERT+/TOLERABLE Ai_UnitSpotted(MININGST|GREENHSE) cheat.
func revealedKeyFacility(e *game.UnitInfo) bool {
	if aiTier < keyFacilityRevealTier {
		return false
	}

	return e.Type == assets.MiningSt || e.Type == assets.Greenhse
}

// spottedByTeam reports whether enemy e is in the team's spotted memory. g.SpottedByTeam keeps its last position after it enters fog and treats all
// units as spotted without a visibility model. At EXPERT+, revealed economy facilities are also spotted.
// C++ ported : aiplayer.cpp - AiPlayer::UnitSpotted + DetermineTargetTeam.
func (t *taskAttackEnemies) spottedByTeam(e *game.UnitInfo) bool {
	return t.ctx.Game.SpottedByTeam(e, t.team) || revealedKeyFacility(e)
}

// targetCell returns the cell the team believes enemy e occupies: its true cell while
// visible, else the frozen last seen cell, so a unit pursuing a fogged enemy marches
// toward where it was LAST seen. Falls back to the live cell with no memory (nil Vis).
// C++: SpottedUnit::GetLastPosition feeds the attack task's destination.
func (t *taskAttackEnemies) targetCell(e *game.UnitInfo) (int, int) {
	g := t.ctx.Game

	if g.Vis != nil && !g.Vis.VisibleToTeam(e, t.team) {
		if x, y, ok := g.LastSeen(t.team, e); ok {
			return x, y
		}
	}

	return e.GridX, e.GridY
}

// nearestEnemy returns the best enemy unit/building for u to attack, or nil. Within a
// raid priority class it ranks by a cost balancing Chebyshev distance against how
// defended the target is (threat map firepower at its cell), preferring the closer,
// softer target. With no threat map the cost is pure distance.
// C++ ported : aiplayer.cpp - AiPlayer::DetermineAttack (value vs damage potential,
// reduced to a key facility class plus distance plus threat cost).
func (t *taskAttackEnemies) nearestEnemy(u *game.UnitInfo) *game.UnitInfo {
	var best *game.UnitInfo
	bestPri := 3
	bestCost := 1 << 30

	for _, e := range t.ctx.Game.Units.Units() {
		if e.Team == u.Team || e.Team < 0 || e.Team >= game.TeamCount {
			continue
		}

		if e.GridX < 0 || e.GridY < 0 {
			continue
		}

		// Only pursue enemies this unit's weapon can actually engage; an anti
		// aircraft or flak unit hunts aircraft, a ground gun hunts ground, sea, and
		// structures, and a weapon with multiple roles engages all of them. C++: the
		// AI weighs only valid attack targets (Access_IsValidAttackTarget).
		if !t.ctx.Game.CanTargetUnit(u, e) {
			continue
		}

		// Only pursue enemies the team has actually SPOTTED; never chase a unit
		// hidden in the fog (the cause of the omniscient turn-1 rush).
		if !t.spottedByTeam(e) {
			continue
		}

		// Skip a target the team has shelved (searched out or chased too long) while
		// it stays fogged, until its cooldown elapses; a fresh sighting unshelves it
		// at once. This stops every armed unit from locking onto the same unreachable
		// cell again.
		if !t.fogged(e) {
			delete(t.abandoned, e) // reacquired: pursue normally again
		} else if since, shelved := t.abandoned[e]; shelved {
			if t.ctx.Game.State.TurnCounter-since < abandonCooldown {
				continue
			}

			delete(t.abandoned, e) // cooldown elapsed: allow a fresh attempt
		}

		// Rank by raid priority class (lower wins), then a distance plus threat cost.
		// Classes: an enemy that can shoot THIS unit from where it stands first (engage the
		// gun trained on you), then an enemy economy/score building, then everything else.
		// Within a class the cost prefers the closer, less defended target. Distance and
		// threat read the last seen cell while fogged.
		tx, ty := t.targetCell(e)
		pri := 2
		if isKeyFacility(e.Type) {
			pri = 1
		}

		if t.immediateThreat(u, e) {
			pri = 0
		}

		cost := chebyshev(u.GridX, u.GridY, tx, ty) + t.tm.at(tx, ty)
		if pri < bestPri || (pri == bestPri && cost < bestCost) {
			best, bestPri, bestCost = e, pri, cost
		}
	}

	if best != nil && bestPri == 0 {
		dbg.Printf("ai: team %d %s engages immediate threat %s at (%d,%d) over any distant objective",
			t.team, u.Type.Tag(), best.Type.Tag(), best.GridX, best.GridY)
	}

	return best
}

// immediateThreat reports whether visible enemy e can fire on u from where it stands:
// armed, its weapon can target u's class, and its range covers u's cell. nearestEnemy
// promotes such a target above a distant objective. Only a VISIBLE enemy qualifies.
// C++ ported : aiattack.cpp - engaging an in range threat ahead of a lower value objective.
func (t *taskAttackEnemies) immediateThreat(u, e *game.UnitInfo) bool {
	if e.Values == nil || t.fogged(e) {
		return false
	}

	if e.Values.Get(game.AttribAttack) <= 0 {
		return false // unarmed: no gun to answer
	}

	if !t.ctx.Game.CanTargetUnit(e, u) {
		return false // e's weapon cannot engage u's class: not a threat to this unit
	}

	rng := e.Values.Get(game.AttribRange)
	if rng < 1 {
		rng = 1
	}

	return sim.SquaredDist(e.GridX, e.GridY, u.GridX, u.GridY) <= rng*rng
}

// chooseTarget picks what unit u attacks this turn. Before the team commits (no shared
// focus) each unit hunts its own nearest enemy, unchanged. Once committed the force
// converges on the team objective (t.focus): u drives at the focus when its weapon can
// engage it, except a unit already under an enemy's gun answers that gun first (self
// defence), and a unit whose weapon cannot hit the focus class falls back to its own
// nearest valid target. C++ ported : aiplayer.cpp - the shared attack task focus.
func (t *taskAttackEnemies) chooseTarget(u *game.UnitInfo) *game.UnitInfo {
	if t.focus == nil {
		return t.nearestEnemy(u) // pre commit or nothing spotted: hunt the nearest
	}

	// A unit an enemy can already shoot answers that threat before marching to the
	// shared objective (mirrors nearestEnemy's immediate threat promotion).
	if thr := t.immediateThreatFor(u); thr != nil {
		return thr
	}

	// Converge on the shared objective when this unit can engage it and the team still
	// holds it spotted; otherwise hunt this unit's own nearest valid target (e.g. an
	// anti air unit whose focus is a ground target).
	if t.ctx.Game.CanTargetUnit(u, t.focus) && t.spottedByTeam(t.focus) {
		return t.focus
	}

	return t.nearestEnemy(u)
}

// immediateThreatFor returns the nearest visible enemy that can fire on u from where it
// stands (immediateThreat), or nil. It lets a unit converging on the shared objective
// break off to answer a gun already trained on it. C++: aiattack.cpp - engaging an in
// range threat first.
func (t *taskAttackEnemies) immediateThreatFor(u *game.UnitInfo) *game.UnitInfo {
	var best *game.UnitInfo
	bestD := 1 << 30

	for _, e := range t.ctx.Game.Units.Units() {
		if e.Team == u.Team || e.Team < 0 || e.Team >= game.TeamCount {
			continue
		}

		if e.GridX < 0 || e.GridY < 0 || !t.immediateThreat(u, e) {
			continue
		}

		if d := sim.SquaredDist(u.GridX, u.GridY, e.GridX, e.GridY); d < bestD {
			best, bestD = e, d
		}
	}

	return best
}

// teamFocusTarget picks the single objective the committed force masses on this turn. It
// first chooses a target enemy team (pickTargetTeam, a reduced port of DetermineTargetTeam),
// then that team's highest raid priority SPOTTED unit; if the target team has nothing
// spotted it falls back to the best spotted unit of ANY enemy, so the force still
// coordinates on whoever it can see. Returns nil when the team has spotted nothing (each
// unit then hunts its own nearest) or owns no base to sortie from. Only meaningful once
// committed. C++ ported : aiplayer.cpp - DetermineTargetTeam + the shared attack focus.
func (t *taskAttackEnemies) teamFocusTarget() *game.UnitInfo {
	g := t.ctx.Game

	bx, by, ok := teamBaseCenter(g, t.team)
	if !ok {
		return nil // no base: the force has no anchor to mass out from
	}

	if best := t.bestSpottedTarget(t.pickTargetTeam(g), bx, by); best != nil {
		return best
	}

	return t.bestSpottedTarget(-1, bx, by) // target team has nothing spotted: any enemy
}

// pickTargetTeam chooses which enemy team the force focuses on: the alive enemy with the
// highest score (g.TeamPoints), ties broken by the fewest buildings then the lowest team
// index (deterministic). Returns -1 when no enemy team is in play. The score is public
// (shown to every player), so this reads no hidden position and non omniscience holds.
// C++ ported : aiplayer.cpp - DetermineTargetTeam (the non-cheating branch).
func (t *taskAttackEnemies) pickTargetTeam(g *sim.Game) int {
	var units, buildings [game.TeamCount]int

	for _, u := range g.Units.Units() {
		if u.Team < 0 || u.Team >= game.TeamCount || u.Team == t.team {
			continue
		}

		units[u.Team]++
		if u.Flags.Has(game.FlagStationary) {
			buildings[u.Team]++
		}
	}

	target := -1
	for tm := 0; tm < game.TeamCount; tm++ {
		if tm == t.team || units[tm] == 0 || !aliveEnemyTeam(g, tm) {
			continue
		}

		if target < 0 {
			target = tm

			continue
		}

		if pt, pc := g.TeamPoints[tm], g.TeamPoints[target]; pt > pc || (pt == pc && buildings[tm] < buildings[target]) {
			target = tm
		}
	}

	return target
}

// aliveEnemyTeam reports whether team slot tm is a real, still playing team (not the empty
// slot or an eliminated one). Guards a nil g.State in a bare test harness.
func aliveEnemyTeam(g *sim.Game, tm int) bool {
	if g.State == nil {
		return true
	}

	info := g.State.Teams[tm]

	return info.Type != game.TeamNone && !info.Eliminated
}

// bestSpottedTarget returns the highest raid priority spotted enemy for the force to mass
// on, scored from the base centre (bx, by): a key facility outranks any other unit, then
// the closer, less defended target wins (distance plus threat at the believed cell). A
// team >= 0 restricts the search to that target team; team < 0 considers every enemy team
// (the fallback when the target team has nothing spotted). A target the team has shelved
// while it stays fogged is skipped. Reads only spotted positions, so it never aims at an
// unseen enemy. C++: aiplayer.cpp - the DetermineAttack value / key facility ranking.
func (t *taskAttackEnemies) bestSpottedTarget(team, bx, by int) *game.UnitInfo {
	g := t.ctx.Game
	var best *game.UnitInfo
	bestPri := 3
	bestCost := 1 << 30

	for _, e := range g.Units.Units() {
		if e.Team == t.team || e.Team < 0 || e.Team >= game.TeamCount {
			continue
		}

		if team >= 0 && e.Team != team {
			continue
		}

		if e.GridX < 0 || e.GridY < 0 || !t.spottedByTeam(e) {
			continue
		}

		// Skip a target shelved while it stays fogged (searched out or chased too long),
		// so the force does not re commit to an unreachable cell every turn.
		if t.fogged(e) {
			if since, shelved := t.abandoned[e]; shelved && g.State != nil && g.State.TurnCounter-since < abandonCooldown {
				continue
			}
		}

		tx, ty := t.targetCell(e)
		pri := 2
		if isKeyFacility(e.Type) {
			pri = 1
		}

		cost := chebyshev(bx, by, tx, ty) + t.tm.at(tx, ty)
		if pri < bestPri || (pri == bestPri && cost < bestCost) {
			best, bestPri, bestCost = e, pri, cost
		}
	}

	return best
}

// chebyshev is the 8-direction grid distance (the unit movement metric).
func chebyshev(ax, ay, bx, by int) int {
	dx, dy := ax-bx, ay-by

	if dx < 0 {
		dx = -dx
	}

	if dy < 0 {
		dy = -dy
	}

	if dx > dy {
		return dx
	}

	return dy
}

// fogged reports whether enemy e is currently out of this team's sight (so the team
// is acting on remembered, not live, intel).
func (t *taskAttackEnemies) fogged(e *game.UnitInfo) bool {
	g := t.ctx.Game

	return g.Vis != nil && !g.Vis.VisibleToTeam(e, t.team)
}
