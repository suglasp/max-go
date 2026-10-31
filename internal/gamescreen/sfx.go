// sfx.go : implements the game screen SFX dispatcher.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : sound_manager.cpp - SoundManager_Play event dispatch
package gamescreen

// sfx.go contains game-screen unit SFX helpers for combat, movement, and positional panning.
// C++: SoundManager::PlaySfx (sound_manager.cpp); pan from grid distance to map center.

import (
	"strconv"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/world"
)

// soundState holds the sound effect hooks and live unit and ambient sound handles.
type soundState struct {
	sfxFn               func(primary, fallback string, pan float64)                          // plays a sound tag at a stereo pan; nil disables in game sfx
	sfxHeldStartFn      func(primary, fallback string, pan, volScale float64, loop bool) int // starts a held unit sound (0 = unsupported)
	sfxIntroLoopStartFn func(primary, fallback string, pan, volScale float64) int            // starts a held intro loop sound
	sfxHeldStopFn       func(id int)                                                         // stops a held sound early
	sfxHeldPanFn        func(id int, pan float64)                                            // re pans a live held sound as the unit moves
	sfxPosFn            func(primary, fallback string, pan, volScale float64)                // plays a unit sound at a pan and positional volume
	sfxHeldVolumeFn     func(id int, volScale float64)                                       // fades a live held sound as the unit or the view moves
	channelsReversed    bool                                                                 // settings channels_reversed: swap left and right
	movingUnits         map[*game.UnitInfo]bool                                              // units in motion last frame (drive start/stop detection)
	driveSfx            map[*game.UnitInfo]int                                               // moving unit's active drive sound handle id
	driveSfxType        map[*game.UnitInfo]game.SfxType                                      // land vs water drive variant currently held per unit
	haltSeen            map[*game.UnitInfo]map[uint16]bool                                   // enemy IDs already within scan range per moving player unit (Halt on enemy pref)
	ambientSfxUnit      *game.UnitInfo                                                       // unit whose ambient loop (idle hum / building hum) is playing
	ambientSfxID        int                                                                  // ambient sound handle id
	ambientSfxTag       string                                                               // tag currently looping (a change restarts it)
	powerCueSfx         int                                                                  // active power on/off transition cue handle id (0 = none)
	powerCueUnit        *game.UnitInfo                                                       // building the power cue belongs to (stopped when selection moves off it)
	powerBounceUnit     *game.UnitInfo                                                       // unit whose "bounce" rev up is playing (nil = none in flight)
	powerBounceSfx      int                                                                  // held rev up sound handle
	powerBounceDue      uint64                                                               // tick the bounce cuts and shows the message
	powerBounceShort    economy.Shortage                                                     // which resource the bounce message names
}

// movementSfx maps land movement sounds to water variants for units using alternate sprite form. Personnel Carriers invert the normal choice, and coast uses the land clip.
// surfaceUnder returns a unit's modified surface through the game grid, falling back to the raw map tile. Water Platforms and Bridges read as land.
func (s *Screen) surfaceUnder(u *game.UnitInfo) uint8 {
	if s.game != nil && s.game.Grid != nil {
		return s.game.Grid.ModifiedSurface(u.GridX, u.GridY, false)
	}

	if s.wmap != nil {
		return s.wmap.SurfaceAt(u.GridX, u.GridY)
	}

	return world.SurfaceLand
}

func (s *Screen) movementSfx(u *game.UnitInfo, land game.SfxType) game.SfxType {
	if s.wmap == nil {
		return land
	}

	onWater := s.surfaceUnder(u) == world.SurfaceWater

	if !units.UsesWaterMovementSfxSlot(u.Type, onWater) {
		return land
	}

	switch land {
	case game.SfxDrive:
		return game.SfxWaterDrive
	case game.SfxStop:
		return game.SfxWaterStop
	case game.SfxIdle:
		return game.SfxWaterIdle
	}

	return land
}

