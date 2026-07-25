// task_retreat.go : M.A.X. AI retreating damaged units from opponents
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// taskRetreat pulls a DAMAGED combat unit out of the fight: each turn it sends
// every armed mobile unit below the health threshold toward the nearest friendly
// repair shop that services its class (Depot land / Hangar air / Dock sea,
// Barracks for foot units), or - when none is reachable - flees it to the safest
// nearby cell (lowest threat, drifting toward home). A healthy unit is left to
// the attack task. This stops the computer throwing damaged units away. It does
// not reschedule itself; TaskManager queues it again each turn.
//
// Faithful but reduced: the original routes the damaged unit through TaskRepair
// (which also spawns a REPAIR operator, a rendezvous, and getting materials); this
// port only POSITIONS the unit at the shop - the depot SERVICING is the gameplay
// layer's repairshop. The flee is a reduced Task_RetreatFromDanger (nearest safe
// cell, no full caution level enum). The choice is decoupled from task scheduling:
// a damaged unit is claimed here and skipped by the attack/explore tasks.
//
// C++ ported : taskattackreserve.cpp - TaskAttackReserve::EndTurn (the
// hits < ATTRIB_HITS/2 -> TaskRepair branch) + taskrepair.cpp -
// TaskRepair::GetRepairShopType / SelectRepairShop + ai.cpp - Task_RetreatFromDanger.
type taskRetreat struct {
	TaskBase
	ctx *aiContext
}

// retreatEnabled gates retreat for all AI teams and provides a test-isolation knob.
var retreatEnabled = true

// retreatHitsNumerator / retreatHitsDenominator define "damaged": hits < HITS *
// num/den. The original uses HITS/2 (taskattackreserve.cpp:243). A named fraction
// keeps the threshold explicit.
const (
	retreatHitsNumerator   = 1
	retreatHitsDenominator = 2

	// retreatFleeRadius is how far a unit with no reachable repair shop looks for
	// a safer cell to fall back to. Small: a local step away from the firing line
	// toward home, not a flight across the whole map.
	retreatFleeRadius = 4
)

func newTaskRetreat(team int, ctx *aiContext) *taskRetreat {
	return &taskRetreat{
		TaskBase: newTaskBase(team, nil, 700), // below the economy roots, above attack (800)
		ctx:      ctx,
	}
}

// BeginTurn disengages each damaged unit toward a repair shop or to safety.
// C++: TaskAttackReserve::EndTurn's repair branch for each unit over the team's military.
func (t *taskRetreat) BeginTurn() {
	if !retreatEnabled {
		return
	}

	g := t.ctx.Game
	if g == nil {
		return
	}

	var tm *threatMap // built lazily for the first unit that has to flee
	for _, u := range g.Units.Units() {
		if u.Team != t.team || !unitDamaged(u) {
			continue
		}

		if g.HasOrder(u) || u.Speed <= 0 {
			continue // already moving / no budget left
		}

		// 1. Head for the nearest friendly repair shop that services this class.
		if shop := t.nearestRepairShop(u); shop != nil {
			if t.driveAdjacent(u, shop.GridX, shop.GridY) {
				dbg.Printf("ai: team %d %s damaged (%d/%d), retreating to %s at (%d,%d)",
					t.team, u.Type.Tag(), u.Hits, u.Values.Get(game.AttribHits),
					shop.Type.Tag(), shop.GridX, shop.GridY)
			}

			continue
		}

		// 2. No shop: fall back to the safest reachable nearby cell.
		if tm == nil {
			tm = buildThreatMap(g, t.team)
		}

		if x, y, ok := t.fleeToSafety(u, tm); ok && g.OrderMove(u, x, y) {
			dbg.Printf("ai: team %d %s damaged (%d/%d), fleeing to safety (%d,%d)",
				t.team, u.Type.Tag(), u.Hits, u.Values.Get(game.AttribHits), x, y)
		}
	}
}

// nearestRepairShop returns the nearest friendly building that services u's class,
// or nil if the team owns none. C++ ported : taskrepair.cpp -
// TaskRepair::GetRepairShopType + SelectRepairShop.
func (t *taskRetreat) nearestRepairShop(u *game.UnitInfo) *game.UnitInfo {
	shopType, ok := repairShopType(u)
	if !ok {
		return nil
	}

	g := t.ctx.Game
	var best *game.UnitInfo
	bestD := 1 << 30

	for _, b := range g.Units.Units() {
		if b.Team != t.team || b.Type != shopType {
			continue
		}

		if d := sim.SquaredDist(u.GridX, u.GridY, b.GridX, b.GridY); best == nil || d < bestD {
			best, bestD = b, d
		}
	}

	return best
}

