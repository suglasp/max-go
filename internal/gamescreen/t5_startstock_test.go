// t5_startstock_test.go : testing unit for customstart.go landStartingStock, customLandFlags and seedCustomWorld
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

// stockMine builds a mining station with a 25 unit raw store and an empty cargo bar.
func stockMine(id int) *game.UnitInfo {
	mine := game.NewUnitInfo(assets.MiningSt, 0, id, 0)
	mine.Values = &game.UnitValues{}
	mine.Values.Set(game.AttribStorage, 25)

	return mine
}

// TestLandStartingStock pins the opening pool per raw richness and that it is added on top of what the complex holds.
func TestLandStartingStock(t *testing.T) {
	cases := []struct {
		name    string
		levels  [3]int
		set     bool
		wantRaw int
	}{
		{"levels never set lands the average buffer", [3]int{}, false, startSupplyRawAverage},
		{"poor raw lands 10", [3]int{0, 1, 1}, true, 10},
		{"average raw lands 13", [3]int{1, 1, 1}, true, 13},
		{"rich raw lands nothing", [3]int{2, 1, 1}, true, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, g := newTestScreen(t, 16)
			s.resourceLevels = tc.levels
			s.resourceLevelsSet = tc.set

			mine := stockMine(1)
			c := economy.NewComplex(1)
			c.Members = []*game.UnitInfo{mine}
			c.Material = 5 // already pooled: the stock is ADDED
			c.Fuel = 2
			second := economy.NewComplex(2)
			g.Complexes = []*economy.Complex{c, second}

			s.landStartingStock()

			if c.Material != 5+tc.wantRaw {
				t.Errorf("complex raw = %d, want %d", c.Material, 5+tc.wantRaw)
			}

			if c.Fuel != 2+startSupplyFuel {
				t.Errorf("complex fuel = %d, want %d", c.Fuel, 2+startSupplyFuel)
			}

			if second.Material != tc.wantRaw || second.Fuel != startSupplyFuel {
				t.Errorf("second complex = %d raw / %d fuel, want %d / %d", second.Material, second.Fuel, tc.wantRaw, startSupplyFuel)
			}

			// The mine bar mirrors the pool so turn 0 reads a partial stock, never above capacity.
			if want := min(c.Material, 25); mine.Storage != want {
				t.Errorf("mine cargo bar = %d, want %d", mine.Storage, want)
			}
		})
	}
}

// TestLandStartingStockNoComplexes pins that landing with no complex is a quiet no op.
func TestLandStartingStockNoComplexes(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.Complexes = nil

	s.landStartingStock()

	if len(g.Complexes) != 0 {
		t.Errorf("complexes = %d, want none", len(g.Complexes))
	}
}

// TestFounderCargoDefaults pins the founder builder loads: first constructor 40 of 60, first engineer 20 of 40.
func TestFounderCargoDefaults(t *testing.T) {
	if startConstructorRaw != 40 {
		t.Errorf("first constructor raw = %d, want 40", startConstructorRaw)
	}

	if startEngineerRaw != 20 {
		t.Errorf("first engineer raw = %d, want 20", startEngineerRaw)
	}

	constructors, engineers := 0, 0
	loads := []int{}

	for _, ut := range []assets.UnitType{assets.Constrct, assets.Constrct, assets.Engineer, assets.Engineer, assets.Engineer} {
		u := &game.UnitInfo{Type: ut, Storage: 999}
		setStarterBuilderCargo(u, &constructors, &engineers)
		loads = append(loads, u.Storage)
	}

	want := []int{40, 0, 20, 0, 0}

	for i := range want {
		if loads[i] != want[i] {
			t.Errorf("builder %d cargo = %d, want %d (all loads %v)", i, loads[i], want[i], loads)
		}
	}
}

// TestCustomLandFlags pins the deploy flags of a buyable unit: builders carry the builder flag, only the tank rotates a head.
func TestCustomLandFlags(t *testing.T) {
	cases := []struct {
		name         string
		unit         assets.UnitType
		wantBuilder  bool
		wantHeadTurn bool
	}{
		{"constructor", assets.Constrct, true, false},
		{"engineer", assets.Engineer, true, false},
		{"tank", assets.Tank, false, true},
		{"scout", assets.Scout, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := customLandFlags(tc.unit)

			if !flags.Has(game.FlagMobileLandUnit) || !flags.Has(game.FlagSelectable) {
				t.Errorf("flags %v miss the land or selectable base", flags)
			}

			if got := flags.Has(game.FlagConstructorUnit); got != tc.wantBuilder {
				t.Errorf("builder flag = %v, want %v", got, tc.wantBuilder)
			}

			headTurn := flags.Has(game.FlagTurretSprite) && flags.Has(game.FlagCanHeadRotate)

			if headTurn != tc.wantHeadTurn {
				t.Errorf("head rotate = %v, want %v", headTurn, tc.wantHeadTurn)
			}
		})
	}
}

// TestLoadoutRosterKeepsOrderAndFlags pins that a purchased list turns into one spec per unit in the same order.
func TestLoadoutRosterKeepsOrderAndFlags(t *testing.T) {
	bought := []assets.UnitType{assets.Tank, assets.Constrct, assets.Scout}
	specs := loadoutRoster(bought)

	if len(specs) != len(bought) {
		t.Fatalf("specs = %d, want %d", len(specs), len(bought))
	}

	for i, ut := range bought {
		if specs[i].t != ut || specs[i].flags != customLandFlags(ut) {
			t.Errorf("spec %d = %v / %v, want %v / %v", i, specs[i].t, specs[i].flags, ut, customLandFlags(ut))
		}
	}

	if got := loadoutRoster(nil); len(got) != 0 {
		t.Errorf("empty loadout gave %d specs", len(got))
	}
}

// TestSeedCustomWorld pins that world seeding builds the resource map at the map size and resolves a random clan but keeps a chosen one.
func TestSeedCustomWorld(t *testing.T) {
	for _, levelsSet := range []bool{false, true} {
		name := "default richness"
		if levelsSet {
			name = "chosen richness"
		}

		t.Run(name, func(t *testing.T) {
			s, g := newTestScreen(t, 16)
			s.resourceLevelsSet = levelsSet
			s.resourceLevels = [3]int{2, 0, 1}
			g.State.Teams[0].Clan = game.ClanRandom
			g.State.Teams[1].Type = game.TeamComputer
			g.State.Teams[1].Clan = 3

			s.seedCustomWorld([]game.Point{{X: 3, Y: 3}, {X: 12, Y: 12}}, 7)

			if g.Resources == nil || g.Resources.W != s.wmap.W || g.Resources.H != s.wmap.H {
				t.Fatalf("resource map = %+v, want one sized %dx%d", g.Resources, s.wmap.W, s.wmap.H)
			}

			if clan := g.State.Teams[0].Clan; clan < 1 || clan > 8 {
				t.Errorf("random clan resolved to %d, want 1..8", clan)
			}

			if clan := g.State.Teams[1].Clan; clan != 3 {
				t.Errorf("chosen clan = %d, want it kept at 3", clan)
			}
		})
	}
}
