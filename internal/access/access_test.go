// access_test.go : tests terrain and unit passability rules.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package access

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// surfStub is a synthetic surface: def everywhere, overridden per cell.
type surfStub struct {
	def   uint8
	cells map[[2]int]uint8
}

func (s surfStub) SurfaceAt(x, y int) uint8 {
	if v, ok := s.cells[[2]int{x, y}]; ok {
		return v
	}

	return s.def
}

// unitStub indexes units by cell.
type unitStub map[[2]int][]*game.UnitInfo

func (u unitStub) At(x, y int) []*game.UnitInfo {
	return u[[2]int{x, y}]
}

func gridWith(def uint8, cells map[[2]int]uint8, units unitStub) *Grid {
	return &Grid{W: 8, H: 8, Surf: surfStub{def: def, cells: cells}, Units: units}
}

// Movement class descriptors used across the tests.
var (
	landUnit = UnitDesc{Type: assets.Tank, Flags: game.FlagMobileLandUnit | game.FlagSelectable, LandType: world.SurfaceLand}
	seaUnit  = UnitDesc{Type: assets.Corvette, Flags: game.FlagMobileSeaUnit, LandType: world.SurfaceWater}
	airUnit  = UnitDesc{Type: assets.Bomber, Flags: game.FlagMobileAirUnit | game.FlagHovering,
		LandType: world.SurfaceLand | world.SurfaceWater | world.SurfaceCoast | world.SurfaceAir}
)

// stationary builds a stationary structure unit of a type/team for a cell.
func stationary(t assets.UnitType, team int) *game.UnitInfo {
	return &game.UnitInfo{Type: t, Team: team, Flags: game.FlagStationary, Order: game.OrderIdle}
}

// mover builds an active (not idle) mobile unit of a movement class.
func mover(t assets.UnitType, team int, flags game.Flags) *game.UnitInfo {
	return &game.UnitInfo{Type: t, Team: team, Flags: flags, Order: game.OrderAwait}
}

// TestUnfoldingBuildingIsPassable checks that a structure remains passable while
// Unfold is positive. This lets its builder leave before the finished structure
// becomes an obstacle.
func TestUnfoldingBuildingIsPassable(t *testing.T) {
	bld := &game.UnitInfo{
		Type:  assets.PowerStn,
		Team:  0,
		Flags: game.FlagStationary | game.FlagBuilding,
		Order: game.OrderIdle,
	}

	g := gridWith(world.SurfaceLand, nil, unitStub{{4, 4}: {bld}})

	// Still unfolding: the builder can drive off the cell.
	bld.Unfold = 3
	if got := g.IsAccessible(landUnit, 0, 4, 4, NoModifiers); got == Blocked {
		t.Error("a building still unfolding (Unfold>0) must be PASSABLE")
	}

	// Fully expanded: a solid obstacle again.
	bld.Unfold = 0
	if got := g.IsAccessible(landUnit, 0, 4, 4, NoModifiers); got != Blocked {
		t.Errorf("a fully-expanded building must Block, got %d", got)
	}
}

// TestEasterEggPropsBlockMovement checks that Waldo and Harvster are solid 1x1
// obstacles. The C++ data marks them passable, but this port spawns them as props
// that land units must drive around.
func TestEasterEggPropsBlockMovement(t *testing.T) {
	for _, et := range []assets.UnitType{assets.Waldo, assets.Harvster} {
		g := gridWith(world.SurfaceLand, nil, unitStub{{4, 4}: {stationary(et, game.AlienTeam)}})
		if got := g.IsAccessible(landUnit, 0, 4, 4, NoModifiers); got != Blocked {
			t.Errorf("%s must block movement, got %d (want Blocked)", et.Tag(), got)
		}
	}
}

