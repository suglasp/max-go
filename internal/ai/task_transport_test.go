// task_transport_test.go : testing unit for task_transport.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// askingTask is a plain task that asks the ferry for a ride each turn, to show the manager
// runs the ferry after it.
type askingTask struct {
	TaskBase
	ferry *taskTransport
	unit  *game.UnitInfo
}

// BeginTurn asks the ferry for a ride across the strait.
func (a *askingTask) BeginTurn() {
	a.ferry.request(a.unit, game.Point{X: 9, Y: 1})
}

// TestTaskTransportIsATransportTask checks the ferry reports the transport type, which is
// what moves it into the manager's last pass.
func TestTaskTransportIsATransportTask(t *testing.T) {
	ferry := newTaskTransport(1, &aiContext{})
	if ferry.Type() != TaskTypeTransport {
		t.Errorf("Type() = %v, want TaskTypeTransport", ferry.Type())
	}
}

// TestTaskTransportKeepsOneRequestPerUnit checks a second request for the same unit
// replaces the destination, and a nil ferry ignores requests.
func TestTaskTransportKeepsOneRequestPerUnit(t *testing.T) {
	g := straitGame()
	tank := ferryTestUnit(1, 1, assets.Tank, flagLandUnit, 1, 1, g.Units)
	other := ferryTestUnit(1, 2, assets.Tank, flagLandUnit, 1, 2, g.Units)
	ferry := newTaskTransport(1, &aiContext{Game: g})

	ferry.request(tank, game.Point{X: 8, Y: 0})
	ferry.request(other, game.Point{X: 8, Y: 2})
	ferry.request(tank, game.Point{X: 9, Y: 1})

	if len(ferry.requests) != 2 {
		t.Fatalf("requests = %d, want 2 (one per unit)", len(ferry.requests))
	}

	if ferry.requests[0].unit != tank || ferry.requests[0].dest != (game.Point{X: 9, Y: 1}) {
		t.Errorf("tank request = %+v, want the newest destination (9,1)", ferry.requests[0])
	}

	var noFerry *taskTransport
	noFerry.request(tank, game.Point{X: 9, Y: 1}) // must not panic
}

// TestTaskTransportClearsRequestsEachTurn checks BeginTurn plans and drops every request,
// a dead unit's included, so the requesters have to ask again next turn.
func TestTaskTransportClearsRequestsEachTurn(t *testing.T) {
	g := straitGame()
	tank := ferryTestUnit(1, 1, assets.Tank, flagLandUnit, 1, 1, g.Units)
	dead := ferryTestUnit(1, 2, assets.Tank, flagLandUnit, 1, 2, g.Units)
	dead.Hits = 0
	ferry := newTaskTransport(1, &aiContext{Game: g})

	ferry.request(tank, game.Point{X: 9, Y: 1})
	ferry.request(dead, game.Point{X: 9, Y: 2})
	ferry.BeginTurn()

	if len(ferry.requests) != 0 {
		t.Errorf("requests after BeginTurn = %d, want 0", len(ferry.requests))
	}
}

// TestTaskManagerRunsTheFerryLast checks the manager's turn start runs the transport task
// after a task registered later than it, so the ferry sees that task's request.
func TestTaskManagerRunsTheFerryLast(t *testing.T) {
	g := straitGame()
	tank := ferryTestUnit(1, 1, assets.Tank, flagLandUnit, 1, 1, g.Units)
	ferry := newTaskTransport(1, &aiContext{Game: g})
	asker := &askingTask{TaskBase: newTaskBase(1, nil, 500), ferry: ferry, unit: tank}

	mgr := NewTaskManager()
	mgr.AppendTask(ferry) // registered first on purpose
	mgr.AppendTask(asker)

	// The first reminders follow the registration order: the ferry runs before the ask.
	mgr.ExecuteReminders()
	if len(ferry.requests) != 1 {
		t.Fatalf("after the first drain: requests = %d, want 1 (the ferry ran first)", len(ferry.requests))
	}

	// A turn start puts transport tasks last: the ask comes in, then the ferry clears it.
	mgr.BeginTurn(1)
	mgr.ExecuteReminders()
	if len(ferry.requests) != 0 {
		t.Errorf("after the turn start: requests = %d, want 0 (the ferry ran last)", len(ferry.requests))
	}
}

// TestAttackerAsksTheFerryForARide checks the live seam: an attacker whose target sits on
// the far island finds no route and leaves a ride request for the target's cell.
func TestAttackerAsksTheFerryForARide(t *testing.T) {
	g := gameFromSurf(newPatternSurf([]string{
		"####~~~~~~~####",
		"####~~~~~~~####",
		"####~~~~~~~####",
	}), 15, 3)
	attacker := armedTank(1, 1, 1, 1, g.Units)
	ctx := &aiContext{Game: g, MakeUnit: testUnitFactory}
	attack := newTaskAttackEnemies(1, ctx)
	attack.ferry = newTaskTransport(1, ctx)

	if attack.advanceToward(attacker, 13, 1) {
		t.Fatal("advanceToward found a land route to the far island")
	}

	if len(attack.ferry.requests) != 1 {
		t.Fatalf("ferry requests = %d, want 1", len(attack.ferry.requests))
	}

	request := attack.ferry.requests[0]
	if request.unit != attacker || request.dest != (game.Point{X: 13, Y: 1}) {
		t.Errorf("request = %+v, want the attacker riding to (13,1)", request)
	}
}
