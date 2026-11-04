// reaction.go : triggers opportunity fire from stationary defenders during enemy movement.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - CheckReaction / reaction fire sweep
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/combat"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// ReactionEvent records one reaction (opportunity) shot: Reactor fired at Mover
// because Mover moved within Reactor's weapon range. The in game screen drains
// these (DrainReactions) to show the "<unit> reacts to enemy <unit>" banner and
// play the reaction voice; the shot's damage and blast already animate through the
// Casualty / ProjectileImpact queues like any other attack. C++: the player team
// MessageManager_DrawMessage + PlayVoice(V_M250, V_F251) inside UnitsManager_CheckReaction.
type ReactionEvent struct {
	Reactor *game.UnitInfo
	Mover   *game.UnitInfo
}

// DrainReactions returns the reaction shots fired since the last call and clears
// the queue.
func (g *Game) DrainReactions() []ReactionEvent {
	out := g.reactions
	g.reactions = nil
	return out
}

// checkReactions fires every enemy unit that may react to mover having just
// settled on a new cell: a stationary turret or a parked mobile unit with shots
// left, in range, that can see the mover, takes one opportunity shot. This is how
// a player's defensive buildings (and the AI's) auto defend - a reactor needs no
// order of its own, it only has to be a valid armed unit in range. Reactors fire
// nearest first and the sweep stops once the mover is destroyed.
//
// C++ ported : units_manager.cpp - UnitsManager_CheckReaction + the reaction sweep
// (UnitsManager_IsReactionPending over the team unit lists).
func (g *Game) checkReactions(mover *game.UnitInfo) {
	if mover == nil || mover.Hits <= 0 || mover.GridX < 0 || mover.GridY < 0 {
		return
	}
	for _, r := range g.reactorsAgainst(mover) {
		if mover.Hits <= 0 {
			break // an earlier reactor already destroyed the mover
		}
		if _, ok := g.Attack(r, mover.GridX, mover.GridY); ok {
			g.reactions = append(g.reactions, ReactionEvent{Reactor: r, Mover: mover})
		}
	}
}

// revealReaction is one queued reveal trigger: revealerTeam newly gained sight of
// enemy (fog lifted on it), so that team's eligible defenders get one opportunity
// shot. Queued in updateSpotted, fired by FireRevealReactions.
type revealReaction struct {
	team  int
	enemy *game.UnitInfo
}

// EnableRevealReactions arms reaction fire after a game resumes mid-turn from a save. A fresh game enables it in BeginTeamTurn.
func (g *Game) EnableRevealReactions() {
	g.reactOnReveal = true
}

// FireRevealReactions resolves queued reveal-triggered opportunity shots during the gameplay update. Eligible defenders fire once using the normal
// reaction checks, then the queue is cleared.
func (g *Game) FireRevealReactions() {
	pending := g.pendingReveals
	g.pendingReveals = nil
	for _, rv := range pending {
		g.checkRevealReactions(rv.team, rv.enemy)
	}
}

// checkRevealReactions fires team's eligible defenders at a just revealed enemy e -
// the opportunity fire analogue of a mover settling in range, but triggered by the
// reveal edge. Only team's reactors are swept (the team that gained sight), each
// passing the full shouldReact checklist (reaction not disabled, reactor not idle /
// powered down / mid move, shots+ammo, in range, correct target class, stealth);
// nearest first, one shot each, stopping if e dies.
func (g *Game) checkRevealReactions(team int, e *game.UnitInfo) {
	if e == nil || e.Hits <= 0 || e.GridX < 0 || e.GridY < 0 {
		return
	}
	var reactors []*game.UnitInfo
	for _, r := range g.Units.Units() {
		if r.Team != team {
			continue // only the team that gained sight reacts
		}
		if g.shouldReact(r, e) {
			reactors = append(reactors, r)
		}
	}
	if dbg.Enabled() {
		dbg.Printf("reveal-fire: team %d revealed enemy id=%d at (%d,%d) -> %d eligible reactor(s)", team, e.ID, e.GridX, e.GridY, len(reactors))
	}
	sortByDistTo(reactors, e)
	for _, r := range reactors {
		if e.Hits <= 0 {
			break // an earlier defender already destroyed the revealed enemy
		}
		if _, ok := g.Attack(r, e.GridX, e.GridY); ok {
			g.reactions = append(g.reactions, ReactionEvent{Reactor: r, Mover: e})
		}
	}
}

// reactorsAgainst collects the enemy units that may reaction fire at mover,
// ordered nearest first with an ID tie break (the nearest defender shoots first -
// close enough to the engine's list order, and the tie break makes the sweep
// fully deterministic regardless of the random unit map iteration order).
func (g *Game) reactorsAgainst(mover *game.UnitInfo) []*game.UnitInfo {
	var out []*game.UnitInfo
	for _, r := range g.Units.Units() {
		if r.Team == mover.Team {
			continue
		}
		if g.shouldReact(r, mover) {
			out = append(out, r)
		}
	}
	sortByDistTo(out, mover)
	return out
}

