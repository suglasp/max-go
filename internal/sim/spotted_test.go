// spotted_test.go : tests spotted.go.
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

// addScanner deploys a unit that contributes scan coverage for its team (the eyes
// that populate the spotted memory).
func addScanner(g *Game, id, team, gx, gy, scan int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Scout, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribScan, scan)
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestSpottedMemoryFreezesOnFog: an enemy seen, then moved out of scan, stays spotted
// with its last seen position FROZEN at the cell where it was last visible - the AI
// remembers a fled enemy rather than forgetting it or reading its true position.
func TestSpottedMemoryFreezesOnFog(t *testing.T) {
	g := newGame(t, 24, 12)
	addScanner(g, 1, 0, 5, 5, 6) // team-0 eyes at (5,5), scan 6
	enemy := addTankTarget(g, 2, 8, 5, 1)
	g.RefreshVisibility()

	if !g.SpottedByTeam(enemy, 0) {
		t.Fatal("the enemy in scan should be spotted")
	}
	if x, y, ok := g.LastSeen(0, enemy); !ok || x != 8 || y != 5 {
		t.Fatalf("last seen = (%d,%d) ok=%v, want (8,5) true", x, y, ok)
	}

	g.Units.Move(enemy, 18, 5) // 13 cells from the eyes -> out of scan 6
	g.RefreshVisibility()

	if g.Vis.VisibleToTeam(enemy, 0) {
		t.Fatal("test setup: the enemy should now be in fog (out of scan)")
	}
	if !g.SpottedByTeam(enemy, 0) {
		t.Error("a once-seen enemy must stay spotted after it slips into fog")
	}
	if x, y, ok := g.LastSeen(0, enemy); !ok || x != 8 || y != 5 {
		t.Errorf("last seen = (%d,%d) ok=%v, want it FROZEN at (8,5)", x, y, ok)
	}
}

// TestSpottedMemoryRefreshesWhileVisible: while the enemy stays in scan, its last seen
// position tracks its real movement.
func TestSpottedMemoryRefreshesWhileVisible(t *testing.T) {
	g := newGame(t, 24, 12)
	addScanner(g, 1, 0, 5, 5, 6)
	enemy := addTankTarget(g, 2, 8, 5, 1)
	g.RefreshVisibility()

	g.Units.Move(enemy, 7, 5) // still inside scan 6
	g.RefreshVisibility()

	if x, y, ok := g.LastSeen(0, enemy); !ok || x != 7 || y != 5 {
		t.Errorf("last seen = (%d,%d) ok=%v, want it refreshed to (7,5)", x, y, ok)
	}
}

// TestNeverSeenEnemyNotSpotted: an enemy that has never entered scan is unknown.
func TestNeverSeenEnemyNotSpotted(t *testing.T) {
	g := newGame(t, 24, 12)
	addScanner(g, 1, 0, 5, 5, 3) // short sight
	enemy := addTankTarget(g, 2, 20, 5, 1)
	g.RefreshVisibility()

	if g.SpottedByTeam(enemy, 0) {
		t.Error("an enemy never in scan must not be spotted")
	}
	if _, _, ok := g.LastSeen(0, enemy); ok {
		t.Error("an unseen enemy has no last-seen position")
	}
}

// TestForgetSpottedOnDeath: a destroyed enemy is dropped from every team's memory, so
// no unit chases a ghost cell.
func TestForgetSpottedOnDeath(t *testing.T) {
	g := newGame(t, 24, 12)
	addScanner(g, 1, 0, 5, 5, 6)
	enemy := addTankTarget(g, 2, 8, 5, 1)
	g.RefreshVisibility()
	if !g.SpottedByTeam(enemy, 0) {
		t.Fatal("setup: the enemy should be spotted before it dies")
	}

	g.DestroyUnit(enemy)
	g.RefreshVisibility()

	if g.SpottedByTeam(enemy, 0) {
		t.Error("a destroyed enemy must be forgotten")
	}
	if _, _, ok := g.LastSeen(0, enemy); ok {
		t.Error("a destroyed enemy has no last-seen position")
	}
}