func TestSurfaceLandTypeGating(t *testing.T) {
	land := gridWith(world.SurfaceLand, nil, nil)
	water := gridWith(world.SurfaceWater, nil, nil)

	if got := land.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("land unit on land = %d, want Clear", got)
	}
	if got := water.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Blocked {
		t.Errorf("land unit on water = %d, want Blocked", got)
	}
	if got := water.IsAccessible(seaUnit, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("sea unit on water = %d, want Clear", got)
	}
	if got := land.IsAccessible(seaUnit, 0, 1, 1, NoModifiers); got != Blocked {
		t.Errorf("sea unit on land = %d, want Blocked", got)
	}
	if got := water.IsAccessible(airUnit, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("air unit over water = %d, want Clear", got)
	}

	// Coast is the amphibious only shoreline band: a pure sea unit (like a pure
	// land unit) is blocked there, so a submarine/boat routes AROUND a coastline
	// rather than cutting across it. An amphibious unit crosses it.
	coast := gridWith(world.SurfaceCoast, nil, nil)
	if got := coast.IsAccessible(seaUnit, 0, 1, 1, NoModifiers); got != Blocked {
		t.Errorf("sea unit on coast = %d, want Blocked (coast is amphibious-only)", got)
	}

	if got := coast.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Blocked {
		t.Errorf("land unit on coast = %d, want Blocked (coast is amphibious-only)", got)
	}

	amphibUnit := UnitDesc{Type: assets.Scout, Flags: game.FlagMobileLandUnit | game.FlagMobileSeaUnit, LandType: LandTypeFor(game.FlagMobileLandUnit | game.FlagMobileSeaUnit)}

	if got := coast.IsAccessible(amphibUnit, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("amphibious unit on coast = %d, want Clear", got)
	}
}

func TestAmphibiousLandUnitOnWater(t *testing.T) {
	// A land unit whose land type also covers water crosses open water at
	// the amphibious cost (4|8 = 12), while the surveyor is excluded from
	// that path.
	amphib := UnitDesc{Type: assets.Tank, Flags: game.FlagMobileLandUnit, LandType: world.SurfaceLand | world.SurfaceWater}
	water := gridWith(world.SurfaceWater, nil, nil)

	if got := water.IsAccessible(amphib, 0, 1, 1, NoModifiers); got != Water {
		t.Errorf("amphibious land unit on water = %d, want Water(12)", got)
	}

	surveyor := UnitDesc{Type: assets.Surveyor, Flags: game.FlagMobileLandUnit, LandType: world.SurfaceLand | world.SurfaceWater}
	if got := water.IsAccessible(surveyor, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("surveyor on water = %d, want Clear (excluded from amphibious cost)", got)
	}
}

func TestLandTypeFor(t *testing.T) {
	const (
		land  = world.SurfaceLand
		water = world.SurfaceWater
		coast = world.SurfaceCoast
		air   = world.SurfaceAir
	)

	cases := []struct {
		name  string
		flags game.Flags
		want  uint8
	}{
		{"land", game.FlagMobileLandUnit, land},
		{"sea", game.FlagMobileSeaUnit, water}, // deep water only; coast is amphibious only
		{"air", game.FlagMobileAirUnit, land | water | coast | air},
		{"amphibious", game.FlagMobileLandUnit | game.FlagMobileSeaUnit, land | water | coast},
		{"stationary", game.FlagStationary, land},
	}

	for _, c := range cases {
		if got := LandTypeFor(c.flags); got != c.want {
			t.Errorf("LandTypeFor(%s) = %#x, want %#x", c.name, got, c.want)
		}
	}
}

