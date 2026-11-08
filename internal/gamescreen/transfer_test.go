// transfer_test.go : tests transfer.go.
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
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/world"
)

// TestTransferTargetValid checks the cursor's target test: a matching cargo
// adjacent unit is valid, a different cargo unit / a non adjacent unit / the
// source itself / an empty cell are not.
func TestTransferTargetValid(t *testing.T) {
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := &sim.Game{Units: units.NewManager(), State: game.NewGameState(p)}
	s := &Screen{game: g}

	src := &game.UnitInfo{ID: 1, Type: assets.Constrct, Team: 0, Flags: game.FlagMobileLandUnit | game.FlagSelectable}
	g.Units.Deploy(src, 5, 5)
	s.orderInput.xferSource = src

	raw := &game.UnitInfo{ID: 2, Type: assets.SplyTrck, Team: 0, Flags: game.FlagMobileLandUnit | game.FlagSelectable}
	g.Units.Deploy(raw, 6, 5) // adjacent, CargoRaw like the constructor
	fuel := &game.UnitInfo{ID: 3, Type: assets.FuelTrck, Team: 0, Flags: game.FlagMobileLandUnit | game.FlagSelectable}
	g.Units.Deploy(fuel, 5, 6) // adjacent, but CargoFuel
	far := &game.UnitInfo{ID: 4, Type: assets.SplyTrck, Team: 0, Flags: game.FlagMobileLandUnit | game.FlagSelectable}
	g.Units.Deploy(far, 9, 9) // matching cargo but not adjacent

	if !s.transferTargetValid(6, 5) {
		t.Error("adjacent matching-cargo unit should be a valid transfer target")
	}
	if s.transferTargetValid(5, 6) {
		t.Error("a different-cargo unit should NOT be valid")
	}
	if s.transferTargetValid(9, 9) {
		t.Error("a non-adjacent unit should NOT be valid")
	}
	if s.transferTargetValid(5, 5) {
		t.Error("the source's own cell should NOT be valid")
	}
	if s.transferTargetValid(7, 7) {
		t.Error("an empty cell should NOT be valid")
	}
}

// TestTransferReloadAcrossConnectorChain checks a random2 scenario: a supply
// truck parked next to a building that is TWO buildings from the mining station,
// linked by a connector, is a valid X-Fer reload target. The connector pools the
// mine + the far building into ONE complex, so the truck draws the mine's raw
// from the building it stands next to. This covers the random2 base
// fragmenting into stranded complexes that showed the NO cursor.)
func TestTransferReloadAcrossConnectorChain(t *testing.T) {
	m := &world.Map{Preview: world.Preview{W: 24, H: 24}, Surface: make([]uint8, 24*24)}
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	mk := func(ty assets.UnitType, gx, gy int, big bool) *game.UnitInfo {
		fl := game.FlagStationary | game.FlagSelectable | game.FlagRequiresSlab
		if big {
			fl |= game.FlagBuilding
		}
		u := &game.UnitInfo{Type: ty, Team: 0, Flags: fl, Values: &game.UnitValues{}}
		u.Values.Set(game.AttribStorage, 50)
		u.Storage = 40
		g.PlaceBuilding(u, gx, gy)
		return u
	}
	mine := mk(assets.MiningSt, 5, 5, true) // cells 5,5..6,6 (stores raw)
	conn := mk(assets.Cnct4W, 7, 5, false)  // 1x1 abutting the mine's east edge
	depot := mk(assets.Depot, 8, 5, true)   // cells 8,5..9,6, abutting the connector
	_ = conn

	// The connector chain must pool the mine and the depot into one complex.
	if g.ComplexOf(mine) == nil || g.ComplexOf(mine) != g.ComplexOf(depot) {
		t.Fatal("connector chain did not pool the mine and depot into one complex")
	}

	// A supply truck parked east of the depot - two buildings from the mine.
	truck := &game.UnitInfo{
		Type:  assets.SplyTrck,
		Team:  0,
		GridX: 10, GridY: 5,
		Flags:  game.FlagMobileLandUnit | game.FlagSelectable,
		Values: &game.UnitValues{},
	}
	truck.Values.Set(game.AttribStorage, 40)
	g.Units.Deploy(truck, 10, 5)

	s := &Screen{game: g}
	s.orderInput.xferSource = truck
	// Hovering any of the depot's footprint cells is a valid reload target.
	if !s.transferTargetValid(9, 5) {
		t.Error("truck next to a connector-linked building should be a valid reload target")
	}
	if !s.transferTargetValid(8, 6) {
		t.Error("any footprint cell of the linked building should be a valid target")
	}
}

