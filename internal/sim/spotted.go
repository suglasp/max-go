// spotted.go : tracks which teams have detected each unit.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - spotted_by_team update on move/build
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// SpottedUnit is one team's memory of a single enemy unit it has seen. LastX/LastY
// is the cell where that unit was last sighted; it stays FROZEN once the unit slips
// back into fog (Visible goes false) and only refreshes while the unit is currently
// visible to the team. This is the intel that lets the AI keep pursuing an enemy it
// has lost sight of instead of forgetting it the moment it leaves scan coverage.
//
// RevealReacted prevents repeated opportunity fire when fog hides and reveals the same contact. It persists until the unit dies or changes team.
// C++ ported : spottedunit.cpp - SpottedUnit (unit / team / last_position / visible_to_team), reduced to the fields the port's AI consumes.
type SpottedUnit struct {
	Unit          *game.UnitInfo
	Team          int
	LastX         int
	LastY         int
	Visible       bool
	RevealReacted bool // reveal reaction already fired
}

// spottedMemory holds each team's detected enemies by live unit pointer. Maps are allocated lazily, so test games without fog need no detection state.
type spottedMemory struct {
	teams [game.TeamCount]map[*game.UnitInfo]*SpottedUnit
}

func (m *spottedMemory) ensure() {
	for t := range m.teams {
		if m.teams[t] == nil {
			m.teams[t] = make(map[*game.UnitInfo]*SpottedUnit)
		}
	}
}

// spottedTrackable reports whether a unit is the kind that enters a team's spotted
// memory at all. Ground cover (roads, slabs, rubble) is ignored - it is terrain, not
// a contact - EXCEPT laid mines, which the AI does track. C++: the GROUND_COVER /
// LANDMINE / SEAMINE gate at the top of AiPlayer::UnitSpotted.
func spottedTrackable(u *game.UnitInfo) bool {
	if !u.Flags.Has(game.FlagGroundCover) {
		return true
	}
	return u.Type == assets.LandMine || u.Type == assets.SeaMine
}

// updateSpotted refreshes every team's spotted memory from the current fog. For each
// enemy a team can currently see it records (or refreshes) the sighting at the unit's
// live cell; an enemy that has dropped out of sight keeps its entry with the position
// FROZEN at the last sighting. It is the visibility driven analogue of the original firing
// UnitSpotted on each enemy as it enters scan coverage - we sweep on every fog
// recompute instead (O(teams x units); the unit count is small). A unit that has
// defected to the team (commando steal) is pruned; death is handled by forgetSpotted.
//
// C++ ported : aiplayer.cpp - AiPlayer::UnitSpotted / SpottedUnit::UpdatePositionIfVisible.
func (g *Game) updateSpotted() {
	if g.Vis == nil {
		return // no fog model wired (minimal harness): SpottedByTeam falls back to "all spotted"
	}
	g.spotted.ensure()
	for t := 0; t < game.TeamCount; t++ {
		mem := g.spotted.teams[t]
		for _, e := range g.Units.Units() {
			if e.Team == t || e.Team < 0 || e.Team >= game.TeamCount {
				continue // own unit or no valid enemy team
			}
			if e.GridX < 0 || e.GridY < 0 || e.Parent != nil {
				continue // off the map or aboard a transport: not on the field to be seen
			}
			if !spottedTrackable(e) {
				continue
			}
			su := mem[e]
			if g.Vis.VisibleToTeam(e, t) {
				if su == nil {
					su = &SpottedUnit{Unit: e, Team: t, LastX: e.GridX, LastY: e.GridY, Visible: true}
					mem[e] = su
					g.queueReveal(su) // a brand new contact: a reveal edge
				} else {
					if !su.Visible {
						g.queueReveal(su) // a remembered contact slipping back into sight: false->true edge
					}
					su.LastX, su.LastY, su.Visible = e.GridX, e.GridY, true
				}
			} else if su != nil {
				su.Visible = false // remembered, but the last seen position stays frozen
			}
		}
		for e, su := range mem {
			if su.Unit.Team == t {
				delete(mem, e) // captured/defected: no longer an enemy of this team
			}
		}
	}
}

// queueReveal records a hidden->visible reveal edge so a team's defenders can take
// one opportunity shot at the newly revealed enemy. It only QUEUES (updateSpotted
// also runs in the render fog refresh and at setup, where firing would be wrong);
// FireRevealReactions resolves the queue in the gameplay update. Gated on
// reactOnReveal so setup/deploy/load - and any enemy visible from the start - never
// queues, and latched per contact (RevealReacted) so a fog flicker does not re fire.
// C++ context: vanilla large range units fire when fog lifts on an in range enemy.
func (g *Game) queueReveal(su *SpottedUnit) {
	if !g.reactOnReveal || su.RevealReacted {
		if dbg.Enabled() {
			dbg.Printf("reveal: SUPPRESSED team %d unit id=%d (reactOnReveal=%v alreadyReacted=%v)", su.Team, su.Unit.ID, g.reactOnReveal, su.RevealReacted)
		}
		return
	}
	su.RevealReacted = true
	g.pendingReveals = append(g.pendingReveals, revealReaction{team: su.Team, enemy: su.Unit})
	if dbg.Enabled() {
		dbg.Printf("reveal: team %d gained sight of unit id=%d type=%v at (%d,%d) -> queued reveal-fire", su.Team, su.Unit.ID, su.Unit.Type, su.Unit.GridX, su.Unit.GridY)
	}
}

// forgetSpotted drops a unit from every team's spotted memory - called when the unit
// is destroyed so no team keeps chasing a ghost cell. (delete on a nil map is a
// no op, so this is safe before the maps are first allocated.)
//
// C++ ported : aiplayer.cpp - AiPlayer::RemoveUnit (spotted_units.Remove).
func (g *Game) forgetSpotted(u *game.UnitInfo) {
	for t := range g.spotted.teams {
		delete(g.spotted.teams[t], u)
	}
}

// SpottedByTeam reports whether enemy e is in team's spotted memory - i.e. the team
// has seen it at least once and it has not been forgotten. This is a SUPERSET of
// "currently visible": a once seen enemy that has ducked back into fog stays spotted
// (with its position frozen, see LastSeen). With no fog model wired (a minimal test
// harness) it treats everything as spotted, matching the AI's prior fallback.
func (g *Game) SpottedByTeam(e *game.UnitInfo, team int) bool {
	if g.Vis == nil {
		return true
	}
	if team < 0 || team >= game.TeamCount {
		return false
	}
	_, ok := g.spotted.teams[team][e]
	return ok
}

// LastSeen returns the cell where team last saw enemy e, frozen while it is fogged and live while visible. It returns ok=false when the team has
// never spotted e. AI pursuit and reports use this last-known position.
//
// C++ ported : spottedunit.cpp - SpottedUnit::GetLastPosition.
func (g *Game) LastSeen(team int, e *game.UnitInfo) (x, y int, ok bool) {
	if team < 0 || team >= game.TeamCount {
		return 0, 0, false
	}
	su := g.spotted.teams[team][e]
	if su == nil {
		return 0, 0, false
	}
	return su.LastX, su.LastY, true
}
