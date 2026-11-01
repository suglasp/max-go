// transport.go : loads and unloads units from transporters and carriers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - OrderLoad / OrderUnload execution
package sim

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/game"
)

// Personnel, sea, and air carriers hold passengers off the map in UnitInfo.Carried.
// LoadUnit boards an adjacent friendly unit into a free slot; UnloadUnit places it
// back on an adjacent free, passable cell with the grow in animation (the screen
// side plays the matching shrink on boarding).
// C++: UnitsManager_ProcessOrderLoad/Unload + UnitInfo::ProcessLoading/
// ProcessUnloading (units_manager.cpp / unitinfo.cpp).

// IsTransport reports whether a unit type ferries other units (the
// CLNTRANS/SEATRANS/AIRTRANS popup callback-11 mapping in unitinfo.cpp).
func IsTransport(t assets.UnitType) bool {
	switch t {
	case assets.ClnTrans, assets.SeaTrans, assets.AirTrans:
		return true
	}
	return false
}

// TransportCapacity is how many units a transport can carry; its ATTRIB_STORAGE
// slots. 0 for a non transport or one without values.
func TransportCapacity(u *game.UnitInfo) int {
	if u == nil || !IsTransport(u.Type) {
		return 0
	}
	return ContainerCapacity(u)
}

// IsStorageBuilding reports whether a building can STORE mobile units inside it
// (the same Carried bay a transport uses): a Depot (land vehicles), Hangar (air),
// Dock (sea) or Barracks (infantry). C++: the DEPOT/HANGAR/DOCK repair shop
// buildings that hold units (Access_IsHeldByUnit / the stored unit bay).
func IsStorageBuilding(t assets.UnitType) bool {
	switch t {
	case assets.Depot, assets.Hangar, assets.Dock, assets.Barracks:
		return true
	}
	return false
}

// isContainer reports whether a unit can hold other units inside it: a mobile
// transport, or a storage building.
func isContainer(u *game.UnitInfo) bool {
	return u != nil && (IsTransport(u.Type) || IsStorageBuilding(u.Type))
}

// ContainerCapacity is how many units a transport or storage building can hold
// (its ATTRIB_STORAGE slots); 0 for anything else.
func ContainerCapacity(u *game.UnitInfo) int {
	if u == nil || u.Values == nil || !isContainer(u) {
		return 0
	}
	return u.Values.Get(game.AttribStorage)
}

// containerAccepts reports whether passenger is a valid occupant for container -
// klei's receiver table (Access_GetReceiverUnit, access.cpp:2097-2124):
//
//	APC (ClnTrans) + Barracks : COMMANDO / INFANTRY only
//	Sea Transport             : any mobile LAND unit (infantry AND vehicles)
//	Depot                     : land VEHICLES only (no commando/infantry)
//	Dock                      : sea AND NOT land (an amphibious unit cannot dock)
//	Hangar                    : air units
//	Air Transport             : any mobile LAND unit (its own pick up flow)
//
// The ONLY sanctioned nesting is klei's: a LAND transport (the APC) may enter a
// Depot or a Sea Transport (access.cpp:2105-2109 treats CLNTRANS as an ordinary
// non infantry land unit there) - a sea or air transport is never cargo. The
// passenger must be mobile, selectable, not a building, not already aboard.
func containerAccepts(container, passenger *game.UnitInfo) bool {
	if passenger.Parent != nil || passenger.Flags.Has(game.FlagStationary) || !passenger.Flags.Has(game.FlagSelectable) {
		return false
	}

	if IsTransport(passenger.Type) {
		if passenger.Type != assets.ClnTrans {
			return false // a sea/air transport never boards anything
		}
		if container.Type != assets.Depot && container.Type != assets.SeaTrans {
			return false // the APC rides only a Depot or a Sea Transport (klei)
		}
	}

	infantry := passenger.Type == assets.Commando || passenger.Type == assets.Infantry
	land := passenger.Flags.Has(game.FlagMobileLandUnit)

	switch container.Type {
	case assets.Hangar:
		return passenger.Flags.Has(game.FlagMobileAirUnit)
	case assets.Dock:
		return passenger.Flags.Has(game.FlagMobileSeaUnit) && !land
	case assets.ClnTrans, assets.Barracks:
		return land && infantry
	case assets.Depot:
		return land && !infantry
	case assets.SeaTrans, assets.AirTrans:
		return land
	}
	return false
}

