// haltreminder_test.go : testing unit for the halt reminder in heldroute.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// haltedComputerMover builds a computer team 0 tank (alive, one hop of a long drive done) wired to a recording AI,
// with its move ordered but not yet halted.
func haltedComputerMover(t *testing.T) (*Game, *game.UnitInfo, *countingAI) {
	t.Helper()

	g := newGame(t, 16, 4)
	g.State.Teams[0].Type = game.TeamComputer
	rec := &countingAI{}
	g.AI = rec
	tank := addTank(g, 1, 6, 1, 1)
	tank.Hits = 10

	g.BeginTeamTurn(0)
	rec.teams = nil // the turn start call is not the subject here

	if !g.OrderMove(tank, 7, 1) {
		t.Fatal("OrderMove should route across open land")
	}

	return g, tank, rec
}

// TestHaltReminderWakesTheAIOnce checks that a halted computer unit reaches the AI on the next Tick, once, and that
// the waiting reminder keeps the turn open until then.
func TestHaltReminderWakesTheAIOnce(t *testing.T) {
	g, tank, rec := haltedComputerMover(t)

	g.HaltForEnemy(tank)
	g.HaltForEnemy(tank) // a second halt in the same turn must not queue a second reminder

	if got := g.ActiveOrders(); got != 1 {
		t.Errorf("active orders = %d after the halt, want 1: the queued reminder keeps the turn open", got)
	}

	if len(rec.finished) != 0 {
		t.Fatal("the AI was reminded before a Tick; the reminder belongs to the next Tick")
	}

	g.Tick()

	if len(rec.finished) != 1 || rec.finished[0] != tank {
		t.Fatalf("AI reminded of %v, want exactly the halted tank", rec.finished)
	}

	if got := g.ActiveOrders(); got != 0 {
		t.Errorf("active orders = %d after the answer, want 0", got)
	}

	g.Tick()

	if len(rec.finished) != 1 {
		t.Errorf("AI reminded %d times, want once per halt", len(rec.finished))
	}
}

// TestHaltReminderSkipsHumanUnits checks that a halted human unit is never queued: it waits for Continue.
func TestHaltReminderSkipsHumanUnits(t *testing.T) {
	g := newGame(t, 16, 4)
	rec := &countingAI{}
	g.AI = rec
	tank := addTank(g, 1, 6, 1, 1)
	tank.Hits = 10

	g.BeginTeamTurn(0)
	g.OrderMove(tank, 7, 1)
	g.HaltForEnemy(tank)

	if got := g.ActiveOrders(); got != 0 {
		t.Errorf("active orders = %d, want 0: a human unit queues no reminder", got)
	}

	g.Tick()

	if len(rec.finished) != 0 {
		t.Errorf("AI reminded of a human unit %d times, want 0", len(rec.finished))
	}
}

// TestHaltReminderWaitsForTheMidCellCrossing checks that a unit halted halfway into a cell reminds the AI only after
// the crossing has settled, never while its order is still running.
func TestHaltReminderWaitsForTheMidCellCrossing(t *testing.T) {
	g, tank, rec := haltedComputerMover(t)

	for i := 0; i < 200 && tank.Moved == 0; i++ {
		g.Tick()
	}

	if tank.Moved == 0 {
		t.Fatal("the tank never started crossing into the next cell")
	}

	g.HaltForEnemy(tank)

	for i := 0; i < 400 && g.ActiveOrders() > 0; i++ {
		before := len(rec.finished)
		g.Tick()

		if len(rec.finished) > before && g.HasOrder(tank) {
			t.Fatal("the AI was reminded while the tank was still crossing its cell")
		}
	}

	if len(rec.finished) != 1 {
		t.Errorf("AI reminded %d times after the crossing, want 1", len(rec.finished))
	}

	if tank.GridX != 2 {
		t.Errorf("tank parked at x=%d, want the crossed cell 2", tank.GridX)
	}
}

// TestHaltReminderDropsADeadUnit checks that a unit destroyed before the answer sends no reminder and leaves nothing
// queued behind.
func TestHaltReminderDropsADeadUnit(t *testing.T) {
	g, tank, rec := haltedComputerMover(t)

	g.HaltForEnemy(tank)
	tank.Hits = 0

	g.Tick()

	if len(rec.finished) != 0 {
		t.Errorf("AI reminded of a dead unit %d times, want 0", len(rec.finished))
	}

	if got := g.ActiveOrders(); got != 0 {
		t.Errorf("active orders = %d, want 0: the dead unit's reminder is dropped", got)
	}
}

// TestHaltReminderWithoutAIIsDropped checks that a game with no AI wired just drops the reminder, so the turn can end.
func TestHaltReminderWithoutAIIsDropped(t *testing.T) {
	g, tank, _ := haltedComputerMover(t)
	g.AI = nil

	g.HaltForEnemy(tank)
	g.Tick()

	if got := g.ActiveOrders(); got != 0 {
		t.Errorf("active orders = %d, want 0 with no AI to answer", got)
	}
}

// TestBeginTeamTurnClearsTheTeamsReminders checks that a reminder never outlives its turn, and that another team's
// reminder is left alone.
func TestBeginTeamTurnClearsTheTeamsReminders(t *testing.T) {
	g, tank, rec := haltedComputerMover(t)
	g.State.Teams[1].Type = game.TeamComputer
	other := addHarmlessMover(g, 2, 1, 1, 3)

	g.OrderMove(other, 7, 3)
	g.HaltForEnemy(other)
	g.HaltForEnemy(tank)

	g.BeginTeamTurn(0) // clears team 0's reminder, keeps team 1's
	rec.teams = nil

	// Run the resumed drive out so a reminder that survived the turn start would fire once its order is gone.
	for i := 0; i < 400 && g.ActiveOrders() > 0; i++ {
		g.Tick()
	}

	g.Tick()

	if len(rec.finished) != 1 || rec.finished[0] != other {
		t.Errorf("AI reminded of %v, want only team 1's unit: team 0's reminder died with its turn", rec.finished)
	}
}
