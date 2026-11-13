// reminder.go : the M.A.X. unit AI reminders.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

// reminderType tags a reminder for logging/accounting. C++: enums.hpp
// REMINDER_TYPE_*.
type reminderType int

// Only the turn start and turn end kinds have a producer here. The original's other
// three (available / move / attack) are raised from unit events as a unit frees up,
// finishes a move or lands a shot; this port resolves a computer turn synchronously
// before anything animates, so no unit event exists to raise them. Same missing seam as
// the tasks pending flag (aiplayer.go).
const (
	reminderTurnStart reminderType = iota
	reminderTurnEnd
)

// Reminder is a deferred task callback. Tasks do not run their logic inline;
// they queue reminders, and the manager drains them in ExecuteReminders so the
// AI's work is spread across the game loop instead of blocking a frame. C++:
// class Reminder (reminders.hpp).
type Reminder interface {
	// Execute runs the deferred callback. C++: Reminder::Execute.
	Execute()

	// Type is the reminder kind. C++: Reminder::GetType.
	Type() reminderType
}

// remindTurnStart fires a task's BeginTurn when drained. C++: class
// RemindTurnStart (reminders.cpp).
type remindTurnStart struct{ task Task }

func newRemindTurnStart(task Task) *remindTurnStart {
	task.setScheduledForTurnStart(true)

	return &remindTurnStart{task: task}
}

// Execute runs the task's start of turn planning, unless the task (or a parent
// above it) was removed while the reminder waited in the queue.
func (r *remindTurnStart) Execute() {
	r.task.setScheduledForTurnStart(false)

	// A task that is no longer needed plans nothing.
	if !r.task.IsNeeded() {
		return
	}

	r.task.BeginTurn()
}

func (r *remindTurnStart) Type() reminderType {
	return reminderTurnStart
}

// remindTurnEnd fires a task's EndTurn when drained. C++: class RemindTurnEnd
// (reminders.cpp).
type remindTurnEnd struct{ task Task }

func newRemindTurnEnd(task Task) *remindTurnEnd {
	task.setScheduledForTurnEnd(true)

	return &remindTurnEnd{task: task}
}

// Execute runs the task's end of turn evaluation, unless the task (or a parent
// above it) was removed while the reminder waited in the queue.
func (r *remindTurnEnd) Execute() {
	r.task.setScheduledForTurnEnd(false)

	// A task that is no longer needed evaluates nothing.
	if !r.task.IsNeeded() {
		return
	}

	r.task.EndTurn()
}

func (r *remindTurnEnd) Type() reminderType {
	return reminderTurnEnd
}
