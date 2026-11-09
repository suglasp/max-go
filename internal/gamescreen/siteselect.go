// siteselect.go : implements the build site selector.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - build mode footprint highlight
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/world"
)

// siteselect.go is the interactive "Select starting location" phase that runs
// before the landing descent: a MASTER lander hovers over a candidate cell, the
// player pans the map and clicks to reposition it, then confirms to deploy the
// base there. This selector accepts any valid land site and does not apply
// proximity rings or enemy distance restrictions.
//
// C++ ported : game_manager.cpp - GameManager_SelectSite / GameManager_Player-
// MissionSetup (spawn MASTER, ORDER_IDLE; reposition; confirm -> deploy).

// turnPhase is the in game screen's high level state. It is derived from the
// existing flags (it does not store separate state) and gives the run loop and
// renderer one vocabulary to branch on. C++: the GAME_STATE_* subset
// (7_SITE_SELECT / 8_IN_GAME / 9_END_TURN) the screen actually distinguishes.
type turnPhase int

const (
	phaseInGame     turnPhase = iota // normal interactive play
	phaseSiteSelect                  // choosing a landing site (pre landing)
	phaseLanding                     // the camera descent / base unfold animation
	phaseLeaving                     // the planet leaving (panels close) animation
	phaseHandover                    // a hot seat pass banner is up
	phaseGameOver                    // win/loss result is shown
)

// phase reports the screen's current high level state, highest priority first.
func (s *Screen) phase() turnPhase {
	switch {
	case s.gameOver:
		return phaseGameOver
	case s.camera.leavingStart != 0:
		return phaseLeaving
	case s.siteSelect != nil:
		return phaseSiteSelect
	case s.camera.landingStep > 0:
		return phaseLanding
	case s.turnFlow.handover != -1:
		return phaseHandover
	default:
		return phaseInGame
	}
}

// Landing proximity ranges (grid cells), from the original DEBUG settings: the
// exclude range is the red ring an enemy start must stay outside; the proximity
// range is the yellow ring inside which the game warns. C++: settings.cpp
// {"exclude_range",{3}} / {"proximity_range",{14}}; GameManager_CheckLandingZones.
const (
	landingExcludeRange   = 3
	landingProximityRange = 14
)

// landingIntroMS is how long the proximity ring grow animation plays after the
// player confirms a site, before the camera descent begins. landingRingFastFrac
// is the fraction of that window in which the (larger) yellow ring reaches full
// size - it grows faster than the inner red ring, which fills over the whole
// window. C++: the GameManager_DrawProximityZones expanding circle animation.
const (
	landingIntroMS     = 600
	landingRingFastNum = 3 // yellow ring fills in 3/5 of the window
	landingRingFastDen = 5
)

// Landing panel slide: after the rings finish, the minimal left panels slide
// apart (top up, bottom down) to reveal the full game panel, over landingSlideMS.
// The bottom panel is the taller of the two, so its height is the full travel.
// C++: MenuLandingSequence::OpenPanel (12 steps) + the WINDOW_INTERFACE_PANEL
// split at y=239. landingPanelSplitY is where the top panel ends / bottom begins.
const (
	landingSlideMS     = 500
	landingPanelSplitY = 239
)

// siteSelectState holds the live landing site selection: the candidate cell (the
// mining station anchor), the MASTER lander drawn there, the other teams' chosen
// start cells (for the proximity check), and the closure that deploys this
// team's base at the chosen cell and starts the descent. introStart is 0 while
// the player is still picking and the confirm tick timestamp once the landing is
// committed (the ring grow animation is playing).
type siteSelectState struct {
	cx, cy     int
	master     *game.UnitInfo
	enemy      []game.Point
	confirm    func(cx, cy int)
	introStart uint64 // ring grow stage start (0 while still picking)
	slideStart uint64 // panel slide stage start (0 until the rings finish)
}