// TestLandTypeForUnit checks that LandTypeForUnit produces the same result as
// LandTypeFor for every non Surveyor type and the Surveyor override for the
// Surveyor itself. The Surveyor carries MOBILE_LAND_UNIT only in the original
// data but its land_type includes water so it can traverse open water at
// normal (not amphibious) cost.
//
// C++ ported : access.cpp - Access_IsAccessible (unit_type != SURVEYOR branch)
// + Surveyor land_type entry (units.cpp - ParseLandType).
func TestLandTypeForUnit(t *testing.T) {
	const (
		land  = world.SurfaceLand
		water = world.SurfaceWater
		coast = world.SurfaceCoast
	)

	landFlags := game.FlagMobileLandUnit

	// Non Surveyor land unit: LandTypeForUnit must match LandTypeFor.
	if got, want := LandTypeForUnit(assets.Tank, landFlags), LandTypeFor(landFlags); got != want {
		t.Errorf("LandTypeForUnit(Tank, land) = %#x, want %#x (same as LandTypeFor)", got, want)
	}

	// Surveyor with MOBILE_LAND_UNIT only gets land|water|coast, not just land.
	if got, want := LandTypeForUnit(assets.Surveyor, landFlags), uint8(land|water|coast); got != want {
		t.Errorf("LandTypeForUnit(Surveyor, land) = %#x, want %#x (land+water+coast)", got, want)
	}

	// The Constructor, Engineer and Scout are Surveyor like: MOBILE_LAND_UNIT only, but
	// their land_type includes water so they can drive onto open water (build platforms /
	// recon). They are NOT dual flag amphibians - they keep their land look and cross
	// bridges on top - and DO pay the +8 amphibious water cost (only the Surveyor is exempt).
	for _, builder := range []assets.UnitType{assets.Constrct, assets.Engineer, assets.Scout} {
		if got, want := LandTypeForUnit(builder, landFlags), uint8(land|water|coast); got != want {
			t.Errorf("LandTypeForUnit(%v, land) = %#x, want %#x (can reach water)", builder, got, want)
		}

		d := UnitDesc{Type: builder, Flags: landFlags, LandType: LandTypeForUnit(builder, landFlags)}
		waterGrid := gridWith(world.SurfaceWater, nil, nil)

		if got := waterGrid.IsAccessible(d, 0, 1, 1, NoModifiers); got != Water {
			t.Errorf("%v on open water = %d, want Water(12) (amphibious cost, not exempt like Surveyor)", builder, got)
		}
	}

	// A Surveyor descriptor built via LandTypeForUnit is passable on water at
	// Clear cost (not amphibious cost, not blocked).
	survDesc := UnitDesc{
		Type:     assets.Surveyor,
		Flags:    landFlags,
		LandType: LandTypeForUnit(assets.Surveyor, landFlags),
	}

	waterGrid := gridWith(world.SurfaceWater, nil, nil)
	if got := waterGrid.IsAccessible(survDesc, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("Surveyor on open water = %d, want Clear (normal, not amphibious cost)", got)
	}

	landGrid := gridWith(world.SurfaceLand, nil, nil)
	if got := landGrid.IsAccessible(survDesc, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("Surveyor on land = %d, want Clear", got)
	}

	// A pure land Tank built via LandTypeForUnit is still blocked on water.
	tankDesc := UnitDesc{
		Type:     assets.Tank,
		Flags:    landFlags,
		LandType: LandTypeForUnit(assets.Tank, landFlags),
	}

	if got := waterGrid.IsAccessible(tankDesc, 0, 1, 1, NoModifiers); got != Blocked {
		t.Errorf("Tank on water = %d, want Blocked", got)
	}
}

// TestConnectorBuildableOnWater checks that an engineer can place a connector on
// land, coast, or open water to join a naval building to a base. A Power Station
// remains blocked on water.
func TestConnectorBuildableOnWater(t *testing.T) {
	const (
		land  = world.SurfaceLand
		water = world.SurfaceWater
		coast = world.SurfaceCoast
	)
	connFlags := game.FlagStationary | game.FlagBuilding
	if got, want := LandTypeForUnit(assets.Cnct4W, connFlags), uint8(land|water|coast); got != want {
		t.Errorf("LandTypeForUnit(Cnct4W) = %#x, want %#x (land+water+coast)", got, want)
	}

	conn := UnitDesc{Type: assets.Cnct4W, Flags: connFlags, LandType: LandTypeForUnit(assets.Cnct4W, connFlags)}
	for _, tc := range []struct {
		name string
		surf uint8
	}{{"water", water}, {"coast", coast}, {"land", land}} {
		g := gridWith(tc.surf, nil, nil)
		if got := g.IsAccessible(conn, 0, 1, 1, NoModifiers); got == Blocked {
			t.Errorf("connector on %s = Blocked, want buildable (land_type spans water+coast+land)", tc.name)
		}
	}

	// A pure land building stays water blocked - the widening is connector specific.
	gen := UnitDesc{Type: assets.PowGen, Flags: connFlags, LandType: LandTypeForUnit(assets.PowGen, connFlags)}
	if got := gridWith(world.SurfaceWater, nil, nil).IsAccessible(gen, 0, 1, 1, NoModifiers); got != Blocked {
		t.Errorf("power generator on water = %d, want Blocked (only the connector spans water)", got)
	}
}