// SetSfxIntroLoopFunc installs the hook for selected mobile unit idle hums: play the full clip
// once, then loop the authored smpl running region (building/work sounds loop immediately;
// normal unit SFX keep their startup). C++ ported : sound_manager.cpp - JOB_TYPE_SFX loop point.
func (s *Screen) SetSfxIntroLoopFunc(start func(primary, fallback string, pan, volScale float64) int) {
	s.sound.sfxIntroLoopStartFn = start
}

// sfxKindName is a short human label for an SFX kind, for the debug log.
func sfxKindName(t game.SfxType) string {
	switch t {
	case game.SfxIdle:
		return "idle"
	case game.SfxWaterIdle:
		return "water-idle"
	case game.SfxDrive:
		return "drive"
	case game.SfxWaterDrive:
		return "water-drive"
	case game.SfxStop:
		return "stop"
	case game.SfxWaterStop:
		return "water-stop"
	case game.SfxTransform:
		return "transform"
	case game.SfxBuilding:
		return "building"
	case game.SfxShrink:
		return "shrink"
	case game.SfxExpand:
		return "expand"
	case game.SfxTurret:
		return "turret"
	case game.SfxFire:
		return "fire"
	case game.SfxHit:
		return "hit"
	case game.SfxExpload:
		return "explode"
	case game.SfxPowerStart:
		return "power-start"
	case game.SfxPowerEnd:
		return "power-end"
	case game.SfxLand:
		return "land"
	case game.SfxTake:
		return "take"
	default:
		return "sfx"
	}
}

// playSfxForUnit plays SFX of kind sfx for a unit, panned by its on screen X position. The
// unit specific clip is preferred; the generic GEN_*/WGN_* default is used when it is absent.
// C++: SoundManager::PlaySfx -> base_unit.GetSoundEffect -> SoundManager_PlaySfxGroup.
func (s *Screen) playSfxForUnit(u *game.UnitInfo, sfx game.SfxType) {
	if u == nil || s.sound.sfxFn == nil {
		return
	}

	tag := units.SfxTagFor(u.Type, sfx)

	if tag == "" {
		return
	}

	dbg.Printf("sfx %s : %s %s", displayName(u.Type), sfxKindName(sfx), tag)
	pan, volScale := s.unitSfxPosition(u)

	if s.sound.sfxPosFn != nil {
		s.sound.sfxPosFn(tag, units.SfxDefaultTag(sfx), pan, volScale)
		return
	}

	s.playSfxPanned(tag, units.SfxDefaultTag(sfx), pan)
}

// playPowerCue plays a stoppable building power-transition sound. The long power-on clip stops when another cue plays, selection changes, or the screen closes.
func (s *Screen) playPowerCue(u *game.UnitInfo, sfx game.SfxType) {
	if u == nil {
		return
	}

	tag := units.SfxTagFor(u.Type, sfx)

	if tag == "" {
		return
	}

	s.stopPowerCue() // drop any cue still ringing from a previous toggle

	pan, volScale := s.unitSfxPosition(u)

	// A held voice (loop=false) plays the clip once but stays stoppable by its handle.
	if s.sound.sfxHeldStartFn != nil {
		s.sound.powerCueSfx = s.sound.sfxHeldStartFn(tag, units.SfxDefaultTag(sfx), pan, volScale, false)
		s.sound.powerCueUnit = u
		return
	}

	s.playSfxForUnit(u, sfx) // no held backend (tests): fall back to fire and forget
}

// stopPowerCue silences a power on/off cue that is still playing.
func (s *Screen) stopPowerCue() {
	if s.sound.powerCueSfx != 0 && s.sound.sfxHeldStopFn != nil {
		s.sound.sfxHeldStopFn(s.sound.powerCueSfx)
	}

	s.sound.powerCueSfx = 0
	s.sound.powerCueUnit = nil
}