// BeginSiteSelect enters the site select phase for a team: it spawns a MASTER
// lander at the candidate cell, shows the planet zoomed for picking, and centres
// the view. confirm is called with the final cell when the player accepts.
// C++: GameManager_SelectSite (UnitsManager_SpawnUnit(MASTER, ...) + ORDER_IDLE).
func (s *Screen) BeginSiteSelect(team, cx, cy int, enemy []game.Point, confirm func(cx, cy int)) {
	master := game.NewUnitInfo(assets.Master, team, s.spawnSeq+1, 0)
	master.Flags = game.FlagStationary | game.FlagBuilding
	master.Order = game.OrderIdle
	s.spawnSeq++
	s.game.Units.Deploy(master, cx, cy)
	s.siteSelect = &siteSelectState{cx: cx, cy: cy, master: master, enemy: enemy, confirm: confirm}
	s.selected = nil // the lander is a marker, not a selectable unit
	s.r.SetZoom(landingStartZoom * 2)
	s.centerSiteSelect()
	s.playVoice(vsSelectSite) // "Select site" / "Choose site" announcer line
}

// centerSiteSelect scrolls the view to the candidate cell.
func (s *Screen) centerSiteSelect() {
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(s.siteSelect.cx, s.siteSelect.cy, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
}

// validSite reports whether (cx, cy) is a usable landing anchor: in bounds with a
// map edge margin, and its 2x2 base footprint is all land. C++: GameManager_Is-
// ValidStartingPosition (a land block clear of the map edge).
func (s *Screen) validSite(cx, cy int) bool {
	if cx < 2 || cy < 2 || cx >= s.wmap.W-3 || cy >= s.wmap.H-3 {
		return false
	}

	for dy := 0; dy < 2; dy++ {
		for dx := 0; dx < 2; dx++ {
			if s.wmap.SurfaceAt(cx+dx, cy+dy)&world.SurfaceLand == 0 {
				return false
			}
		}
	}

	return true
}

// confirmSiteAt moves the candidate to a clicked cell and confirms the landing
// there (a click both positions and commits, as in vanilla). An invalid cell is
// rejected with a banner.
func (s *Screen) confirmSiteAt(cx, cy int) {
	if !s.validSite(cx, cy) {
		s.showBanner("Cannot land there - pick a clear inland spot.")
		return
	}

	ss := s.siteSelect
	ss.cx, ss.cy = cx, cy
	s.game.Units.Move(ss.master, cx, cy)
	s.confirmSite()
}

// tooCloseToEnemy reports whether (cx, cy) sits within the exclude range of any
// other team's chosen start cell - a landing that is not allowed. The test is
// per axis (Chebyshev), matching the original. C++: GameManager_CheckLandingZones
// (distance_x <= exclude_range && distance_y <= exclude_range -> status 3).
func (s *Screen) tooCloseToEnemy(cx, cy int) bool {
	for _, e := range s.siteSelect.enemy {
		if iabsInt(cx-e.X) <= landingExcludeRange && iabsInt(cy-e.Y) <= landingExcludeRange {
			return true
		}
	}

	return false
}

// confirmSite commits the chosen landing site: a site too close to an enemy
// start is rejected (the player must pick another). Otherwise it starts the
// ring grow intro animation - the proximity rings expand, the confirm sound
// plays, and the pointer becomes the hand - which tickSiteIntro finishes by
// deploying the base. C++: GameManager_SelectSite accepting the site (PlaySfx +
// DrawProximityZones), then GameManager_ProcessTeamMissionSupplyUnits.
func (s *Screen) confirmSite() {
	ss := s.siteSelect

	if ss == nil || ss.introStart != 0 {
		return // not selecting, or the landing animation is already playing
	}

	if s.tooCloseToEnemy(ss.cx, ss.cy) {
		// A blocking placement error (the deploy exclusion zone overlap): the original reds
		// it (type 2, game_manager.cpp:1430 _(3a97)). No log (site select phase).
		s.showAlertBanner("Too close to the enemy - choose another site.")
		return
	}

	ss.introStart = platform.Ticks()
	s.installCursor(cursors.Hand) // pointer switches to the hand on confirm

	if s.sound.sfxFn != nil {
		s.sound.sfxFn("NDONE0", "", 0) // the confirm cue
	}
}

// playLandingMovie plays the custom game intro cinematic (the launcher set
// landingMovieTag) once the landing rings have grown, before the panels slide open.
// Does nothing unless a tag is set (custom game only) and movies are enabled (-nomovie).
// C++ ported : game_manager.cpp - Movie_Play(DEMO1FLC) on a MISSION_CATEGORY_CUSTOM landing.
func (s *Screen) playLandingMovie() {
	if s.noMovie || s.landingMovieTag == "" || s.hooks.movieFn == nil {
		return
	}

	_ = s.hooks.movieFn(s.landingMovieTag) // non fatal: a missing movie just skips
}

// tickSiteIntro advances the ring grow animation; once the window elapses it
// removes the MASTER marker and runs the deploy closure (which lands the base
// and begins the camera descent). The rings themselves are drawn by
// drawSiteSelect from introStart.
func (s *Screen) tickSiteIntro() {
	ss := s.siteSelect
	now := platform.Ticks()

	if ss.slideStart == 0 {
		// Ring grow stage: when the landing rings finish, play the custom game intro
		// cinematic (MAXMVE1, set by the launcher) in its own window, THEN start the
		// panel slide (the left panels part to reveal the full game UI) with its sound.
		if now-ss.introStart >= landingIntroMS {
			s.playLandingMovie()
			ss.slideStart = platform.Ticks() // a fresh tick: the movie blocks the loop

			if s.sound.sfxFn != nil {
				s.sound.sfxFn("IOPEN0", "", 0) // the panel open slide sound
			}
		}

		return
	}

	// Panel slide stage: once the panels have parted, deploy + start the descent.
	if now-ss.slideStart >= landingSlideMS {
		s.finishSiteIntro()
	}
}

// landingPanelOffset is how far the minimal panels have slid apart (0 = closed,
// covering the full panel; rising to the panel height = fully open). It is 0
// while picking and during the ring grow stage, then ramps over the slide stage.
func (s *Screen) landingPanelOffset() int {
	ss := s.siteSelect

	if ss == nil || ss.slideStart == 0 {
		return 0
	}

	travel := landingPanelSplitY + 2 // enough for both halves to clear the screen
	p := float64(platform.Ticks()-ss.slideStart) / float64(landingSlideMS)

	if p > 1 {
		p = 1
	}

	return int(p * float64(travel))
}

// Cancel / Help button positions on the bottom landing panel (top left corner),
// from MenuLandingSequence::Init. They show only while picking (vanilla deletes
// them when the panel opens).
const (
	landingCancelX = 40
	landingHelpX   = 103
	landingBtnY    = 252
)

func (s *Screen) landingCancelRect() gfx.Rect {
	if s.hudArt.pnlCancelArt == nil {
		return gfx.Rect{}
	}

	return gfx.Rect{ULX: landingCancelX, ULY: landingBtnY, LRX: landingCancelX + s.hudArt.pnlCancelArt.W - 1, LRY: landingBtnY + s.hudArt.pnlCancelArt.H - 1}
}

func (s *Screen) landingHelpRect() gfx.Rect {
	if s.hudArt.pnlHelpArt == nil {
		return gfx.Rect{}
	}

	return gfx.Rect{ULX: landingHelpX, ULY: landingBtnY, LRX: landingHelpX + s.hudArt.pnlHelpArt.W - 1, LRY: landingBtnY + s.hudArt.pnlHelpArt.H - 1}
}

// cancelLanding aborts site selection and returns to the previous menu (the
// cargo/purchase screen for a custom game): it plays the cancel cue, removes the
// MASTER marker, and asks the run loop to leave the game screen. C++:
// MenuLandingSequence button_2 (NCANC0) -> GameManager_SelectSite returns false.
func (s *Screen) cancelLanding() {
	if s.sound.sfxFn != nil {
		s.sound.sfxFn("NCANC0", "", 0)
	}

	s.cancelSiteSelect()
	s.exitToMenu = true
}

// startLeaving begins the planet leaving animation (the reverse landing): hand
// cursor, the close sound, then the panels slide shut over the next
// landingSlideMS. tickLeaving exits to the menu when it finishes. C++:
// MenuLandingSequence::ClosePanel (ICLOS0 + AnimateStep in reverse).
func (s *Screen) startLeaving() {
	s.camera.leavingStart = platform.Ticks()
	s.installCursor(cursors.Hand)

	if s.sound.sfxFn != nil {
		s.sound.sfxFn("ICLOS0", "", 0)
	}
}

// tickLeaving leaves the game screen once the close animation has finished.
func (s *Screen) tickLeaving() {
	if platform.Ticks()-s.camera.leavingStart >= landingSlideMS {
		s.exitToMenu = true
	}
}

// leavingPanelOffset ramps the panel offset from fully open (travel) back to 0
// (closed) over the leaving window - the reverse of landingPanelOffset.
func (s *Screen) leavingPanelOffset() int {
	travel := landingPanelSplitY + 2
	p := float64(platform.Ticks()-s.camera.leavingStart) / float64(landingSlideMS)

	if p > 1 {
		p = 1
	}

	return int((1 - p) * float64(travel))
}

// drawLandingPanels overlays the minimal landing panels over the left column at
// the current slide offset (0 = closed/covering, travel = open/off screen). On
// landing they part (open); on leaving they slide back (close). Drawn after the
// full panel so the full UI shows underneath. C++: MenuLandingSequence::Animate-
// Step (PANELTOP at uly offset, PANELBTM at uly+offset).
func (s *Screen) drawLandingPanels(buf *gfx.Buffer) {
	var off int

	switch {
	case s.siteSelect != nil:
		off = s.landingPanelOffset()
	case s.camera.leavingStart != 0:
		off = s.leavingPanelOffset()
	default:
		return
	}

	blitPanelClipped(buf, s.hudArt.panelTopArt, 0, -off)
	blitPanelClipped(buf, s.hudArt.panelBtmArt, 0, landingPanelSplitY+off)

	// Cancel / Help buttons, only while picking (the panel opening removes them).
	if s.siteSelect != nil && s.siteSelect.slideStart == 0 {
		blitButtonArt(buf, s.hudArt.pnlCancelArt, gfx.Rect{ULX: landingCancelX, ULY: landingBtnY})
		blitButtonArt(buf, s.hudArt.pnlHelpArt, gfx.Rect{ULX: landingHelpX, ULY: landingBtnY})

		if a := s.hudArt.pnlCancelArt; a != nil { // "Cancel" caption (the art is a blank face)
			f := s.sys.Fonts().Get(fonts.Font5)
			s.blitStatText(buf, landingCancelX, landingBtnY, a.W, a.H, f, s.lang.Get("773a", "Cancel"), colorPopupGold, true)
		}
	}
}

// blitPanelClipped copies art to buf at (dx, dy), clipping when it runs off
// any edge (so a panel slid partly off screen blits only its visible part).
// A transparent copy: MAX's sprite convention keys transparent pixels to
// index 0 (KeyToZero), and these flipbook/panel frames carry a transparent
// border column/row, so an opaque copy would paint it as solid black over
// the freshly repainted background underneath instead of letting it show
// through - most visible in widescreen, where the shifted position no
// longer sits next to whatever masked it in the original 640-wide layout.
func blitPanelClipped(buf, art *gfx.Buffer, dx, dy int) {
	if art == nil {
		return
	}

	sx, sy, w, h := 0, 0, art.W, art.H

	if dx < 0 {
		sx, w, dx = -dx, w+dx, 0
	}

	if dy < 0 {
		sy, h, dy = -dy, h+dy, 0
	}

	if dx+w > buf.W {
		w = buf.W - dx
	}

	if dy+h > buf.H {
		h = buf.H - dy
	}

	if w <= 0 || h <= 0 {
		return
	}

	buf.Sub(dx, dy, w, h).TransCopy(art.Sub(sx, sy, w, h))
}

// finishSiteIntro removes the MASTER marker, clears the site select state, and
// runs the deploy closure (lands the base + starts the descent).
func (s *Screen) finishSiteIntro() {
	ss := s.siteSelect
	cx, cy, confirm := ss.cx, ss.cy, ss.confirm
	s.game.Units.Remove(ss.master)
	s.siteSelect = nil

	if s.sys != nil {
		s.repaintBackground(s.sys.Buf(bgWindow)) // restore the full UI frame after the slide
	}

	confirm(cx, cy)
}

// cancelSiteSelect aborts selection (used when the player abandons to the menu),
// removing the MASTER marker so it does not linger.
func (s *Screen) cancelSiteSelect() {
	if s.siteSelect != nil {
		s.game.Units.Remove(s.siteSelect.master)
		s.siteSelect = nil
	}
}

// handleSiteSelectMouse pans the map at the screen edge and repositions the
// candidate on a left click in the map view.
func (s *Screen) handleSiteSelectMouse() {
	mouse := s.plat.Mouse()
	step := scrollStep

	if s.camera.scrollStepPx > 0 {
		step = s.camera.scrollStepPx
	}

	dx, dy := edgeScroll(mouse.X, mouse.Y, step)
	s.camera.scrollX += dx
	s.camera.scrollY += dy
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)

	if mouse.Buttons&platform.MouseReleaseLeft != 0 {
		if inRect(s.landingCancelRect(), mouse.X, mouse.Y) {
			s.cancelLanding() // back to the cargo screen
			return
		}

		if inRect(s.landingHelpRect(), mouse.X, mouse.Y) {
			s.showBanner("Click a clear land tile to land your forces there.")
			return
		}

		if cx, cy, ok := s.mapCell(mouse.X, mouse.Y); ok {
			s.confirmSiteAt(cx, cy) // a click lands the base at that cell
		}
	}
}