// driveAdjacent orders u toward the nearest reachable cell adjacent to (tx, ty),
// returning whether a move was issued. A unit already adjacent holds (it is
// at/by the shop). C++: TaskRepair / TaskMove driving the unit to the shop.
func (t *taskRetreat) driveAdjacent(u *game.UnitInfo, tx, ty int) bool {
	g := t.ctx.Game

	if chebyshev(u.GridX, u.GridY, tx, ty) <= 1 {
		return false // already at the shop
	}

	for _, c := range g.AdjacentApproachCells(u, tx, ty) {
		if g.OrderMove(u, c.X, c.Y) {
			return true
		}
	}

	return false
}

// fleeToSafety picks the safest reachable cell within retreatFleeRadius of u: the
// lowest threat, with ties broken by the smallest distance to home (the team's
// nearest building), so a cornered unit drifts back toward base. It returns
// ok=false when the unit's own cell is already the safest available (hold rather
// than flee into danger that is equal or worse). C++ ported : ai.cpp -
// Task_RetreatFromDanger (reduced: nearest safe cell, no full CAUTION_LEVEL_* enum).
func (t *taskRetreat) fleeToSafety(u *game.UnitInfo, tm *threatMap) (int, int, bool) {
	g := t.ctx.Game
	home := t.nearestFriendlyBuilding(u)

	homeDist := func(x, y int) int {
		if home == nil {
			return 0
		}

		return sim.SquaredDist(x, y, home.GridX, home.GridY)
	}

	bestX, bestY := u.GridX, u.GridY
	bestThreat := tm.at(u.GridX, u.GridY)
	bestHome := homeDist(u.GridX, u.GridY)
	found := false
	zw := newZoneWalker(game.Point{X: u.GridX, Y: u.GridY}, retreatFleeRadius, g.W, g.H)

	for {
		x, y := zw.x, zw.y

		if (x != u.GridX || y != u.GridY) && g.CanEnter(u, x, y) {
			th := tm.at(x, y)
			hd := homeDist(x, y)

			if th < bestThreat || (th == bestThreat && hd < bestHome) {
				bestX, bestY, bestThreat, bestHome, found = x, y, th, hd, true
			}
		}

		if !zw.findNext() {
			break
		}
	}

	if found {
		return bestX, bestY, true
	}

	return 0, 0, false
}

// nearestFriendlyBuilding returns the team's nearest building to u, or nil.
func (t *taskRetreat) nearestFriendlyBuilding(u *game.UnitInfo) *game.UnitInfo {
	g := t.ctx.Game
	var best *game.UnitInfo
	bestD := 1 << 30

	for _, b := range g.Units.Units() {
		if b.Team != t.team || !b.Flags.Has(game.FlagBuilding) {
			continue
		}

		if d := sim.SquaredDist(u.GridX, u.GridY, b.GridX, b.GridY); best == nil || d < bestD {
			best, bestD = b, d
		}
	}

	return best
}

// repairShopType returns the building type that services u's movement class:
// Barracks for foot units, else Depot (land) / Hangar (air) / Dock (sea). The
// foot unit check comes first, matching the original. C++ ported : taskrepair.cpp -
// TaskRepair::GetRepairShopType.
func repairShopType(u *game.UnitInfo) (assets.UnitType, bool) {
	switch u.Type {
	case assets.Commando, assets.Infantry:
		return assets.Barracks, true
	}

	switch {
	case u.Flags.Has(game.FlagMobileLandUnit):
		return assets.Depot, true
	case u.Flags.Has(game.FlagMobileAirUnit):
		return assets.Hangar, true
	case u.Flags.Has(game.FlagMobileSeaUnit):
		return assets.Dock, true
	}

	return 0, false
}

// unitDamaged reports whether u is an armed mobile unit below the retreat health
// threshold (hits < HITS * num/den). Such a unit is claimed by taskRetreat and
// skipped by the attack/explore tasks, so it disengages instead of fighting or
// scouting. A building, constructor, or unarmed support unit is never "damaged"
// for retreat purposes. C++: taskattackreserve.cpp - the hits < ATTRIB_HITS/2 gate.
func unitDamaged(u *game.UnitInfo) bool {
	if u.Values == nil {
		return false
	}

	if !isMobileUnit(u.Flags) || u.Flags.Has(game.FlagConstructorUnit) {
		return false
	}

	if u.Values.Get(game.AttribAttack) <= 0 {
		return false // unarmed (scout / truck / surveyor): not a combat unit to retreat
	}

	maxHits := u.Values.Get(game.AttribHits)
	if maxHits <= 0 {
		return false
	}

	return u.Hits*retreatHitsDenominator < maxHits*retreatHitsNumerator
}

// The 8-neighbour approach search these tasks use is sim.Game.AdjacentApproachCells
// now (shared with sim combat and the attack task); the callers below invoke it
// directly on g.
