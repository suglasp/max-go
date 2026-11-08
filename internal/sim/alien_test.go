// alien_test.go : tests alien team slot iteration and IsAlien guards.
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

// TestAlienReactorNeverFires: an armed alien derelict that would otherwise fire in
// reaction (same stats as a normal enemy reactor) holds its fire - the neutral faction
// never initiates. A normal enemy reactor in the same setup DOES fire, isolating the
// alien guard.
func TestAlienReactorNeverFires(t *testing.T) {
	g := newGame(t, 20, 12)
	mobile := game.FlagMobileLandUnit | game.FlagSelectable

	alien := addReactor(g, assets.AlnTank, 1, game.AlienTeam, 5, 5, 4, 8, mobile)
	enemy := addReactor(g, assets.Tank, 2, 1, 5, 6, 4, 8, mobile)
	mover := addTankTarget(g, 3, 6, 5, 0) // player unit, in range of both reactors
	g.RefreshVisibility()

	g.checkReactions(mover)

	if alien.Shots != 1 {
		t.Errorf("the alien derelict fired in reaction (shots %d, want 1 unspent) - it must stay inert", alien.Shots)
	}
	if enemy.Shots != 0 {
		t.Error("the normal enemy reactor should have fired (control) - the test setup is otherwise valid")
	}
	if got := len(g.DrainReactions()); got != 1 {
		t.Errorf("reaction events = %d, want 1 (only the non-alien enemy)", got)
	}
}

// TestPlayerCanAttackAlien: a player reactor fires on an alien derelict that moves into
// range - proving the alien is hostile to (and a valid target for) every playable team,
// so it can be attacked and salvaged.
func TestPlayerCanAttackAlien(t *testing.T) {
	g := newGame(t, 20, 12)
	/* mobile := game.FlagMobileLandUnit | game.FlagSelectable: note: unused code in Go port, never read (was _ = mobile) */

	turret := addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 8, reactBuilding)
	alienMover := addTankTarget(g, 2, 7, 5, game.AlienTeam) // alien, 2 tiles east -> in range
	g.RefreshVisibility()

	g.checkReactions(alienMover)

	if alienMover.Hits >= 30 {
		t.Errorf("the player turret should have fired on the alien derelict (hits %d)", alienMover.Hits)
	}
	if turret.Shots != 0 {
		t.Error("the player turret should have spent its shot attacking the alien")
	}
}

// TestPlaceAlienBuildingHasNoComplex verifies that an alien building deploys without joining a complex network because the alien faction has no economy.
func TestPlaceAlienBuildingHasNoComplex(t *testing.T) {
	g := newGame(t, 20, 12)
	u := game.NewUnitInfo(assets.ShieldGn, game.AlienTeam, 1, 0)
	u.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, 50)
	u.Hits = 50

	c := g.PlaceBuilding(u, 8, 6)
	if c != nil {
		t.Error("an alien building must not join a complex network")
	}
	if g.ComplexOf(u) != nil {
		t.Error("ComplexOf an alien building must be nil")
	}
	if len(g.Units.Cells.At(8, 6)) == 0 {
		t.Error("the alien building should still be deployed on the map")
	}
}