// TestTransferBuildingSourceRejectsWrongCargo verifies the rule that a STATIONARY
// source (a building) cannot push its cargo into an adjacent building of a
// different cargo kind via the complex pool: a complex already pools its own
// buildings, so a mining station must not X-Fer raw into a power station it
// merely sits beside. A mobile supply truck, by contrast, may still draw raw from
// the same complex through any connected building.
func TestTransferBuildingSourceRejectsWrongCargo(t *testing.T) {
	m := &world.Map{Preview: world.Preview{W: 24, H: 24}, Surface: make([]uint8, 24*24)}
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	mk := func(ty assets.UnitType, gx, gy int) *game.UnitInfo {
		u := &game.UnitInfo{
			Type:   ty,
			Team:   0,
			Flags:  game.FlagStationary | game.FlagBuilding | game.FlagSelectable | game.FlagRequiresSlab,
			Values: &game.UnitValues{},
		}
		u.Values.Set(game.AttribStorage, 50)
		u.Storage = 30
		g.PlaceBuilding(u, gx, gy)
		return u
	}
	mine := mk(assets.MiningSt, 5, 5) // raw, cells 5,5..6,6
	powr := mk(assets.PowerStn, 7, 5) // fuel, cells 7,5..8,6 (abuts the mine -> one complex)
	if g.ComplexOf(mine) == nil || g.ComplexOf(mine) != g.ComplexOf(powr) {
		t.Fatal("mine and power station should pool into one complex")
	}

	s := &Screen{game: g}
	// A mining station (stationary, raw) hovering the power station (fuel): the
	// wrong cargo kind, and a building source may not use the complex path.
	s.orderInput.xferSource = mine
	if s.transferTargetValid(7, 5) {
		t.Error("a mining station must NOT X-Fer raw into an adjacent power station")
	}

	// A mobile supply truck (raw) parked by the power station DOES refill from the
	// complex (the connected mine stores the raw).
	truck := &game.UnitInfo{
		Type:  assets.SplyTrck,
		Team:  0,
		GridX: 9, GridY: 5,
		Flags:  game.FlagMobileLandUnit | game.FlagSelectable,
		Values: &game.UnitValues{},
	}
	truck.Values.Set(game.AttribStorage, 40)
	g.Units.Deploy(truck, 9, 5)
	s.orderInput.xferSource = truck
	if !s.transferTargetValid(8, 5) {
		t.Error("a supply truck should refill raw from the complex via the connected power station")
	}
}

// TestAdjustStoredMobile verifies the unit to unit transfer write back: a mobile
// receiver's Storage rises/falls by the transferred amount and clamps to its
// capacity, so the stat window reflects the change after an X-Fer.
func TestAdjustStoredMobile(t *testing.T) {
	s := &Screen{}
	u := &game.UnitInfo{Type: assets.SplyTrck, Values: &game.UnitValues{}}
	u.Values.Set(game.AttribStorage, 50)
	u.Storage = 10

	s.adjustStored(u, nil, economy.CargoRaw, 20) // receive 20
	if u.Storage != 30 {
		t.Errorf("after +20: storage = %d, want 30", u.Storage)
	}
	s.adjustStored(u, nil, economy.CargoRaw, -5) // give 5
	if u.Storage != 25 {
		t.Errorf("after -5: storage = %d, want 25", u.Storage)
	}
	s.adjustStored(u, nil, economy.CargoRaw, 100) // clamp to capacity
	if u.Storage != 50 {
		t.Errorf("clamped storage = %d, want 50", u.Storage)
	}
}

// TestTransferRefillFromComplexBuilding checks that a constructor can draw raw
// material from the pooled store of an adjacent complex building: the connectors
// distribute resources, so any connected building represents the complex's
// cargo even when the building's own type does not store raw.
func TestTransferRefillFromComplexBuilding(t *testing.T) {
	m := &world.Map{Preview: world.Preview{W: 16, H: 16}, Surface: make([]uint8, 16*16)}
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	// A mining station forms a complex holding 40 of 50 raw.
	mine := &game.UnitInfo{
		Type:   assets.MiningSt,
		Team:   0,
		Flags:  game.FlagBuilding | game.FlagStationary | game.FlagSelectable,
		Values: &game.UnitValues{},
	}
	mine.Values.Set(game.AttribStorage, 50)
	mine.Storage = 40
	if c := g.PlaceBuilding(mine, 5, 5); c == nil {
		t.Fatal("mining station did not form a complex")
	}

	// A constructor (carries raw build material) parked next to it, empty.
	con := &game.UnitInfo{
		Type:  assets.Constrct,
		Team:  0,
		GridX: 7, GridY: 5,
		Flags:  game.FlagMobileLandUnit | game.FlagSelectable | game.FlagConstructorUnit,
		Values: &game.UnitValues{},
	}
	con.Values.Set(game.AttribStorage, 40)

	s := &Screen{game: g}
	ctype := economy.CargoTypeOf(con.Type)
	if ctype != economy.CargoRaw {
		t.Fatalf("constructor cargo type = %v, want raw", ctype)
	}

	mat, capc, cx := s.cargoSide(mine, con, ctype)
	if cx == nil {
		t.Fatal("the building side did not resolve to its complex")
	}
	if mat != 40 || capc != 50 {
		t.Errorf("complex raw side = %d/%d, want 40/50", mat, capc)
	}

	// Refill: move 30 raw from the complex into the constructor.
	s.adjustStored(con, nil, ctype, 30)
	s.adjustStored(mine, cx, ctype, -30)
	if con.Storage != 30 {
		t.Errorf("constructor storage after refill = %d, want 30", con.Storage)
	}
	if mat2, _, _ := s.cargoSide(mine, con, ctype); mat2 != 10 {
		t.Errorf("complex raw after refill = %d, want 10", mat2)
	}
}

