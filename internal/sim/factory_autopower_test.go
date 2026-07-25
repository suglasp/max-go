// factory_autopower_test.go : tests a factory powering on automatically when queued / off when its queue empties.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// TestFactoryAutoPowerOnQueuedAndOffWhenEmpty is the powered production polish the
// by design: queuing a unit automatically powers the factory ON (with a "started"
// message) and an emptied queue automatically powers it OFF (with a "powered down"
// message). The messages ride the
// ProdStarted / ProdStopped events the gamescreen already drains.
func TestFactoryAutoPowerOnQueuedAndOffWhenEmpty(t *testing.T) {
	g := newGame(t, 16, 16)
	f := stationaryBuilding(assets.LightPlt, 0, 4, 2)
	g.PlaceBuilding(f, 4, 2)
	f.Order = game.OrderPowerOff // the factory starts OFF

	// Queuing a unit automatically powers it ON and emits exactly one ProdStarted.
	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false)
	if !economy.IsActive(f) {
		t.Error("queuing a unit should auto-power-on the factory")
	}
	if evs := g.DrainProductionEvents(); len(evs) != 1 || evs[0].Kind != ProdStarted {
		t.Fatalf("queuing should emit one ProdStarted, got %+v", evs)
	}

	// Emptying the queue automatically powers it OFF and emits exactly one ProdStopped.
	g.popFactoryQueue(f) // the single queued unit finished -> queue empties
	if economy.IsActive(f) {
		t.Error("a factory whose queue ran out should auto-power-down")
	}
	if len(f.BuildQueue) != 0 {
		t.Errorf("queue should be empty, got %v", f.BuildQueue)
	}
	if evs := g.DrainProductionEvents(); len(evs) != 1 || evs[0].Kind != ProdStopped {
		t.Fatalf("emptying the queue should emit one ProdStopped, got %+v", evs)
	}
}
