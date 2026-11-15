// airmove.go : the straight line flight of an air unit's move order.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : airpath.cpp - AirPath::Execute / CancelMovement; paths.cpp - Paths_GetAirPath
package units

import (
	"math"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
)

// airTakeOffTicks is how many order ticks a landed plane climbs before it flies on: the
// unit becomes hovering on the fifth tick.
//
// C++ ported : unitinfo.cpp - UnitInfo::Take
const airTakeOffTicks = 5

// airMove is the state of a straight flight: a plane does not follow grid steps, it flies a
// line from where it is toward the end cell at a fixed pace per tick, spending one speed
// point every maxVelocity ticks. Pixel offsets are 16.16 fixed point.
//
// C++ ported : airpath.hpp - AirPath
type airMove struct {
	length      int   // order ticks of flight left
	angle       uint8 // heading the plane turns to before it flies
	startX      int   // pixel position the line starts from
	startY      int   //
	stepX       int   // 16.16 pixels flown so far, from the start
	stepY       int   //
	deltaX      int   // 16.16 pixels per tick
	deltaY      int   //
	maxVelocity int   // ticks per speed point: 4, or 2 with fast movement
	landX       int   // cell the plane settles on when the flight ends
	landY       int   //
	takeOff     int   // take off ticks done (a plane already hovering needs none)
}

// quarterDistance is the straight line length between two cells in quarter movement points.
func quarterDistance(dx, dy int) int {
	return int(math.Sqrt(float64(dx*dx+dy*dy))*4.0 + 0.5)
}

// planAirFlight lays out the flight of u toward (endX, endY) the way the reference path builder
// does. The flight is cut to what the unit's movement budget pays for, and the leftover
// quarter points ride in MoveFraction. It returns nil when u is already on the end cell.
//
// C++ ported : paths.cpp - Paths_GetAirPath, airpath.cpp - AirPath::AirPath
func planAirFlight(u *game.UnitInfo, endX, endY int, fast bool) *airMove {
	dx, dy := endX-u.GridX, endY-u.GridY
	distance := quarterDistance(dx, dy)
	if distance == 0 {
		return nil
	}

	distance -= u.MoveFraction
	if distance < 2 {
		distance = 2
	}

	maxDistance := u.Speed
	if maxDistance != 0 {
		maxDistance *= 4
	} else {
		maxDistance = math.MaxInt16 // no budget: the whole way is planned, the first tick stops it
	}

	stepsDistance := 0

	if maxDistance < distance {
		targetX := dx*maxDistance/distance + u.GridX
		targetY := dy*maxDistance/distance + u.GridY
		dx, dy = targetX-u.GridX, targetY-u.GridY
		distance = quarterDistance(dx, dy)

		if distance > maxDistance {
			distance = maxDistance
		}

		stepsDistance = maxDistance / 4
		u.MoveFraction = maxDistance - distance

		if u.MoveFraction > 3 {
			u.MoveFraction = 3
		}

		if stepsDistance == 0 || distance == 0 {
			u.Speed = 0
			dx, dy = endX-u.GridX, endY-u.GridY
			distance = quarterDistance(dx, dy)
			stepsDistance = (distance + 3) / 4
		}
	} else {
		stepsDistance = (distance + 3) / 4

		if maxDistance != math.MaxInt16 {
			u.MoveFraction = distance % 4
		}
	}

	maxVelocity := 4
	if fast {
		maxVelocity = 2
	}

	u.MaxVelocity = maxVelocity
	ticks := stepsDistance * maxVelocity

	if ticks <= 0 {
		return nil
	}

	return &airMove{
		length:      ticks,
		angle:       game.TargetAngle(dx, dy),
		startX:      u.X,
		startY:      u.Y,
		deltaX:      (dx << 22) / ticks,
		deltaY:      (dy << 22) / ticks,
		maxVelocity: maxVelocity,
		landX:       u.GridX + dx,
		landY:       u.GridY + dy,
	}
}

// NewAirMoveOrder starts u flying a straight line toward (destX, destY). The order keeps one
// synthetic step, the destination, so the remaining route, a queued route, a held route and the
// path arrows work as they do for a ground order. It returns nil when u already stands on the
// destination cell. The caller checks that the destination can be taken.
//
// C++ ported : paths.cpp - Paths_GetAirPath (the order's AirPath)
func NewAirMoveOrder(mgr *Manager, u *game.UnitInfo, destX, destY int, fast bool) *MoveOrder {
	flight := planAirFlight(u, destX, destY, fast)
	if flight == nil {
		return nil
	}

	u.SetOrder(game.OrderMove)
	u.SetOrderState(game.OrderStateInProgress)
	u.Velocity = 0

	steps := []paths.Step{{To: game.Point{X: destX, Y: destY}, Dir: int(flight.angle)}}

	return &MoveOrder{Mgr: mgr, Unit: u, Steps: steps, Fast: fast, air: flight}
}

// airOffCentre reports whether the plane is between cell centres.
func airOffCentre(u *game.UnitInfo) bool {
	return u.X%tileSize != tileSize/2 || u.Y%tileSize != tileSize/2
}

