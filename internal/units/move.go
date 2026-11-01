// move.go : Order_Move state machine that drives a unit along a pathfinder route one tick at a time.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - UnitInfo::Move; paths.cpp - Paths_CalculateStep
package units

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
)

// tileSize is the map tile edge in pixels (GFX_MAP_TILE_SIZE); a unit fully
// enters a cell after advancing this many pixels.
const tileSize = 64

// MoveStatus reports what a MoveOrder.Tick did this frame.
type MoveStatus int

const (
	// MoveTurning: the unit rotated one step toward its next heading and
	// has not moved yet.
	MoveTurning MoveStatus = iota
	// MoveDriving: the unit advanced (and may have crossed into the next
	// cell) this tick.
	MoveDriving
	// MoveBlocked: the unit cannot continue, out of speed for this turn,
	// or the next cell became impassable. The order pauses; resume next
	// turn after ResetMovement, or find a new path.
	MoveBlocked
	// MoveDone: the unit reached the end of the path.
	MoveDone
	// MoveWaiting: the next cell is held by another unit's in-transit
	// reservation (a chokepoint traffic jam, not impassable terrain) - the
	// unit stands fast this tick and retries, spending no movement budget.
	// Handled identically to MoveTurning by the caller (order stays active).
	MoveWaiting
)

// maxWaitTicks caps how long a unit will stand fast for a contested cell to
// clear before falling back to the ordinary MoveBlocked handling - a bounded
// wait so two units queued nose to nose can never deadlock the game forever.
const maxWaitTicks = 90

// MoveOrder is an ORDER_MOVE in progress: a unit, the route it is following
// (the pathfinder's Steps), the index of the next step to take, and the cost
// to enter each cell used to debit movement points. It owns no goroutine;
// the caller advances it with Tick each frame.
//
// C++ ported : unitinfo.cpp - ORDER_MOVE driven by UnitInfo::Move over a GroundPath
type MoveOrder struct {
	Mgr   *Manager
	Unit  *game.UnitInfo
	Steps []paths.Step
	Index int
	Cost  paths.CostFunc
	Fast  bool // GameManager_FastMovement
	Delay int  // animation ticks to hold before pulling out (a group's departure stagger)

	reserved  bool // the current step's destination is claimed in Mgr.Cells (AddAt)
	waitTicks int  // consecutive ticks stood fast for a contested next cell
}

// NewMoveOrder starts a unit moving along steps (the route from a
// paths.Find result). cost gives the entry cost of each cell so the
// speed budget for the turn can be spent; mgr keeps the cell hash correct
// as the unit changes cells (may be nil for a pure logical or headless move).
func NewMoveOrder(mgr *Manager, u *game.UnitInfo, steps []paths.Step, cost paths.CostFunc, fast bool) *MoveOrder {
	u.SetOrder(game.OrderMove)
	u.SetOrderState(game.OrderStateInProgress)
	return &MoveOrder{Mgr: mgr, Unit: u, Steps: steps, Index: 0, Cost: cost, Fast: fast}
}

// Done reports whether the whole route has been consumed.
func (m *MoveOrder) Done() bool {
	return m.Index >= len(m.Steps)
}

// ReleaseReservation drops the current step's destination reservation (see
// AddAt) when a unit is interrupted mid-slide - an explicit Stop, a fresh
// order, HaltForEnemy, or the unit dying - so a claimed cell is never left
// permanently blocked by a mover that never actually arrives there. Safe to
// call on an order with no live reservation.
func (m *MoveOrder) ReleaseReservation() {
	if !m.reserved {
		return
	}
	step := m.Steps[m.Index].To
	m.Mgr.Cells.RemoveAt(m.Unit, step.X, step.Y)
	m.reserved = false
}

// RemainingSteps returns the steps of the route not yet taken. The slice
// aliases the order's Steps; copy it if it must outlive the order.
func (m *MoveOrder) RemainingSteps() []paths.Step {
	if m.Done() {
		return nil
	}
	return m.Steps[m.Index:]
}

// NextCellPassable reports whether the next step's destination cell is still
// enterable (entry cost > 0). It distinguishes "paused, out of movement
// points" (next cell passable, unit Speed exhausted) from "genuinely blocked"
// (next cell impassable), so a route halted only by that turn's budget can be
// queued and resumed next turn instead of discarded.
func (m *MoveOrder) NextCellPassable() bool {
	if m.Done() || m.Cost == nil {
		return false
	}
	c := m.Steps[m.Index].To
	return m.Cost(c.X, c.Y) > 0
}

