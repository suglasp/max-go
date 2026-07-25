// movement.go : issues, ticks and resolves the unit move orders.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : game_manager.cpp - movement resolution
package sim

import (
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
	"github.com/suglasp/max-go/internal/units"
)

// OrderMove routes u to (cx, cy) and, if a path exists, starts a MoveOrder (replacing
// any prior order). Reports whether a route was found. A player move also drops any
// running Path build so its continuation stops dragging the unit back along the old
// line; the path's own cell to cell moves go through orderMoveTo (which keeps it).
// C++: ORDER_MOVE; a new order calls ClearBuildListAndPath (game_manager.cpp).
func (g *Game) OrderMove(u *game.UnitInfo, cx, cy int) bool {
	delete(g.buildPaths, u) // a fresh manual move cancels any Path build
	delete(g.held, u)       // and supersedes a route held by an enemy halt
	u.AttackTarget = nil    // and cancels any move to attack pursuit (orderMoveTo preserves it)
	return g.orderMoveTo(u, cx, cy)
}

// orderMoveTo is the core move: route u to (cx, cy) and start a MoveOrder, without
// touching any Path build (so tickBuildPaths can drive a path cell to cell).
func (g *Game) orderMoveTo(u *game.UnitInfo, cx, cy int) bool {
	return g.orderMoveToAvoiding(u, cx, cy, nil)
}

// OrderMoveAvoiding is OrderMove with a per cell danger penalty that bends the route
// AWAY from high penalty ground (cells under enemy fire) while still reaching (cx, cy).
// The AI passes its threat map so a combat unit skirts a defended approach. A nil
// penalty equals OrderMove; like it, supersedes a Path build and clears pursuit.
func (g *Game) OrderMoveAvoiding(u *game.UnitInfo, cx, cy int, penalty func(x, y int) int) bool {
	delete(g.buildPaths, u) // a fresh move cancels any Path build
	u.AttackTarget = nil    // and any move to attack pursuit
	return g.orderMoveToAvoiding(u, cx, cy, penalty)
}

// orderMoveToAvoiding is orderMoveTo with an optional route choice penalty; a nil
// penalty gives the plain terrain route. It leaves any Path build untouched.
func (g *Game) orderMoveToAvoiding(u *game.UnitInfo, cx, cy int, penalty func(x, y int) int) bool {
	if g.IsDisabled(u) {
		return false // a disabled unit cannot move
	}
	if g.IsClearingRubble(u) {
		return false // a bulldozer busy clearing rubble cannot move
	}
	if IsAirUnit(u) && !IsHovering(u) {
		g.TakeOff(u) // a landed air unit rises on its own when ordered to move (auto take off)
	}
	g.cancel(u)
	delete(g.pending, u) // a fresh explicit order supersedes any queued route
	steps, cost, ok := g.findRouteAvoiding(u, cx, cy, penalty)
	if !ok {
		return false
	}
	g.orders = append(g.orders, units.NewMoveOrder(g.Units, u, steps, cost, g.Fast))
	return true
}

// QueueMove plans a route for u to (cx, cy) and shelves it as a pending route WITHOUT
// advancing this turn (the 0-movement case): the unit keeps its GroundPath (drawn as
// the green/blue next turn arrows) and resumePending drives it once the budget refills.
// Returns false (queuing nothing) if unreachable.
// C++ ported : a 0-speed unit still shows its GroundPath and defers to next turn.
func (g *Game) QueueMove(u *game.UnitInfo, cx, cy int) bool {
	// A disabled or rubble clearing unit cannot take a route, even a queued one.
	if g.IsDisabled(u) || g.IsClearingRubble(u) {
		return false
	}
	steps, _, ok := g.findRoute(u, cx, cy)
	if !ok {
		return false // unreachable: queue nothing so the caller can report no path
	}
	g.cancel(u)             // drop any active order; the unit is not moving this turn
	delete(g.buildPaths, u) // a fresh manual route supersedes a Path build
	u.AttackTarget = nil    // and any move to attack pursuit
	// Mirror the mid path pending state: the unit holds OrderMove with its route
	// shelved (see the move state machine), so it reads as en route and resumes.
	u.SetOrder(game.OrderMove)
	u.SetOrderState(game.OrderStateInProgress)
	if g.pending == nil {
		g.pending = make(map[*game.UnitInfo][]paths.Step)
	}
	g.pending[u] = steps
	return true
}

// cancel drops any active order driving u. If the order had already claimed
// its next cell (a reservation mid-slide), that claim is released first so a
// unit interrupted before it arrives never leaves a phantom block behind.
func (g *Game) cancel(u *game.UnitInfo) {
	kept := g.orders[:0]
	for _, o := range g.orders {
		if o.Unit != u {
			kept = append(kept, o)
		} else {
			o.ReleaseReservation()
		}
	}
	g.orders = kept
}

// CancelOrder stops a unit's active move order, if any; the popup "Stop"
// action. It also drops any queued over budget route so the unit will not
// resume next turn. C++: UnitsManager_Popup_OnClick_Stop (units_manager.cpp).
func (g *Game) CancelOrder(u *game.UnitInfo) {
	g.cancel(u)
	delete(g.pending, u)
	delete(g.held, u)    // an explicit Stop means the player is done with that route
	u.AttackTarget = nil // an explicit Stop also ends a move to attack pursuit
}