// TestWaterBuildingLandType checks the naval building override: a Shipyard, Dock
// and Water Platform are placed on open water (land_type water), so each is
// accessible on a water cell and blocked on land - the inverse of a normal
// land building (a Power Station). IsWaterBuilding names exactly that set.
func TestWaterBuildingLandType(t *testing.T) {
	buildingFlags := game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	waterGrid := gridWith(world.SurfaceWater, nil, nil)
	landGrid := gridWith(world.SurfaceLand, nil, nil)

	// Every naval building places on water and is blocked on land.
	for _, t2 := range []assets.UnitType{assets.Shipyard, assets.Dock, assets.WtrPltfm} {
		if !IsWaterBuilding(t2) {
			t.Errorf("IsWaterBuilding(%v) = false, want true", t2)
		}
		d := UnitDesc{Type: t2, Flags: buildingFlags, LandType: LandTypeForUnit(t2, buildingFlags)}
		if got := waterGrid.IsAccessible(d, 0, 1, 1, NoModifiers); !got.Passable() {
			t.Errorf("%v on water = %d, want passable", t2, got)
		}
		if got := landGrid.IsAccessible(d, 0, 1, 1, NoModifiers); got != Blocked {
			t.Errorf("%v on land = %d, want Blocked (naval building)", t2, got)
		}
	}

	// A normal land building (Power Station) is the inverse: land yes, water no.
	if IsWaterBuilding(assets.PowerStn) {
		t.Error("IsWaterBuilding(PowerStn) = true, want false")
	}

	powerStn := UnitDesc{Type: assets.PowerStn, Flags: buildingFlags, LandType: LandTypeForUnit(assets.PowerStn, buildingFlags)}
	if got := landGrid.IsAccessible(powerStn, 0, 1, 1, NoModifiers); !got.Passable() {
		t.Errorf("PowerStn on land = %d, want passable", got)
	}

	if got := waterGrid.IsAccessible(powerStn, 0, 1, 1, NoModifiers); got != Blocked {
		t.Errorf("PowerStn on water = %d, want Blocked (land building)", got)
	}
}

// TestLandBuildingOnWaterPlatform checks that a Water Platform makes its cell
// valid for land buildings. The same building remains blocked on bare water.
func TestLandBuildingOnWaterPlatform(t *testing.T) {
	platform := &game.UnitInfo{
		Type:  assets.WtrPltfm,
		Team:  0,
		Flags: game.FlagStationary,
		Order: game.OrderIdle,
	}

	withPlatform := gridWith(world.SurfaceWater, nil, unitStub{{1, 1}: {platform}})
	bareWater := gridWith(world.SurfaceWater, nil, nil)

	landFlags := game.FlagStationary | game.FlagSelectable

	powerGen := UnitDesc{
		Type:     assets.PowGen,
		Flags:    landFlags,
		LandType: LandTypeForUnit(assets.PowGen, landFlags),
	}

	// On a platform the land building is buildable; on bare water it is blocked.
	if got := withPlatform.IsAccessible(powerGen, 0, 1, 1, SameClassBlocks); !got.Passable() {
		t.Errorf("land building on a water platform = %d, want passable", got)
	}

	if got := bareWater.IsAccessible(powerGen, 0, 1, 1, SameClassBlocks); got != Blocked {
		t.Errorf("land building on bare water = %d, want Blocked", got)
	}
}