// tickMovementSfx advances the sim one frame and fires movement SFX: the drive sound when a
// unit starts moving, cut on arrival/stall, then the stop sound. The drive sound is held on a
// per unit handle and stopped on arrival so a short hop does not play the full clip.
// C++: UnitInfo::Move plays SFX_TYPE_DRIVE/STOP on state changes (per unit channel).
// haltMoversOnEnemy stops a moving human or computer unit when it newly spots a hostile. Enemies already visible at move start do not retrigger it; a new move reseeds the baseline.
//
// C++ ported : access.cpp - the enemy_halt preference in Access_UpdateMapStatus.
func (s *Screen) haltMoversOnEnemy() {
	if s.sound.haltSeen == nil {
		s.sound.haltSeen = make(map[*game.UnitInfo]map[uint16]bool)
	}

	moving := make(map[*game.UnitInfo]bool)
	var halt []*game.UnitInfo

	for _, o := range s.game.MoveOrders() {
		u := o.Unit

		if u.Values == nil || !s.haltingTeam(u.Team) {
			continue
		}

		moving[u] = true
		scan := u.Values.Get(game.AttribScan)

		if scan <= 0 {
			continue
		}

		prev, tracked := s.sound.haltSeen[u] // nil on the first tick of this move
		cur := make(map[uint16]bool)
		newSpot := false

		for _, e := range s.game.Units.Units() {
			if e.Team == u.Team || e.Team < 0 || !e.Flags.Has(game.FlagSelectable) || e.Flags.Has(game.FlagGroundCover) {
				continue
			}

			dx, dy := e.GridX-u.GridX, e.GridY-u.GridY

			if dx*dx+dy*dy <= scan*scan {
				cur[e.ID] = true

				if tracked && !prev[e.ID] {
					newSpot = true // an enemy that was NOT in range last tick just entered
				}
			}
		}

		// Seed the known enemy set on the first tick (no halt); afterwards halt only
		// when a genuinely new enemy has come into range.
		if tracked && newSpot {
			halt = append(halt, u)
		}

		s.sound.haltSeen[u] = cur
	}

	// Forget units that are no longer moving, so a fresh move reseeds its baseline.
	for u := range s.sound.haltSeen {
		if !moving[u] {
			delete(s.sound.haltSeen, u)
		}
	}

	held := 0
	last := ""

	for _, u := range halt {
		// HOLD the route rather than cancel it: the unit stops, but it remembers where it
		// was going and keeps its arrows, so the player is not left to recall the
		// destination and re-issue the whole move by hand.
		s.game.HaltForEnemy(u)

		if s.game.State != nil && u.Team == s.game.State.PlayerTeam {
			held++
			last = displayName(u.Type)
		}
	}

	// Say WHY the unit stopped short. Without this the player sees a column halt in open
	// ground for no visible reason (the enemy that tripped it may still be under fog at
	// the edge of scan range) and reads it as the move having failed.
	switch {
	case held == 1:
		s.showMessage(last + " halted: enemy detected.")
	case held > 1:
		s.showMessage(strconv.Itoa(held) + " units halted: enemy detected.")
	}
}

// haltingTeam reports whether a team's movers obey the halt on contact rule: a human's or a
// computer's, which is every team that actually plays. C++: access.cpp:827.
func (s *Screen) haltingTeam(team int) bool {
	if team < 0 || team >= game.TeamCount {
		return false
	}

	switch s.game.State.Teams[team].Type {
	case game.TeamPlayer, game.TeamComputer:
		return true
	default:
		return false
	}
}

