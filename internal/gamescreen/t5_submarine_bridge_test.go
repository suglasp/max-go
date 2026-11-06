// t5_submarine_bridge_test.go : testing unit for submarine.go updateSubmarines and bridge.go updateBridges
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
)

// subFixture is a player screen with a computer rival on team 1 and a fixed announcer clock.
type subFixture struct {
	s     *Screen
	g     *sim.Game
	clock uint64
}

// newSubFixture builds the fixture; the clock field drives the screen time source.
func newSubFixture(t *testing.T) *subFixture {
	t.Helper()

	s, g := newClickScreen(t)
	g.State.Teams[1].Type = game.TeamComputer

	f := &subFixture{s: s, g: g, clock: 10000}
	s.announcer.clock = func() uint64 { return f.clock }

	return f
}

// deploySub puts a submarine of the given team at a cell with a live (non idle) order.
func (f *subFixture) deploySub(team, id, cellX, cellY int) *game.UnitInfo {
	sub := game.NewUnitInfo(assets.Submarne, team, id, 0)
	sub.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	sub.Values = &game.UnitValues{}
	sub.Order = game.OrderAwait
	f.g.Units.Deploy(sub, cellX, cellY)

	return sub
}

// deployScanner puts a scanning unit of the given type and team at a cell.
func (f *subFixture) deployScanner(ut assets.UnitType, team, id, cellX, cellY int) *game.UnitInfo {
	u := game.NewUnitInfo(ut, team, id, 0)
	u.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribScan, 4)
	u.Order = game.OrderAwait
	f.g.Units.Deploy(u, cellX, cellY)

	return u
}

// TestUpdateSubmarinesSonarOnly pins that only an enemy corvette sonar surfaces a submerged submarine.
func TestUpdateSubmarinesSonarOnly(t *testing.T) {
	f := newSubFixture(t)
	sub := f.deploySub(0, 1, 6, 5)

	t.Run("no rival leaves it submerged", func(t *testing.T) {
		sub.Surfaced = true // stale state must be cleared
		f.g.Vis.Rebuild(f.g.Units.Units())
		f.s.updateSubmarines()

		if sub.Surfaced {
			t.Error("a submarine nobody detects should resubmerge")
		}
	})

	t.Run("enemy plain scan does not spot it", func(t *testing.T) {
		tank := f.deployScanner(assets.Tank, 1, 2, 5, 5)
		f.g.Vis.Rebuild(f.g.Units.Units())
		f.s.updateSubmarines()

		if sub.Surfaced {
			t.Error("an enemy tank scan must not surface a submerged submarine")
		}

		f.g.Units.Remove(tank)
	})

	t.Run("enemy AWAC does not spot it", func(t *testing.T) {
		awac := f.deployScanner(assets.Awac, 1, 3, 5, 5)
		f.g.Vis.Rebuild(f.g.Units.Units())
		f.s.updateSubmarines()

		if sub.Surfaced {
			t.Error("an enemy AWAC must not surface a submerged submarine")
		}

		f.g.Units.Remove(awac)
	})

	t.Run("enemy corvette surfaces it and leaving resubmerges it", func(t *testing.T) {
		corvette := f.deployScanner(assets.Corvette, 1, 4, 5, 5)
		f.g.Vis.Rebuild(f.g.Units.Units())
		f.s.updateSubmarines()

		if !sub.Surfaced {
			t.Fatal("an enemy corvette sonar should surface the submarine")
		}

		f.g.Units.Remove(corvette)
		f.g.Vis.Rebuild(f.g.Units.Units())
		f.s.updateSubmarines()

		if sub.Surfaced {
			t.Error("the submarine should resubmerge once the sonar is gone")
		}
	})

	t.Run("own corvette does not surface it", func(t *testing.T) {
		f.deployScanner(assets.Corvette, 0, 5, 5, 5)
		f.g.Vis.Rebuild(f.g.Units.Units())
		f.s.updateSubmarines()

		if sub.Surfaced {
			t.Error("a corvette of the submarine own team is not a detector")
		}
	})
}

// TestSubSpottedByEnemyTeamRules pins that only a configured rival team counts as a spotter.
func TestSubSpottedByEnemyTeamRules(t *testing.T) {
	f := newSubFixture(t)
	sub := f.deploySub(0, 1, 6, 5)

	// Team 2 has no team type, so its corvette must be ignored.
	f.deployScanner(assets.Corvette, 2, 2, 5, 5)
	f.g.Vis.Rebuild(f.g.Units.Units())

	if f.s.subSpottedByEnemy(sub) {
		t.Error("a corvette of a team that is not in the game must not spot the submarine")
	}

	f.g.State.Teams[2].Type = game.TeamComputer

	if !f.s.subSpottedByEnemy(sub) {
		t.Error("once team 2 is a computer team its corvette should spot the submarine")
	}
}

