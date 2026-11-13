// taskmanager.go : M.A.X. manages tasks for the AI
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

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

	// owner is the unit ledger: which task holds which unit (C++ UnitInfo::GetTask).
	// A task keeps a unit across turns for as long as it claims it again in each of
	// its passes (claimedIn holds the owner's pass number at the claim); the units it
	// did not claim again are let go at the end of its pass. Memory only, never saved.
	owner     map[*game.UnitInfo]Task
	claimedIn map[*game.UnitInfo]int
	passOf    map[Task]int // each task's pass number, raised at the start of its pass

	// readyForOrders tells whether a held unit may be taken by a more urgent task
	// (C++ Task_IsRecruitable's order check). Set from the live game by AI.BeginTurn;
	// nil (tests) counts every unit as ready.
	readyForOrders func(u *game.UnitInfo) bool

	// firstSeen is the turn the dispatcher first saw a unit: a unit seen first this
	// turn has had no task pass yet, so the production check does not count it idle.
	firstSeen map[*game.UnitInfo]int
	// handed holds the units the dispatcher gave to a request this turn, so a second
	// request's own find does not take the same unit again.
	handed map[*game.UnitInfo]bool
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

		// A turn start is one pass of its task: the units it does not claim again in
		// the pass are let go at its end.
		if start, ok := r.(*remindTurnStart); ok {
			m.beginPass(start.task)
			r.Execute()
			m.endPass(start.task)

			continue
		}

		r.Execute()
	}

	return true
}

// beginPass opens a new pass of task: the claims it makes from now on carry the new
// pass number.
func (m *TaskManager) beginPass(task Task) {
	if m.passOf == nil {
		m.passOf = make(map[Task]int)
	}
	m.passOf[task]++
}