// drawSiteSelect draws the proximity rings around the candidate: a yellow
// proximity ring and an inner red exclude ring, centred on the anchor cell's
// own centre - the same point the MASTER lander itself is drawn at (its
// UnitInfo.X/Y come from SetPosition's gridX*tile + tile/2, the same
// convention every unit uses; a 2x2 footprint does not shift it). C++:
// GameManager_DrawProximityZones / GameManager_DrawCircle centre on
// GameManager_SelectedUnit->x/y (UnitInfo::SetPosition, unitinfo.cpp:1276 -
// also just gridX*tile + tile/2, footprint agnostic).
func (s *Screen) drawSiteSelect(view *gfx.Buffer) {
	ss := s.siteSelect

	if ss == nil || ss.introStart == 0 {
		return // the rings appear only once the landing is confirmed
	}

	z := s.r.Zoom()
	cx, cy := siteSelectRingCenter(ss.cx, ss.cy, z, s.camera.scrollX, s.camera.scrollY)
	// Grow both rings from the centre over the intro window; the larger yellow
	// proximity ring fills faster than the inner red exclude ring.
	p := float64(platform.Ticks()-ss.introStart) / float64(landingIntroMS)

	if p > 1 {
		p = 1
	}

	yellowP := p * landingRingFastDen / landingRingFastNum

	if yellowP > 1 {
		yellowP = 1
	}

	view.DrawCircleClip(cx, cy, int(float64(landingProximityRange*z)*yellowP), gfx.ColorYellow)
	view.DrawCircleClip(cx, cy, int(float64(landingExcludeRange*z)*p), gfx.ColorRed)
}