func TestDualFlagAmphibiousPassability(t *testing.T) {
	// An amphibious unit deployed with both movement flags (as the game does)
	// derives a land|water|coast land type, so it crosses water at the amphibious
	// cost and stays passable on land; a pure land tank is blocked on water.
	flags := game.FlagMobileLandUnit | game.FlagMobileSeaUnit
	amphib := UnitDesc{Type: assets.Scout, Flags: flags, LandType: LandTypeFor(flags)}
	water := gridWith(world.SurfaceWater, nil, nil)
	land := gridWith(world.SurfaceLand, nil, nil)

	if got := water.IsAccessible(amphib, 0, 1, 1, NoModifiers); got != Water {
		t.Errorf("amphibious unit on water = %d, want Water(12)", got)
	}

	if got := land.IsAccessible(amphib, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("amphibious unit on land = %d, want Clear", got)
	}

	if got := water.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Blocked {
		t.Errorf("pure-land unit on water = %d, want Blocked", got)
	}
}

func TestOutOfBoundsAndIgnoreSurface(t *testing.T) {
	g := gridWith(world.SurfaceLand, nil, nil)
	if got := g.IsAccessible(landUnit, 0, -1, 0, NoModifiers); got != Blocked {
		t.Errorf("off-map = %d, want Blocked", got)
	}

	// IgnoreSurfaceClass passes even where the surface would block.
	water := gridWith(world.SurfaceWater, nil, nil)

	if got := water.IsAccessible(landUnit, 0, 1, 1, IgnoreSurfaceClass); got != Clear {
		t.Errorf("IgnoreSurfaceClass = %d, want Clear", got)
	}
}

func TestModifiedSurfacePlatformAndBridge(t *testing.T) {
	units := unitStub{
		{1, 1}: {stationary(assets.WtrPltfm, 0)},
		{2, 2}: {stationary(assets.Bridge, 0)},
	}
	g := gridWith(world.SurfaceWater, nil, units)

	if s := g.ModifiedSurface(1, 1, false); s != world.SurfaceLand {
		t.Errorf("platform over water (land view) = %d, want Land", s)
	}

	if s := g.ModifiedSurface(2, 2, false); s != world.SurfaceLand {
		t.Errorf("bridge over water (land view) = %d, want Land", s)
	}

	// Sea view: the platform still reads land, the bridge leaves open water.
	if s := g.ModifiedSurface(1, 1, true); s != world.SurfaceLand {
		t.Errorf("platform (sea view) = %d, want Land", s)
	}

	if s := g.ModifiedSurface(2, 2, true); s != world.SurfaceWater {
		t.Errorf("bridge (sea view) = %d, want Water", s)
	}

	// A land unit can stand on a water platform.
	if got := g.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("land unit on water platform = %d, want Clear", got)
	}
}

func TestSameClassBlocking(t *testing.T) {
	units := unitStub{
		{1, 1}: {mover(assets.Tank, 0, game.FlagMobileLandUnit)},
		{2, 2}: {mover(assets.Bomber, 0, game.FlagMobileAirUnit|game.FlagHovering)},
		{3, 3}: {idleMover(assets.Tank, 0, game.FlagMobileLandUnit)},
	}

	g := gridWith(world.SurfaceLand, nil, units)

	// Another land unit is blocked by the occupying land unit...
	if got := g.IsAccessible(landUnit, 0, 1, 1, SameClassBlocks); got != Blocked {
		t.Errorf("land-on-land (SameClassBlocks) = %d, want Blocked", got)
	}

	// ...but an air unit passes over it.
	if got := g.IsAccessible(airUnit, 0, 1, 1, SameClassBlocks); got != Clear {
		t.Errorf("air over a land unit = %d, want Clear", got)
	}

	// Air blocks air.
	if got := g.IsAccessible(airUnit, 0, 2, 2, SameClassBlocks); got != Blocked {
		t.Errorf("air-on-air (SameClassBlocks) = %d, want Blocked", got)
	}

	// Without the modifier, nothing blocks on movement class.
	if got := g.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("land-on-land (no modifier) = %d, want Clear", got)
	}

	// An idle, not stationary mobile unit does not block.
	if got := g.IsAccessible(landUnit, 0, 3, 3, SameClassBlocks); got != Clear {
		t.Errorf("land over an idle unit = %d, want Clear", got)
	}
}

