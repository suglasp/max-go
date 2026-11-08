// reveal_reaction_test.go : tests reveal triggered reaction fire (fog lifts on an in range enemy).
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

// moveUnit repositions u to (x, y) in the cell hash (a test stand in for a resolved move).
func moveUnit(g *Game, u *game.UnitInfo, x, y int) {
	g.Units.Cells.Remove(u)
	u.GridX, u.GridY = x, y
	g.Units.Cells.Add(u)
}

// TestRevealTriggersReaction: a stationary enemy already inside a turret's weapon
// range but in FOG draws a reaction shot the moment a friendly recon lifts the fog
// on it. A second reveal (fog flicker) does NOT re fire (the RevealReacted latch).
func TestRevealTriggersReaction(t *testing.T) {
	g := newGame(t, 30, 12)
	g.reactOnReveal = true // turns have begun (setup would not reveal fire)

	// A turret that can SHOOT the enemy (range 4) but cannot SEE it on its own (scan 1).
	turret := addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 1, reactBuilding)
	// A pure recon (no weapon) that starts far away, then drives up to reveal the enemy.
	recon := addScanner(g, 2, 0, 25, 5, 10)
	// Enemy tank at (8,5): distance 3 from the turret -> in range 4, outside scan 1.
	tank := addTankTarget(g, 3, 8, 5, 1)
	tank.DisabledReactionFire = true // it must not react to the recon, isolating the reveal shot

	// The enemy starts fogged: no reveal, no shot.
	g.RefreshVisibility()
	g.FireRevealReactions()
	if g.Vis.VisibleToTeam(tank, 0) {
		t.Fatal("setup: the enemy should start in fog (recon far away)")
	}
	if tank.Hits != 30 || turret.Shots != 1 {
		t.Fatalf("no shot should fire while the enemy is fogged (hits %d, shots %d)", tank.Hits, turret.Shots)
	}

	// The recon drives next to the enemy -> team 0 now sees it -> reveal edge -> shot.
	moveUnit(g, recon, 9, 5)
	g.RefreshVisibility()
	if !g.Vis.VisibleToTeam(tank, 0) {
		t.Fatal("the recon should reveal the enemy to team 0")
	}
	g.FireRevealReactions()
	if tank.Hits >= 30 {
		t.Error("the turret should have reaction-fired on the revealed in-range enemy")
	}
	if turret.Shots != 0 {
		t.Error("the turret should have spent its shot on the reveal reaction")
	}
	if got := len(g.DrainReactions()); got != 1 {
		t.Errorf("reveal reaction events = %d, want 1", got)
	}

	// Fog flicker: recon away then back. The latch must prevent a second shot even
	// though the turret has a shot again.
	turret.Shots = 1
	moveUnit(g, recon, 25, 5)
	g.RefreshVisibility()
	g.FireRevealReactions()
	moveUnit(g, recon, 9, 5)
	g.RefreshVisibility()
	g.FireRevealReactions()
	if turret.Shots != 1 {
		t.Error("a re-revealed enemy must NOT draw a second reveal reaction (RevealReacted latch)")
	}
}

// TestRevealReactionRespectsChecklist: the reveal trigger only QUEUES a candidate -
// the full reaction checklist still gates the shot, so a reactor with reaction fire
// disabled or no shots left stays silent on a reveal.
func TestRevealReactionRespectsChecklist(t *testing.T) {
	g := newGame(t, 30, 12)
	g.reactOnReveal = true
	addScanner(g, 2, 0, 9, 5, 10) // already adjacent: the enemy is revealed at once
	/* recon := ...; _ = recon: note: unused code in Go port, the name was never read */
	tank := addTankTarget(g, 3, 8, 5, 1)
	tank.DisabledReactionFire = true

	disabled := addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 1, reactBuilding)
	disabled.DisabledReactionFire = true // reaction fire off -> silent
	spent := addReactor(g, assets.GunTurrt, 4, 0, 6, 6, 4, 1, reactBuilding)
	spent.Shots = 0 // out of shots -> silent

	g.RefreshVisibility()
	g.FireRevealReactions()

	if tank.Hits != 30 {
		t.Error("a disabled / out-of-shots reactor must not reveal-fire")
	}
	if disabled.Shots != 1 {
		t.Error("a disabled reactor keeps its shot on a reveal")
	}
}

// TestEnableRevealReactionsArmsLoadedGame: a game resumed by load starts with
// reactOnReveal false (no BeginTeamTurn for the human), which would suppress the
// first reveal. EnableRevealReactions (called by the loader for an in game save)
// arms it so the next fog lift on an in range enemy fires.
func TestEnableRevealReactionsArmsLoadedGame(t *testing.T) {
	g := newGame(t, 30, 12)
	// Simulate a fresh load: the flag defaults off and BeginTeamTurn has not run.
	turret := addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 1, reactBuilding)
	recon := addScanner(g, 2, 0, 25, 5, 10)
	tank := addTankTarget(g, 3, 8, 5, 1)
	tank.DisabledReactionFire = true

	g.RefreshVisibility()
	g.EnableRevealReactions() // the loader arms reveal fire for an in game save

	// The recon drives up and lifts the fog: the turret must now reveal fire.
	moveUnit(g, recon, 9, 5)
	g.RefreshVisibility()
	g.FireRevealReactions()
	if tank.Hits >= 30 || turret.Shots != 0 {
		t.Error("after EnableRevealReactions, a loaded game must reveal-fire on the first fog-lift")
	}
}

// TestRevealReactionGatedToGameplay: with reactOnReveal false (setup / load), a
// revealed in range enemy is NOT reveal fired - an enemy visible from the start
// never draws a turn-0 shot.
func TestRevealReactionGatedToGameplay(t *testing.T) {
	g := newGame(t, 30, 12)
	g.reactOnReveal = false                                                    // still in setup
	turret := addReactor(g, assets.GunTurrt, 1, 0, 5, 5, 4, 10, reactBuilding) // can see AND shoot
	tank := addTankTarget(g, 3, 8, 5, 1)

	g.RefreshVisibility()
	g.FireRevealReactions()

	if tank.Hits != 30 || turret.Shots != 1 {
		t.Error("with reactOnReveal off (setup), no reveal reaction should fire")
	}
}
