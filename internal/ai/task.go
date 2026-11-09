// task.go : M.A.X. AI task interface and base implementation
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

// TaskType identifies a concrete task kind. The manager treats most types
// uniformly; only a few are special cased (transport tasks are scheduled last
// each turn, obtain units tasks are also tracked in a request list). C++:
// enums.hpp TaskType_* and task.hpp Task::GetType.
//
// The ferry task (taskTransport) returns TaskTypeTransport, so the manager runs it after
// the other tasks. TaskTypeObtainUnits stays unwired: it is the landing pad for the
// unported unit dispatcher (AI-IMPROVE item AI2).
type TaskType uint8

const (
	// TaskTypeGeneric is the default for the foundation; concrete C2/C3 tasks
	// return their own TaskType_* value from Type().
	TaskTypeGeneric TaskType = iota
	TaskTypeObtainUnits
	TaskTypeTransport
)

// Task is one unit of AI planning. It owns zero or more units and drives them
// toward an objective across turns through the lifecycle hooks below, which the
// TaskManager invokes via reminders. The interface is the faithful subset of
// task.hpp's virtual surface that the foundation needs; concrete tasks embed
// TaskBase for the default (do nothing) behaviour and override what they need.
//
// C++: class Task (task.hpp).
type Task interface {
	// ID is the unique task id (debug/logging + identity). C++: Task::GetId.
	ID() uint32
	// Team is the team this task plans for. C++: Task::GetTeam.
	Team() int
	// Type is the task kind. C++: Task::GetType (pure virtual).
	Type() TaskType
	// IsNeeded reports whether the task should keep running; an obsolete task
	// is removed. C++: Task::IsNeeded.
	IsNeeded() bool

	// Init sets up the task after it is appended to the manager (create child
	// tasks, schedule the first turn start reminder). C++: Task::Init.
	Init()
	// BeginTurn runs the task's start of turn planning. C++: Task::BeginTurn.
	BeginTurn()
	// EndTurn runs the task's end of turn evaluation. C++: Task::EndTurn.
	EndTurn()
	/* // note: ported but unused code in Go port - no task has children here, see TaskBase.parent
	ChildComplete(child Task)
	*/
	// RemoveSelf tears the task down and unlinks it from the manager. C++:
	// Task::RemoveSelf.
	RemoveSelf()

	// RemindTurnStart / RemindTurnEnd queue this task's start and end of turn
	// reminders (no ops if one is already pending). C++: Task::RemindTurnStart /
	// Task::RemindTurnEnd.
	RemindTurnStart(priority bool)
	RemindTurnEnd(priority bool)

	// scheduling state, used by the manager + reminders to avoid double queuing.
	isScheduledForTurnStart() bool
	setScheduledForTurnStart(bool)
	/* // note: ported but unused code in Go port
	isScheduledForTurnEnd() bool
	*/
	setScheduledForTurnEnd(bool)
	setProcessingNeeded(bool)

	// bind wires the task to its manager and its own interface value (the Go
	// stand in for C++'s `this`), so the embedded TaskBase can schedule
	// reminders that dispatch back to the concrete task.
	bind(self Task, mgr *TaskManager)
}

// taskIDCounter hands out monotonically increasing task ids. C++: Task::task_id
// (a process global counter).
var taskIDCounter uint32

// TaskBase is the embeddable default implementation of Task. A concrete task
// embeds it, sets its team/priority via newTaskBase, and overrides the
// lifecycle hooks it cares about. C++: the non pure Task methods (task.cpp).
type TaskBase struct {
	id                    uint32
	team                  int
	parent                Task
	basePriority          uint16
	needsProcessing       bool
	scheduledForTurnStart bool
	scheduledForTurnEnd   bool

	self Task         // the concrete task value (C++ `this`); set by bind.
	mgr  *TaskManager // the owning manager; set by bind.
}

// newTaskBase builds the embedded base for a concrete task. C++: Task::Task.
func newTaskBase(team int, parent Task, priority uint16) TaskBase {
	taskIDCounter++
	return TaskBase{
		id:              taskIDCounter,
		team:            team,
		parent:          parent,
		basePriority:    priority,
		needsProcessing: true,
	}
}

func (t *TaskBase) ID() uint32 {
	return t.id
}

func (t *TaskBase) Team() int {
	return t.team
}

func (t *TaskBase) Type() TaskType {
	return TaskTypeGeneric
}

// IsNeeded defers to the parent chain (a child is only needed while its parent
// is); a root task is always needed. C++: Task::IsNeeded.
//
// VESTIGIAL, kept on purpose: every task this port registers is a ROOT (all newTaskBase
// sites pass a nil parent), so the parent chain is always empty and this is constant
// true. The field and the walk stay because they mirror Task::Task's parent argument and
// are what a ported child task would hang from. Its sibling ChildComplete is commented
// out above.
func (t *TaskBase) IsNeeded() bool {
	if t.parent != nil {
		return t.parent.IsNeeded()
	}

	return true
}

// Init schedules the first turn start reminder (priority). C++: Task::Init.
func (t *TaskBase) Init() {
	t.RemindTurnStart(true)
}

// BeginTurn's default falls through to EndTurn, like the C++ base. C++:
// Task::BeginTurn.
func (t *TaskBase) BeginTurn() {
	t.self.EndTurn()
}

// EndTurn's default does nothing, and no task overrides it yet: the end of turn
// evaluation lives in the start of turn planning here (see AI.BeginTurn). C++:
// Task::EndTurn.
func (t *TaskBase) EndTurn() {

}

/* // note: ported but unused code in Go port - a child task would notify its parent through this
func (t *TaskBase) ChildComplete(_ Task) {

}
*/

func (t *TaskBase) setProcessingNeeded(v bool) {
	t.needsProcessing = v
}

// RemoveSelf unlinks the task from its parent and the manager. C++:
// Task::RemoveSelf.
func (t *TaskBase) RemoveSelf() {
	t.parent = nil

	if t.mgr != nil {
		t.mgr.RemoveTask(t.self)
	}
}

// RemindTurnStart queues a start of turn reminder for this task unless one is
// already pending. C++: Task::RemindTurnStart.
func (t *TaskBase) RemindTurnStart(priority bool) {
	if !t.scheduledForTurnStart && t.mgr != nil {
		t.mgr.AppendReminder(newRemindTurnStart(t.self), priority)
	}
}

// RemindTurnEnd queues an end of turn reminder for this task unless one is
// already pending. C++: Task::RemindTurnEnd.
func (t *TaskBase) RemindTurnEnd(priority bool) {
	if !t.scheduledForTurnEnd && t.mgr != nil {
		t.mgr.AppendReminder(newRemindTurnEnd(t.self), priority)
	}
}

func (t *TaskBase) isScheduledForTurnStart() bool {
	return t.scheduledForTurnStart
}

func (t *TaskBase) setScheduledForTurnStart(v bool) {
	t.scheduledForTurnStart = v
}

/* // note: ported but unused code in Go port
func (t *TaskBase) isScheduledForTurnEnd() bool {
	return t.scheduledForTurnEnd
}
*/

func (t *TaskBase) setScheduledForTurnEnd(v bool) {
	t.scheduledForTurnEnd = v
}

func (t *TaskBase) bind(self Task, mgr *TaskManager) {
	t.self = self
	t.mgr = mgr
}
