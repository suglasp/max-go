// taskmanager.go : M.A.X. manages tasks for the AI
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import "github.com/suglasp/max-go/internal/dbg"

// reminderDrainCap prevents a task that repeatedly rearms itself from blocking the turn loop.
const reminderDrainCap = 200000

// TaskManager owns every team's tasks and the reminder queues that schedule
// their work. Tasks register reminders; the game loop calls ExecuteReminders to
// drain them. There is one manager shared across all computer teams, matching
// the single global TaskManager in C++. C++: class TaskManager
// (task_manager.cpp / .hpp).
type TaskManager struct {
	tasks             []Task
	normalReminders   []Reminder
	priorityReminders []Reminder
	reminderCounter   int
}

// NewTaskManager builds an empty manager. C++: TaskManager::TaskManager.
func NewTaskManager() *TaskManager {
	return &TaskManager{}
}

// AppendTask registers a task and initialises it. The task is bound to this
// manager first so its Init (which schedules the first reminder) can reach the
// queues. C++: TaskManager::AppendTask.
func (m *TaskManager) AppendTask(task Task) {
	task.bind(task, m)
	m.tasks = append(m.tasks, task)
	task.Init()
}

// AppendReminder queues a reminder on the priority or normal queue. C++:
// TaskManager::AppendReminder.
func (m *TaskManager) AppendReminder(r Reminder, priority bool) {
	if priority {
		m.priorityReminders = append(m.priorityReminders, r)
	} else {
		m.normalReminders = append(m.normalReminders, r)
	}
}

// ExecuteReminders drains the reminder queues, interleaving priority and normal
// reminders the way the engine does: it favours the priority queue but lets a
// normal reminder through after every two priority ones (reminder_counter) so a
// flood of priority work cannot starve the normal queue. Returns whether any
// reminders were present. The C++ version also yields on a per frame time
// budget (TickTimer_HaveTimeToThink): this implementation drains fully in one call
// since a turn is resolved synchronously here. C++: TaskManager::ExecuteReminders.
func (m *TaskManager) ExecuteReminders() bool {
	if len(m.normalReminders)+len(m.priorityReminders) == 0 {
		return false
	}

	for drained := 0; len(m.normalReminders)+len(m.priorityReminders) > 0; drained++ {
		// Guard against a self rescheduling task spinning this drain forever:
		// log who is still queued and bail rather than freeze the frame.
		if drained >= reminderDrainCap {
			m.reportRunawayDrain()
			break
		}

		if len(m.normalReminders) == 0 {
			m.reminderCounter = 0
		}

		var r Reminder
		if m.reminderCounter >= 2 || len(m.priorityReminders) == 0 {
			r = m.normalReminders[0]
			m.normalReminders = m.normalReminders[1:]
			m.reminderCounter = 0
		} else {
			r = m.priorityReminders[0]
			m.priorityReminders = m.priorityReminders[1:]
			m.reminderCounter++
		}

		r.Execute()
	}

	return true
}

// reportRunawayDrain logs a sample of reminders left when ExecuteReminders reaches its drain cap.
func (m *TaskManager) reportRunawayDrain() {
	dbg.Logf(dbg.AI, "RUNAWAY reminder drain hit cap %d: priority=%d normal=%d", reminderDrainCap, len(m.priorityReminders), len(m.normalReminders))

	logged := 0
	for _, queue := range [][]Reminder{m.priorityReminders, m.normalReminders} {
		for _, r := range queue {
			if logged >= 12 {
				return // a sample is enough to name the offender
			}
			if task := reminderTask(r); task != nil {
				dbg.Logf(dbg.AI, "  queued reminder type=%d team=%d taskID=%d", r.Type(), task.Team(), task.ID())
			}
			logged++
		}
	}
}

// reminderTask extracts the task a turn start / turn end reminder fires, for
// diagnostics; returns nil for reminder kinds that carry no task.
func reminderTask(r Reminder) Task {
	switch v := r.(type) {
	case *remindTurnStart:
		return v.task
	case *remindTurnEnd:
		return v.task
	}

	return nil
}

// BeginTurn schedules a turn start reminder for each of the team's tasks,
// ordering transport tasks last so they react to the other tasks' moves. C++:
// TaskManager::BeginTurn.
//
// The third pass is INERT until a transport task exists: nothing overrides Type(), so
// every task reads as TaskTypeGeneric and the first two passes take them all. The pass
// is kept because it is the original's scheduling and is what the unported TaskTransport
// (cargo truck material delivery) will need. TODO sec S.
func (m *TaskManager) BeginTurn(team int) {
	for _, t := range m.tasks {
		if t.Team() == team && t.Type() != TaskTypeTransport {
			t.setProcessingNeeded(true)
		}
	}

	for _, t := range m.tasks {
		if t.Team() == team && t.Type() != TaskTypeTransport {
			t.RemindTurnStart(false)
		}
	}

	for _, t := range m.tasks {
		if t.Team() == team && t.Type() == TaskTypeTransport {
			t.RemindTurnStart(false)
		}
	}
}

// EndTurn schedules an end of turn reminder for each of the team's tasks. C++:
// TaskManager::EndTurn.
func (m *TaskManager) EndTurn(team int) {
	for _, t := range m.tasks {
		if t.Team() == team {
			t.RemindTurnEnd(false)
		}
	}
}

// MarkTasksForProcessing flags the team's tasks as needing work this turn
// without scheduling reminders. C++: TaskManager::MarkTasksForProcessing.
func (m *TaskManager) MarkTasksForProcessing(team int) {
	for _, t := range m.tasks {
		if t.Team() == team {
			t.setProcessingNeeded(true)
		}
	}
}

// RemoveTask drops a task from the list (called by Task.RemoveSelf). C++:
// TaskManager::RemoveTask.
func (m *TaskManager) RemoveTask(task Task) {
	for i, t := range m.tasks {
		if t == task {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			return
		}
	}
}

// Clear tears down every task and empties the queues, e.g. between games. C++:
// TaskManager::Clear.
func (m *TaskManager) Clear() {
	for _, t := range m.tasks {
		t.RemoveSelf()
	}

	m.tasks = nil
	m.normalReminders = nil
	m.priorityReminders = nil
	m.reminderCounter = 0
}

// TaskCount reports the number of live tasks (test/diagnostic helper). C++:
// TaskManager::GetTaskList (count of).
func (m *TaskManager) TaskCount() int {
	return len(m.tasks)
}

// RemindersCount reports the number of queued reminders (test/diagnostic
// helper). C++: TaskManager::GetRemindersCount.
func (m *TaskManager) RemindersCount() int {
	return len(m.normalReminders) + len(m.priorityReminders)
}