// allLandGame returns a sim game over an all land w x h map for transfer tests.
func allLandGame(w, h int) *sim.Game {
	m := &world.Map{Preview: world.Preview{W: w, H: h}, Surface: make([]uint8, w*h)}
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	return sim.New(p, m)
}

// placeStorer places and registers a storage building (1x1 or 2x2) holding cargo.
func placeStorer(g *sim.Game, ty assets.UnitType, gx, gy int, big bool) *game.UnitInfo {
	fl := game.FlagStationary | game.FlagSelectable | game.FlagRequiresSlab
	if big {
		fl |= game.FlagBuilding
	}
	u := &game.UnitInfo{Type: ty, Team: 0, Flags: fl, Values: &game.UnitValues{}}
	u.Values.Set(game.AttribStorage, 50)
	u.Storage = 40
	g.PlaceBuilding(u, gx, gy)
	return u
}

// mobileCargo returns a mobile cargo unit (truck/constructor) at a cell.
func mobileCargo(g *sim.Game, ty assets.UnitType, gx, gy int) *game.UnitInfo {
	u := &game.UnitInfo{
		Type:  ty,
		Team:  0,
		GridX: gx, GridY: gy,
		Flags:  game.FlagMobileLandUnit | game.FlagSelectable,
		Values: &game.UnitValues{},
	}
	u.Values.Set(game.AttribStorage, 40)
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestTransferReachesAcrossComplex verifies the manual's rule (p.64): a connected
// complex is ONE transfer endpoint, so a unit adjacent to ANY connected structure
// can transfer with a building anywhere in that complex - whichever side the player
// selects first. A truck touching only the FAR end of a connector chain may target
// the mining station at the near end, and the mine (selected) may target that truck.
func TestTransferReachesAcrossComplex(t *testing.T) {
	g := allLandGame(24, 24)

	// A connector chain: mine - conn - storage - conn - far, all one complex.
	mine := placeStorer(g, assets.MiningSt, 5, 5, true) // raw, cells 5,5..6,6
	placeStorer(g, assets.Cnct4W, 7, 5, false)          // 1x1, abuts the mine
	placeStorer(g, assets.ADump, 8, 5, false)           // 1x1 raw storage, abuts conn1
	placeStorer(g, assets.Cnct4W, 9, 5, false)          // 1x1, abuts the storage
	far := placeStorer(g, assets.Depot, 10, 5, true)    // 2x2, cells 10,5..11,6, abuts conn2
	if g.ComplexOf(mine) == nil || g.ComplexOf(mine) != g.ComplexOf(far) {
		t.Fatal("the connector chain did not pool the whole base into one complex")
	}

	// A supply truck touching only the FAR building (7 cells from the mine).
	truck := mobileCargo(g, assets.SplyTrck, 12, 5)

	s := &Screen{game: g}
	// Truck selected, hovering the distant mining station: valid (truck is next to a
	// connected structure).
	s.orderInput.xferSource = truck
	if !s.transferTargetValid(5, 5) {
		t.Error("a truck next to a connected building should reach the whole complex (hovering the mine)")
	}
	// Mine selected, targeting the far truck: valid (the truck is next to the complex).
	s.orderInput.xferSource = mine
	if !s.transferTargetValid(12, 5) {
		t.Error("a selected complex building should reach a truck next to any connected structure")
	}

	// A truck nowhere near the complex is NOT reachable.
	stray := mobileCargo(g, assets.SplyTrck, 20, 20)
	s.orderInput.xferSource = stray
	if s.transferTargetValid(5, 5) {
		t.Error("a truck not adjacent to any connected structure must NOT reach the complex")
	}
}

// TestTransferTargetMatchesCargoKind proves the transfer target rule is cargo kind generic: a Fuel Truck may
// transfer with a fuel complex, a Gold Truck with a gold complex, and neither with a complex of another kind
// (a Fuel Truck with a raw only complex, a Gold Truck with a fuel store).
func TestTransferTargetMatchesCargoKind(t *testing.T) {
	g := allLandGame(24, 24)
	placeStorer(g, assets.FDump, 5, 5, false)    // fuel, 1x1
	placeStorer(g, assets.GoldSm, 15, 5, false)  // gold, 1x1
	placeStorer(g, assets.MiningSt, 5, 15, true) // raw, 2x2
	/* fuelStore, goldStore, rawMine := ...; _ = fuelStore; _ = rawMine; _ = goldStore: note: unused code in Go port, the names were never read */

	s := &Screen{game: g}

	// Fuel Truck next to the fuel store: valid (same cargo kind).
	fuelTruck := mobileCargo(g, assets.FuelTrck, 6, 5)
	s.orderInput.xferSource = fuelTruck
	if economy.CargoTypeOf(fuelTruck.Type) != economy.CargoFuel {
		t.Fatalf("fuel truck cargo = %v, want fuel", economy.CargoTypeOf(fuelTruck.Type))
	}
	if !s.transferTargetValid(5, 5) {
		t.Error("a fuel truck next to a fuel store should be a valid target")
	}
	// The same fuel truck must NOT draw from the raw mine complex.
	if s.transferTargetValid(5, 15) {
		t.Error("a fuel truck must NOT transfer with a raw-only complex")
	}

	// Gold Truck next to the gold vault: valid.
	goldTruck := mobileCargo(g, assets.GoldTrck, 16, 5)
	s.orderInput.xferSource = goldTruck
	if economy.CargoTypeOf(goldTruck.Type) != economy.CargoGold {
		t.Fatalf("gold truck cargo = %v, want gold", economy.CargoTypeOf(goldTruck.Type))
	}
	if !s.transferTargetValid(15, 5) {
		t.Error("a gold truck next to a gold vault should be a valid target")
	}

	// A Gold Truck right next to the fuel store must NOT transfer with it.
	strayGold := mobileCargo(g, assets.GoldTrck, 4, 5)
	s.orderInput.xferSource = strayGold
	if s.transferTargetValid(5, 5) {
		t.Error("a gold truck must NOT transfer with a fuel store")
	}
}

// TestTransferTwoMobileUnitsNeedAdjacency checks that two mobile cargo units (no
// complex) get NO complex range shortcut: they transfer only when directly adjacent.
func TestTransferTwoMobileUnitsNeedAdjacency(t *testing.T) {
	g := allLandGame(24, 24)
	a := mobileCargo(g, assets.SplyTrck, 5, 5)
	mobileCargo(g, assets.SplyTrck, 6, 5) // touching a
	mobileCargo(g, assets.SplyTrck, 9, 5) // 4 cells from a
	/* adjacent, apart := ...; _ = adjacent; _ = apart: note: unused code in Go port, the names were never read */

	s := &Screen{game: g}
	s.orderInput.xferSource = a
	if !s.transferTargetValid(6, 5) {
		t.Error("two adjacent supply trucks should be a valid transfer pair")
	}
	if s.transferTargetValid(9, 5) {
		t.Error("two non-adjacent mobile units must NOT transfer (no complex range)")
	}
}

// TestMineLayerReloadsRaw checks a reported gap: a mine layer (land or sea)
// carries raw cargo (its mine store), so it can X-Fer raw - reloading from an
// adjacent supply truck (Path A, same cargo kind) or a raw storage complex (Path B).
func TestMineLayerReloadsRaw(t *testing.T) {
	for _, layer := range []assets.UnitType{assets.MineLayr, assets.SeaMnLyr} {
		g := allLandGame(24, 24)
		rawMine := placeStorer(g, assets.MiningSt, 5, 15, true) // raw, 2x2 complex
		_ = rawMine
		truck := mobileCargo(g, assets.SplyTrck, 6, 5) // raw supply truck
		_ = truck

		s := &Screen{game: g}
		ml := mobileCargo(g, layer, 5, 5) // next to the supply truck
		ml.Storage = 3                    // partly spent: room to reload
		s.orderInput.xferSource = ml

		if economy.CargoTypeOf(layer) != economy.CargoRaw {
			t.Fatalf("%s cargo = %v, want raw", layer.Tag(), economy.CargoTypeOf(layer))
		}
		// Path A: reload from an adjacent supply truck (same raw cargo).
		if !s.transferTargetValid(6, 5) {
			t.Errorf("%s next to a supply truck should be a valid raw transfer", layer.Tag())
		}
		// Path B: refill from a raw storage complex it stands beside.
		ml2 := mobileCargo(g, layer, 5, 14) // adjacent to the 2x2 mine at 5,15
		s.orderInput.xferSource = ml2
		if !s.transferTargetValid(5, 15) {
			t.Errorf("%s beside a raw mining complex should be able to reload raw", layer.Tag())
		}
	}
}
