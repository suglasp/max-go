// buildexit_resume_test.go : tests a build ready builder never being auto driven by a stale queued route.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
)

// TestStartBuildClearsQueuedRoute verifies that starting construction drops any earlier zero-budget QueueMove route.
// A build-ready builder must leave through ExitBuildSite and clearBuildReady rather than resume an unrelated route.
func TestStartBuildClearsQueuedRoute(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	c := addConstructor(g, assets.Constrct, 0, 7, 8, 16, 30, 40)
	g.SetPendingDestination(c, 12, 8) // a route shelved before the build (0 move points)
	if !g.HasPendingRoute(c) {
		t.Fatal("setup: constructor should have a shelved route before building")
	}

	g.StartBuild(c, miningProduct(), 8, 8, 1)

	if g.HasPendingRoute(c) {
		t.Error("StartBuild did not drop the builder's stale queued route; it will auto resume and strand the parked builder")
	}
}

// TestResumePendingSkipsBuildReadyBuilder verifies that resumePending drops a build-ready builder's stale route.
// Build-ready units leave their sites only through ExitBuildSite.
func TestResumePendingSkipsBuildReadyBuilder(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	c := addConstructor(g, assets.Constrct, 0, 7, 8, 16, 30, 40)
	g.StartBuild(c, miningProduct(), 8, 8, 1)
	g.BeginTeamTurn(0) // one refilled turn finishes the one turn build; the builder parks build ready

	if !g.IsBuildReady(c) {
		t.Fatal("constructor is not build-ready after finishing the structure")
	}
	// Seed a stale route directly on the parked builder (models a loaded save / stray path).
	g.SetPendingDestination(c, 3, 8)
	wasX, wasY := c.GridX, c.GridY

	g.resumePending(0)

	if g.ActiveOrders() != 0 {
		t.Errorf("resumePending drove a build-ready builder (%d active order(s)); it must wait for an explicit ExitBuildSite", g.ActiveOrders())
	}
	if c.GridX != wasX || c.GridY != wasY {
		t.Errorf("build-ready builder moved from (%d,%d) to (%d,%d) on a stale route resume", wasX, wasY, c.GridX, c.GridY)
	}
	if !g.IsBuildReady(c) {
		t.Error("build-ready state lost after a stale route resume")
	}
	if g.HasPendingRoute(c) {
		t.Error("the stale queued route was not dropped when resumePending skipped the build-ready builder")
	}
}
