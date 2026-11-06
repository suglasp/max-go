// t4_transfer_test.go : testing unit for transfer.go X-Fer rules
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"errors"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// t4NoArt is a sprite reader that holds no art, so icons come out blank.
type t4NoArt struct{}

// Read always reports a missing resource.
func (t4NoArt) Read(tag string) ([]byte, error) {
	return nil, errors.New("no art in tests: " + tag)
}

// t4ArtlessScreen gives the screen a sprite cache without art so unit icons can be built headless.
func t4ArtlessScreen(s *Screen) {
	s.sprites = units.NewSpriteCache(t4NoArt{}, &gfx.Palette{})
}

// t4Truck deploys a mobile cargo unit with a storage capacity and a current load.
func t4Truck(g *sim.Game, typ assets.UnitType, id, gridX, gridY, capacity, stored int) *game.UnitInfo {
	u := mkUnit(g, typ, id, gridX, gridY, game.FlagMobileLandUnit|game.FlagSelectable)
	u.Values.Set(game.AttribStorage, capacity)
	u.Storage = stored

	return u
}

// t4MiningStation places a mining station that forms a raw complex holding material.
func t4MiningStation(t *testing.T, g *sim.Game, gridX, gridY, material int) (*game.UnitInfo, *economy.Complex) {
	t.Helper()

	mine := &game.UnitInfo{
		Type:   assets.MiningSt,
		Team:   0,
		ID:     50,
		Flags:  game.FlagBuilding | game.FlagStationary | game.FlagSelectable,
		Values: &game.UnitValues{},
	}
	mine.Values.Set(game.AttribStorage, 50)
	mine.Storage = material

	c := g.PlaceBuilding(mine, gridX, gridY)

	if c == nil {
		t.Fatal("the mining station did not form a complex")
	}

	c.Material = material

	return mine, c
}

// TestT4ArmTransfer pins that only units that carry cargo can arm an X-Fer pick.
func TestT4ArmTransfer(t *testing.T) {
	cases := []struct {
		name  string
		typ   assets.UnitType
		armed bool
	}{
		{"supply truck arms", assets.SplyTrck, true},
		{"fuel truck arms", assets.FuelTrck, true},
		{"gold truck arms", assets.GoldTrck, true},
		{"tank has no cargo", assets.Tank, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, g := newClickScreen(t)
			u := t4Truck(g, c.typ, 1, 5, 5, 40, 10)
			s.armTransfer(u)

			if s.orderInput.xferArmed != c.armed {
				t.Fatalf("xferArmed = %v, want %v", s.orderInput.xferArmed, c.armed)
			}

			if c.armed && s.orderInput.xferSource != u {
				t.Error("the pick must remember its source")
			}

			if !c.armed && (s.orderInput.xferSource != nil || s.banner.msg == "") {
				t.Error("a refusal leaves no source and says why")
			}
		})
	}
}

// TestT4TryTransferTargetUnarmed pins that an unarmed screen does not consume clicks.
func TestT4TryTransferTargetUnarmed(t *testing.T) {
	s, _ := newClickScreen(t)

	if s.tryTransferTarget(5, 5) {
		t.Error("nothing armed: the click must pass through")
	}
}

// TestT4TryTransferTargetRefusals pins the clicks that cancel the pick instead of opening the screen.
func TestT4TryTransferTargetRefusals(t *testing.T) {
	cases := []struct {
		name  string
		build func(g *sim.Game) (src *game.UnitInfo, clickX, clickY int)
		want  string
	}{
		{"empty cell", func(g *sim.Game) (*game.UnitInfo, int, int) {
			return t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 10), 9, 9
		}, "cancelled"},
		{"source itself", func(g *sim.Game) (*game.UnitInfo, int, int) {
			return t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 10), 5, 5
		}, "cancelled"},
		{"not adjacent", func(g *sim.Game) (*game.UnitInfo, int, int) {
			src := t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 10)
			t4Truck(g, assets.SplyTrck, 2, 9, 9, 40, 10)

			return src, 9, 9
		}, "Move next to"},
		{"incompatible cargo", func(g *sim.Game) (*game.UnitInfo, int, int) {
			src := t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 10)
			t4Truck(g, assets.FuelTrck, 2, 6, 5, 40, 10)

			return src, 6, 5
		}, "no matching cargo"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, g := newClickScreen(t)
			called := false
			s.SetTransferFunc(func(screendata.TransferInfo) (int, error) {
				called = true

				return 0, nil
			})

			src, clickX, clickY := c.build(g)
			s.armTransfer(src)

			if !s.tryTransferTarget(clickX, clickY) {
				t.Fatal("the click must be consumed")
			}

			if s.orderInput.xferArmed || s.orderInput.xferSource != nil {
				t.Error("the pick must disarm")
			}

			if called {
				t.Error("a refused pick must not open the transfer screen")
			}

			if !strings.Contains(s.banner.msg, c.want) {
				t.Errorf("message = %q, want it to contain %q", s.banner.msg, c.want)
			}
		})
	}
}