// siteSelectRingCenter returns the screen pixel of the anchor cell's own
// centre at the given zoom and scroll - the same point every unit (MASTER
// included) is drawn at, per UnitInfo.SetPosition's gridX*tile + tile/2.
func siteSelectRingCenter(cellX, cellY, zoom, scrollX, scrollY int) (int, int) {
	return cellX*zoom - scrollX + zoom/2, cellY*zoom - scrollY + zoom/2
}

// landingStepMS paces the landing descent (one zoom doubling + one unfold step
// per tick); landingStartZoom is the orbit zoom the descent begins from. C++:
// GameManager_GameSetup climbs the zoom 4 -> 64 at ~48fps.
const (
	landingStepMS    = 130
	landingStartZoom = 4
	// The post load focus zoom (focusOnLoadedBase) ramps LINEARLY: loadZoomInc pixels per cell every loadZoomPaceMS,
	// for a smooth, unhurried glide to the base instead of a single doubling snap.
	loadZoomPaceMS = 33
	loadZoomInc    = 1
)

// StartLanding begins the landing sequence: camera zoomed out to targetZoom while
// the landed structures unfold, input locked until it finishes.
// C++: GameManager_GameSetup's zoom climb + UnitsManager_ScaleUnit(EXPAND).
func (s *Screen) StartLanding(targetZoom int) {
	if targetZoom < landingStartZoom {
		targetZoom = landingStartZoom
	}

	s.camera.landingZoom = targetZoom
	s.camera.landingSmooth = false // the new game landing uses the orbit doubling descent
	s.r.SetZoom(landingStartZoom)
	steps := 1 // a final settle step so the full size frame shows

	for z := landingStartZoom; z < targetZoom; z *= 2 {
		steps++
	}

	s.camera.landingStep = steps
	s.camera.landingStamp = platform.Ticks()

	// Every just landed structure unfolds from small to full (the lander opening).
	for _, u := range s.game.Units.Units() {
		if u.Flags.Has(game.FlagStationary) && !u.Flags.Has(game.FlagGroundCover) {
			u.Unfold = 4
		}
	}

	s.centerOnTeam(s.game.State.ActiveTeam)
}

