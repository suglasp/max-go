// commando.go : implements commando disabling, theft, and stealth.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - commando/spy action execution
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// A commando beside an enemy electronic unit can disable it for several turns or steal it for the commando's team. Disabled units remain blocked for
// their configured number of turns, then recover.
//
// C++ ported : units_manager.cpp - UnitsManager_ProcessOrderAwaitDisableStealUnit /
// AttemptStealthAction / CaptureUnit / DisableUnit.

// IsCommando reports whether u is a commando (the only disable/steal performer).
func IsCommando(u *game.UnitInfo) bool {
	return u != nil && u.Type == assets.Commando
}

// footprintOf is the cell edge of a unit's footprint (2 for a building, else 1).
func footprintOf(u *game.UnitInfo) int {
	if u.Flags.Has(game.FlagBuilding) {
		return 2
	}
	return 1
}

// commandoAdjacent reports whether the commando is in the target footprint's eight-cell neighborhood. A 2x2 building's full envelope counts.
// Disable and steal require adjacency in the original. They do not use ATTRIB_RANGE or ATTRIB_SCAN.
//
// C++ ported : game_manager.cpp - GameManager_IsUnitNextToPosition
func commandoAdjacent(c, target *game.UnitInfo) bool {
	fp := footprintOf(target)
	return axisGap(c.GridX, target.GridX, target.GridX+fp-1) <= 1 &&
		axisGap(c.GridY, target.GridY, target.GridY+fp-1) <= 1
}

// targetInAir reports whether the target is an airborne unit, which cannot be disabled or stolen.
// C++: !GameManager_IsUnitNotInAir.
func targetInAir(target *game.UnitInfo) bool {
	return target.Flags.Has(game.FlagMobileAirUnit) && target.Flags.Has(game.FlagHovering)
}

// StealthChance is the percent success chance for the commando's disable/steal
// against target, capped at 90. Steal is one quarter the disable chance.
//
// C++ ported : units_manager.cpp - UnitsManager_GetStealthChancePercentage
func (g *Game) StealthChance(c, target *game.UnitInfo, steal bool) int {
	turns := 1
	if target.Values != nil {
		if t := target.Values.Get(game.AttribTurns); t > 0 {
			turns = t
		}
	}
	chance := (96*c.GetExperience() + 640) / turns
	if steal {
		chance /= 4
	}
	if chance > 90 {
		chance = 90
	}
	if chance < 0 {
		chance = 0
	}
	return chance
}

// canInfiltrate checks shared disable and steal eligibility: an adjacent enemy electronic unit and a commando with a shot left.
// Steal rejects buildings and loaded transports. Disable rejects an already disabled target.
//
// C++ ported : game_manager.cpp - GameManager_IsValidDisableTarget /
// GameManager_IsValidStealTarget + the COMMANDO branch of GameManager_GetMilitaryCursor.
func (g *Game) canInfiltrate(c, target *game.UnitInfo, steal bool) bool {
	if !IsCommando(c) || target == nil || c.Shots <= 0 {
		return false
	}
	if target.Team == c.Team {
		return false
	}
	if !target.Flags.Has(game.FlagElectronicUnit) {
		return false // organic units (infantry/commando) cannot be disabled/stolen
	}
	if busyOrDying(target) {
		return false
	}
	if targetInAir(target) {
		return false
	}
	if !commandoAdjacent(c, target) {
		return false
	}
	if steal {
		if target.Flags.Has(game.FlagStationary) {
			return false // buildings cannot be stolen
		}
		if len(target.Carried) > 0 {
			return false // a loaded transport cannot be stolen
		}
	} else if target.Order == game.OrderDisable {
		return false // already disabled
	}
	return true
}

// busyOrDying reports a target a commando cannot infiltrate: one transforming, firing, exploding or being destroyed.
//
// C++ ported : game_manager.cpp - GameManager_IsValidDisableTarget / GameManager_IsValidStealTarget
func busyOrDying(target *game.UnitInfo) bool {
	switch target.Order {
	case game.OrderTransform, game.OrderFire, game.OrderExplode:
		return true
	}
	return target.State == game.OrderStateDestroy
}

// CanDisable reports whether the commando may disable target right now.
func (g *Game) CanDisable(c, target *game.UnitInfo) bool {
	return g.canInfiltrate(c, target, false)
}