// HasOrder reports whether a unit currently has an active move order.
func (g *Game) HasOrder(u *game.UnitInfo) bool {
	for _, o := range g.orders {
		if o.Unit == u {
			return true
		}
	}
	return false
}

// Tick advances every active move order one animation frame and drops the
// finished/stalled ones. It returns the number of orders still active; the
// in game screen calls this once per frame.
func (g *Game) Tick() int {
	kept := g.orders[:0]
	var settled []*game.UnitInfo // movers that reached a new cell this tick -> draw reaction fire
	for _, o := range g.orders {
		switch o.Tick() {
		case units.MoveDone:
			delete(g.pending, o.Unit) // route finished; no queued remainder
			if o.Unit.AttackTarget != nil {
				// A move to attack unit reached its stand off cell: fire if in range,
				// else continue the chase toward the target's current position.
				g.advancePursuit(o.Unit)
			}
			// A unit that drove in to shoot takes its shot NOW, as it parks, so the attack
			// reads as the end of that drive instead of arriving before it (the AI).
			g.fireArrivalShot(o.Unit)
			settled = append(settled, o.Unit)
		case units.MoveBlocked:
			// Paused only by the movement budget (Speed spent, next cell passable):
			// queue the remainder to resume next turn. A genuine block is dropped.
			if o.Unit.AttackTarget != nil {
				// A move to attack approach stalled out of budget: resumePursuit re paths
				// next turn, so do not queue a plain route (OrderMove would clear pursuit).
				delete(g.pending, o.Unit)
			} else if o.Unit.Speed == 0 && o.NextCellPassable() {
				if g.pending == nil {
					g.pending = make(map[*game.UnitInfo][]paths.Step)
				}
				g.pending[o.Unit] = append([]paths.Step(nil), o.RemainingSteps()...)
			} else {
				delete(g.pending, o.Unit)
			}
			settled = append(settled, o.Unit)
		default:
			kept = append(kept, o)
		}
	}
	g.orders = kept
	// Reaction (opportunity) fire: every enemy that can see a mover which just
	// settled on a new cell takes a shot at it - so a base's defensive turrets and
	// parked units auto defend. Run AFTER the order loop closes: a reaction kill
	// rewrites g.orders (g.cancel / DestroyUnit), which must not happen mid range.
	for _, m := range settled {
		g.checkReactions(m)
	}
	// A Path build engineer that just reached its next cell starts that cell's build
	// THIS turn (vanilla lays a path cell the same turn the engineer arrives on it -
	// move and build in one turn), instead of idling until next turn's tickBuildPaths.
	for _, m := range settled {
		g.resumePathBuildOnArrival(m)
	}
	g.updateExpansions()   // release a building to unfold once its builder has cleared it
	g.sweepFactoryReady()  // free a factory once its parked output unit has driven off
	g.detonateMines()      // explode a mine an enemy has just driven onto
	g.advanceProjectiles() // fly missiles toward their target; resolve on arrival
	return len(g.orders)
}

// ResolveMovement fast forwards every active move order to completion (or to
// a stall), the headless equivalent of letting the animation play out. It is
// bounded against a runaway order.
func (g *Game) ResolveMovement() {
	// Keep ticking while move orders OR missiles in flight remain, so a headless
	// run (tests, the AI fast path) also flies projectiles to impact.
	for i := 0; i < 1_000_000 && (g.Tick() > 0 || len(g.projectiles) > 0); i++ {
	}
	g.RefreshVisibility()   // recompute fog from the final positions (queues any reveal edges)
	g.FireRevealReactions() // a defender fires on an enemy this move just revealed in its range
}

// ActiveOrders reports how many move orders are still running.
func (g *Game) ActiveOrders() int {
	return len(g.orders)
}

// MoveOrders returns the active move orders for read only inspection (the
// in game screen draws their remaining steps as a path preview). Do not
// retain the slice across a Tick.
func (g *Game) MoveOrders() []*units.MoveOrder {
	return g.orders
}

// PendingRoutes returns the queued (over budget) routes, units that ran out
// of movement points mid path. The in game screen draws these as the green/blue
// destination arrows. The order is unspecified.
func (g *Game) PendingRoutes() []PendingRoute {
	if len(g.pending) == 0 {
		return nil
	}
	out := make([]PendingRoute, 0, len(g.pending))
	for u, steps := range g.pending {
		out = append(out, PendingRoute{Unit: u, Steps: steps})
	}
	return out
}

// HasPendingRoute reports whether u has a queued (over budget) route waiting to resume
// next turn. Distinct from an active move (MoveOrders): a paused route is NOT moving
// this frame but drives on at the next BeginTeamTurn, so callers waiting out a unit's
// travel must treat it as "still on its way", not "arrived/blocked".
func (g *Game) HasPendingRoute(u *game.UnitInfo) bool {
	if u == nil || g.pending == nil {
		return false
	}
	_, queued := g.pending[u]
	return queued
}

// SetPendingDestination queues a one step resumable route for u to (destX, destY) so
// the next BeginTeamTurn re pathfinds and drives it on. Used by save restore to re
// establish an over budget move: only the destination matters (resumePending re paths
// to the route's final cell). C++: the unit keeps its GroundPath across turns.
func (g *Game) SetPendingDestination(u *game.UnitInfo, destX, destY int) {
	if u == nil {
		return
	}
	if g.pending == nil {
		g.pending = make(map[*game.UnitInfo][]paths.Step)
	}
	g.pending[u] = []paths.Step{{To: game.Point{X: destX, Y: destY}}}
}
