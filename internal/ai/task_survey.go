// task_survey.go : M.A.X. AI drive idle Surveyors over the ground around the base first, then toward the nearest unsurveyed cell
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// taskSurvey drives a team's idle Surveyor over the unsurveyed ground around its
// base first, then toward the nearest cell its team has not yet surveyed, so the computer reveals resource deposits the same way the
// player does - by moving a surveyor over the ground - instead of reading the
// deposit map omnisciently. As the surveyor moves, the gamescreen survey tick
// (which runs for every team, not just the player) marks the team's survey
// overlay around it; that overlay is exactly what taskManageBuildings.depositScore
// consults to site mining stations. So a deposit only becomes a candidate mine
// once the surveyor has driven within reach of it.
//
// It runs alongside the other roots and does not reschedule itself;
// TaskManager.BeginTurn queues it again each turn.
//
// The base first ring scan is in sim.Game.NearestUnsurveyedMove (shared with
// the human player's Auto Survey feature so the two stay in sync).
//
// C++ ported : tasksurvey.cpp - TaskSurvey
type taskSurvey struct {
	TaskBase
	ctx *aiContext
	// tm is this turn's team threat map, built lazily for the first ready surveyor. A
	// surveyor is unarmed and fragile, so it does not drive into a cell under enemy
	// fire (or a spot where the team recently lost a unit) and routes around defended
	// ground, the same self preservation task_explore gives the scanners and scouts.
	tm *threatMap
}

func newTaskSurvey(team int, ctx *aiContext) *taskSurvey {
	return &taskSurvey{
		TaskBase: newTaskBase(team, nil, taskPriorityAutoSurvey),
		ctx:      ctx,
	}
}

// BeginTurn sends each ready idle surveyor to the next unsurveyed cell, base first.
// With no resource map (the headless harness) there is nothing to survey.
// C++: TaskSurvey::Execute issuing a move for its surveyor.
func (t *taskSurvey) BeginTurn() {
	g := t.ctx.Game
	if g == nil || g.Resources == nil {
		return
	}

	t.tm = nil // built lazily on the first ready surveyor (buildThreatMap is O(enemies x range^2))
	for _, u := range g.Units.Units() {
		// A ready surveyor is this task's for the pass.
		if !t.isReadySurveyor(u) || !t.claim(u) {
			continue
		}

		if t.tm == nil {
			t.tm = buildThreatMap(g, t.team)
		}

		// Delegate to the shared sim ring scan, but with the AI's danger hooks so
		// the surveyor skips ground under enemy fire and routes around it (the
		// player's Auto Survey passes neither hook and drives the plain path).
		skip := func(x, y int) bool { return t.tm.at(x, y) > 0 }
		g.NearestUnsurveyedMoveAvoiding(u, t.team, skip, dangerPenalty(t.tm))
	}
}

// isReadySurveyor reports whether u is one of this team's surveyors free to scout
// for resources this turn: a Surveyor with movement budget left and no order yet.
func (t *taskSurvey) isReadySurveyor(u *game.UnitInfo) bool {
	if u.Team != t.team || u.Type != assets.Surveyor {
		return false
	}

	if u.Speed <= 0 {
		return false // no movement budget left this turn
	}

	return !t.ctx.Game.HasOrder(u)
}
