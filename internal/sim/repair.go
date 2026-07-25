// repair.go : lets mobile Repair units restore adjacent friendly units.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : units_manager.cpp - UnitsManager_Popup_InitRepair / ProcessOrderRepair (the mobile Repair unit)
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// IsRepairer reports whether u is the mobile Repair unit: it carries material in its
// cargo and mends an adjacent friendly unit's hit points. The Depot/Hangar/Dock
// repair screen is the building side service (repairshop.go); this is the vehicle.
// It mirrors IsReloader (the Supply Truck) for hits instead of ammo.
func IsRepairer(u *game.UnitInfo) bool {
	return u != nil && u.Type == assets.Repair
}

// CanRepair reports whether repairer can mend target right now: repairer is a Repair
// unit with material left, target is a same team adjacent unit that is damaged (hits
// below its maximum) and not currently flying. Mirrors CanReload.
//
// C++ ported : units_manager.cpp - the InitRepair reloadable target filter
func (g *Game) CanRepair(repairer, target *game.UnitInfo) bool {
	return g.canService(repairer, target, IsRepairer, game.AttribHits, func(u *game.UnitInfo) int { return u.Hits })
}

// Repair spends the repairer's stored material to mend the target's hit points, one
// material per four hits, bounded by the damage and the material carried. Returns
// true if it happened. Mirrors Reload (same heal rate as the repair shop repairHeal).
//
// C++ ported : unitinfo.cpp - UnitInfo::ProcessRepair
func (g *Game) Repair(repairer, target *game.UnitInfo) bool {
	if !g.CanRepair(repairer, target) {
		return false
	}
	maxHits := target.Values.Get(game.AttribHits)
	// Restore up to four hits per material, capped by the missing hits.
	gain := maxHits - target.Hits
	if gain > repairer.Storage*4 {
		gain = repairer.Storage * 4
	}
	cost := (gain + 3) / 4 // one material per four hits (rounded up)
	target.Hits += gain
	repairer.Storage -= cost
	return true
}

// repairTargetAt returns the topmost unit at (cx, cy) that repairer can mend, or nil.
// Mirrors reloadTargetAt.
func (g *Game) repairTargetAt(repairer *game.UnitInfo, cx, cy int) *game.UnitInfo {
	for _, u := range g.Units.Cells.At(cx, cy) {
		if g.CanRepair(repairer, u) {
			return u
		}
	}
	return nil
}

// CanRepairAt reports whether repairer can mend a unit standing at (cx, cy). Used by
// the gamescreen to light the repair (FRND_FIX) cursor over a valid cell.
func (g *Game) CanRepairAt(repairer *game.UnitInfo, cx, cy int) bool {
	return g.repairTargetAt(repairer, cx, cy) != nil
}

// RepairAt mends the repairable friendly at (cx, cy), returning true if it happened.
func (g *Game) RepairAt(repairer *game.UnitInfo, cx, cy int) bool {
	target := g.repairTargetAt(repairer, cx, cy)
	if target == nil {
		return false
	}
	return g.Repair(repairer, target)
}
