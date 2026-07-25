// sfxtype.go : maps unit events to sound table indices.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

// SfxType identifies a unit sound effect. Each value indexes the sound table and its tag suffix (TANK1 = idle, TANK5 = drive).
//
// C++ ported : unit.hpp - Unit::SfxType
type SfxType uint8

const (
	SfxInvalid    SfxType = 0
	SfxIdle       SfxType = 1  // ambient idle hum (looping)
	SfxWaterIdle  SfxType = 2  // amphibious unit idle on water
	SfxDrive      SfxType = 3  // moving on land (looping while moving)
	SfxWaterDrive SfxType = 4  // moving on water
	SfxStop       SfxType = 5  // engine off / deceleration cue
	SfxWaterStop  SfxType = 6  // stop on water
	SfxTransform  SfxType = 7  // transform animation
	SfxBuilding   SfxType = 8  // construction build loop
	SfxShrink     SfxType = 9  // shrink / pack up
	SfxExpand     SfxType = 10 // expand / deploy
	SfxTurret     SfxType = 11 // turret rotation servo
	SfxFire       SfxType = 12 // weapon fire shot
	SfxHit        SfxType = 13 // unit hit impact
	SfxExpload    SfxType = 14 // unit destroyed / explodes
	SfxPowerStart SfxType = 15 // building powers on
	SfxPowerEnd   SfxType = 16 // building powers off
	SfxLand       SfxType = 17 // air unit lands
	SfxTake       SfxType = 18 // air unit takes off
	sfxTypeLimit  SfxType = 19
)
