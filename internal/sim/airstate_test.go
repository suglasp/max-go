// airstate_test.go : tests airstate.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// addAirUnit deploys a player air unit (Bomber) at (gx, gy), flying or grounded.
func addAirUnit(g *Game, id, gx, gy int, hovering bool) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Bomber, 0, id, 0)
	u.Flags = game.FlagMobileAirUnit | game.FlagSelectable
	if hovering {
		u.Flags |= game.FlagHovering
	}
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribSpeed, 8)
	u.Values.Set(game.AttribHits, 20)
	u.InitState()
	g.Units.Deploy(u, gx, gy)
	return u
}

// addLandPad deploys a player landing pad at (gx, gy).
func addLandPad(g *Game, id, gx, gy int) *game.UnitInfo {
	p := game.NewUnitInfo(assets.LandPad, 0, id, 0)
	p.Flags = game.FlagStationary
	p.Hits = 1
	g.Units.Deploy(p, gx, gy)
	return p
}

// TestAirTakeOffAndLand: Land (on a pad) clears FlagHovering (grounded, can't move),
// Take Off sets it (flying, can move).
func TestAirTakeOffAndLand(t *testing.T) {
	g := newGame(t, 16, 16)
	addLandPad(g, 9, 5, 5)
	bomber := addAirUnit(g, 1, 5, 5, true) // flying, over the pad

	if !g.CanLand(bomber) || g.CanTakeOff(bomber) {
		t.Fatalf("flying over a pad: CanLand=%v CanTakeOff=%v, want true/false", g.CanLand(bomber), g.CanTakeOff(bomber))
	}
	if !g.LandAir(bomber) {
		t.Fatal("LandAir returned false over a pad")
	}
	if IsHovering(bomber) {
		t.Error("bomber still hovering after LandAir")
	}
	if g.CanLand(bomber) || !g.CanTakeOff(bomber) {
		t.Errorf("landed: CanLand=%v CanTakeOff=%v, want false/true", g.CanLand(bomber), g.CanTakeOff(bomber))
	}
	// Explicit Take Off (the popup action) lifts a landed plane back into the air.
	if !g.TakeOff(bomber) {
		t.Fatal("TakeOff returned false")
	}
	if !IsHovering(bomber) || !g.OrderMove(bomber, 2, 2) {
		t.Error("a bomber could not fly after taking off")
	}
}

// TestAirAutoTakeOffOnMove: ordering a LANDED air unit to move automatically takes off
// (it rises on its own) and then moves - the player need not take off manually first.
func TestAirAutoTakeOffOnMove(t *testing.T) {
	g := newGame(t, 16, 16)
	addLandPad(g, 9, 5, 5)
	bomber := addAirUnit(g, 1, 5, 5, true)
	if !g.LandAir(bomber) {
		t.Fatal("LandAir failed")
	}
	if IsHovering(bomber) {
		t.Fatal("bomber should be grounded after landing")
	}
	if !g.OrderMove(bomber, 9, 9) {
		t.Error("ordering a landed air unit to move should automatically take off and move")
	}
	if !IsHovering(bomber) {
		t.Error("a landed air unit ordered to move should automatically take off (become hovering)")
	}
}

// TestAirLandsOnlyOnPad checks that air units land only on friendly landing pads, not bare ground.
func TestAirLandsOnlyOnPad(t *testing.T) {
	g := newGame(t, 16, 16)
	bomber := addAirUnit(g, 1, 4, 4, true) // flying over bare ground

	if g.CanLand(bomber) {
		t.Error("the port should land an air unit only on a landing pad, not on bare ground")
	}
	addLandPad(g, 9, 4, 4)
	if !g.CanLand(bomber) {
		t.Error("an air unit cannot land on a friendly landing pad; it should")
	}
}

// TestLandedAirBlocksAndVulnerable: a flying air unit lets ground units pass under it
// and dodges ground guns; a grounded (not hovering) one blocks ground movers and is a
// ground target. (Flips Hovering directly to test the access/combat consequence.)
func TestLandedAirBlocksAndVulnerable(t *testing.T) {
	g := newGame(t, 16, 16)
	bomber := addAirUnit(g, 1, 5, 5, true) // flying
	tank := addTank(g, 2, 1, 4, 5)         // friendly ground unit

	gun := game.NewUnitInfo(assets.Tank, 1, 3, 0) // enemy ground gun (targets ground, not air)
	gun.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	g.Units.Deploy(gun, 7, 5)

	if !g.CanEnter(tank, 5, 5) {
		t.Error("a ground unit cannot pass under a flying air unit")
	}
	if g.CanTargetUnit(gun, bomber) {
		t.Error("a ground gun can target a FLYING air unit; it must not")
	}

	bomber.Flags &^= game.FlagHovering // grounded (the access/combat consequence)
	if g.CanEnter(tank, 5, 5) {
		t.Error("a ground unit can enter a LANDED air unit's cell; it must be blocked")
	}
	if !g.CanTargetUnit(gun, bomber) {
		t.Error("a ground gun cannot target a LANDED air unit; it should be vulnerable")
	}
}