// TestSpottedTrackableGate: ground cover (roads/slabs) is terrain, not a contact, and
// is never spotted - but a laid mine IS a tracked contact.
func TestSpottedTrackableGate(t *testing.T) {
	tank := game.NewUnitInfo(assets.Tank, 1, 1, 0)
	tank.Flags = game.FlagMobileLandUnit
	if !spottedTrackable(tank) {
		t.Error("a mobile unit is trackable")
	}

	road := game.NewUnitInfo(assets.Tank, 1, 2, 0) // type irrelevant; the flag drives the gate
	road.Flags = game.FlagGroundCover
	if spottedTrackable(road) {
		t.Error("plain ground cover (road/slab/rubble) must not be tracked")
	}

	mine := game.NewUnitInfo(assets.LandMine, 1, 3, 0)
	mine.Flags = game.FlagGroundCover
	if !spottedTrackable(mine) {
		t.Error("a laid land mine is a tracked contact despite the ground-cover flag")
	}
	seaMine := game.NewUnitInfo(assets.SeaMine, 1, 4, 0)
	seaMine.Flags = game.FlagGroundCover
	if !spottedTrackable(seaMine) {
		t.Error("a laid sea mine is a tracked contact")
	}
}

// TestSpottedByTeamNoFogModel: with no visibility model wired (a minimal harness),
// the gate treats every unit as spotted - the AI's prior fallback.
func TestSpottedByTeamNoFogModel(t *testing.T) {
	g := newGame(t, 24, 12)
	g.Vis = nil
	enemy := addTankTarget(g, 1, 8, 5, 1)
	g.RefreshVisibility() // updateSpotted is a no op without Vis

	if !g.SpottedByTeam(enemy, 0) {
		t.Error("with no fog model, every unit is treated as spotted")
	}
}

// sightingsOf counts the drained sightings of enemy for team.
func sightingsOf(list []Sighting, team int, enemy *game.UnitInfo) int {
	n := 0
	for _, s := range list {
		if s.Team == team && s.Enemy == enemy {
			n++
		}
	}
	return n
}

// TestSightingsOnEveryRevealEdge: once turns run, a new contact and a remembered one coming
// back out of the fog each queue a sighting for the halt check; an enemy that stays in
// sight does not, and the drain empties the list.
func TestSightingsOnEveryRevealEdge(t *testing.T) {
	g := newGame(t, 24, 12)
	addScanner(g, 1, 0, 5, 5, 6)
	enemy := addTankTarget(g, 2, 18, 5, 1) // out of scan
	g.BeginTeamTurn(0)
	g.DrainSightings()

	g.Units.Move(enemy, 8, 5) // into scan: a new contact
	g.RefreshVisibility()
	if n := sightingsOf(g.DrainSightings(), 0, enemy); n != 1 {
		t.Fatalf("new contact: %d sightings, want 1", n)
	}

	g.RefreshVisibility() // still in sight: no new edge
	if n := len(g.DrainSightings()); n != 0 {
		t.Fatalf("an enemy staying in sight queued %d sightings, want 0", n)
	}

	g.Units.Move(enemy, 18, 5) // back into the fog
	g.RefreshVisibility()
	g.Units.Move(enemy, 9, 5) // and out of it again
	g.RefreshVisibility()
	if n := sightingsOf(g.DrainSightings(), 0, enemy); n != 1 {
		t.Errorf("remembered contact back in sight: %d sightings, want 1 (not latched like the reveal fire)", n)
	}
}

// TestNoSightingsBeforeTurnsBegin: setup and landing reveal nothing for the halt check, and
// a turn start drops any sighting left over from the previous turn.
func TestNoSightingsBeforeTurnsBegin(t *testing.T) {
	g := newGame(t, 24, 12)
	addScanner(g, 1, 0, 5, 5, 6)
	enemy := addTankTarget(g, 2, 8, 5, 1)

	g.RefreshVisibility() // setup: turns have not begun
	if n := len(g.DrainSightings()); n != 0 {
		t.Fatalf("setup queued %d sightings, want 0", n)
	}

	g.BeginTeamTurn(0)
	g.Units.Move(enemy, 18, 5)
	g.RefreshVisibility()
	g.Units.Move(enemy, 8, 5)
	g.RefreshVisibility() // a sighting is now queued and never drained
	g.BeginTeamTurn(0)
	if n := sightingsOf(g.DrainSightings(), 0, enemy); n != 0 {
		t.Errorf("a turn start kept %d stale sightings, want 0", n)
	}
}