func (s *Screen) tickMovementSfx() {
	if s.sound.driveSfx == nil {
		s.sound.driveSfx = make(map[*game.UnitInfo]int)
	}

	// Units in an active move order at the start of this tick.
	nowDriving := make(map[*game.UnitInfo]bool)

	for _, o := range s.game.MoveOrders() {
		nowDriving[o.Unit] = true
	}

	s.game.Tick()

	// Halt movement when an enemy is detected: cancel a moving player unit's order
	// once a hostile unit comes within its scan range, before recomputing who is
	// still driving (so the halted unit gets its stop sound). C++: the enemy_halt pref.
	if s.enemyHalt {
		s.haltMoversOnEnemy()
	}

	// Units still in an active move order after the tick.
	afterDriving := make(map[*game.UnitInfo]bool)

	for _, o := range s.game.MoveOrders() {
		afterDriving[o.Unit] = true
	}

	// Start the drive sound for units that just began moving.
	for u := range nowDriving {
		if _, driving := s.sound.driveSfx[u]; !driving {
			s.startDriveSfx(u)
		}
	}

	// Repick the held drive clip for units that crossed the land<->water boundary
	// mid move (the clip is chosen once at move start).
	s.reswapDriveSfxOnSurfaceChange(afterDriving)

	// Stop the drive sound for any unit no longer moving (arrived, stalled,
	// cancelled or destroyed) and play its stop sound.
	for u, id := range s.sound.driveSfx {
		if !afterDriving[u] {
			s.stopDriveSfx(u, id)

			// Soldiers (infantry/commando) march; they have no engine "stop" clip, so
			// suppress the generic GEN_STOP fallback (it sounds like a vehicle halting).
			// Also stay silent for a unit that drove out of sight (fog) before stopping.
			if u.Type != assets.Infantry && u.Type != assets.Commando && s.audibleMover(u) {
				s.playSfxForUnit(u, s.movementSfx(u, game.SfxStop))
			}

			// Auto land: an air unit that came to rest ON its friendly landing pad sets
			// down by itself (vanilla lands a plane by MOVING it onto the pad - there is
			// no Land popup). Gated on no pending route so a mid flight budget stop that
			// happens to be over a pad continues next turn instead of landing. The
			// descend glide + set down sound ride the Hovering transition (the AirAlt
			// tween in the idle loop). C++: an air unit's ORDER_MOVE onto a pad lands it.
			if !s.game.HasPendingRoute(u) && s.game.CanLand(u) {
				s.game.LandAir(u)
			}

			// Auto Select Next Unit: when the selected unit arrives with no movement
			// left, advance to the next unit that can still move.
			if u == s.selected && s.autoSelect && u.Speed <= 0 {
				s.selectNextWithMoves()
			}
		}
	}

	s.updateHeldSfxPositions(afterDriving)

	s.sound.movingUnits = afterDriving

	// Follow Unit: keep the camera centred on the selected unit while it moves.
	// C++ ported : game_manager.cpp - the follow_unit preference scrolling the view.
	if s.followUnit && s.selected != nil && afterDriving[s.selected] {
		s.centerOnSelected()
	}

	s.updateAmbientSfx(afterDriving)
}

// audibleMover reports whether the player hears u move: own units always and enemies only while visible.
func (s *Screen) audibleMover(u *game.UnitInfo) bool {
	return s.unitVisibleToPlayer(u)
}

// unitVisibleToPlayer reports whether the human (s.fogTeam) can perceive u: it is
// the player's own unit, or an enemy currently inside the player's vision. Used to
// gate anything that must not reveal a fogged unit (movement SFX, the inspect
// cursor during the opponent's turn).
func (s *Screen) unitVisibleToPlayer(u *game.UnitInfo) bool {
	if u == nil {
		return false
	}

	if u.Team == s.fogTeam {
		return true // your own unit is always visible to you
	}

	return s.game.Vis != nil && s.game.Vis.VisibleToTeam(u, s.fogTeam)
}

// startDriveSfx begins the engine drive sound for a unit just put in motion, recording its
// handle so it can be stopped on arrival. With no held sfx hook it falls back to a fire and
// forget one shot clip. A unit the player cannot hear (an enemy in fog) starts no sound.
func (s *Screen) startDriveSfx(u *game.UnitInfo) {
	if u == nil || !s.audibleMover(u) {
		return
	}

	drive := s.movementSfx(u, game.SfxDrive)

	if s.sound.sfxHeldStartFn == nil {
		s.playSfxForUnit(u, drive)
		return
	}

	tag := units.SfxTagFor(u.Type, drive)

	if tag == "" {
		return
	}

	dbg.Printf("sfx %s : %s %s", displayName(u.Type), sfxKindName(drive), tag)

	pan, volScale := s.unitSfxPosition(u)

	if id := s.sound.sfxHeldStartFn(tag, units.SfxDefaultTag(drive), pan, volScale, false); id != 0 {
		if s.sound.driveSfxType == nil {
			s.sound.driveSfxType = make(map[*game.UnitInfo]game.SfxType)
		}

		s.sound.driveSfx[u] = id
		s.sound.driveSfxType[u] = drive // remember the variant so a land<->water crossing can repick
	}
}

