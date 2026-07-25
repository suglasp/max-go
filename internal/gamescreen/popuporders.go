// popuporders.go : carries out the popup's power and sentry orders.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// powerItem returns the appropriate power toggle for a building. C++: UnitsManager_Popup_OnClick_PowerOn/PowerOff (units_manager.cpp).
func (s *Screen) powerItem(u *game.UnitInfo, on bool) popupItem {
	if on {
		return popupItem{label: s.lang.Get("7a07", "Power Off"), action: func() { s.setPower(u, false) }}
	}

	return popupItem{label: s.lang.Get("e7b6", "Power On"), action: func() { s.setPower(u, true) }}
}

// setPower switches a building on or off, closes the popup, and reports the outcome. Ambient sound follows economy.IsActive. C++: SetNewOrder(ORDER_POWER_ON/OFF).
func (s *Screen) setPower(u *game.UnitInfo, on bool) {
	s.orderInput.popupOpen = false

	// The production optimizer manages generators, so their toggles reconcile immediately. Consumer toggles use the sim methods that add or release generating capacity.
	if u.Type == assets.PowGen || u.Type == assets.PowerStn {
		s.setGeneratorPower(u, on)
		return
	}

	if on {
		// A consumer needs a worker and enough generator capacity. Otherwise it bounces and tickPowerBounce reports the shortage.
		// C++: ProcessOrderPowerOn -> ValidateAuxilaryIndustry / CheckPowerNeed.
		if c := s.game.ComplexOf(u); c != nil {
			if short := c.StartShortage(u); short != economy.ShortageNone {
				u.Order = game.OrderPowerOff
				u.State = game.OrderStateExecutingOrder
				s.beginPowerBounce(u, short)
				return
			}
		}

		if !s.game.PowerOnConsumer(u) {
			u.Order = game.OrderPowerOff
			u.State = game.OrderStateExecutingOrder
			s.beginPowerBounce(u, economy.ShortagePower)
			return
		}

		s.playPowerCue(u, game.SfxPowerStart) // stoppable power-on cue
	} else {
		s.game.SetConsumerPowerOff(u)       // releases capacity, allowing idle generators to power down
		s.playPowerCue(u, game.SfxPowerEnd) // power-off cue
	}

	u.State = game.OrderStateExecutingOrder
	verb := "on"

	if !on {
		verb = "off"
	}

	s.showMessage(fmt.Sprintf("%s turned %s.", displayName(u.Type), verb))

	// A research centre's run state follows its allocation. Sync the research pool after changing its power state.
	if u.Type == assets.Research {
		s.syncResearchOrders(u.Team)
	}

	// Eco Sphere activation only announces an opposing team's colony start. See checkColonyStarted and unitinfo.cpp UnitInfo::UpdateProduction.
}

// setGeneratorPower applies the player's order, optimizes the complex, and reports the resulting state. A required generator returns to service immediately.
// C++: InitPowerGenerators + ProductionManager_OptimizeProduction deriving the set again.
func (s *Screen) setGeneratorPower(u *game.UnitInfo, on bool) {
	if on {
		u.Order = game.OrderPowerOn
	} else {
		u.Order = game.OrderPowerOff
	}

	u.State = game.OrderStateExecutingOrder

	// Recompute the active generator set so a required generator does not appear off until the next turn.
	if c := s.game.ComplexOf(u); c != nil {
		c.OptimizeProduction()
	}

	nowOn := u.Order == game.OrderPowerOn

	if nowOn {
		s.playPowerCue(u, game.SfxPowerStart)
	} else {
		s.playPowerCue(u, game.SfxPowerEnd)
	}

	switch {
	case !on && nowOn:
		// The base still needs this generator.
		s.showMessage(displayName(u.Type) + " is needed and stays online. Power off a factory to save fuel.")
	case on && !nowOn:
		// The base already has enough power.
		s.showMessage(displayName(u.Type) + " is not needed; the base already has enough power.")
	case on:
		s.showMessage(displayName(u.Type) + " turned on.")
	default:
		s.showMessage(displayName(u.Type) + " turned off.")
	}
}

// toggleSentry holds a mobile unit on guard or releases it to awaiting orders. A sentry unit remains in reaction-fire coverage and leaves guard when moved manually.
// C++ ported : units_manager.cpp - UnitsManager_Popup_OnClick_Sentry.
func (s *Screen) toggleSentry(u *game.UnitInfo) {
	// Already on guard: release it back to awaiting orders.
	if u.Order == game.OrderSentry {
		u.Order = game.OrderAwait
		u.State = game.OrderStateExecutingOrder
		s.showMessage(displayName(u.Type) + " off sentry.")
		return
	}

	// Drop any queued move before placing the unit on guard. Keep it selected so its sentry state remains visible.
	s.game.CancelOrder(u)
	u.Order = game.OrderSentry
	u.State = game.OrderStateExecutingOrder
	s.showMessage(displayName(u.Type) + " on sentry.")
	s.playVoice(vsOnSentry) // V_F158 "on sentry" acknowledgement
}