// adjacentToContainer reports whether (px, py) touches the container's footprint
// (a transport is 1x1; a storage building 2x2). C++: Access_GetReceiverUnit's
// adjacency over the footprint.
func (g *Game) adjacentToContainer(container *game.UnitInfo, px, py int) bool {
	fp := build.Footprint(container.Flags)
	inside := px >= container.GridX && px < container.GridX+fp &&
		py >= container.GridY && py < container.GridY+fp
	return !inside &&
		axisGap(px, container.GridX, container.GridX+fp-1) <= 1 &&
		axisGap(py, container.GridY, container.GridY+fp-1) <= 1
}

// CanLoad reports whether passenger can board container (a transport OR a storage
// building) with a free slot: a same team, mobile, class matching unit (not a
// building/transport, not already aboard) adjacent to the container's footprint.
// C++: the enter_mode boarding check via Access_GetReceiverUnit (access.cpp).
func (g *Game) CanLoad(container, passenger *game.UnitInfo) bool {
	if !g.CanContain(container, passenger) {
		return false
	}
	return g.adjacentToContainer(container, passenger.GridX, passenger.GridY)
}

// CanContain reports whether passenger is a valid occupant for container by class +
// free capacity + same team, but WITHOUT the adjacency requirement - so a "Load"
// command can pick a unit anywhere and path it to the container first. C++: the load
// eligibility before Access_GetReceiverUnit's adjacency check.
func (g *Game) CanContain(container, passenger *game.UnitInfo) bool {
	if container == nil || passenger == nil || container == passenger {
		return false
	}
	if !isContainer(container) || container.Team != passenger.Team {
		return false
	}
	// klei's receiver gate: an ORDER_IDLE container takes nothing (access.cpp:2097).
	if container.Order == game.OrderIdle {
		return false
	}
	// klei refuses to load DISABLED units (_(9897), game_manager.cpp:6719-6720).
	if g.IsDisabled(passenger) {
		return false
	}
	// A builder raising a building or parked on its finished one (ORDER_BUILD), and a bulldozer clearing rubble
	// (ORDER_CLEAR), cannot be picked up. The Load action selects these units instead of loading them.
	// Source: game_manager.cpp - GameManager_GetCursor and Task_IsReadyToTakeOrders.
	if passenger.Order == game.OrderBuild || passenger.Order == game.OrderClear {
		return false
	}
	if len(container.Carried) >= ContainerCapacity(container) {
		return false
	}
	return containerAccepts(container, passenger)
}

// AdjacentToContainer reports whether passenger currently touches container's
// footprint (so it can be sucked in). Exported wrapper of adjacentToContainer.
func (g *Game) AdjacentToContainer(container, passenger *game.UnitInfo) bool {
	if container == nil || passenger == nil {
		return false
	}
	return g.adjacentToContainer(container, passenger.GridX, passenger.GridY)
}

