// alien_test.go : tests the fixed neutral alien team.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import "testing"

// TestAlienTeamIsFixedAndUnclaimable checks that the alien slot always uses
// TeamAlien and never enters turn rotation or becomes active.
func TestAlienTeamIsFixedAndUnclaimable(t *testing.T) {
	if AlienTeam != TeamCount || TeamSlots != TeamCount+1 {
		t.Fatalf("AlienTeam/TeamSlots = %d/%d, want %d/%d", AlienTeam, TeamSlots, TeamCount, TeamCount+1)
	}

	gs := NewGameState(twoPlayerParams())
	if gs.Teams[AlienTeam].Type != TeamAlien {
		t.Errorf("alien slot type = %v, want TeamAlien", gs.Teams[AlienTeam].Type)
	}
	if gs.IsActive(AlienTeam) {
		t.Error("the alien slot must never be active")
	}

	// The turn rotation walks the active teams; the alien slot must never appear.
	for team := gs.FirstActiveTeam(); team != -1; team = gs.NextActiveTeam(team) {
		if team == AlienTeam {
			t.Fatalf("the turn rotation returned the alien slot %d - it must be unclaimable", team)
		}
	}

	// Finishing every playable turn must not hand the active slot to the alien.
	for i := 0; i < 2*TeamSlots; i++ {
		gs.FinishTurn(gs.ActiveTeam)
		if gs.ActiveTeam == AlienTeam {
			t.Fatal("FinishTurn handed the active slot to the alien team")
		}
	}
}

// TestIsAlienAndString covers the small helpers the guards at the call site rely on.
func TestIsAlienAndString(t *testing.T) {
	if !IsAlien(AlienTeam) {
		t.Error("IsAlien(AlienTeam) = false")
	}
	for team := 0; team < TeamCount; team++ {
		if IsAlien(team) {
			t.Errorf("IsAlien(%d) = true, want false", team)
		}
	}
	if got := TeamAlien.String(); got != "Alien" {
		t.Errorf("TeamAlien.String() = %q, want \"Alien\"", got)
	}
}