// Tick advances the order by one animation frame. It first turns the unit
// to face the next step (one eighth per tick), then (once facing) pays
// for the cell and slides the unit into it pixel by pixel, snapping to the
// cell center on arrival and advancing to the next step.
//
// C++ ported : unitinfo.cpp - UnitInfo::Move
func (m *MoveOrder) Tick() MoveStatus {
	u := m.Unit
	if m.Done() {
		u.SetOrder(game.OrderAwait)
		u.SetOrderState(game.OrderStateExecutingOrder)
		return MoveDone
	}

	// A staggered departure: hold still for a few animation frames so a group pulls out in
	// order rather than every unit lurching off on the same one. It costs no movement
	// budget - the unit has simply not set off yet - and a fast (headless) move skips it.
	if m.Delay > 0 && !m.Fast {
		m.Delay--

		return MoveTurning
	}

	step := m.Steps[m.Index]

	// Turn to face the step before driving (ORDER_STATE_IN_TRANSITION).
	if uint8(step.Dir) != u.Angle {
		u.SetOrderState(game.OrderStateInTransition)
		u.TurnToward(uint8(step.Dir))
		return MoveTurning
	}

	// Entering a fresh cell: charge it against the speed budget for the turn.
	if u.Moved == 0 {
		c := m.Cost(step.To.X, step.To.Y)
		if c == 0 {
			// An occupant physically sitting there (another mover mid-slide or
			// parked) is a transient chokepoint jam, not impassable terrain -
			// stand fast and retry rather than abandoning the whole route,
			// bounded so two units nose to nose can never deadlock forever.
			if m.Mgr != nil && len(m.Mgr.Cells.At(step.To.X, step.To.Y)) > 0 && m.waitTicks < maxWaitTicks {
				m.waitTicks++
				return MoveWaiting
			}
			return MoveBlocked
		}
		if !calculateStep(u, c, step.Dir&1 != 0, m.Fast) {
			return MoveBlocked
		}
		m.waitTicks = 0
		if m.Mgr != nil {
			m.Mgr.Cells.AddAt(u, step.To.X, step.To.Y) // reserve the destination for the whole crossing
			m.reserved = true
		}
		u.SetOrderState(game.OrderStateInProgress)
	}

	// Pixel ramp (UnitInfo::Move): accelerate toward max_velocity, clamp at
	// the tile boundary, and ease to a stop on the final cell.
	inc := 1
	if m.Fast {
		inc = 2
	}
	if u.Velocity < u.MaxVelocity {
		u.Velocity += inc
	}
	if u.Velocity > u.MaxVelocity {
		u.Velocity = u.MaxVelocity
	}
	vel := u.Velocity
	if vel+u.Moved >= tileSize-vel/2 {
		vel = tileSize - u.Moved
		if u.Type != assets.Commando && u.Type != assets.Infantry {
			if u.Speed == 0 || m.Index == len(m.Steps)-1 {
				vel /= 2
				if vel == 0 {
					vel = 1
				}
				u.Velocity = vel
				u.MaxVelocity = vel
			}
		}
	}
	u.Moved += vel

	off := paths.StepOffset(step.Dir)
	u.X += off.X * vel
	u.Y += off.Y * vel

	if u.Moved >= tileSize {
		u.Moved = 0
		u.Velocity = 0
		if m.Mgr != nil {
			m.Mgr.Arrive(u, step.To.X, step.To.Y) // completes the reservation, snaps to cell center
			m.reserved = false
		} else {
			u.SetPosition(step.To.X, step.To.Y)
		}
		m.Index++
		if m.Done() {
			u.SetOrder(game.OrderAwait)
			u.SetOrderState(game.OrderStateExecutingOrder)
			return MoveDone
		}
	}
	return MoveDriving
}

// calculateStep sets the velocity ramp for entering a cell of the given
// cost and debits the unit's speed budget for the turn. It returns false (and
// zeroes the remaining speed) when the unit cannot afford the step this
// turn. Diagonal steps cost (cost*3)/2, matching the pathfinder. An unaffordable
// step adds the leftover speed to the banked move fraction (it no longer
// overwrites it).
//
// C++ ported : paths.cpp - Paths_CalculateStep. The shots clamp
// (ClampShotsToSpeed) is ported. Omitted versus the original (this is a turn
// based port): group_speed and the RealTime branch: recorded as deviations.
func calculateStep(u *game.UnitInfo, cost int, diagonal, fast bool) bool {
	maxVel := 8
	if fast {
		maxVel = 16
	}
	u.MaxVelocity = maxVel
	if u.Type == assets.Commando || u.Type == assets.Infantry {
		u.MaxVelocity >>= 1
		u.Velocity = u.MaxVelocity
	}
	if cost > 4 {
		u.MaxVelocity /= 2
	} else if cost < 4 {
		u.MaxVelocity *= 2
	}

	if diagonal {
		cost = (cost * 3) / 2
	}
	if u.MoveFraction > cost {
		u.MoveFraction = cost
	}
	cost -= u.MoveFraction

	normalized := (cost + 3) / 4
	if normalized > u.Speed {
		u.MoveFraction += u.Speed * 4
		u.Speed = 0
		return false
	}
	u.Speed -= normalized
	u.ClampShotsToSpeed() // spending movement lowers the shots the shared budget still allows
	u.MoveFraction = normalized*4 - cost
	return true
}