// LoadCell finds a free passable cell adjacent to container's footprint that
// passenger can stand on, so a distant unit can path to the container before being
// loaded. ok=false when the footprint is boxed in. C++: Access_FindReachableSpot.
func (g *Game) LoadCell(container, passenger *game.UnitInfo) (int, int, bool) {
	if container == nil || passenger == nil {
		return 0, 0, false
	}
	fp := build.Footprint(container.Flags)
	for y := container.GridY - 1; y <= container.GridY+fp; y++ {
		for x := container.GridX - 1; x <= container.GridX+fp; x++ {
			inside := x >= container.GridX && x < container.GridX+fp &&
				y >= container.GridY && y < container.GridY+fp
			if inside {
				continue
			}
			if g.cellFreeFor(passenger, x, y) {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// LoadUnit boards passenger into container (a transport or storage building): it
// leaves the map and fills a slot. Returns false when CanLoad does not hold.
func (g *Game) LoadUnit(container, passenger *game.UnitInfo) bool {
	if !g.CanLoad(container, passenger) {
		return false
	}
	g.cancel(passenger)
	delete(g.pending, passenger)
	/* // Loading a builder at work is refused, so no loaded unit is parked
	g.releaseParkedBuilding(passenger) // a builder still parked on its finished building lets it unfold
	*/
	g.Units.Remove(passenger)
	passenger.Parent = container
	container.Carried = append(container.Carried, passenger)
	g.RefreshVisibility()
	return true
}

// UnloadUnit drops a carried passenger onto (cx, cy); a free, passable cell
// adjacent to the transport. The unloaded unit keeps its remaining move budget
// (C++: ProcessUnloading leaves the client at its turn-refreshed speed).
// Returns false when the unit is not aboard or the cell is invalid.
func (g *Game) UnloadUnit(transport, passenger *game.UnitInfo, cx, cy int) bool {
	idx := -1
	for i, c := range transport.Carried {
		if c == passenger {
			idx = i
			break
		}
	}
	if idx < 0 || !adjacent(transport.GridX, transport.GridY, cx, cy) || !g.cellFreeFor(passenger, cx, cy) {
		return false
	}
	transport.Carried = append(transport.Carried[:idx], transport.Carried[idx+1:]...)
	passenger.Parent = nil
	g.Units.Deploy(passenger, cx, cy)
	passenger.Unfold = 4                                   // start collapsed; tickBuildExpansions grows it to full size
	passenger.HiddenUntilExpand = false                    // drawn (scaling up) from the moment it appears
	g.expandReleased = append(g.expandReleased, passenger) // the grow in animation + expand sound (klei ScaleUnit EXPAND)
	g.RefreshVisibility()
	g.detonateMines() // a unit placed onto a mined cell triggers it AT placement (klei units_manager.cpp:3831)
	return true
}

// DropUnloadedUnit drops a carried passenger onto the transport's OWN cell - the
// ground directly beneath an air transport - and starts its grow in (Unfold 4 -> 0),
// instead of the adjacent cell drop (UnloadUnit) or the build ready marker exit
// (EjectStoredUnit). The unit keeps its remaining move budget (C++: ProgressUnloading
// leaves the client at its turn-refreshed speed). Returns false when the passenger is
// not aboard or the ground beneath cannot take it (CanUnloadBeneath).
// C++ ported : units_manager.cpp - UnitsManager_ProgressUnloading (the client is placed
// at the transport's grid_x/grid_y and UnitsManager_ScaleUnit(ORDER_STATE_EXPAND) grows it).
func (g *Game) DropUnloadedUnit(transport, passenger *game.UnitInfo) bool {
	idx := -1
	for i, c := range transport.Carried {
		if c == passenger {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false // not aboard
	}
	if !g.CanUnloadBeneath(transport, passenger) {
		return false // blocked cell or terrain the passenger cannot stand on
	}
	transport.Carried = append(transport.Carried[:idx], transport.Carried[idx+1:]...)
	passenger.Parent = nil
	g.Units.Deploy(passenger, transport.GridX, transport.GridY)
	passenger.Unfold = 4                                   // start collapsed; tickBuildExpansions grows it to full size
	passenger.HiddenUntilExpand = false                    // drawn (scaling up) from the moment it appears
	g.expandReleased = append(g.expandReleased, passenger) // queue the grow in animation + expand sound
	g.RefreshVisibility()
	g.detonateMines() // a unit dropped onto a mined cell triggers it AT placement (klei units_manager.cpp:3831)
	return true
}

// PickUpUnit boards passenger into an AIR transport that has descended directly
// ONTO the unit's cell - the mirror of DropUnloadedUnit. It gates on CanContain
// (class + free capacity + same team) but NOT adjacency, because the carrier sits
// on the same cell, not next to it (LoadUnit's CanLoad neighbour check would fail).
// Returns false when the unit cannot be carried. C++ ported : units_manager.cpp -
// UnitsManager_ProgressLoading (the client is removed at the transport's grid cell).
func (g *Game) PickUpUnit(transport, passenger *game.UnitInfo) bool {
	if !g.CanContain(transport, passenger) {
		return false
	}
	// klei requires a SETTLED target for the same cell pick up (ORDER_STATE_EXECUTING_ORDER,
	// game_manager.cpp:6726): a unit still moving or mid transition is not grabbed. The
	// port keys on the transitional states rather than the exact settled constant, since
	// a freshly spawned parked unit can still sit at the zero Init state.
	if g.HasOrder(passenger) {
		return false
	}
	switch passenger.State {
	case game.OrderStateInProgress, game.OrderStateInTransition, game.OrderStateIssuingPath,
		game.OrderStatePrepareStore, game.OrderStateStore:
		return false
	}
	g.cancel(passenger)
	delete(g.pending, passenger)
	g.Units.Remove(passenger)
	passenger.Parent = transport
	transport.Carried = append(transport.Carried, passenger)
	g.RefreshVisibility()
	return true
}

// CanUnloadBeneath reports whether the ground directly beneath an air transport
// can take the passenger: in bounds, terrain the passenger's move class can stand
// on (an air transport hovers anywhere, but a land unit dropped on water or rock
// would be stuck forever), and no blocking unit on the cell (the transport itself,
// mid touch down, is ignored). The Activate flow asks it BEFORE the descent so an
// impossible drop is refused up front. C++ ported : repairshopmenu.cpp -
// Access_IsAccessible gates ORDER_UNLOAD, refusal message _(eeb4).
func (g *Game) CanUnloadBeneath(transport, passenger *game.UnitInfo) bool {
	x, y := transport.GridX, transport.GridY
	if !g.Grid.InBounds(x, y) {
		return false
	}
	// Terrain only (modifier 0): SameClassBlocks would count the TRANSPORT itself
	// as a blocker once it touches down (a landed air unit blocks ground movers),
	// so unit blocking is done by the explicit sweep below, which can ignore it.
	d := access.UnitDesc{Type: passenger.Type, Flags: passenger.Flags, LandType: access.LandTypeForUnit(passenger.Type, passenger.Flags)}
	if g.Grid.IsAccessible(d, passenger.Team, x, y, 0) <= 0 {
		return false
	}
	for _, e := range g.Units.Cells.At(x, y) {
		if e != transport && !e.Flags.Has(game.FlagGroundCover) {
			return false
		}
	}
	return true
}

// UnloadSpot finds a free, passable cell adjacent to the transport for the
// passenger, ok=false when the transport is boxed in. C++: Access_FindReachableSpot.
func (g *Game) UnloadSpot(transport, passenger *game.UnitInfo) (int, int, bool) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			x, y := transport.GridX+dx, transport.GridY+dy
			if g.cellFreeFor(passenger, x, y) {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// cellFreeFor reports whether (x, y) is in bounds, passable for u's move type,
// and not occupied by a blocking (non ground cover) unit.
func (g *Game) cellFreeFor(u *game.UnitInfo, x, y int) bool {
	if !g.Grid.InBounds(x, y) {
		return false
	}
	d := access.UnitDesc{Type: u.Type, Flags: u.Flags, LandType: access.LandTypeForUnit(u.Type, u.Flags)}
	if g.Grid.IsAccessible(d, u.Team, x, y, access.SameClassBlocks) <= 0 {
		return false
	}
	for _, e := range g.Units.Cells.At(x, y) {
		if !e.Flags.Has(game.FlagGroundCover) {
			return false
		}
	}
	return true
}

// adjacent reports whether (bx, by) is one of the eight cells around (ax, ay).
func adjacent(ax, ay, bx, by int) bool {
	dx, dy := bx-ax, by-ay
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx <= 1 && dy <= 1 && (dx != 0 || dy != 0)
}
