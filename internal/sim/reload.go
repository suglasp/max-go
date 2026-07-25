// reload.go : replenishes ammunition at depots and landing pads, landing air units first when required.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - OrderReload handling
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// IsReloader reports whether u is a mobile field reloader: the Supply Truck, which
// carries material in its cargo and refills an adjacent friendly unit's ammo. Depot
// and complex reload is the building side service (repairshop.go); this is the truck.
//
// C++ ported : units_manager.cpp - UnitsManager_Popup_InitReloaders (reloader = SUPPLY_TRUCK with storage)
func IsReloader(u *game.UnitInfo) bool {
	return u != nil && u.Type == assets.SplyTrck
}

// canService is the shared guard behind CanReload and CanRepair: server must be
// the given mobile service unit (isServer) with material left, and target a same
// team, alive, grounded, adjacent friendly whose serviced attribute (attrib,
// valued by currentOf against its maximum) is not already full. currentOf is a
// closure so target is read only after the nil guards.
//
// C++ ported : units_manager.cpp - the InitReloaders / InitRepair target filter
func (g *Game) canService(server, target *game.UnitInfo, isServer func(*game.UnitInfo) bool, attrib game.Attrib, currentOf func(*game.UnitInfo) int) bool {
	if server == nil || target == nil || server == target {
		return false
	}
	if !isServer(server) || server.Storage <= 0 {
		return false
	}
	if target.Team != server.Team || target.Hits <= 0 {
		return false
	}
	// A flying air unit cannot be serviced - it must land first (the server reaches
	// only the ground). A landed fighter clears FlagHovering, so this lets it through.
	// C++ ported : game_manager.cpp - GetUnitActionCursor (MOBILE_AIR_UNIT -> CURSOR_FRIEND).
	if targetInAir(target) {
		return false
	}
	if target.Values == nil {
		return false
	}
	max := target.Values.Get(attrib)
	if max <= 0 || currentOf(target) >= max {
		return false
	}
	return commandoAdjacent(server, target)
}

// CanReload reports whether supplier can reload target right now: supplier is a
// reloader with material left, target is a same team adjacent unit that has ammo
// capacity and is not already full. Mirrors the popup's reloadable target filter.
//
// C++ ported : units_manager.cpp - UnitsManager_Popup_InitReloaders / Access_GetReloadTarget
func (g *Game) CanReload(supplier, target *game.UnitInfo) bool {
	return g.canService(supplier, target, IsReloader, game.AttribAmmo, func(u *game.UnitInfo) int { return u.Ammo })
}

// Reload spends one of the supplier's stored materials to fully refill the target's
// ammo, returning true if it happened. One material reloads the target completely.
//
// C++ ported : unitinfo.cpp - UnitInfo::Reload
func (g *Game) Reload(supplier, target *game.UnitInfo) bool {
	if !g.CanReload(supplier, target) {
		return false
	}
	supplier.Storage--
	target.Ammo = target.Values.Get(game.AttribAmmo)
	return true
}

// reloadTargetAt returns the topmost unit at (cx, cy) that supplier can reload, or
// nil. Mirrors topTarget but keeps the same team (friendly) reload filter.
func (g *Game) reloadTargetAt(supplier *game.UnitInfo, cx, cy int) *game.UnitInfo {
	for _, u := range g.Units.Cells.At(cx, cy) {
		if g.CanReload(supplier, u) {
			return u
		}
	}
	return nil
}

// CanReloadAt reports whether supplier can reload a unit standing at (cx, cy). Used
// by the gamescreen to light the PTR_RLD cursor over a valid cell.
func (g *Game) CanReloadAt(supplier *game.UnitInfo, cx, cy int) bool {
	return g.reloadTargetAt(supplier, cx, cy) != nil
}

// ReloadAt reloads the reloadable friendly at (cx, cy), returning true if it happened.
func (g *Game) ReloadAt(supplier *game.UnitInfo, cx, cy int) bool {
	target := g.reloadTargetAt(supplier, cx, cy)
	if target == nil {
		return false
	}
	return g.Reload(supplier, target)
}
