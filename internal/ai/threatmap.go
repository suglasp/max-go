// threatmap.go : M.A.X. AI threat map
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// threatMap stores the weighted firepower that spotted enemies can project into each cell. Units prefer lower values when fleeing or choosing firing positions.
// C++: threatmap.cpp, ThreatMap.
type threatMap struct {
	w, h int
	cell [][]int
}

// at returns the threat at (x, y), or 0 off map / for a nil map.
func (m *threatMap) at(x, y int) int {
	if m == nil || x < 0 || y < 0 || x >= m.w || y >= m.h {
		return 0
	}

	return m.cell[x][y]
}

// buildThreatMap stamps each spotted enemy's weighted firepower over its weapon range, then adds recent friendly loss sites. Unseen enemies contribute nothing.
// Each caller receives a fresh map because aiContext is shared by all teams. C++: aiattack.cpp range stamps feeding ThreatMap.
func buildThreatMap(g *sim.Game, team int) *threatMap {
	m := &threatMap{w: g.W, h: g.H}
	m.cell = make([][]int, g.W)
	for x := range m.cell {
		m.cell[x] = make([]int, g.H)
	}

	for _, e := range g.Units.Units() {
		if e.Team == team || e.Team < 0 || e.Team >= game.TeamCount || e.Values == nil {
			continue
		}

		if e.GridX < 0 || e.GridY < 0 {
			continue
		}

		if e.Values.Get(game.AttribAttack) <= 0 {
			continue // unarmed: no firepower to project
		}

		if !g.SpottedByTeam(e, team) {
			continue // never seen: the team cannot know it threatens this ground
		}

		rng := e.Values.Get(game.AttribRange)
		if rng < 1 {
			rng = 1
		}

		shots := e.Values.Get(game.AttribRounds)
		if shots < 1 {
			shots = 1
		}

		weight := shots * riskLevel(e.Type, e.Flags)

		for dy := -rng; dy <= rng; dy++ {
			for dx := -rng; dx <= rng; dx++ {
				if dx*dx+dy*dy > rng*rng {
					continue // outside the circular weapon range disc (matches combat.InRange)
				}

				x, y := e.GridX+dx, e.GridY+dy
				if x < 0 || y < 0 || x >= g.W || y >= g.H {
					continue
				}

				m.cell[x][y] += weight
			}
		}
	}

	stampLossSites(m, g, team)
	stampKnownMines(m, g, team)

	return m
}

const (
	// lossDangerRadius is how far a remembered unit loss radiates danger: the disc a
	// recon or combat unit steers around after the team lost a unit near its centre.
	lossDangerRadius = 3
	// lossDangerValue is the danger a loss site adds per covered cell. Positive is
	// enough for recon to hard skip the cell; scaled by dangerRouteWeight it also
	// makes a combat unit route around the spot rather than driving back into it.
	lossDangerValue = 5
)

// stampLossSites adds a fading danger disc where the team recently lost a mobile unit, including losses caused by an unseen attacker.
func stampLossSites(m *threatMap, g *sim.Game, team int) {
	losses := g.RecentLosses(team)
	if len(losses) == 0 || g.State == nil {
		return
	}

	now := g.State.TurnCounter
	for _, ls := range losses {
		if now-ls.Turn >= aiLossMemoryTurns {
			continue // forgotten: the team may probe here again
		}

		for dy := -lossDangerRadius; dy <= lossDangerRadius; dy++ {
			for dx := -lossDangerRadius; dx <= lossDangerRadius; dx++ {
				if dx*dx+dy*dy > lossDangerRadius*lossDangerRadius {
					continue
				}

				x, y := ls.X+dx, ls.Y+dy
				if x < 0 || y < 0 || x >= g.W || y >= g.H {
					continue
				}

				m.cell[x][y] += lossDangerValue
			}
		}
	}
}

const (
	// mineDangerValue is the danger a mine the team has FOUND adds to its own cell. Well
	// above lossDangerValue: a mine is not a place a unit might get shot, it is a place it
	// WILL be hit, and scaled by dangerRouteWeight this makes A* walk a long way round
	// rather than across. It stays finite, so a unit with no other way through still takes
	// the hit instead of freezing.
	mineDangerValue = 30
	// mineFieldDangerValue is what the ring around a found mine gets. Mines come in fields:
	// the original spreads a fading threat several cells out from a spotted mine (it assumes
	// its neighbours are mined too), so the AI gives the whole patch a wide berth rather
	// than clipping the corner of it.
	mineFieldDangerValue = 6
	// mineFieldRadius is how far that assumed field reaches.
	mineFieldRadius = 2
)

// stampKnownMines marks the enemy mines this team has UNCOVERED (the sticky SpotBy flag its
// surveyors and mine layers set) so the AI steers around them. The team's own mines are not
// on it: it knows perfectly well where those are and drives over them safely.
//
// Known mines contribute to the threat map, so pathfinding routes around their immediate field.
//
// C++ ported : aiplayer.cpp - AiPlayer::MineSpotted / AddThreatToMineMap (the mine_map layer
// added into damage_potential_map), fed by survey.cpp - Survey_SurveyArea -> Ai_MarkMineMapPoint.
func stampKnownMines(m *threatMap, g *sim.Game, team int) {
	for _, e := range g.Units.Units() {
		if !sim.IsMine(e) || e.Team == team || !e.SpottedByTeam(team) {
			continue
		}

		for dy := -mineFieldRadius; dy <= mineFieldRadius; dy++ {
			for dx := -mineFieldRadius; dx <= mineFieldRadius; dx++ {
				x, y := e.GridX+dx, e.GridY+dy
				if x < 0 || y < 0 || x >= g.W || y >= g.H {
					continue
				}

				if dx == 0 && dy == 0 {
					m.cell[x][y] += mineDangerValue // the mine itself
					continue
				}

				if dx*dx+dy*dy <= mineFieldRadius*mineFieldRadius {
					m.cell[x][y] += mineFieldDangerValue // the field it probably sits in
				}
			}
		}
	}
}

// riskLevel weights how dangerous a unit type is as an attacker, biasing the
// threat map toward the units a fleeing unit most wants to avoid: a Personnel
// Carrier (full of infantry), a submarine, a commando, then air, then everything
// else. The air case reads the unit's live flags.
//
// C++ ported : threatmap.cpp - ThreatMap::GetRiskLevel(ResourceID)
func riskLevel(t assets.UnitType, flags game.Flags) int {
	switch t {
	case assets.ClnTrans:
		return 7
	case assets.Submarne:
		return 6
	case assets.Commando:
		return 5
	case assets.Surveyor:
		return 2
	default:
		if flags.Has(game.FlagMobileAirUnit) {
			return 3
		}

		return 1
	}
}