// TestUpdateSubmarinesCombatWindow pins that a combat surfacing holds for subCombatSurfaceMS and then expires.
func TestUpdateSubmarinesCombatWindow(t *testing.T) {
	f := newSubFixture(t)
	sub := f.deploySub(0, 1, 6, 5)
	f.g.Vis.Rebuild(f.g.Units.Units())

	f.s.surfaceSubmarine(sub)

	if !sub.Surfaced {
		t.Fatal("surfaceSubmarine should surface the submarine at once")
	}

	f.clock += subCombatSurfaceMS - 1
	f.s.updateSubmarines()

	if !sub.Surfaced {
		t.Error("the submarine should still be surfaced just inside the combat window")
	}

	f.clock++
	f.s.updateSubmarines()

	if sub.Surfaced {
		t.Error("the submarine should resubmerge when the combat window ends")
	}
}

// TestUpdateSubmarinesIgnoresOtherUnits pins that updateSubmarines never touches a non submarine.
func TestUpdateSubmarinesIgnoresOtherUnits(t *testing.T) {
	f := newSubFixture(t)
	boat := f.deployScanner(assets.FastBoat, 0, 1, 6, 5)
	boat.Surfaced = true
	f.g.Vis.Rebuild(f.g.Units.Units())

	f.s.updateSubmarines()

	if !boat.Surfaced {
		t.Error("updateSubmarines must leave a non submarine alone")
	}
}

// bridgeFixture deploys a flat bridge at (3,3) on a player screen.
func bridgeFixture(t *testing.T) (*Screen, *game.UnitInfo) {
	t.Helper()

	s, g := newClickScreen(t)
	bridge := mkUnit(g, assets.Bridge, 1, 3, 3, game.FlagStationary|game.FlagSelectable)

	return s, bridge
}

// dueBridgeStep makes the throttle stamp old enough that the next updateBridges call steps.
func dueBridgeStep(s *Screen) {
	s.bridgeStamp = platform.Ticks() - bridgeStepMS
}

// TestUpdateBridgesThrottle pins that updateBridges holds inside bridgeStepMS and steps one frame once it is due.
func TestUpdateBridgesThrottle(t *testing.T) {
	s, bridge := bridgeFixture(t)
	mkUnit(s.game, assets.FastBoat, 2, 3, 3, game.FlagMobileSeaUnit|game.FlagSelectable)

	// A step is due: exactly one frame moves and the stamp advances.
	dueBridgeStep(s)
	s.updateBridges()

	if bridge.BridgeRaise != 1 {
		t.Fatalf("raise after the first due step = %d, want 1", bridge.BridgeRaise)
	}

	// The next call comes right after the stamp: the frame holds.
	s.updateBridges()

	if bridge.BridgeRaise != 1 {
		t.Errorf("raise inside the throttle window = %d, want it held at 1", bridge.BridgeRaise)
	}
}

// TestUpdateBridgesRaisesToMaxAndLowersBack pins that due steps move a bridge toward the raised or flat target one frame at a time.
func TestUpdateBridgesRaisesToMaxAndLowersBack(t *testing.T) {
	s, bridge := bridgeFixture(t)
	boat := mkUnit(s.game, assets.FastBoat, 2, 3, 3, game.FlagMobileSeaUnit|game.FlagSelectable)

	for i := 0; i < game.BridgeRaiseMax+3; i++ {
		dueBridgeStep(s)
		s.updateBridges()
	}

	if bridge.BridgeRaise != game.BridgeRaiseMax {
		t.Fatalf("raise under a sea unit = %d, want the cap %d", bridge.BridgeRaise, game.BridgeRaiseMax)
	}

	s.game.Units.Remove(boat)

	dueBridgeStep(s)
	s.updateBridges()

	if bridge.BridgeRaise != game.BridgeRaiseMax-1 {
		t.Errorf("raise one step after the boat left = %d, want %d", bridge.BridgeRaise, game.BridgeRaiseMax-1)
	}

	for i := 0; i < game.BridgeRaiseMax+3; i++ {
		dueBridgeStep(s)
		s.updateBridges()
	}

	if bridge.BridgeRaise != 0 {
		t.Errorf("raise with no sea unit = %d, want flat", bridge.BridgeRaise)
	}
}

// TestUpdateBridgesLandUnitDoesNotRaise pins that a land unit crossing a bridge leaves it flat.
func TestUpdateBridgesLandUnitDoesNotRaise(t *testing.T) {
	s, bridge := bridgeFixture(t)
	mkUnit(s.game, assets.Tank, 2, 3, 3, game.FlagMobileLandUnit|game.FlagSelectable)

	for i := 0; i < game.BridgeRaiseMax+2; i++ {
		dueBridgeStep(s)
		s.updateBridges()
	}

	if bridge.BridgeRaise != 0 {
		t.Errorf("raise under a land unit = %d, want it flat", bridge.BridgeRaise)
	}
}