// tickLanding advances the landing sequence one step: zoom toward the target,
// shrink each unit's unfold counter, re centre. Clears landingStep once done.
func (s *Screen) tickLanding() {
	pace := landingStepMS
	if s.camera.landingSmooth {
		pace = loadZoomPaceMS // the post load focus glides faster + in finer steps
	}

	if platform.Ticks()-s.camera.landingStamp < uint64(pace) {
		return
	}

	s.camera.landingStamp = platform.Ticks()
	if z := s.r.Zoom(); z < s.camera.landingZoom {
		if s.camera.landingSmooth {
			s.r.SetZoom(min(z+loadZoomInc, s.camera.landingZoom)) // a smooth LINEAR ramp
		} else {
			s.r.SetZoom(min(z*2, s.camera.landingZoom)) // the orbit DOUBLING descent
		}
	}

	for _, u := range s.game.Units.Units() {
		if u.Unfold > 0 {
			u.Unfold--
		}
	}

	if s.camera.landingSmooth {
		// A loaded base's units can be spread across the map, and centerOnTeam follows
		// the RANDOM order "first" unit - which jumps the view every frame. Re centre on
		// the stable building CENTROID instead so the zoom glides cleanly to the base.
		s.centerOnPlayerBase()
	} else {
		s.centerOnTeam(s.game.State.ActiveTeam) // the new game landing: units are clustered at the site
	}

	if s.camera.landingStep--; s.camera.landingStep <= 0 {
		s.camera.landingStep = 0
		s.camera.landingSmooth = false // back to the normal (doubling) descent for any later landing
		for _, u := range s.game.Units.Units() {
			u.Unfold = 0 // guarantee full size when control returns
		}
		s.turnFlow.turnStart = platform.Ticks() // start the turn clock on hand over
		s.logTurnClock("landing done, control handed over")
	}
}