// shouldReact ports UnitsManager_ShouldAttack + the UnitsManager_CheckReaction
// gates: the reactor must not have reaction fire disabled; must not be powered
// down / disabled / self destructing / mid move; must be able to SEE the mover
// (fog); and must have a valid in range enemy target with shots and ammo. A
// stealthy reactor (commando / submarine) the mover's team cannot see holds its
// fire so it does not give itself away.
//
// SIMULTANEOUS mode adds the HUMAN reactor gate (units_manager.cpp:4329-4348):
// during live concurrent movement a player unit auto fires only ON SENTRY, once
// its team has committed the turn (the drain), or when the mover could shoot one
// of the team's units from where it stands - a plain awaiting unit does NOT open
// up on every enemy that drives by. Turn based always reacts (the
// PLAY_MODE_TURN_BASED short circuits true) and a computer reactor keeps the
// AI's own targeting.
//
// C++ ported : units_manager.cpp - UnitsManager_ShouldAttack / UnitsManager_CheckReaction.
func (g *Game) shouldReact(reactor, mover *game.UnitInfo) bool {
	if reactor == nil || mover == nil {
		return false
	}
	// The neutral alien derelict faction never initiates fire: it has no orders and
	// stays inert until a player attacks or salvages it.
	if game.IsAlien(reactor.Team) {
		return false
	}
	if reactor.DisabledReactionFire {
		return false
	}
	// A unit still parked in its factory is idle in the original and takes no shot.
	// C++ ported : units_manager.cpp - UnitsManager_ShouldAttack (ORDER_IDLE never attacks)
	if reactor.HiddenUntilExpand {
		return false
	}
	// The simultaneous mode human gate (see the doc block above).
	// C++ ported : units_manager.cpp:4329-4348.
	if g.State.PlayMode == game.PlaySimultaneous && g.State.Teams[reactor.Team].Type != game.TeamComputer {
		switch {
		case reactor.Order == game.OrderSentry:
			// a unit ON GUARD always answers

		case g.State.Teams[reactor.Team].FinishedTurn:
			// the team committed its turn: the commit drain may return fire

		case g.moverThreatensTeam(mover, reactor.Team):
			// the mover can shoot a friendly from where it stands: a real threat

		default:
			return false
		}
	}
	// A reactor that is powered down, disabled, or self destructing does not react.
	switch reactor.Order {
	case game.OrderIdle, game.OrderDisable, game.OrderExplode:
		return false
	}
	// A reactor in the middle of its own move holds (the original: ORDER_MOVE not executing).
	if g.HasOrder(reactor) {
		return false
	}
	if g.Vis != nil {
		// Fog: a reactor only reacts to a mover its own team can actually see.
		if !g.Vis.VisibleToTeam(mover, reactor.Team) {
			return false
		}
		// A hidden stealth unit does not reveal itself by reacting.
		switch reactor.Type {
		case assets.Commando, assets.Submarne:
			if !g.Vis.VisibleToTeam(reactor, mover.Team) {
				return false
			}
		}
	}
	// A valid enemy target, in range, with shots and ammo: combat.CanAttack covers the
	// team / target class / surface / range / shots / ammo checks, including the
	// gate that lets anti air weapons fire only at air targets (the SP_FLAK/ANTIAIR/FASTBOAT clause of ShouldAttack).
	return combat.CanAttack(reactor, mover, g.Grid.Surf)
}

// moverThreatensTeam reports whether mover, from where it stands, could attack
// one of team's units: the heuristic that lets a plain awaiting human unit answer
// a REAL threat during concurrent enemy movement. combat.CanAttack covers the
// mover's shots/ammo, its weapon range from the current cell and the target class,
// which is the shots > 0 + squared range sweep over the team's unit lists.
// C++ ported : units_manager.cpp:4336-4362.
func (g *Game) moverThreatensTeam(mover *game.UnitInfo, team int) bool {
	for _, u := range g.Units.Units() {
		if u.Team == team && combat.CanAttack(mover, u, g.Grid.Surf) {
			return true
		}
	}
	return false
}

// sortByDistTo orders units nearest first by squared grid distance to ref,
// breaking equal distances on unit ID (insertion sort; reactor lists are short).
// Without the ID key, equidistant reactors would keep the random Units() map
// iteration order and the reaction sequence would vary run to run.
func sortByDistTo(list []*game.UnitInfo, ref *game.UnitInfo) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && reactBefore(list[j], list[j-1], ref); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

// reactBefore reports whether a fires before b: nearer to ref first, lowest ID
// on a distance tie (the project's standing Units() ordering convention).
func reactBefore(a, b, ref *game.UnitInfo) bool {
	da, db := reactDistSq(a, ref), reactDistSq(b, ref)
	if da != db {
		return da < db
	}
	return a.ID < b.ID
}

func reactDistSq(a, b *game.UnitInfo) int {
	dx, dy := a.GridX-b.GridX, a.GridY-b.GridY
	return dx*dx + dy*dy
}
