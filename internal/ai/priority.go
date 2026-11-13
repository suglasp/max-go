// priority.go : the AI task urgency tiers and how two task priorities compare.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : task.hpp - the task priority tiers; task.cpp - Task::ComparePriority
package ai

// A task priority is one 16 bit value: the LOWER it is, the MORE urgent the task. The
// high byte is the tier, the low byte a fine bias inside the tier. Priority decides who
// keeps a unit when two tasks want it (the unit ledger in taskmanager.go); it does not
// decide the run order, which stays the registration order.
const (
	// taskPriorityTolerance is the window inside which two priorities compare finely;
	// outside it only the side matters.
	taskPriorityTolerance = 0xFF

	taskPriorityEmergency       uint16 = 0x0000 // retreat from danger: the most urgent claim
	taskPriorityRemoveRubble    uint16 = 0x0200 // clear rubble off build sites
	taskPriorityEscort          uint16 = 0x0F00 // guard a fragile mover
	taskPriorityDefenseReserve  uint16 = 0x1900 // the home defence reserve
	taskPriorityFrontier        uint16 = 0x1A00 // frontier work and mine laying
	taskPriorityExplore         uint16 = 0x1B00 // map exploration
	taskPriorityManageBuildings uint16 = 0x1D00 // the economy: buildings and unit production
	taskPriorityAttackDefault   uint16 = 0x1F00 // the default attack tier
	taskPriorityAutoSurvey      uint16 = 0x2200 // resource surveying: the weakest claim tier
)

// comparePriority compares the task against another priority p. It returns a positive
// value when the task is LESS urgent than p (so p may take its units), a negative one
// when it is more urgent, and inside the tolerance window the fine difference of the
// task's own Priority() and p. C++: Task::ComparePriority.
func (t *TaskBase) comparePriority(p uint16) int {
	base := int(t.basePriority)
	other := int(p)

	// Far more urgent than p (a much lower value): a coarse "p does not win".
	if base+taskPriorityTolerance < other {
		return -1
	}

	// Far less urgent than p (a much higher value): a coarse "p wins".
	if base-taskPriorityTolerance > other {
		return 1
	}

	// Inside the window the task's current priority decides.
	return int(t.self.Priority()) - other
}
