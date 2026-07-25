// refine_test.go : tests refinery credits and fuel consumption.
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

// TestRefineGold covers the Gold Refinery (COMMTWR) credit calculation: 5 credits/turn
// per powered refinery, capped by the raw gold reserve available at the start of the
// turn, and nothing when the reserve is empty, the refinery is unpowered or starved.
// RefineGold is credit only (it does NOT draw the reserve - BalanceTurn does that).
func TestRefineGold(t *testing.T) {
	newRefinery := func() (*Complex, *game.UnitInfo) {
		c := NewComplex(1)
		ref := building(assets.CommTwr, 0, 5, 5, game.OrderPowerOn, 0)
		c.Grow(ref)
		return c, ref
	}

	// Full credit: a powered refinery yields 5 from a reserve of 20; the reserve is
	// untouched here (BalanceTurn owns the draw).
	c, _ := newRefinery()
	c.Gold = 20
	if got := c.RefineGold(20); got != 5 {
		t.Errorf("RefineGold full = %d, want 5", got)
	}
	if c.Gold != 20 {
		t.Errorf("RefineGold must not draw the reserve, c.Gold = %d, want 20", c.Gold)
	}

	// Short reserve: credit only what was available.
	if got := c.RefineGold(3); got != 3 {
		t.Errorf("RefineGold short = %d, want 3", got)
	}

	// Empty reserve: no credit.
	if got := c.RefineGold(0); got != 0 {
		t.Errorf("RefineGold empty = %d, want 0", got)
	}

	// Powered off: no credit.
	c, ref := newRefinery()
	ref.Order = game.OrderPowerOff
	if got := c.RefineGold(20); got != 0 {
		t.Errorf("RefineGold unpowered = %d, want 0", got)
	}

	// Power starved: no credit.
	c, ref = newRefinery()
	ref.PowerStarved = true
	if got := c.RefineGold(20); got != 0 {
		t.Errorf("RefineGold starved = %d, want 0", got)
	}

	// Two powered refineries split the available reserve: 10 from 20, 7 from 7.
	c, _ = newRefinery()
	c.Grow(building(assets.CommTwr, 0, 6, 6, game.OrderPowerOn, 0))
	if got := c.RefineGold(20); got != 10 {
		t.Errorf("RefineGold two (ample) = %d, want 10", got)
	}
	if got := c.RefineGold(7); got != 7 {
		t.Errorf("RefineGold two (short) = %d, want 7", got)
	}
}