// reswapDriveSfxOnSurfaceChange restarts the held drive clip of any still moving unit whose
// cell crossed the land<->water boundary since the clip was picked, so it swaps to the matching
// variant (e.g. APC5 <-> APC6) instead of keeping the move start clip. Targets are collected
// then restarted so driveSfx is not mutated mid range. afterDriving = units still moving.
func (s *Screen) reswapDriveSfxOnSurfaceChange(afterDriving map[*game.UnitInfo]bool) {
	var reswap []*game.UnitInfo

	for u, id := range s.sound.driveSfx {
		if afterDriving[u] && s.movementSfx(u, game.SfxDrive) != s.sound.driveSfxType[u] {
			s.stopDriveSfx(u, id) // delete during range is safe
			reswap = append(reswap, u)
		}
	}

	for _, u := range reswap {
		s.startDriveSfx(u)
	}
}

// ambientVolumeScale keeps the selected unit ambient sound (a mobile unit's idle
// hum or a building's running hum) in the background; vanilla plays it well
// under the other effects, which also reads as the unit being "farther" in the
// viewport. 1.0 = full SFX volume.
const ambientVolumeScale = 0.5

// stopDriveSfx stops a unit's drive sound and forgets its handle.
func (s *Screen) stopDriveSfx(u *game.UnitInfo, id int) {
	if s.sound.sfxHeldStopFn != nil {
		s.sound.sfxHeldStopFn(id)
	}

	delete(s.sound.driveSfx, u)
	delete(s.sound.driveSfxType, u)
}

// updateAmbientSfx keeps the SELECTED unit's looping ambient sound in sync: an idle mobile
// unit hums its idle sound, a powered operational building hums its running sound, anything
// else is silent. Stops on selection change, move start, power off or deselect. moving = units
// still moving. C++: SFX_TYPE_IDLE (mobile) / SFX_TYPE_POWER_CONSUMPTION_START (buildings).
func (s *Screen) updateAmbientSfx(moving map[*game.UnitInfo]bool) {
	// A power on/off cue belongs to the building it fired for; stop it the instant the
	// player selects something else (or deselects) so it never rings on. Runs every frame.
	if s.sound.powerCueSfx != 0 && s.selected != s.sound.powerCueUnit {
		s.stopPowerCue()
	}

	want := s.selected
	tag, fallback := "", ""

	if want != nil {
		tag, fallback = s.ambientSfxTags(want, moving)

		if tag == "" {
			want = nil
		}
	}

	if want == s.sound.ambientSfxUnit && tag == s.sound.ambientSfxTag {
		return // no change (same unit AND same sound)
	}

	// Trace the selected unit ambient (running/idle hum) so the debug log shows which
	// clip a selected unit loops - or that it is silent (e.g. a passive store).
	if s.selected != nil {
		if tag == "" {
			dbg.Printf("sfx %s : ambient silent", displayName(s.selected.Type))
		} else {
			dbg.Printf("sfx %s : ambient %s", displayName(s.selected.Type), tag)
		}
	}

	if s.sound.ambientSfxID != 0 && s.sound.sfxHeldStopFn != nil {
		s.sound.sfxHeldStopFn(s.sound.ambientSfxID)
	}

	s.sound.ambientSfxID = 0
	s.sound.ambientSfxUnit = want
	s.sound.ambientSfxTag = tag

	if want == nil || s.sound.sfxHeldStartFn == nil {
		return
	}

	pan, volScale := s.unitSfxPosition(want)

	if s.ambientSfxUsesIntroLoop(want, moving) && s.sound.sfxIntroLoopStartFn != nil {
		s.sound.ambientSfxID = s.sound.sfxIntroLoopStartFn(tag, fallback, pan, ambientVolumeScale*volScale)
		return
	}

	s.sound.ambientSfxID = s.sound.sfxHeldStartFn(tag, fallback, pan, ambientVolumeScale*volScale, true)
}

