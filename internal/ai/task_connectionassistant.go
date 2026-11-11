// task_connectionassistant.go : M.A.X. AI building connections rigging.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

// taskConnectionAssistant is the thin child task of the building manager that, once per turn, asks the
// building manager to repair a split base by laying connectors. It holds the same
// taskManageBuildings instance the team's economy runs on, so it shares its
// pending map and build helpers - the connector site it seeds is driven and raised
// by that manager's own BeginTurn. Registered before the manager so it seeds the
// site first. C++ ported : taskconnectionassistant.cpp - TaskConnectionAssistant
// (BeginTurn just calls manager->ReconnectBuildings()).
type taskConnectionAssistant struct {
	TaskBase
	manager *taskManageBuildings
}

// newTaskConnectionAssistant builds the assistant as a child of the team's
// building manager: it takes the manager's priority and is removed with it.
// C++: TaskConnectionAssistant::TaskConnectionAssistant.
func newTaskConnectionAssistant(manager *taskManageBuildings) *taskConnectionAssistant {
	return &taskConnectionAssistant{
		TaskBase: newTaskBase(manager.team, manager, manager.basePriority),
		manager:  manager,
	}
}

// BeginTurn delegates to the building manager's reconnection pass. C++:
// TaskConnectionAssistant::BeginTurn -> manager->ReconnectBuildings().
func (t *taskConnectionAssistant) BeginTurn() {
	t.manager.reconnectBuildings()
}
