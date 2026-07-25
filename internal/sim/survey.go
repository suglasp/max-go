// survey.go : drives Surveyors toward the nearest unsurveyed cell.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : tasksurvey.cpp - TaskSurvey::FindDestination
//
// NearestUnsurveyedMove serves player AutoSurvey and the AI taskSurvey routine.
package sim

import "github.com/suglasp/max-go/internal/game"

// NearestUnsurveyedMove orders u toward the nearest cell the given team has
// not yet surveyed, returning true when a move was issued. It rings outward
// from the unit's current cell; the surveyor reveals resource deposits as it
// moves, so the first unsurveyed ring cell is the natural frontier. CanEnter
// doubles as the surface gate (a land surveyor never targets water). Returns
// false when every reachable cell is already surveyed or the game has no
// resource map (headless tests).
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
// behaviour, so the human Auto Survey path is byte identical.
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
				// OrderMoveAvoiding returns false for unreachable targets but
				// keeps searching outer rings.
				if g.OrderMoveAvoiding(u, x, y, penalty) {
					return true
				}
			}
		}
	}
	return false
}
