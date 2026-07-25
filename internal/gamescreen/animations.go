// animations.go : paces the clock driven unit animations of the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
)

// turretStepMS sets the delay between one-eighth turret turns toward the aim target. A half turn takes about 0.3 seconds.
const turretStepMS = 80

// spinStepMS sets the delay between frames of a spinning turret, such as a scanner or AWAC radar dish. A 16-frame dish completes a turn in about a second.
const spinStepMS = 64

// bodyStepMS sets the delay between animation frames for buildings such as mining pumps and radar dishes. It matches spinStepMS.
const bodyStepMS = spinStepMS

// buildSlideStepMS and buildSlideStep move a builder 8 world pixels per step from a 2x2 site's corner to its centre. C++: ORDER_STATE_TAPE_POSITIONING_ENTER.
const (
	buildSlideStepMS = 40
	buildSlideStep   = 8
)

// stepAnimations advances the independent unit animations due on this frame: turrets, spinning dishes, building bodies, walking, build slides, and air altitude.
func (s *Screen) stepAnimations() {
	// Animate turreted units rotating their gun toward its aim target.
	if now := platform.Ticks(); now-s.turretStamp >= turretStepMS {
		s.turretStamp = now
		for _, u := range s.game.Units.Units() {
			u.StepTurret()
		}
	}

	// Spinning turrets, including radar dishes with more than eight frames, stop when Animate Effects is off. C++: ProcessOrderSentry.
	if now := platform.Ticks(); now-s.spinStamp >= spinStepMS && s.idleAnimationsRun() {
		s.spinStamp = now
		for _, u := range s.game.Units.Units() {
			if cnt := s.sprites.TurretCount(u.Type); cnt > 8 || u.Flags.Has(game.FlagSpinningTurret) {
				u.AdvanceSpin(cnt)
			}
		}
	}

	// Buildings with a moving part (radar dish, mining pump) cycle body frames.
	if now := platform.Ticks(); now-s.bodyStamp >= bodyStepMS && s.idleAnimationsRun() {
		s.bodyStamp = now
		for _, u := range s.game.Units.Units() {
			u.AdvanceBodyAnim(s.sprites.BodyAnimCount(u.Type, u.Flags))
		}
	}

	// Marching soldiers cycle their walk frames while moving, or continuously in the demo view.
	if now := platform.Ticks(); now-s.walkStamp >= walkStepMS {
		s.walkStamp = now
		s.advanceWalkAnims()
	}

	// Slide a builder onto the centre of its footprint instead of snapping it there.
	if now := platform.Ticks(); now-s.buildSlideStamp >= buildSlideStepMS {
		s.buildSlideStamp = now

		for _, u := range s.game.Units.Units() {
			switch {
			case u.Building || u.Clearing:
				u.AdvanceBuildSlide(buildSlideStep) // glide a builder / bulldozer onto its site centre

			case u.Leaving:
				if u.AdvanceBuildSlide(buildSlideStep) { // slide back out of a finished clear site
					u.Leaving = false
				}
			}

			// Air units glide toward their flying or landed altitude. C++: shadow_offset slide.
			if u.Flags.Has(game.FlagMobileAirUnit) {
				switch {
				case u.Flags.Has(game.FlagHovering) && u.AirAlt < game.AirAltMax:
					if u.AirAlt == 0 && u == s.selected {
						s.playSfxForUnit(u, game.SfxTake) // first rise tick: the lift off whoosh
					}
					u.AirAlt++

				case !u.Flags.Has(game.FlagHovering) && u.AirAlt > 0:
					if u.AirAlt == game.AirAltMax && u == s.selected {
						s.playSfxForUnit(u, game.SfxLand) // first descend tick: the set down sound
					}
					u.AirAlt--
				}
			}
		}
	}
}
