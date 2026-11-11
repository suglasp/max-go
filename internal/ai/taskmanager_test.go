// taskmanager_test.go : testing unit for taskmanager.go and the reminders of task.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// orderReminder is a reminder that writes its label into a shared log when it runs.
type orderReminder struct {
	label string
	log   *[]string
}

// Execute records the label.
func (r *orderReminder) Execute() {
	*r.log = append(*r.log, r.label)
}

// Type is a plain reminder kind.
func (r *orderReminder) Type() reminderType {
	return reminderTurnStart
}

// TestExecuteRemindersInterleavesTheQueues checks the drain order: the priority queue goes
// first, but a normal reminder gets through after every two priority ones.
func TestExecuteRemindersInterleavesTheQueues(t *testing.T) {
	m := NewTaskManager()
	var got []string
	for _, label := range []string{"n1", "n2"} {
		m.AppendReminder(&orderReminder{label, &got}, false)
	}
	for _, label := range []string{"p1", "p2", "p3", "p4", "p5"} {
		m.AppendReminder(&orderReminder{label, &got}, true)
	}

	m.ExecuteReminders()

	want := []string{"p1", "p2", "n1", "p3", "p4", "n2", "p5"}
	if len(got) != len(want) {
		t.Fatalf("drain order %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("drain order %v, want %v", got, want)
		}
	}
}

// TestRemindTurnEndQueuesOnce checks the guard: a second end of turn reminder for a task
// that already has one pending queues nothing.
func TestRemindTurnEndQueuesOnce(t *testing.T) {
	m := NewTaskManager()
	task := newProbeTask(0, 0)
	m.AppendTask(task)
	m.ExecuteReminders()

	task.RemindTurnEnd(false)
	task.RemindTurnEnd(false)

	if n := m.RemindersCount(); n != 1 {
		t.Errorf("reminders after two RemindTurnEnd calls = %d, want 1", n)
	}
}

// TestBeginTurnSchedulesOnlyThatTeam checks every pass of the turn start, the ferry pass
// included: another team's tasks get no reminder.
func TestBeginTurnSchedulesOnlyThatTeam(t *testing.T) {
	m := NewTaskManager()
	mine := newProbeTask(1, 0)
	theirs := newProbeTask(2, 0)
	theirFerry := newTaskTransport(2, &aiContext{})
	m.AppendTask(mine)
	m.AppendTask(theirs)
	m.AppendTask(theirFerry)
	m.ExecuteReminders()

	m.BeginTurn(1)

	if theirs.isScheduledForTurnStart() || theirFerry.isScheduledForTurnStart() {
		t.Error("team 1's turn start scheduled a task of team 2")
	}
	if !mine.isScheduledForTurnStart() {
		t.Error("team 1's turn start did not schedule its own task")
	}
}

// TestLedgerSameTierCannotTakeAHeldUnit checks the strict compare: a task exactly as urgent
// as the owner does not take its unit.
func TestLedgerSameTierCannotTakeAHeldUnit(t *testing.T) {
	m := NewTaskManager()
	unit := &game.UnitInfo{ID: 1, Team: 1, Hits: 10}
	owner := newLedgerProbe(m, 1, taskPriorityExplore)
	peer := newLedgerProbe(m, 1, taskPriorityExplore)
	owner.claim(unit)

	if peer.claim(unit) {
		t.Error("a task of the same priority took the owner's unit")
	}
}
