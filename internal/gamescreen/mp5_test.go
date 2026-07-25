// mp5_test.go : tests MP5 multiplayer scenario startup and map loading on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

func playedHas(played []string, tag string) bool {
	for _, t := range played {
		if t == tag {
			return true
		}
	}
	return false
}

// TestProducingFactorySaysBuilding verifies that selecting a factory that is
// actively producing speaks "Building!" (V_F284); an idle factory does not (a building
// is otherwise silent on the voice channel).
func TestProducingFactorySaysBuilding(t *testing.T) {
	s, g := newClickScreen(t)
	var played []string
	s.announcer.voiceFn = func(tag string) { played = append(played, tag) }
	s.sound.sfxFn = func(string, string, float64) {}
	s.announcer.clock = func() uint64 { return 0 }

	f := mkUnit(g, assets.LandPlt, 1, 4, 4, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	f.Order = game.OrderPowerOn
	f.State = game.OrderStateExecutingOrder
	f.BuildQueue = []assets.UnitType{assets.Tank}
	f.BuildRate = 1

	s.unitStatusVoice(f)
	if !playedHas(played, "V_F284") {
		t.Errorf("selecting a producing factory should say Building! (V_F284), played=%v", played)
	}

	// Idle (empty queue): no "Building!".
	played = nil
	f.BuildQueue = nil
	s.unitStatusVoice(f)
	if playedHas(played, "V_F284") {
		t.Errorf("an idle factory must not say Building!, played=%v", played)
	}
}

// TestFactoryExitAutoSelectsDroppedUnit verifies that clicking a build-ready
// factory's BLDMRK marker drops the produced unit onto the marker and selects it
// (the factory was selected; the output becomes the new selection).
func TestFactoryExitAutoSelectsDroppedUnit(t *testing.T) {
	s, g := newClickScreen(t)
	if g.Resources == nil {
		g.Resources = economy.NewResourceMap(16, 16)
	}
	g.MakeUnit = func(team int, ut assets.UnitType, flags game.Flags) *game.UnitInfo {
		u := game.NewUnitInfo(ut, team, 0, 0)
		u.Flags = flags
		u.Values = &game.UnitValues{}
		u.Values.Set(game.AttribTurns, 1)
		u.Values.Set(game.AttribHits, 10)
		u.Values.Set(game.AttribSpeed, 8) // budget to roll out of the plant
		return u
	}
	s.sound.sfxFn = func(string, string, float64) {}
	s.announcer.voiceFn = func(string) {}
	s.announcer.clock = func() uint64 { return 0 }

	f := game.NewUnitInfo(assets.LandPlt, 0, 1, 0)
	f.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	f.Values = &game.UnitValues{}
	g.PlaceBuilding(f, 4, 2)
	f.Storage = 1000 // own cargo funds every build turn
	f.Order = game.OrderPowerOn
	f.State = game.OrderStateExecutingOrder

	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false)
	for turn := 0; turn < 12 && g.BuildReadyInfo(f) == nil; turn++ {
		g.AdvanceBuilds(0)
	}
	if g.BuildReadyInfo(f) == nil {
		t.Fatal("factory never produced a build-ready unit")
	}

	markers := g.ExitMarkers(f)
	if len(markers) == 0 {
		t.Fatal("no exit markers for the parked factory output")
	}
	mx, my := markers[0][0], markers[0][1]

	s.selected = f // the factory is selected; clicking its marker drops the output
	if !s.tryBuildExit(mx, my) {
		t.Fatal("tryBuildExit refused a valid marker")
	}
	if s.selected == nil || s.selected == f {
		t.Fatalf("after the exit the dropped unit should be selected, got %v", s.selected)
	}
	if s.selected.Type != assets.Tank {
		t.Errorf("auto-selected unit = %s, want Tank (the factory output)", s.selected.Type.Tag())
	}
	if s.selected.GridX != mx || s.selected.GridY != my {
		t.Errorf("selected unit at (%d,%d), want the marker (%d,%d)", s.selected.GridX, s.selected.GridY, mx, my)
	}
}