func TestEnemySameClassBlocks(t *testing.T) {
	units := unitStub{{1, 1}: {mover(assets.Tank, 1, game.FlagMobileLandUnit)}} // team 1
	g := gridWith(world.SurfaceLand, nil, units)

	// Mover is team 0: an enemy land unit blocks under EnemySameClassBlocks.
	if got := g.IsAccessible(landUnit, 0, 1, 1, EnemySameClassBlocks); got != Blocked {
		t.Errorf("enemy land unit = %d, want Blocked", got)
	}

	// A friendly unit (same team) does not block under EnemySameClassBlocks.
	friendly := unitStub{{1, 1}: {mover(assets.Tank, 0, game.FlagMobileLandUnit)}}
	g2 := gridWith(world.SurfaceLand, nil, friendly)

	if got := g2.IsAccessible(landUnit, 0, 1, 1, EnemySameClassBlocks); got != Clear {
		t.Errorf("friendly land unit (EnemySameClassBlocks) = %d, want Clear", got)
	}
}

func TestRoadAndSlabReduceCost(t *testing.T) {
	g := gridWith(world.SurfaceLand, nil, unitStub{
		{1, 1}: {stationary(assets.Road, 0)},
		{2, 2}: {stationary(assets.LrgSlab, 0)},
	})

	if got := g.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Reduced {
		t.Errorf("land unit on road = %d, want Reduced", got)
	}

	if got := g.IsAccessible(landUnit, 0, 2, 2, NoModifiers); got != Reduced {
		t.Errorf("land unit on slab = %d, want Reduced", got)
	}

	// A road cannot be built on top of another road or a slab.
	roadDesc := UnitDesc{Type: assets.Road, Flags: game.FlagStationary, LandType: world.SurfaceLand}
	if got := g.IsAccessible(roadDesc, 0, 1, 1, NoModifiers); got != Blocked {
		t.Errorf("road on road = %d, want Blocked", got)
	}

	if got := g.IsAccessible(roadDesc, 0, 2, 2, NoModifiers); got != Blocked {
		t.Errorf("road on slab = %d, want Blocked", got)
	}
}

// TestBridgeAndConnectorShareACell pins the original's connector rule: a BRIDGE
// (like a Water Platform or a Road) may be laid on a cell that already holds a
// connector, and a connector may be laid over a bridge. A bridge stays a water
// structure though, so bare land refuses it whatever sits there.
//
// C++ ported : access.cpp - Access_IsAccessible, the CNCT_4W case exempts
// WTRPLTFM / BRIDGE / ROAD; units_manager.cpp - a BRIDGE build does not clear the
// connectors on its site.
func TestBridgeAndConnectorShareACell(t *testing.T) {
	bridgeFlags := game.FlagStationary | game.FlagSelectable
	bridge := UnitDesc{Type: assets.Bridge, Flags: bridgeFlags, LandType: LandTypeForUnit(assets.Bridge, bridgeFlags)}
	connector := UnitDesc{Type: assets.Cnct4W, Flags: bridgeFlags, LandType: LandTypeForUnit(assets.Cnct4W, bridgeFlags)}

	// An own team connector spanning open water, and the same over coast.
	for _, surf := range []uint8{world.SurfaceWater, world.SurfaceCoast} {
		g := gridWith(surf, nil, unitStub{{2, 2}: {stationary(assets.Cnct4W, 0)}})

		if got := g.IsAccessible(bridge, 0, 2, 2, SameClassBlocks); got == Blocked {
			t.Errorf("bridge under a connector on surface %#x = Blocked, want a buildable site", surf)
		}
	}

	// The reverse: a connector goes over an existing bridge.
	overBridge := gridWith(world.SurfaceWater, nil, unitStub{{2, 2}: {stationary(assets.Bridge, 0)}})
	if got := overBridge.IsAccessible(connector, 0, 2, 2, SameClassBlocks); got == Blocked {
		t.Error("connector over a bridge = Blocked, want a buildable site")
	}

	// A bridge is still a water structure: land refuses it, connector or not.
	onLand := gridWith(world.SurfaceLand, nil, unitStub{{2, 2}: {stationary(assets.Cnct4W, 0)}})
	if got := onLand.IsAccessible(bridge, 0, 2, 2, SameClassBlocks); got != Blocked {
		t.Errorf("bridge on land = %d, want Blocked", got)
	}
}