// endPass lets go of the units task held from an earlier pass and did not claim
// again in this one: a repaired unit, a guard no longer wanted, a dead unit. C++:
// Task::RemoveUnit, done here for every unit the task stopped using.
func (m *TaskManager) endPass(task Task) {
	for u, owner := range m.owner {
		if owner == task && m.claimedIn[u] != m.passOf[task] {
			delete(m.owner, u)
			delete(m.claimedIn, u)
		}
	}
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
// The third pass runs the team's ferry task (taskTransport), so it sees every ride the
// other tasks asked for this turn.
func (m *TaskManager) BeginTurn(team int) {
	m.pruneUnneeded(team)
	/* // note: unused code in Go port - the ledger keeps units across turns now (step 6005): a task lets go at the end of its pass (endPass)
	m.releaseTeamUnits(team)
	*/

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

// mayClaimUnit reports whether task may take u: a free unit, one the task holds
// already, or one whose owner is gone; else only when the unit is ready for orders,
// and the owner is less urgent than task and lets the unit go. C++ ported : task.cpp
// - Task_IsRecruitable.
func (m *TaskManager) mayClaimUnit(task Task, u *game.UnitInfo) bool {
	owner := m.owner[u]

	// Nobody, the task itself, or a task that is gone: free to take.
	if owner == nil || owner == task || !owner.IsNeeded() {
		return true
	}

	// A unit at work (building, clearing rubble) is not taken from its task.
	if !m.isReadyForOrders(u) {
		return false
	}

	return owner.comparePriority(task.Priority()) > 0 && owner.isUnitTransferable(u)
}

// isReadyForOrders applies the readyForOrders check; with none set every unit is ready.
func (m *TaskManager) isReadyForOrders(u *game.UnitInfo) bool {
	if m.readyForOrders == nil {
		return true
	}

	return m.readyForOrders(u)
}

// unitReadyForOrders is the live readyForOrders check: alive, not building, not
// clearing rubble. This is the live equivalent of the reference's await, sentry, and move orders.
func unitReadyForOrders(g *sim.Game, u *game.UnitInfo) bool {
	return u.Hits > 0 && !u.Building && !g.IsClearingRubble(u)
}

// isFreeUnit reports whether no live task holds u.
func (m *TaskManager) isFreeUnit(u *game.UnitInfo) bool {
	owner := m.owner[u]

	return owner == nil || !owner.IsNeeded()
}

// dispatchUnits gives each free, ready unit of the team to the most urgent open unit
// request that wants its type (ties: the first registered), and notes the units seen
// for the first time this turn. A request's own build is left to that request. C++
// ported : task_manager.cpp - TaskManager::FindTaskForUnit (the unit request half; the
// fallback to the nearest usable task stays folded into the root tasks' own passes).
func (m *TaskManager) dispatchUnits(g *sim.Game, team int) {
	if m.firstSeen == nil {
		m.firstSeen = make(map[*game.UnitInfo]int)
	}
	m.handed = nil
	turn := g.State.TurnCounter

	onMap := make(map[*game.UnitInfo]bool)
	for _, u := range g.Units.Units() {
		if u.Team != team {
			continue
		}
		onMap[u] = true

		// A unit seen for the first time this turn.
		if _, seen := m.firstSeen[u]; !seen {
			m.firstSeen[u] = turn
		}

		// Only a free unit that can take orders is handed out.
		if !m.isFreeUnit(u) || !unitReadyForOrders(g, u) || m.isRequestBuild(u) {
			continue
		}

		if request := m.mostUrgentRequest(team, u); request != nil {
			if m.handed == nil {
				m.handed = make(map[*game.UnitInfo]bool)
			}
			m.handed[u] = true
			request.takeUnit(u)
		}
	}

	// Forget the team's units that left the map.
	for u := range m.firstSeen {
		if u.Team == team && !onMap[u] {
			delete(m.firstSeen, u)
		}
	}
}

// mostUrgentRequest returns the team's open unit request with the lowest priority
// value that wants u, or nil.
func (m *TaskManager) mostUrgentRequest(team int, u *game.UnitInfo) *taskObtainUnits {
	var best *taskObtainUnits
	for _, task := range m.tasks {
		request, ok := task.(*taskObtainUnits)
		if !ok || request.Team() != team || !request.IsNeeded() || !request.wantsUnit(u) {
			continue
		}

		// Strictly more urgent wins, so a tie keeps the first registered request.
		if best == nil || request.Priority() < best.Priority() {
			best = request
		}
	}

	return best
}

// isRequestBuild reports whether u is a unit an open request had built for itself.
func (m *TaskManager) isRequestBuild(u *game.UnitInfo) bool {
	for _, task := range m.tasks {
		if request, ok := task.(*taskObtainUnits); ok && request.ordered[u.Type] == u {
			return true
		}
	}

	return false
}

// isUnitNeeded reports whether the team should build another unitType: not while a
// unit of the type stands idle, that is free of every task and not new this turn
// (a new unit has had no task pass yet). C++ ported : task_manager.cpp -
// TaskManager::IsUnitNeeded (the "a unit of the type has no task" rule).
func (m *TaskManager) isUnitNeeded(g *sim.Game, team int, unitType assets.UnitType) bool {
	turn := g.State.TurnCounter
	for _, u := range g.Units.Units() {
		if u.Team != team || u.Type != unitType || u.Hits <= 0 {
			continue
		}

		// A unit never seen by the dispatcher, or seen first this turn, is not idle yet.
		seenTurn, seen := m.firstSeen[u]
		if !seen || seenTurn >= turn {
			continue
		}

		// A free unit of the type: no need for another.
		if m.isFreeUnit(u) {
			dbg.Logf(dbg.AI, "team %d does not build a %s: %d stands idle", team, unitType.Tag(), u.ID)

			return false
		}
	}

	return true
}

// claimUnit gives u to task when mayClaimUnit allows it and reports whether it did.
func (m *TaskManager) claimUnit(task Task, u *game.UnitInfo) bool {
	if !m.mayClaimUnit(task, u) {
		return false
	}

	if m.owner == nil {
		m.owner = make(map[*game.UnitInfo]Task)
		m.claimedIn = make(map[*game.UnitInfo]int)
	}
	m.owner[u] = task
	m.claimedIn[u] = m.passOf[task]

	return true
}

/* // note: unused code in Go port - the ledger keeps units across turns now (step 6005): a task lets go at the end of its pass (endPass)
// releaseTeamUnits empties the ledger entries of the team's tasks, so they claim
// again in run order this turn.
func (m *TaskManager) releaseTeamUnits(team int) {
	for u, owner := range m.owner {
		if owner.Team() == team {
			delete(m.owner, u)
		}
	}
}
*/

// pruneUnneeded removes the team's tasks that are no longer needed: a removed
// parent takes its children with it, since a child's IsNeeded follows its
// parent. The list is collected first because RemoveSelf edits m.tasks.
// C++ ported : task.cpp - the children's own IsNeeded checks, done here in one pass.
func (m *TaskManager) pruneUnneeded(team int) {
	var unneeded []Task
	for _, t := range m.tasks {
		if t.Team() == team && !t.IsNeeded() {
			unneeded = append(unneeded, t)
		}
	}

	for _, t := range unneeded {
		dbg.Logf(dbg.AI, "team %d: task %d is no longer needed, removed", team, t.ID())
		t.RemoveSelf()
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
	// A removed task holds no units.
	for u, owner := range m.owner {
		if owner == task {
			delete(m.owner, u)
			delete(m.claimedIn, u)
		}
	}
	delete(m.passOf, task)

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
	m.owner = nil
	m.claimedIn = nil
	m.passOf = nil
	m.firstSeen = nil
	m.handed = nil
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
