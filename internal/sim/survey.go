// survey.go : drives Surveyors over the ground around the base first, then toward the nearest unsurveyed cell.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : tasksurvey.cpp - TaskSurvey::FindDestination
// C++ ported : taskautosurvey.cpp - TaskAutoSurvey::Execute (the base centred search)
//
// NearestUnsurveyedMove serves player AutoSurvey and the AI taskSurvey routine.
package sim

import "github.com/suglasp/max-go/internal/game"

// baseSurveyRadius is how far around the team's buildings a Surveyor maps before it
// heads out over the rest of the map.
const baseSurveyRadius = 8

// NearestUnsurveyedMove orders u toward the next cell the given team has not yet
// surveyed, returning true when a move was issued. Ground within baseSurveyRadius
// of the team's base comes first, nearest ring first; once that is mapped it rings
// outward from the unit over the whole map. The surveyor reveals resource deposits
// as it moves, so the first unsurveyed ring cell is the natural frontier. CanEnter
// doubles as the surface gate (a land surveyor never targets water). Returns false
// when every reachable cell is already surveyed or the game has no resource map
// (headless tests).
//
// Shared by the human player (gamescreen per turn driver) and the AI
// taskSurvey (which delegates here instead of duplicating the logic).
//
// C++ ported : tasksurvey.cpp - TaskSurvey::FindDestination (ring scan
// approximating the AreaMap weighted search).
func (g *Game) NearestUnsurveyedMove(u *game.UnitInfo, team int) bool {
	return g.NearestUnsurveyedMoveAvoiding(u, team, nil, nil)
}

// NearestUnsurveyedMoveAvoiding is NearestUnsurveyedMove with two optional AI
// hooks so a computer surveyor stays out of a kill zone the way the AI's other
// recon does (see task_explore). skip rejects a candidate destination cell (the
// AI passes "under enemy fire"), so the surveyor never picks a dangerous frontier
// to drive to; penalty bends the chosen route away from covered ground the same
// way OrderMoveAvoiding does for combat units. Both nil gives the plain player
// behaviour. Both passes (base first, then the whole map) apply the hooks.
//
// C++ ported : tasksurvey.cpp - TaskSurvey folding the AI's danger map into its
// destination search (the AreaMap weighting the port reduces to skip + penalty).
func (g *Game) NearestUnsurveyedMoveAvoiding(u *game.UnitInfo, team int, skip func(x, y int) bool, penalty func(x, y int) int) bool {
	// No resource map - nothing to survey (headless harness).
	if g == nil || g.Resources == nil {
		return false
	}
	if u.GridX < 0 || u.GridY < 0 {
		return false
	}

	// Choose the larger map dimension as the ring limit so the scan can reach
	// every cell even on a non square map.
	maxR := g.W
	if g.H > maxR {
		maxR = g.H
	}

	// Pass 1: the unsurveyed ground around the base.
	if zone, reach := g.baseSurveyZone(u, team); zone != nil {
		inZone := func(x, y int) bool { return zone[y*g.W+x] }
		if g.ringSurveyMove(u, team, reach, inZone, skip, penalty) {
			return true
		}
	}

	// Pass 2: the nearest unsurveyed cell anywhere.
	return g.ringSurveyMove(u, team, maxR, nil, skip, penalty)
}

// ringSurveyMove rings outward from u up to maxR and orders a move to the first
// unsurveyed, enterable, reachable cell that accept (nil = any cell) and skip let
// through. OrderMoveAvoiding returns false for an unreachable target, so the scan
// keeps going on the outer rings.
func (g *Game) ringSurveyMove(u *game.UnitInfo, team, maxR int, accept func(x, y int) bool, skip func(x, y int) bool, penalty func(x, y int) int) bool {
	for r := 1; r <= maxR; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				// Only visit the outer ring perimeter; inner cells are
				// handled by smaller values of r.
				if dx > -r && dx < r && dy > -r && dy < r {
					continue
				}
				x, y := u.GridX+dx, u.GridY+dy
				if x < 0 || y < 0 || x >= g.W || y >= g.H {
					continue
				}
				// Outside the area this pass covers.
				if accept != nil && !accept(x, y) {
					continue
				}
				// Skip already surveyed cells; stop if this cell is new.
				if g.Resources.IsSurveyed(x, y, team) {
					continue
				}
				// A fragile surveyor does not drive into ground under enemy
				// fire; ring on past it to a safer frontier (AI hook).
				if skip != nil && skip(x, y) {
					continue
				}
				// Surface gate: a land surveyor does not target water, etc.
				if !g.CanEnter(u, x, y) {
					continue
				}
				// Issue the move; a nil penalty is the plain terrain route.
				if g.OrderMoveAvoiding(u, x, y, penalty) {
					return true
				}
			}
		}
	}
	return false
}

// baseSurveyZone marks the cells within baseSurveyRadius of the team's base and
// returns them with the ring radius from u that reaches the whole zone. The base is
// every own building; before the first one stands it is the landing group, the
// mean position of the team's other units. Returns nil when there is no base or no
// unsurveyed cell left in it, so the caller goes straight to the whole map.
func (g *Game) baseSurveyZone(u *game.UnitInfo, team int) ([]bool, int) {
	var centres []game.Point
	sumX, sumY, others := 0, 0, 0
	for _, o := range g.Units.Units() {
		if o.Team != team || o == u || o.GridX < 0 || o.GridY < 0 {
			continue
		}
		if o.Flags.Has(game.FlagStationary) && !o.Flags.Has(game.FlagGroundCover) {
			centres = append(centres, game.Point{X: o.GridX, Y: o.GridY})
		}
		sumX += o.GridX
		sumY += o.GridY
		others++
	}
	if len(centres) == 0 {
		if others == 0 {
			return nil, 0
		}
		centres = append(centres, game.Point{X: sumX / others, Y: sumY / others})
	}

	zone := make([]bool, g.W*g.H)
	open, reach := 0, 0
	for _, c := range centres {
		for y := c.Y - baseSurveyRadius; y <= c.Y+baseSurveyRadius; y++ {
			for x := c.X - baseSurveyRadius; x <= c.X+baseSurveyRadius; x++ {
				if x < 0 || y < 0 || x >= g.W || y >= g.H || zone[y*g.W+x] {
					continue
				}
				dx, dy := x-c.X, y-c.Y
				if dx*dx+dy*dy > baseSurveyRadius*baseSurveyRadius {
					continue
				}
				zone[y*g.W+x] = true
				if g.Resources.IsSurveyed(x, y, team) {
					continue
				}
				open++
				// The ring radius that reaches this cell from the surveyor.
				reach = max(reach, abs(x-u.GridX), abs(y-u.GridY))
			}
		}
	}
	if open == 0 {
		return nil, 0
	}
	return zone, reach
}