// TestGroundCoverRoadOnRoadBlocked checks that both player and AI placement
// descriptors reject a second road on the same cell. Mobile units may still use
// the road, and the rule does not block a different structure type.
func TestGroundCoverRoadOnRoadBlocked(t *testing.T) {
	// The existing road on the map as the player build path leaves it: ground cover, idle.
	road := &game.UnitInfo{Type: assets.Road, Team: 0, Flags: game.FlagGroundCover, Order: game.OrderIdle}
	g := gridWith(world.SurfaceLand, nil, unitStub{{1, 1}: {road}})

	// A road placed the manual way (ground cover descriptor) cannot double build.
	roadGC := UnitDesc{Type: assets.Road, Flags: game.FlagGroundCover, LandType: LandTypeForUnit(assets.Road, game.FlagGroundCover)}
	if got := g.IsAccessible(roadGC, 0, 1, 1, SameClassBlocks); got != Blocked {
		t.Errorf("ground-cover road on road = %d, want Blocked", got)
	}

	// A road placed the AI way (stationary descriptor) is blocked over the same ground cover road too.
	roadStat := UnitDesc{Type: assets.Road, Flags: game.FlagStationary, LandType: world.SurfaceLand}
	if got := g.IsAccessible(roadStat, 0, 1, 1, SameClassBlocks); got != Blocked {
		t.Errorf("stationary road on ground-cover road = %d, want Blocked", got)
	}

	// A mobile land unit still drives over the road (the block is placement only).
	if got := g.IsAccessible(landUnit, 0, 1, 1, SameClassBlocks); got == Blocked {
		t.Error("a mobile unit must still drive over a ground-cover road")
	}

	// A different structure type is not caught by the same-type rule (a building over a road).
	bldFlags := game.FlagStationary | game.FlagBuilding
	powerStn := UnitDesc{Type: assets.PowerStn, Flags: bldFlags, LandType: LandTypeForUnit(assets.PowerStn, bldFlags)}
	if got := g.IsAccessible(powerStn, 0, 1, 1, SameClassBlocks); got == Blocked {
		t.Error("a different structure type must not be blocked by the same-type road rule")
	}
}

// TestConnectorPassableAndRoadUnderneath checks that mobile units pass beneath
// connectors. A road on the same cell still provides Reduced cost regardless of
// the order in which the cell index returns both objects.
func TestConnectorPassableAndRoadUnderneath(t *testing.T) {
	// A connector alone does not block a mobile mover; it drives underneath.
	conn := gridWith(world.SurfaceLand, nil, unitStub{{1, 1}: {stationary(assets.Cnct4W, 0)}})

	if got := conn.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("land unit under connector = %d, want Clear", got)
	}

	// Road + connector on the same cell, either listing order: the road's Reduced
	// cost wins, so the unit takes the faster road while passing under the connector.
	for name, us := range map[string]unitStub{
		"road-then-connector": {{1, 1}: {stationary(assets.Road, 0), stationary(assets.Cnct4W, 0)}},
		"connector-then-road": {{1, 1}: {stationary(assets.Cnct4W, 0), stationary(assets.Road, 0)}},
	} {
		g := gridWith(world.SurfaceLand, nil, us)
		if got := g.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Reduced {
			t.Errorf("%s: land unit = %d, want Reduced (road bonus under connector)", name, got)
		}
	}
}

func TestBridgeCostByMoverClass(t *testing.T) {
	g := gridWith(world.SurfaceWater, nil, unitStub{{1, 1}: {stationary(assets.Bridge, 0)}})

	if got := g.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Reduced {
		t.Errorf("land unit on bridge = %d, want Reduced", got)
	}

	// A sea unit goes under the bridge: surface stays water, fully passable.
	if got := g.IsAccessible(seaUnit, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("sea unit under bridge = %d, want Clear", got)
	}
}

