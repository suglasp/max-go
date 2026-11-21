// commando_select_test.go : tests the selection upkeep after a capture or disable lands on a selected unit.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// movableRaidTank is a raidable tank with movement left, so Auto Select may pick it.
var movableRaidTank = tankStats{angle: 2, turns: 8, hits: 30, speed: 4, flags: game.FlagElectronicUnit}

// raidSetup deploys an enemy commando at (1, 1), the player's tank 7 at (2, 1) (the raid victim) and the player's tank 8 far away.
func raidSetup(t *testing.T, autoSelect bool) (s *Screen, g *sim.Game, raider, victim, other *game.UnitInfo) {
	t.Helper()

	s, g, _ = newRaidScreen(t)
	s.r = drawmap.New(s.wmap) // centring on the next unit needs a map view
	s.autoSelect = autoSelect
	raider = raidCommando(g, 1, 1, 1)
	victim = mkTank(g.Units, 0, 7, 2, 1, movableRaidTank)
	other = mkTank(g.Units, 0, 8, 9, 9, movableRaidTank)

	return s, g, raider, victim, other
}

// runRaid resolves a forced successful steal or disable of victim and drains the events into the screen.
func runRaid(t *testing.T, s *Screen, g *sim.Game, raider, victim *game.UnitInfo, steal bool) {
	t.Helper()

	forceRoll(t, g.StealthChance(raider, victim, steal), true)

	var ok bool
	if steal {
		ok, _ = g.Steal(raider, victim.GridX, victim.GridY)
	} else {
		ok, _ = g.Disable(raider, victim.GridX, victim.GridY)
	}

	if !ok {
		t.Fatal("the raid failed despite a forced roll")
	}

	s.tickCommandoEvents()
}

func TestStolenSelectedUnitIsDeselected(t *testing.T) {
	s, g, raider, victim, _ := raidSetup(t, false)
	s.setSelected(victim)
	s.orderInput.popupOpen = true

	runRaid(t, s, g, raider, victim, true)

	if s.selected != nil {
		t.Errorf("selected = %v, want nil after the unit was stolen", s.selected)
	}

	if s.orderInput.popupOpen {
		t.Error("the popup stayed open")
	}
}

func TestStolenGroupMemberLeavesSelection(t *testing.T) {
	s, g, raider, victim, other := raidSetup(t, false)
	third := mkTank(g.Units, 0, 9, 8, 8, movableRaidTank)
	s.setSelectionSet([]*game.UnitInfo{other, victim, third}, other)

	runRaid(t, s, g, raider, victim, true)

	for _, u := range s.selection {
		if u == victim {
			t.Fatal("the stolen tank is still in the group")
		}
	}

	if len(s.selection) != 2 || s.selected != other {
		t.Errorf("group = %d units, leader %v, want 2 units led by the other tank", len(s.selection), s.selected)
	}
}

func TestStolenSelectedUnitAutoSelectsNext(t *testing.T) {
	for _, auto := range []bool{true, false} {
		s, g, raider, victim, other := raidSetup(t, auto)
		s.setSelected(victim)

		runRaid(t, s, g, raider, victim, true)

		want := (*game.UnitInfo)(nil)
		if auto {
			want = other
		}

		if s.selected != want {
			t.Errorf("autoSelect=%v: selected = %v, want %v", auto, s.selected, want)
		}
	}
}

func TestStolenSelectedUnitNoAutoSelectOnComputerTurn(t *testing.T) {
	s, g, raider, victim, _ := raidSetup(t, true)
	s.setSelected(victim)
	g.State.ActiveTeam = 1

	runRaid(t, s, g, raider, victim, true)

	if s.selected != nil {
		t.Errorf("selected = %v, want nil: no auto select during a computer turn", s.selected)
	}
}

func TestDisabledSelectedUnitAutoSelectMovesOn(t *testing.T) {
	s, g, raider, victim, other := raidSetup(t, true)
	s.setSelected(victim)
	s.orderInput.popupOpen = true

	runRaid(t, s, g, raider, victim, false)

	if s.selected != other {
		t.Errorf("selected = %v, want the other tank after the disable", s.selected)
	}

	if s.orderInput.popupOpen {
		t.Error("the popup stayed open")
	}

	// Without Auto Select the disabled unit stays selected.
	s, g, raider, victim, _ = raidSetup(t, false)
	s.setSelected(victim)
	runRaid(t, s, g, raider, victim, false)

	if s.selected != victim {
		t.Errorf("autoSelect off: selected = %v, want the disabled unit kept", s.selected)
	}
}

func TestRaidOnUnselectedUnitKeepsSelection(t *testing.T) {
	s, g, raider, victim, other := raidSetup(t, true)
	s.setSelected(other)
	s.orderInput.popupOpen = true

	runRaid(t, s, g, raider, victim, true)

	if s.selected != other || !s.orderInput.popupOpen {
		t.Errorf("selected = %v popupOpen = %v, want the other tank and an open popup untouched", s.selected, s.orderInput.popupOpen)
	}
}
