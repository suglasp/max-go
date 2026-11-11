// task_transport.go : the team's ferry task, which collects the units that need a ride across water.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : tasktransport.cpp - TaskTransport; taskmove.cpp - TaskMove::FindTransport
package ai

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// ferryRequest is one unit that found no route of its own, and the cell it wants to reach.
type ferryRequest struct {
	unit *game.UnitInfo
	dest game.Point
}

// taskTransport ferries the team's units that have no land route. Other tasks ask for a
// ride when their move fails; the manager runs this task last in the turn, after every
// request is in. The requests are rebuilt each turn, so a save holds no ferry state.
// This first stage plans each ride and logs it; the transporter does not move yet.
// C++ ported : tasktransport.cpp - TaskTransport (one flat task per team instead of one
// task per transporter)
type taskTransport struct {
	TaskBase
	ctx      *aiContext
	requests []ferryRequest
}

// newTaskTransport builds the team's ferry task.
// C++ ported : tasktransport.cpp - TaskTransport::TaskTransport
func newTaskTransport(team int, ctx *aiContext) *taskTransport {
	return &taskTransport{
		TaskBase: newTaskBase(team, nil, taskPriorityAttackDefault), // a ride takes the priority of the move that asked for it (an attack move)
		ctx:      ctx,
	}
}

// Type marks the task as a transport task, so the manager schedules it after the others.
// C++ ported : tasktransport.cpp - TaskTransport::GetType
func (t *taskTransport) Type() TaskType {
	return TaskTypeTransport
}

// request asks for a ride for unit to dest. A unit has one request at a time; a newer
// destination replaces the older one. A nil task takes no requests.
// C++ ported : taskmove.cpp - TaskMove::FindTransport (reduced to a per team request list)
func (t *taskTransport) request(unit *game.UnitInfo, dest game.Point) {
	// A task without a ferry (some tests) simply drops the request.
	if t == nil || unit == nil {
		return
	}

	for i := range t.requests {
		// The same unit asks again: keep its newest destination.
		if t.requests[i].unit == unit {
			t.requests[i].dest = dest
			return
		}
	}

	t.requests = append(t.requests, ferryRequest{unit: unit, dest: dest})
}

// BeginTurn plans a ride for each request of this turn and logs it, then clears the list.
// C++ ported : tasktransport.cpp - TaskTransport::ChooseNewTask (planning part)
func (t *taskTransport) BeginTurn() {
	g := t.ctx.Game

	// Without a game there is nothing to plan against.
	if g == nil {
		t.requests = t.requests[:0]
		return
	}

	for _, request := range t.requests {
		unit := request.unit

		// A unit that died or already rides a transporter needs no plan.
		if unit.Hits <= 0 || unit.Parent != nil {
			continue
		}

		transporter, ok := chooseTransporter(g, t.team, unit)
		if !ok {
			continue
		}

		route, ok := planFerry(g, unit, request.dest, transporter)
		if !ok {
			dbg.Logf(dbg.AI, "team %d ferry: no ride for %s at (%d,%d) to (%d,%d)", t.team, unit.Type.Tag(), unit.GridX, unit.GridY, request.dest.X, request.dest.Y)
			continue
		}

		dbg.Logf(dbg.AI, "team %d ferry: %s at (%d,%d) to (%d,%d) by %s, pickup (%d,%d), landing (%d,%d)", t.team, unit.Type.Tag(), unit.GridX, unit.GridY, request.dest.X, request.dest.Y, transporter.Tag(), route.pickup.X, route.pickup.Y, route.landing.X, route.landing.Y)
	}

	t.requests = t.requests[:0]
}