// CanSteal reports whether the commando may steal target right now.
func (g *Game) CanSteal(c, target *game.UnitInfo) bool {
	return g.canInfiltrate(c, target, true)
}

// CanDisableAt / CanStealAt resolve the enemy occupant of a cell and test
// eligibility, for the cursor and the click intercept.
func (g *Game) CanDisableAt(c *game.UnitInfo, cx, cy int) bool {
	return g.CanDisable(c, g.topTarget(c, cx, cy))
}

func (g *Game) CanStealAt(c *game.UnitInfo, cx, cy int) bool {
	return g.CanSteal(c, g.topTarget(c, cx, cy))
}

// Disable attempts to shut down the enemy unit at (cx, cy).
// It reports success and whether a failed attempt exposed the commando to the victim's team. It always spends a shot.
//
// C++ ported : units_manager.cpp - GameManager_DisableUnit + AttemptStealthAction.
func (g *Game) Disable(c *game.UnitInfo, cx, cy int) (ok, exposed bool) {
	return g.infiltrate(c, g.topTarget(c, cx, cy), false)
}

// Steal attempts to capture the enemy unit in (cx, cy) onto the commando's team.
//
// C++ ported : units_manager.cpp - GameManager_StealUnit + AttemptStealthAction.
func (g *Game) Steal(c *game.UnitInfo, cx, cy int) (ok, exposed bool) {
	return g.infiltrate(c, g.topTarget(c, cx, cy), true)
}

// infiltrate rolls against the chance, spends a shot, and captures or disables the target on success.
// C++: AttemptStealthAction.
func (g *Game) infiltrate(c, target *game.UnitInfo, steal bool) (ok, exposed bool) {
	if !g.canInfiltrate(c, target, steal) {
		return false, false
	}
	chance := g.StealthChance(c, target, steal)
	roll := game.RandomizerGenerate(101) // 0..100, matching Randomizer_Generate(101)
	c.Shots--                            // a stealth action always costs one shot
	victimTeam := target.Team            // read it BEFORE a capture rewrites it

	if roll <= chance {
		kind, turns := CommandoCapture, 0

		if steal {
			g.captureUnit(target, c.Team)
		} else {
			g.applyDisable(c, target)
			kind, turns = CommandoDisable, g.disabled[target]
		}
		c.Experience++ // a successful action against a live target earns experience
		g.queueCommandoEvent(CommandoEvent{Kind: kind, Commando: c, Victim: target, VictimTeam: victimTeam, Turns: turns})
		return true, false
	}
	// A miss exposes the commando to the victim's team so the screen can warn the player.
	// Reaction fire and visibility remain outside this operation.
	g.queueCommandoEvent(CommandoEvent{Kind: CommandoExposed, Commando: c, Victim: target, VictimTeam: victimTeam, Steal: steal})
	return false, true
}

// CommandoKind is what a commando's infiltration did to its target.
type CommandoKind int

const (
	CommandoCapture CommandoKind = iota // the target changed hands
	CommandoDisable                     // the target was shut down for Turns turns
	CommandoExposed                     // the attempt failed and the infiltrator was spotted
)

// CommandoEvent records one resolved infiltration so the screen can tell BOTH sides
// about it. The victim's side is the half that was missing: the sim mutates the target
// in place (it silently changes team or freezes), and an AI commando's results went only
// to the debug log - so a player who lost a unit to an enemy commando got no banner, no
// voice, nothing. VictimTeam is captured before the steal rewrites the target's team.
//
// C++ ported : units_manager.cpp - UnitsManager_CaptureUnit / UnitsManager_DisableUnit /
// AttemptStealthAction, each of which notifies the attacker AND the victim.
type CommandoEvent struct {
	Kind       CommandoKind   // capture, disable, or a failed attempt
	Commando   *game.UnitInfo // the infiltrator
	Victim     *game.UnitInfo // the unit it acted on
	VictimTeam int            // the team that OWNED the victim when the action resolved
	Turns      int            // disable duration in turns (CommandoDisable only)
	Steal      bool           // the failed attempt was a capture, not a disable (CommandoExposed only)
}

// queueCommandoEvent shelves a resolved infiltration for the screen's drain.
func (g *Game) queueCommandoEvent(e CommandoEvent) {
	g.commandoEvents = append(g.commandoEvents, e)
}