// TestT4CanExchangeCargo pins which source and target pairs may trade cargo.
func TestT4CanExchangeCargo(t *testing.T) {
	t.Run("same cargo trucks trade", func(t *testing.T) {
		s, g := newClickScreen(t)
		a := t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 10)
		b := t4Truck(g, assets.Constrct, 2, 6, 5, 60, 0)

		if !s.canExchangeCargo(a, b) || !s.canExchangeCargo(b, a) {
			t.Error("two raw carriers should trade in both directions")
		}
	})

	t.Run("different cargo kinds refuse", func(t *testing.T) {
		s, g := newClickScreen(t)
		a := t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 10)
		b := t4Truck(g, assets.GoldTrck, 2, 6, 5, 40, 10)

		if s.canExchangeCargo(a, b) {
			t.Error("raw to gold must be refused")
		}
	})

	t.Run("target mid build refuses", func(t *testing.T) {
		s, g := newClickScreen(t)
		a := t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 10)
		b := t4Truck(g, assets.Constrct, 2, 6, 5, 60, 0)
		b.Order = game.OrderBuild

		if s.canExchangeCargo(a, b) {
			t.Error("a builder on a build order is not a transfer target")
		}

		b.Order = game.OrderClear

		if s.canExchangeCargo(a, b) {
			t.Error("a bulldozer clearing is not a transfer target")
		}
	})

	t.Run("no cargo source refuses", func(t *testing.T) {
		s, g := newClickScreen(t)
		tank := t4Truck(g, assets.Tank, 1, 5, 5, 0, 0)
		b := t4Truck(g, assets.SplyTrck, 2, 6, 5, 40, 10)

		if s.canExchangeCargo(tank, b) {
			t.Error("a unit without cargo cannot start a transfer")
		}
	})

	t.Run("mobile source refills from a stationary complex", func(t *testing.T) {
		s, g := newClickScreen(t)
		mine, _ := t4MiningStation(t, g, 5, 5, 30)
		truck := t4Truck(g, assets.Constrct, 2, 7, 5, 60, 0)

		if !s.canExchangeCargo(truck, mine) {
			t.Error("a mobile raw source should draw from a complex that stores raw")
		}
	})

	t.Run("mobile source cannot use a complex without that cargo", func(t *testing.T) {
		s, g := newClickScreen(t)
		mine, _ := t4MiningStation(t, g, 5, 5, 30)
		fuel := t4Truck(g, assets.FuelTrck, 2, 7, 5, 40, 0)

		if s.canExchangeCargo(fuel, mine) {
			t.Error("a fuel truck cannot draw fuel from a base that stores none")
		}
	})

	t.Run("stationary source cannot push into another building", func(t *testing.T) {
		s, g := newClickScreen(t)
		mine, _ := t4MiningStation(t, g, 5, 5, 30)
		plant := &game.UnitInfo{
			Type: assets.PowerStn, Team: 0, ID: 51,
			Flags:  game.FlagBuilding | game.FlagStationary | game.FlagSelectable,
			Values: &game.UnitValues{},
		}
		g.PlaceBuilding(plant, 7, 5)

		if s.canExchangeCargo(mine, plant) {
			t.Error("a mining station must not push raw into a neighbouring power plant")
		}
	})
}

