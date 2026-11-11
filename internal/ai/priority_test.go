// priority_test.go : testing unit for priority.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import "testing"

// TestComparePriorityAcrossAndInsideTiers checks the coarse sign outside the tolerance
// window and the fine difference inside it.
func TestComparePriorityAcrossAndInsideTiers(t *testing.T) {
	m := NewTaskManager()
	task := &probeTask{TaskBase: newTaskBase(0, nil, taskPriorityExplore)}
	m.AppendTask(task)

	cases := []struct {
		name  string
		other uint16
		want  int
	}{
		{"a far more urgent tier wins", taskPriorityEmergency, 1},
		{"a far less urgent tier loses", taskPriorityAttackDefault, -1},
		{"the same priority ties", taskPriorityExplore, 0},
		{"inside the window, slightly more urgent", taskPriorityExplore - 0x10, 0x10},
		{"inside the window, slightly less urgent", taskPriorityExplore + 0x10, -0x10},
		{"the window edge still compares finely", taskPriorityExplore + taskPriorityTolerance, -taskPriorityTolerance},
		{"one past the window edge is coarse", taskPriorityExplore + taskPriorityTolerance + 1, -1},
	}
	for _, c := range cases {
		if got := task.comparePriority(c.other); got != c.want {
			t.Errorf("%s: comparePriority(%#x) = %d, want %d", c.name, c.other, got, c.want)
		}
	}

	// The emergency tier sits at zero: the window below it must not wrap around.
	urgent := &probeTask{TaskBase: newTaskBase(0, nil, taskPriorityEmergency)}
	m.AppendTask(urgent)
	if got := urgent.comparePriority(taskPriorityAutoSurvey); got != -1 {
		t.Errorf("emergency vs survey = %d, want -1 (the emergency task is more urgent)", got)
	}
}

// TestRootTasksUseTheReferenceTiers checks each root task's tier and the urgency order the
// unit ledger relies on.
func TestRootTasksUseTheReferenceTiers(t *testing.T) {
	ctx := &aiContext{}
	manager := newTaskManageBuildings(1, StrategyCombinedArms, 0, false, ctx)
	cases := []struct {
		name string
		task Task
		want uint16
	}{
		{"retreat", newTaskRetreat(1, ctx), taskPriorityEmergency},
		{"bulldozer", newTaskBulldozer(1, ctx), taskPriorityRemoveRubble},
		{"escort", newTaskEscort(1, ctx), taskPriorityEscort},
		{"defense reserve", newTaskDefenseReserve(1, ctx), taskPriorityDefenseReserve},
		{"mines", newTaskMines(1, ctx), taskPriorityFrontier},
		{"explore", newTaskExplore(1, ctx), taskPriorityExplore},
		{"manage buildings", manager, taskPriorityManageBuildings},
		{"connection assistant", newTaskConnectionAssistant(manager), taskPriorityManageBuildings},
		{"produce units", newTaskProduceUnits(1, StrategyCombinedArms, ctx), taskPriorityManageBuildings},
		{"attack", newTaskAttackEnemies(1, ctx), taskPriorityAttackDefault},
		{"commando", newTaskCommando(1, ctx), taskPriorityAttackDefault},
		{"turrets", newTaskFireTurrets(1, ctx), taskPriorityAttackDefault},
		{"ferry", newTaskTransport(1, ctx), taskPriorityAttackDefault},
		{"survey", newTaskSurvey(1, ctx), taskPriorityAutoSurvey},
	}
	for _, c := range cases {
		if got := c.task.Priority(); got != c.want {
			t.Errorf("%s: Priority() = %#x, want %#x", c.name, got, c.want)
		}
	}

	// The claim order the ledger relies on: retreat > escort > reserve > attack.
	if !(taskPriorityEmergency < taskPriorityEscort && taskPriorityEscort < taskPriorityDefenseReserve && taskPriorityDefenseReserve < taskPriorityAttackDefault) {
		t.Error("the tiers no longer rank retreat, escort, reserve, attack from most to least urgent")
	}
}