// DrainCommandoEvents returns the infiltrations resolved since the last call and
// clears the queue; the in game screen drains them each frame to notify the victim.
func (g *Game) DrainCommandoEvents() []CommandoEvent {
	if len(g.commandoEvents) == 0 {
		return nil
	}
	out := g.commandoEvents
	g.commandoEvents = nil
	return out
}

// applyDisable puts the target in ORDER_DISABLE for a duration based on commando experience and inverse target build turns.
// It cancels the current order. Disabled buildings drop out of complex production because economy.IsActive treats ORDER_DISABLE as inactive.
//
// C++ ported : units_manager.cpp - UnitsManager_DisableUnit
func (g *Game) applyDisable(c, target *game.UnitInfo) {
	turns := 1
	if target.Values != nil {
		if t := target.Values.Get(game.AttribTurns); t > 0 {
			turns = t
		}
	}
	dur := (c.GetExperience() + 8) / turns
	if dur < 1 {
		dur = 1
	}
	g.cancel(target) // stop any active move
	delete(g.pending, target)
	if g.BuildOrderFor(target) != nil {
		g.CancelBuild(target) // a disabled builder drops its build
	}
	target.SetOrder(game.OrderDisable)
	target.SetOrderState(game.OrderStateExecutingOrder)
	if g.disabled == nil {
		g.disabled = make(map[*game.UnitInfo]int)
	}
	g.disabled[target] = dur
}

// captureUnit transfers a stolen mobile unit to newTeam, stops its orders, and refreshes visibility.
// Sprite colour and unit hashes read the live team, so they need no rekeying.
//
// C++ ported : units_manager.cpp - UnitsManager_CaptureUnit / UnitInfo::ChangeTeam
func (g *Game) captureUnit(target *game.UnitInfo, newTeam int) {
	g.cancel(target)
	delete(g.pending, target)
	delete(g.disabled, target) // a captured unit is no longer disabled
	if g.BuildOrderFor(target) != nil {
		g.CancelBuild(target)
	}
	target.Team = newTeam
	target.Order = game.OrderAwait
	target.State = game.OrderStateExecutingOrder
	g.RefreshVisibility()
}

// IsDisabled reports whether u is currently shut down (cannot act).
func (g *Game) IsDisabled(u *game.UnitInfo) bool {
	if u.Order == game.OrderDisable {
		return true
	}
	_, ok := g.disabled[u]
	return ok
}

// DisabledTurns is how many more turns u stays disabled (0 if not disabled),
// for the status display.
func (g *Game) DisabledTurns(u *game.UnitInfo) int {
	return g.disabled[u]
}

// DisabledState records a commando-disabled unit and its remaining recovery turns for a save snapshot.
type DisabledState struct {
	Unit  *game.UnitInfo
	Turns int
}

// DisabledStates returns each unit disabled by a commando and its remaining recovery countdown.
// Saves store units through the registry; this preserves the countdowns that live here.
func (g *Game) DisabledStates() []DisabledState {
	out := make([]DisabledState, 0, len(g.disabled))
	for u, turns := range g.disabled {
		if u != nil {
			out = append(out, DisabledState{Unit: u, Turns: turns})
		}
	}
	return out
}

// RestoreDisabled restores a commando-disabled unit's recovery countdown on load.
// The matching OrderDisable is restored with the unit.
func (g *Game) RestoreDisabled(u *game.UnitInfo, turns int) {
	if u == nil || turns <= 0 {
		return
	}
	if g.disabled == nil {
		g.disabled = make(map[*game.UnitInfo]int)
	}
	g.disabled[u] = turns
}

// tickDisabled blocks units disabled this turn and decrements their counters. A unit returns to an idle order at zero.
// BeginTeamTurn calls it after movement and combat refill, so disabled units get no budget. C++: ORDER_DISABLE decrements in unitinfo.cpp's regen pass.
func (g *Game) tickDisabled(team int) {
	for u, left := range g.disabled {
		if u.Team != team {
			continue
		}
		if left > 0 {
			u.Speed = 0 // blocked: no movement or shots this turn
			u.Shots = 0
		}
		g.disabled[u] = left - 1
		if left-1 <= 0 {
			delete(g.disabled, u)
			u.Order = game.OrderAwait
			u.State = game.OrderStateExecutingOrder
		}
	}
}