// ambientSfxUsesIntroLoop reports the C++ normal unit SFX path: play the clip once from the
// start, then loop the internal smpl region. Building/work hums start directly on the running
// region. C++ ported : sound_manager.cpp - SoundManager::ProcessJobs loop point setup.
func (s *Screen) ambientSfxUsesIntroLoop(u *game.UnitInfo, moving map[*game.UnitInfo]bool) bool {
	switch {
	case (u.Building && u.State == game.OrderStateBuildInProgress) || u.Clearing:
		return false
	case isStructure(u):
		return false
	case idleEligible(u):
		return !moving[u]
	default:
		return false
	}
}

// ambientSfxTags returns the looping ambient sound (unit specific primary + generic fallback)
// for the selected unit, or "","" when silent: a building hums only while operational (powered
// on); a mobile unit hums its idle sound only while not moving; anything else is silent.
func (s *Screen) ambientSfxTags(u *game.UnitInfo, moving map[*game.UnitInfo]bool) (tag, fallback string) {
	switch {
	case (u.Building && u.State == game.OrderStateBuildInProgress) || u.Clearing:
		// A builder actively raising a structure or a dozer clearing rubble loops its
		// SFX_TYPE_BUILDING working sound. A builder parked build ready (State UNIT_READY)
		// keeps the pose but falls silent. C++: GameManager_MenuUnitSelect gates on
		// ORDER_BUILD/BUILD_IN_PROGRESS or ORDER_CLEAR, not UNIT_READY.
		return units.SfxTagFor(u.Type, game.SfxBuilding), units.SfxDefaultTag(game.SfxBuilding)
	case isStructure(u):
		// A building hums only while RUNNING (powered on, IsActive) and in the hum set
		// (buildingHums); a consumer/producer with no clip falls back to the generic hum.
		// NOT gated on fuel - the original keeps the hum regardless of the pool.
		if !economy.IsActive(u) || !buildingHums(u.Type) {
			return "", ""
		}

		// A factory runs its plant hum only while it is actually producing (drawing
		// power); a powered on but HALTED / empty queue factory is silent, engine off.
		// The original charges a factory's power upkeep only under ORDER_BUILD, so a stopped
		// factory (popup "Start") must not keep playing its running sound.
		if isFactory(u.Type) && !s.game.FactoryProducing(u) {
			return "", ""
		}

		return units.SfxTagFor(u.Type, game.SfxPowerStart), units.SfxDefaultTag(game.SfxPowerStart)
	case idleEligible(u):
		if moving[u] || u.Building {
			// Moving units use the drive sound, not the idle hum; a builder still in
			// its build pose (u.Building) - parked build READY after finishing - falls
			// silent like the original's UNIT_READY (its work is done), even though it is
			// idle eligible. Only a genuinely idle mobile unit hums.
			return "", ""
		}

		idle := s.movementSfx(u, game.SfxIdle) // water idle hum when afloat
		return units.SfxTagFor(u.Type, idle), units.SfxDefaultTag(idle)
	default:
		return "", ""
	}
}

// stopUnitSfx stops every active drive and ambient sound (e.g. on leaving the
// game screen) so none is left playing.
func (s *Screen) stopUnitSfx() {
	for u, id := range s.sound.driveSfx {
		s.stopDriveSfx(u, id)
	}

	if s.sound.ambientSfxID != 0 && s.sound.sfxHeldStopFn != nil {
		s.sound.sfxHeldStopFn(s.sound.ambientSfxID)
	}

	s.sound.ambientSfxID, s.sound.ambientSfxUnit, s.sound.ambientSfxTag = 0, nil, ""

	s.stopPowerCue() // a power on/off cue must not survive leaving the screen (quit to menu)
}

