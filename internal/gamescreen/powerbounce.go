// powerbounce.go : implements the power grid bounce animation.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

// powerbounce.go - the "bounce" when the player powers on a building that cannot
// be powered (a structure sited off base with no connected generator). Vanilla
// plays the rev up briefly, cuts it with the wind down, and shows a "needs power"
// message; the port mirrors that over the frame loop because setPower is
// synchronous. C++ ported : units_manager.cpp ProcessOrderPowerOn (rev up) +
// production_manager.cpp ValidateAuxilaryIndustry (bounce to POWER_OFF + the
// "Cannot turn %s on, %s needed" message).

import (
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/units"
)

// powerBounceMS is how long the rev up plays before the wind down cuts it - a brief
// "tried to start, bounced straight back off" cue. Tunable feel.
const powerBounceMS = 250

// beginPowerBounce starts the rev up START as a HELD cue and arms the deferred cut
// (the building is already set OrderPowerOff by the caller). tickPowerBounce cuts
// the rev up and shows the message once powerBounceMS has passed.
func (s *Screen) beginPowerBounce(u *game.UnitInfo, short economy.Shortage) {
	s.cancelPowerBounce() // drop any in flight bounce first
	s.sound.powerBounceUnit = u
	s.sound.powerBounceShort = short
	s.sound.powerBounceDue = platform.Ticks() + powerBounceMS

	// A held start so tickPowerBounce can CUT it (two single plays would just overlap).
	if s.sound.sfxHeldStartFn == nil {
		return
	}

	tag := units.SfxTagFor(u.Type, game.SfxPowerStart)

	if tag == "" {
		return
	}

	s.sound.powerBounceSfx = s.sound.sfxHeldStartFn(tag, units.SfxDefaultTag(game.SfxPowerStart), s.unitPan(u), 1.0, false)
}

// tickPowerBounce, called once per idle frame, cuts the rev up after powerBounceMS,
// plays the wind down END, and shows the vanilla "needs power" message.
func (s *Screen) tickPowerBounce() {
	if s.sound.powerBounceUnit == nil {
		return
	}

	if platform.Ticks() < s.sound.powerBounceDue {
		return // still in the brief rev up window
	}

	u := s.sound.powerBounceUnit

	if s.sound.powerBounceSfx != 0 && s.sound.sfxHeldStopFn != nil {
		s.sound.sfxHeldStopFn(s.sound.powerBounceSfx) // cut the rev up
	}

	s.playSfxForUnit(u, game.SfxPowerEnd) // the wind down "powers off" cue
	// A world tint event naming the DEFICIENT resource (power vs workers/personnel).
	// C++: ComposeIndustryMessage(_(961d) = "Cannot turn %s on, %s needed") is drawn
	// MessageManager_DrawMessage(buffer, 1, ...) = type 1 (world tint), not red.
	need := "power"

	if s.sound.powerBounceShort == economy.ShortageWorkers {
		need = "workers"
	}

	s.showInfo("Cannot turn " + displayName(u.Type) + " on, " + need + " needed.")
	s.cancelPowerBounce()
}

// cancelPowerBounce clears an in flight bounce, stopping the held rev up if it is
// still playing (e.g. a fresh power on supersedes it).
func (s *Screen) cancelPowerBounce() {
	if s.sound.powerBounceSfx != 0 && s.sound.sfxHeldStopFn != nil {
		s.sound.sfxHeldStopFn(s.sound.powerBounceSfx)
	}

	s.sound.powerBounceUnit = nil
	s.sound.powerBounceSfx = 0
}