// TestT4TransferTargetValidNeedsSource pins that the cursor test is false with no armed source.
func TestT4TransferTargetValidNeedsSource(t *testing.T) {
	s, g := newClickScreen(t)
	t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 10)
	t4Truck(g, assets.SplyTrck, 2, 6, 5, 40, 10)

	if s.transferTargetValid(6, 5) {
		t.Error("with no armed source no cell is a valid target")
	}
}

// TestT4OpenTransferApplies pins how the chosen amount moves cargo between two units.
func TestT4OpenTransferApplies(t *testing.T) {
	cases := []struct {
		name             string
		amount           int
		fnErr            error
		wantSrc, wantDst int
	}{
		{"source gives", 15, nil, 15, 25},
		{"source takes", -5, nil, 35, 5},
		{"cancelled amount", 0, nil, 30, 10},
		{"hook error", 15, errors.New("boom"), 30, 10},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, g := newClickScreen(t)
			t4ArtlessScreen(s)
			src := t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 30)
			dst := t4Truck(g, assets.Constrct, 2, 6, 5, 60, 10)

			var got screendata.TransferInfo

			s.SetTransferFunc(func(info screendata.TransferInfo) (int, error) {
				got = info

				return c.amount, c.fnErr
			})
			s.openTransfer(src, dst)

			if src.Storage != c.wantSrc || dst.Storage != c.wantDst {
				t.Errorf("storage src=%d dst=%d, want src=%d dst=%d", src.Storage, dst.Storage, c.wantSrc, c.wantDst)
			}

			if got.Cargo != economy.CargoRaw {
				t.Errorf("Cargo = %v, want raw", got.Cargo)
			}

			if got.SourceMaterials != 30 || got.SourceCapacity != 40 || got.TargetMaterials != 10 || got.TargetCapacity != 60 {
				t.Errorf("sides = %d/%d and %d/%d, want 30/40 and 10/60",
					got.SourceMaterials, got.SourceCapacity, got.TargetMaterials, got.TargetCapacity)
			}
		})
	}
}

// TestT4OpenTransferWithoutHook pins that a screen with no transfer hook leaves cargo alone.
func TestT4OpenTransferWithoutHook(t *testing.T) {
	s, g := newClickScreen(t)
	src := t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 30)
	dst := t4Truck(g, assets.Constrct, 2, 6, 5, 60, 10)
	s.openTransfer(src, dst)

	if src.Storage != 30 || dst.Storage != 10 {
		t.Errorf("cargo changed without a hook: src=%d dst=%d", src.Storage, dst.Storage)
	}
}

// TestT4OpenTransferFromComplex pins that a truck draws from the pooled store of an adjacent base.
func TestT4OpenTransferFromComplex(t *testing.T) {
	s, g := newClickScreen(t)
	t4ArtlessScreen(s)
	mine, c := t4MiningStation(t, g, 5, 5, 40)
	truck := t4Truck(g, assets.Constrct, 2, 7, 5, 60, 0)

	var got screendata.TransferInfo

	s.SetTransferFunc(func(info screendata.TransferInfo) (int, error) {
		got = info

		return -25, nil // the target (the base) gives 25 to the source (the truck)
	})
	s.openTransfer(truck, mine)

	if got.TargetMaterials != 40 || got.TargetCapacity != 50 {
		t.Errorf("base side = %d/%d, want 40/50", got.TargetMaterials, got.TargetCapacity)
	}

	if truck.Storage != 25 {
		t.Errorf("truck storage = %d, want 25", truck.Storage)
	}

	if c.Material != 15 {
		t.Errorf("complex material = %d, want 15", c.Material)
	}
}

// TestT4OpenTransferClampsToCapacity pins that the receiver never exceeds its capacity.
func TestT4OpenTransferClampsToCapacity(t *testing.T) {
	s, g := newClickScreen(t)
	t4ArtlessScreen(s)
	src := t4Truck(g, assets.SplyTrck, 1, 5, 5, 40, 30)
	dst := t4Truck(g, assets.Constrct, 2, 6, 5, 60, 55)

	s.SetTransferFunc(func(screendata.TransferInfo) (int, error) { return 30, nil })
	s.openTransfer(src, dst)

	if dst.Storage != 60 {
		t.Errorf("receiver storage = %d, want clamp at capacity 60", dst.Storage)
	}

	if src.Storage < 0 {
		t.Errorf("giver storage = %d, must not go negative", src.Storage)
	}
}