// unitSfxPosition returns the stereo pan and the positional volume factor of a
// unit's sound: its cell offset from the view centre, the zoom, and the
// channels_reversed setting. Without a renderer the sound plays centred at full volume.
// C++ ported : sound_manager.cpp - SoundManager::PlaySfx (grid offset from GameManager_MapView's centre)
func (s *Screen) unitSfxPosition(u *game.UnitInfo) (pan, volScale float64) {
	if s.r == nil || u == nil {
		return 0, 1
	}

	cellX, cellY := s.viewCenterCell()

	return units.SfxPosition(u.GridX-cellX, u.GridY-cellY, s.r.Zoom(), s.sound.channelsReversed)
}

// updateHeldSfxPositions pans and fades every live held sound again for where its
// unit stands now against the view: the drive sounds of units still moving, the
// ambient hum, the power cue and the power bounce. Called once per frame, so
// scrolling or zooming away from a humming building fades it too.
// C++ ported : sound_manager.cpp - SoundManager::UpdateSfxPosition
func (s *Screen) updateHeldSfxPositions(stillDriving map[*game.UnitInfo]bool) {
	for u, id := range s.sound.driveSfx {
		if stillDriving[u] {
			s.repositionHeldSfx(id, u, 1)
		}
	}

	s.repositionHeldSfx(s.sound.ambientSfxID, s.sound.ambientSfxUnit, ambientVolumeScale)
	s.repositionHeldSfx(s.sound.powerCueSfx, s.sound.powerCueUnit, 1)
	s.repositionHeldSfx(s.sound.powerBounceSfx, s.sound.powerBounceUnit, 1)
}

// repositionHeldSfx sends one held sound its unit's current pan and its base
// volume times the positional fade. A zero id or a missing unit does nothing.
func (s *Screen) repositionHeldSfx(id int, u *game.UnitInfo, baseVolume float64) {
	if id == 0 || u == nil {
		return
	}

	pan, volScale := s.unitSfxPosition(u)

	if s.sound.sfxHeldPanFn != nil {
		s.sound.sfxHeldPanFn(id, pan)
	}

	if s.sound.sfxHeldVolumeFn != nil {
		s.sound.sfxHeldVolumeFn(id, baseVolume*volScale)
	}
}

// isStructure reports whether a unit is a building. FlagBuilding marks 2x2 structures; 1x1
// buildings (generator, connector, comm tower) carry FlagStationary without FlagBuilding, so
// both checks are needed. Ground cover slabs are stationary but excluded.
func isStructure(u *game.UnitInfo) bool {
	if u.Flags.Has(game.FlagGroundCover) {
		return false
	}

	return u.Flags.Has(game.FlagBuilding) || u.Flags.Has(game.FlagStationary)
}

// buildingHums reports whether a building type plays a looping running sound on select. SC_UNITS
// (which held the per building clip) is absent from retail data, so we reconstruct the audible
// set: every power consumer/producer (PowerConsumptionRate != 0), plus the radar and habitat
// (which hum in vanilla though they use no power). Passive stores/pad/depot/hangar/dock stay
// silent. C++ ported : units_manager.cpp - ProcessOrderPowerOn; cargo.cpp - Cargo_GetPowerConsumptionRate.
func buildingHums(ut assets.UnitType) bool {
	// Power consumers and producers always run, so they always hum.
	if economy.PowerConsumptionRate(ut) != 0 {
		return true
	}

	// Two operational buildings hum though they draw no power: the radar (its
	// dish rotate RADAR13) and the habitat (the general running hum).
	switch ut {
	case assets.Radar, assets.Habitat:
		return true
	}

	return false
}

// idleEligible reports whether a unit hums an idle sound while selected: only
// mobile units (with an engine) idle; buildings use their power hum instead.
func idleEligible(u *game.UnitInfo) bool {
	return u.Flags.Has(game.FlagMobileLandUnit) ||
		u.Flags.Has(game.FlagMobileSeaUnit) ||
		u.Flags.Has(game.FlagMobileAirUnit)
}