// airStopAtNextCell aims the flight at the cell centre ahead so the plane parks on a centre
// after an emergency stop. It returns the destination step when that is no longer where the
// flight ends, so the caller can hold it.
//
// C++ ported : airpath.cpp - AirPath::CancelMovement
func (m *MoveOrder) airStopAtNextCell() []paths.Step {
	u, a := m.Unit, m.air

	if a.length == 0 || u.Angle != a.angle {
		return nil // nothing in flight, or still turning: the caller cancels the order instead
	}

	endX, endY := u.X/tileSize, u.Y/tileSize

	if a.deltaX > 0 {
		endX = (u.X + tileSize/2) / tileSize
	}

	if a.deltaY > 0 {
		endY = (u.Y + tileSize/2) / tileSize
	}

	a.startX, a.startY = u.X, u.Y
	a.stepX, a.stepY = 0, 0
	a.length %= a.maxVelocity

	if a.length == 0 {
		a.length++
		u.Speed++ // the last tick of the flight pays this point again
	}

	a.deltaX = ((endX*tileSize - u.X + tileSize/2) << 16) / a.length
	a.deltaY = ((endY*tileSize - u.Y + tileSize/2) << 16) / a.length
	a.landX, a.landY = endX, endY

	dest := m.Steps[len(m.Steps)-1]
	if dest.To.X == endX && dest.To.Y == endY {
		return nil
	}

	m.Steps = []paths.Step{{To: game.Point{X: endX, Y: endY}, Dir: int(a.angle)}}

	return []paths.Step{dest}
}

// airTick advances a straight flight by one order tick: take off, turn one eighth toward the
// heading, then fly one stride of the line.
//
// C++ ported : airpath.cpp - AirPath::Execute
func (m *MoveOrder) airTick() MoveStatus {
	u, a := m.Unit, m.air

	if m.Done() {
		u.Velocity = 0
		u.SetOrder(game.OrderAwait)
		u.SetOrderState(game.OrderStateExecutingOrder)
		return MoveDone
	}

	// A staggered departure, as for a ground order.
	if m.Delay > 0 && !m.Fast {
		m.Delay--
		return MoveTurning
	}

	// A landed plane climbs for a few ticks first. It costs no movement points.
	// C++ ported : unitinfo.cpp - UnitInfo::Take
	if !u.Flags.Has(game.FlagHovering) {
		a.takeOff++

		if a.takeOff >= airTakeOffTicks {
			if m.OnTakeOff != nil {
				m.OnTakeOff(u)
			} else {
				u.Flags |= game.FlagHovering
			}
		}

		return MoveTurning
	}

	if u.Speed <= 0 {
		u.Velocity = 0
		return MoveBlocked
	}

	if u.Angle != a.angle {
		u.SetOrderState(game.OrderStateInTransition)
		u.TurnToward(a.angle)
		return MoveTurning
	}

	u.SetOrderState(game.OrderStateInProgress)

	// The last tick halves the stride again and again, so the plane eases into the cell.
	if a.length == 1 {
		strideX := absInt(a.deltaX>>16) >> 1
		strideY := absInt(a.deltaY>>16) >> 1

		if strideX != 0 || strideY != 0 {
			a.length++
		}

		a.deltaX >>= 1
		a.deltaY >>= 1
	}

	a.stepX += a.deltaX
	a.stepY += a.deltaY
	posX := (a.stepX >> 16) + a.startX
	posY := (a.stepY >> 16) + a.startY
	cellX, cellY := posX/tileSize, posY/tileSize
	changed := cellX != u.GridX || cellY != u.GridY

	if changed && m.Mgr != nil {
		m.Mgr.Cells.Remove(u) // still keyed off the old cell
	}

	u.X, u.Y = posX, posY
	u.GridX, u.GridY = cellX, cellY
	a.length--

	if changed {
		if m.Mgr != nil {
			m.Mgr.Cells.Add(u)
		}

		if m.OnCellChange != nil {
			m.OnCellChange(u)
		}
	}

	if a.length%a.maxVelocity == 0 {
		if u.Speed > 0 {
			u.Speed--
		}

		u.ClampShotsToSpeed()
	}

	if a.length == 0 {
		return m.airFinish()
	}

	return MoveDriving
}

// airFinish ends a flight: the plane settles on the centre of its end cell. On the destination
// the order is done; on a cut short flight it reports MoveBlocked with the budget spent, so the
// route is queued for the next turn.
//
// C++ ported : airpath.cpp - Paths_FinishMove
func (m *MoveOrder) airFinish() MoveStatus {
	u, a := m.Unit, m.air

	if u.GridX != a.landX || u.GridY != a.landY {
		if m.Mgr != nil {
			m.Mgr.Cells.Remove(u)
		}

		u.SetPosition(a.landX, a.landY)

		if m.Mgr != nil {
			m.Mgr.Cells.Add(u)
		}

		if m.OnCellChange != nil {
			m.OnCellChange(u)
		}
	} else {
		u.SetPosition(a.landX, a.landY)
	}

	u.Velocity = 0
	dest := m.Steps[len(m.Steps)-1].To

	if a.landX == dest.X && a.landY == dest.Y {
		m.Index = len(m.Steps)
		u.SetOrder(game.OrderAwait)
		u.SetOrderState(game.OrderStateExecutingOrder)
		return MoveDone
	}

	return MoveBlocked
}

// AirRouteMarkers lists the cells along the straight line from (fromX, fromY) to (toX, toY),
// one per movement point, each facing the heading of the flight. The path arrows draw them.
//
// C++ ported : airpath.cpp - AirPath::Draw
func AirRouteMarkers(fromX, fromY, toX, toY int) []paths.Step {
	dx, dy := toX-fromX, toY-fromY
	steps := (quarterDistance(dx, dy) + 3) / 4

	if steps <= 0 {
		return nil
	}

	heading := int(game.TargetAngle(dx, dy))
	markers := make([]paths.Step, 0, steps)

	for i := 1; i <= steps; i++ {
		pixelX := fromX*tileSize + tileSize/2 + dx*tileSize*i/steps
		pixelY := fromY*tileSize + tileSize/2 + dy*tileSize*i/steps
		markers = append(markers, paths.Step{To: game.Point{X: pixelX / tileSize, Y: pixelY / tileSize}, Dir: heading})
	}

	return markers
}

// absInt returns the absolute value of v.
func absInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
