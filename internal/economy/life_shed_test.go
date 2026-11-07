// life_shed_test.go : tests worker shortage shedding during turn reconciliation.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package economy

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// research returns a powered research centre (a worker consumer, shed rank 0).
func research(order game.Order) *game.UnitInfo {
	return &game.UnitInfo{Type: assets.Research, Order: order, State: game.OrderStateExecutingOrder}
}

// powerStn returns a powered power station (6 power, enough to staff the test's
// consumers so the POWER shed never fires and only the LIFE shed is exercised).
func powerStn(order game.Order) *game.UnitInfo {
	return &game.UnitInfo{Type: assets.PowerStn, Order: order, State: game.OrderStateExecutingOrder}
}

// TestLifeShedPowersOffUnstaffedConsumers checks the worker shed: a complex
// whose habitats cannot staff all its personnel buildings powers off the lowest
// priority one (research before the Eco Sphere) until the worker balance is not
// negative, leaving the rest running. Power is plentiful so only the LIFE pass acts.
func TestLifeShedPowersOffUnstaffedConsumers(t *testing.T) {
	// One habitat houses 3 workers; research + three Eco Spheres draw 4. Surplus -1.
	res := research(game.OrderPowerOn)
	gh1 := greenhouse(game.OrderPowerOn)
	gh2 := greenhouse(game.OrderPowerOn)
	gh3 := greenhouse(game.OrderPowerOn)
	c := &Complex{Members: []*game.UnitInfo{
		habitat(game.OrderPowerOn), powerStn(game.OrderPowerOn), res, gh1, gh2, gh3,
	}, Fuel: 50}

	c.BalanceTurn(4)

	// Research is the lowest shed rank, so it is the one powered off; one shed
	// restores the balance (3 housed vs 3 remaining draws), so the Eco Spheres stay.
	if res.Order != game.OrderPowerOff {
		t.Errorf("research order = %v, want OrderPowerOff (shed first for lack of workers)", res.Order)
	}
	for i, gh := range []*game.UnitInfo{gh1, gh2, gh3} {
		if gh.Order != game.OrderPowerOn {
			t.Errorf("eco-sphere %d order = %v, want OrderPowerOn (kept; balance restored)", i, gh.Order)
		}
	}
	if w := c.workerSurplus(); w < 0 {
		t.Errorf("complex worker surplus = %d, want >= 0 after the life shed", w)
	}
	if shed := c.LastLifeShed(); len(shed) != 1 || shed[0] != res {
		t.Errorf("LastLifeShed = %v, want exactly the research centre", shed)
	}
}

// TestLifeShedNoneWhenStaffed checks a balanced complex sheds nobody: enough housing
// for its personnel buildings leaves every one running and LastLifeShed empty.
func TestLifeShedNoneWhenStaffed(t *testing.T) {
	gh := greenhouse(game.OrderPowerOn)
	c := &Complex{Members: []*game.UnitInfo{
		habitat(game.OrderPowerOn), powerStn(game.OrderPowerOn), gh,
	}, Fuel: 50}

	c.BalanceTurn(4)

	if gh.Order != game.OrderPowerOn {
		t.Errorf("eco-sphere order = %v, want OrderPowerOn (staffed, not shed)", gh.Order)
	}
	if shed := c.LastLifeShed(); len(shed) != 0 {
		t.Errorf("LastLifeShed = %v, want empty (nobody short of workers)", shed)
	}
}

// TestLifeShedAfterHabitatDestroyed checks the "blow up the habitat" case:
// a running Eco Sphere whose only habitat is removed mid game is powered off
// automatically at the next reconcile, instead of carrying a negative worker
// balance forever.
func TestLifeShedAfterHabitatDestroyed(t *testing.T) {
	hab := habitat(game.OrderPowerOn)
	gh := greenhouse(game.OrderPowerOn)
	c := &Complex{Members: []*game.UnitInfo{hab, powerStn(game.OrderPowerOn), gh}, Fuel: 50}

	// Balanced first: the Eco Sphere stays on.
	c.BalanceTurn(4)
	if gh.Order != game.OrderPowerOn {
		t.Fatalf("pre-condition: eco-sphere should be running, got %v", gh.Order)
	}

	// The habitat is destroyed (removed from the complex); next reconcile sheds the
	// now unstaffed Eco Sphere.
	c.Shrink(hab)
	c.BalanceTurn(4)
	if gh.Order != game.OrderPowerOff {
		t.Errorf("eco-sphere order after losing the habitat = %v, want OrderPowerOff", gh.Order)
	}
	if shed := c.LastLifeShed(); len(shed) != 1 || shed[0] != gh {
		t.Errorf("LastLifeShed = %v, want exactly the eco-sphere", shed)
	}
}