// TestBridgePlacement checks the buildable bridge: a Bridge is laid across open
// water OR the coast/shore band, so its placement land_type is water+coast -
// site valid on a bare water cell AND on a shore cell (so it can start at the
// shoreline), blocked on plain land. A water platform stays water only. (Once laid,
// land units cross it and sea units pass under it - TestBridgeCostByMoverClass.)
// C++: a bridge deploys on SURFACE_TYPE_WATER || SURFACE_TYPE_COAST (game_manager.cpp:4767).
func TestBridgePlacement(t *testing.T) {
	bridgeFlags := game.FlagStationary | game.FlagSelectable

	bridge := UnitDesc{
		Type:     assets.Bridge,
		Flags:    bridgeFlags,
		LandType: LandTypeForUnit(assets.Bridge, bridgeFlags),
	}

	water := gridWith(world.SurfaceWater, nil, nil)
	coast := gridWith(world.SurfaceCoast, nil, nil)
	land := gridWith(world.SurfaceLand, nil, nil)

	if got := water.IsAccessible(bridge, 0, 1, 1, SameClassBlocks); !got.Passable() {
		t.Errorf("bridge on open water = %d, want passable", got)
	}

	if got := coast.IsAccessible(bridge, 0, 1, 1, SameClassBlocks); !got.Passable() {
		t.Errorf("bridge on the coast/shore = %d, want passable (a bridge starts at the shoreline)", got)
	}

	if got := land.IsAccessible(bridge, 0, 1, 1, SameClassBlocks); got != Blocked {
		t.Errorf("bridge on plain land = %d, want Blocked (a bridge spans water/coast)", got)
	}

	// A water platform must NOT be placeable on the coast - water only.
	platFlags := game.FlagStationary | game.FlagSelectable
	plat := UnitDesc{Type: assets.WtrPltfm, Flags: platFlags, LandType: LandTypeForUnit(assets.WtrPltfm, platFlags)}
	if got := coast.IsAccessible(plat, 0, 1, 1, SameClassBlocks); got != Blocked {
		t.Errorf("water platform on the coast = %d, want Blocked (platforms are water-only)", got)
	}
}

func TestEnemyMineBlocks(t *testing.T) {
	// Mines remain passable because hidden enemies must enter their cell to trigger
	// detonation in sim.
	enemyMine := gridWith(world.SurfaceLand, nil, unitStub{{1, 1}: {stationary(assets.LandMine, 1)}})

	if got := enemyMine.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("enemy mine = %d, want Clear (mines no longer block; they detonate)", got)
	}

	ownMine := gridWith(world.SurfaceLand, nil, unitStub{{1, 1}: {stationary(assets.LandMine, 0)}})
	if got := ownMine.IsAccessible(landUnit, 0, 1, 1, NoModifiers); got != Clear {
		t.Errorf("own mine = %d, want Clear", got)
	}
}

func TestLandPresenceForBuildingFootprint(t *testing.T) {
	// (1,1) is water but its 2x2 building footprint reaches land at (2,1).
	cells := map[[2]int]uint8{
		{1, 1}: world.SurfaceWater,
		{2, 1}: world.SurfaceLand,
		{1, 2}: world.SurfaceWater,
		{2, 2}: world.SurfaceWater,
	}

	g := gridWith(world.SurfaceWater, cells, nil)
	if !g.IsAnyLandPresent(1, 1, true) {
		t.Error("building footprint touching land should report land present")
	}

	if g.IsFullyLandCovered(1, 1, true) {
		t.Error("partly-water footprint is not fully land covered")
	}

	// A single cell (not a building) query only looks at its own cell.
	if g.IsAnyLandPresent(1, 1, false) {
		t.Error("non-building water cell should report no land")
	}
}

// idleMover builds an idle, not stationary mobile unit (does not block).
func idleMover(t assets.UnitType, team int, flags game.Flags) *game.UnitInfo {
	return &game.UnitInfo{Type: t, Team: team, Flags: flags, Order: game.OrderIdle}
}
