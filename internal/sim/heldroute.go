// heldroute.go : the route a unit keeps when it halts on spotting an enemy.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
)

// A unit that spots an enemy stops and holds the remainder of its route. Its arrows remain visible until the player continues it or issues a new order.
// A held route is not pending: pending routes resume at the next team turn, whereas only the player may resume a held route.

// HaltForEnemy stops a unit that spotted an enemy and stores the remainder of its route until the player continues it or issues a new order.
func (g *Game) HaltForEnemy(u *game.UnitInfo) {
	if u == nil {
		return
	}

	// What is left of the drive: the active order's remainder, or an already shelved
	// route if the unit was between turns when it spotted something.
	steps := append([]paths.Step(nil), g.remainingRoute(u)...)

	g.cancel(u)
	delete(g.pending, u) // it must not auto resume: the player decides
	u.AttackTarget = nil // and it is no longer chasing anything

	if len(steps) == 0 {
		return // nowhere left to go: nothing worth holding
	}

	if g.held == nil {
		g.held = make(map[*game.UnitInfo][]paths.Step)
	}

	g.held[u] = steps

	dest := steps[len(steps)-1].To
	dbg.Printf("halt: %s at (%d,%d) holds its route to (%d,%d)", u.Type.Tag(), u.GridX, u.GridY, dest.X, dest.Y)
}

// remainingRoute is what a unit still has to drive: the live order's remainder, else its
// shelved (over budget) route.
func (g *Game) remainingRoute(u *game.UnitInfo) []paths.Step {
	for _, o := range g.orders {
		if o.Unit == u {
			if rest := o.RemainingSteps(); len(rest) > 0 {
				return rest
			}

			break
		}
	}

	return g.pending[u]
}

// HeldRoute is the route a halted unit is still holding, or nil.
func (g *Game) HeldRoute(u *game.UnitInfo) []paths.Step {
	if u == nil {
		return nil
	}

	return g.held[u]
}

// HeldRoutes lists every held route, for the path arrows and the save.
func (g *Game) HeldRoutes() []PendingRoute {
	if len(g.held) == 0 {
		return nil
	}

	out := make([]PendingRoute, 0, len(g.held))
	for u, steps := range g.held {
		out = append(out, PendingRoute{Unit: u, Steps: steps})
	}

	return out
}

// ClearHeldRoute forgets a unit's held route. Any fresh order supersedes it.
func (g *Game) ClearHeldRoute(u *game.UnitInfo) {
	delete(g.held, u)
}

// ContinueHeldRoute drives a halted unit toward its stored destination, finding a new path from its current position. It returns false and clears the
// held route when no route remains or the destination is unreachable.
func (g *Game) ContinueHeldRoute(u *game.UnitInfo) bool {
	steps := g.held[u]
	if len(steps) == 0 {
		return false
	}

	dest := steps[len(steps)-1].To
	delete(g.held, u) // OrderMove lays a fresh, current route

	return g.OrderMove(u, dest.X, dest.Y)
}

// SetHeldDestination restores a held route as one step to its destination. Continuing finds a new path from the unit's current position.
func (g *Game) SetHeldDestination(u *game.UnitInfo, destX, destY int) {
	if u == nil {
		return
	}

	if g.held == nil {
		g.held = make(map[*game.UnitInfo][]paths.Step)
	}

	g.held[u] = []paths.Step{{To: game.Point{X: destX, Y: destY}}}
}
