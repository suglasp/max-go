// services.go : wires the host owned service hooks and the modal screens they open.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/world"
)

// SetScrollSpeed sets edge scroll speed from quick_scroll. The default value 16 maps to 8 pixels per frame. C++: GameManager_QuickScroll.
func (s *Screen) SetScrollSpeed(quickScroll int) {
	if quickScroll < 4 {
		quickScroll = 16 // C++ resets out of range quick_scroll to 16
	}

	if px := quickScroll / 2; px > 0 {
		s.camera.scrollStepPx = px
	}
}

// SetTurnTimer sets the per turn time budget (seconds) from the turn_timer
// setting; <=0 disables the countdown (an untimed turn). C++: the turn timer
// from the game options.
func (s *Screen) SetTurnTimer(seconds int) {
	if seconds < 0 {
		seconds = 0
	}

	s.turnFlow.turnSeconds = seconds
	s.turnFlow.fullTurnSeconds = seconds // the budget each round restores (the End Turn grace shortens the running one)
	s.turnFlow.turnStart = platform.Ticks()
}

// applyTimerPrefs applies the turn timer and End Turn Time settings. The first call (game creation) sets both, as
// the setup does; a custom game or a mission may then set its own. Later calls (the Options menu closing) never
// restart the running turn clock and only follow a setting the player changed: a new turn timer takes effect at the
// next round, a new End Turn Time at the next countdown. An unchanged setting leaves the game's own timers alone.
func (s *Screen) applyTimerPrefs(turnTimer, endTurn int) {
	if !s.turnFlow.prefsApplied {
		s.turnFlow.prefsApplied = true
		s.turnFlow.prefTurnTimer, s.turnFlow.prefEndTurn = turnTimer, endTurn
		s.SetTurnTimer(turnTimer)
		s.SetEndTurnTimer(endTurn)

		return
	}

	if turnTimer != s.turnFlow.prefTurnTimer {
		s.turnFlow.prefTurnTimer = turnTimer
		s.turnFlow.fullTurnSeconds = max(turnTimer, 0) // the next round restores this budget
	}

	if endTurn != s.turnFlow.prefEndTurn {
		s.turnFlow.prefEndTurn = endTurn
		s.SetEndTurnTimer(endTurn)
	}
}

// LivePrefs contains settings a running screen can apply immediately. Plain fields keep gamescreen independent of the settings package.
type LivePrefs struct {
	QuickScroll    int  // edge scroll speed
	TurnTimer      int  // per turn seconds (0 = untimed)
	EndTurn        int  // simultaneous End Turn grace seconds (0 = off)
	AnimateEffects bool // play explosion/impact animations
	DoubleSteps    bool // "Fast Unit Moves": two cells per animation step
	FollowUnit     bool // camera tracks the selected unit's movement
	ClickScroll    bool // edge scroll only while the left button is held
	AutoSelect     bool // auto pick the next unit with moves once one is spent
	EnemyHalt      bool // halt a moving unit when an enemy is spotted
	AutoSave       bool // auto save to the fixed slot at each player turn end
}

// ApplyPrefs applies live game settings to a running screen. The host calls it at game creation and after the Options menu closes; that menu applies audio itself.
func (s *Screen) ApplyPrefs(p LivePrefs) {
	s.SetScrollSpeed(p.QuickScroll)
	s.applyTimerPrefs(p.TurnTimer, p.EndTurn)
	s.effectsOff = !p.AnimateEffects
	s.followUnit = p.FollowUnit
	s.clickScroll = p.ClickScroll
	s.autoSelect = p.AutoSelect
	s.enemyHalt = p.EnemyHalt
	s.autoSave = p.AutoSave

	if s.game != nil {
		s.game.Fast = p.DoubleSteps
	}
}

// SetDebug enables the developer keys (U/A/V/B) and the on screen key hint. The
// host wires this to the DEBUG_GAME build constant; off for a release build.
func (s *Screen) SetDebug(on bool) {
	s.debug = on
}

// SetConfirmFunc installs the yes/no dialog hook (wired to menus.Confirm), used
// by the ESC "OK to exit game?" prompt.
func (s *Screen) SetConfirmFunc(fn func(text string) bool) {
	s.hooks.confirmFn = fn
}

// SetResearchFunc installs the hook a research building's popup calls to open
// the Research screen (wired to menus.RunResearch).
func (s *Screen) SetResearchFunc(fn func(research *[game.ResearchTopicCount]game.ResearchTopic, centers int) error) {
	s.hooks.researchFn = fn
}

// openResearch runs the Research screen for the player team via the hook.
func (s *Screen) openResearch() {
	if s.hooks.researchFn == nil {
		return
	}

	team := s.game.State.PlayerTeam

	if err := s.hooks.researchFn(&s.game.State.Teams[team].Research, s.activeResearchCenters(team)); err != nil {
		dbg.Printf("research: %v", err)
	}

	// Reconcile the centres' run/idle state with the just committed allocation, so
	// the hum and power draw reflect it immediately (not only at the next turn end).
	s.syncResearchOrders(team)
}

// SetUpgradeFunc installs the hook a building's popup calls to open the Upgrade
// screen (wired to menus.RunUpgrade). It returns whether the player committed.
func (s *Screen) SetUpgradeFunc(fn func(units []*game.UpgradeUnit, gold *int) (bool, error)) {
	s.hooks.upgradeFn = fn
}

// upgradableTypes returns the player's buildable unit types plus types the team owns. The Upgrade menu filters them by category.
func (s *Screen) upgradableTypes(team int) []*game.UpgradeUnit {
	type ownedType struct {
		t     assets.UnitType
		flags game.Flags
	}
	seen := map[assets.UnitType]bool{}
	var order []ownedType

	for _, bt := range buildableTypes {
		if !seen[bt.t] {
			seen[bt.t] = true
			order = append(order, ownedType{bt.t, game.FlagStationary | game.FlagBuilding})
		}
	}

	for _, u := range s.game.Units.Units() {
		if u.Team == team && !seen[u.Type] {
			seen[u.Type] = true
			order = append(order, ownedType{u.Type, u.Flags})
		}
	}

	var out []*game.UpgradeUnit

	for _, e := range order {
		base := s.baseValues(e.t)
		uu := &game.UpgradeUnit{
			Type:     e.t,
			Name:     displayName(e.t),
			Base:     base,
			Cur:      s.teamValues(team, e.t),
			Ground:   e.flags.Has(game.FlagMobileLandUnit),
			Air:      e.flags.Has(game.FlagMobileAirUnit),
			Sea:      e.flags.Has(game.FlagMobileSeaUnit),
			Building: e.flags.Has(game.FlagStationary),
			Combat:   base.Get(game.AttribAttack) > 0,
		}

		// Research discount per upgradeable attribute (combat attrs use the
		// team's research level for that topic; ammo has no research topic).
		for topic := 0; topic < game.ResearchTopicCount; topic++ {
			if topic == game.ResearchCost {
				continue
			}

			a := game.Attrib(topic)
			lvl := s.game.State.Teams[team].Research[topic].Level
			uu.ResearchFactor[a] = game.UpgradeResearchFactor(lvl, base.Get(a))
		}

		uu.Attribs = upgradableAttribs(base)

		if len(uu.Attribs) > 0 {
			out = append(out, uu)
		}
	}

	return out
}

// upgradableAttribs lists the upgradable attributes a unit type offers, in the
// menu row order (attack family, then armor/hits/scan, then speed), gating the
// attack family on a nonzero attack value. C++: AddUpgradeMilitary /
// AddUpgradeGeneric / AddUpgradeMobile.
func upgradableAttribs(base *game.UnitValues) []game.Attrib {
	var a []game.Attrib

	if base.Get(game.AttribAttack) > 0 {
		a = append(a, game.AttribAttack)

		if base.Get(game.AttribRounds) > 0 {
			a = append(a, game.AttribRounds)
		}

		if base.Get(game.AttribRange) > 0 {
			a = append(a, game.AttribRange)
		}

		if base.Get(game.AttribAmmo) > 0 {
			a = append(a, game.AttribAmmo)
		}
	}

	if base.Get(game.AttribArmor) > 0 {
		a = append(a, game.AttribArmor)
	}

	if base.Get(game.AttribHits) > 0 {
		a = append(a, game.AttribHits)
	}

	if base.Get(game.AttribScan) > 0 {
		a = append(a, game.AttribScan)
	}

	if base.Get(game.AttribSpeed) > 0 {
		a = append(a, game.AttribSpeed)
	}

	return a
}

// openUpgrade runs the Upgrade screen and, on commit, stores purchased offsets and refreshes the team's in-service units.
func (s *Screen) openUpgrade() {
	if s.hooks.upgradeFn == nil {
		return
	}

	team := s.game.State.PlayerTeam
	units := s.upgradableTypes(team)
	// g.Gold[team] is the persistent team balance, seeded at landing and increased by Gold Refineries.
	goldBefore := s.game.Gold[team]
	committed, err := s.hooks.upgradeFn(units, &s.game.Gold[team])

	if err != nil {
		dbg.Printf("upgrade: %v", err)
		return
	}

	if !committed {
		return
	}

	// Score screen tally: the gold just spent on upgrades (start minus remaining).
	// C++ ported : abstractupgrademenu.cpp - stats_gold_spent_on_upgrades += start_gold - team_gold.
	s.game.RecordUpgradeGold(team, goldBefore-s.game.Gold[team])

	for _, u := range units {
		baseline := s.researchValues(team, u.Type)

		for _, a := range u.Attribs {
			s.game.State.Teams[team].SetUpgradeOffset(u.Type, a, u.Cur.Get(a)-baseline.Get(a))
		}
	}

	for _, u := range s.game.Units.Units() {
		if u.Team == team {
			// Scale current HP by the max HITS delta so a gold bought HITS upgrade keeps a
			// full unit full instead of rendering it as damaged (UnitInfo::UpgradeInt).
			s.upgradeUnitValues(team, u)
		}
	}
}

// SetAllocFunc installs the hook a complex building's popup calls to open the
// Allocate screen (wired to menus.RunAllocate).
func (s *Screen) SetAllocFunc(fn func(c *economy.Complex, rmap *economy.ResourceMap, difficulty int) error) {
	s.hooks.allocFn = fn
}

// openAlloc runs the Allocate screen for unit u's complex via the hook.
func (s *Screen) openAlloc(u *game.UnitInfo) {
	if s.hooks.allocFn == nil || u == nil {
		return
	}

	c := s.game.ComplexOf(u)

	if c == nil {
		s.showMessage("Not part of a complex.")
		return
	}

	if err := s.hooks.allocFn(c, s.game.Resources, s.game.MiningDifficulty); err != nil {
		dbg.Printf("allocate: %v", err)
	}
}

// SetSelfDestructFunc installs the hook a building's popup Remove calls to open
// the SELFDSTR arm/detonate dialog (wired to menus.RunSelfDestruct).
func (s *Screen) SetSelfDestructFunc(fn func() (bool, error)) {
	s.hooks.selfDestructFn = fn
}

// SetSfxFunc installs the hook that plays in game sfx: a unit tag + generic fallback
// tag + stereo pan ([-1, 1], 0 = centre), playing the first that resolves. Wired to the
// audio Manager's PlaySfxPannedFallback. C++: SoundManager::PlaySfx (sound_manager.cpp).
func (s *Screen) SetSfxFunc(fn func(primary, fallback string, pan float64)) {
	s.sound.sfxFn = fn
}

// SetSfxHeldFuncs installs the hooks for held unit sounds (drive sound, idle hum):
// start returns an opaque handle id (0 = none) for the first resolving tag, looping
// when loop is set; stop ends it early; setPan re pans a live sound as its unit moves.
// Wired to the audio Manager's PlaySfxHeld/StopSfxHeld/SetSfxHeldPan; unset falls back
// to a fire and forget single play.
// C++: per unit persistent sfx channel freed by SFX_TYPE_STOP (sound_manager.cpp).
func (s *Screen) SetSfxHeldFuncs(start func(primary, fallback string, pan, volScale float64, loop bool) int, stop func(id int), setPan func(id int, pan float64)) {
	s.sound.sfxHeldStartFn = start
	s.sound.sfxHeldStopFn = stop
	s.sound.sfxHeldPanFn = setPan
}

// SetSfxPositionalFunc installs the hook that plays a unit's one shot sound at a
// stereo pan and a positional volume factor (distance from the view centre and
// zoom). Wired to the audio Manager's PlaySfxPositional; unset falls back to SetSfxFunc.
// C++: SoundManager::PlaySfx (volume_1 = volume_2 * attenuation).
func (s *Screen) SetSfxPositionalFunc(fn func(primary, fallback string, pan, volScale float64)) {
	s.sound.sfxPosFn = fn
}

// SetSfxHeldVolumeFunc installs the hook that fades a live held sound (drive
// sound, hum) as its unit or the view moves. Wired to the audio Manager's SetSfxHeldVolume.
// C++: SoundManager::UpdateSfxPosition.
func (s *Screen) SetSfxHeldVolumeFunc(fn func(id int, volScale float64)) {
	s.sound.sfxHeldVolumeFn = fn
}

// SetChannelsReversed swaps left and right for positional unit sounds (settings
// channels_reversed). C++: SoundManager::GetPanning reading channels_reversed.
func (s *Screen) SetChannelsReversed(reversed bool) {
	s.sound.channelsReversed = reversed
}

// playSfx plays a single sound effect centred (pan=0) by tag, with no fallback,
// if an sfx hook is wired.
func (s *Screen) playSfx(tag string) {
	if s.sound.sfxFn != nil {
		s.sound.sfxFn(tag, "", 0)
	}
}

// playSfxPanned plays the first resolvable of primary/fallback at the given pan
// if an sfx hook is wired.
func (s *Screen) playSfxPanned(primary, fallback string, pan float64) {
	if s.sound.sfxFn != nil {
		s.sound.sfxFn(primary, fallback, pan)
	}
}

// openSelfDestruct runs the self destruct dialog for u and, on confirmed
// detonation, blows it up and removes it. With no hook wired it destroys
// immediately (the prior direct Remove behaviour).
func (s *Screen) openSelfDestruct(u *game.UnitInfo) {
	if u == nil {
		return
	}

	detonate := true

	if s.hooks.selfDestructFn != nil {
		ok, err := s.hooks.selfDestructFn()

		if err != nil {
			dbg.Printf("self-destruct: %v", err)
			return
		}

		detonate = ok
	}

	if detonate {
		s.destroyUnit(u)
	}
}

// TurnTimerSnapshot returns the turn time for the Save/Load screen's frozen MM:SS readout. Untimed turns and the main-menu load path show nothing.
// C++ ported : game_manager.cpp - GameManager_DrawTimer reads GameManager_TurnTimerValue.
func (s *Screen) TurnTimerSnapshot() (seconds int, show bool) {
	if s.turnFlow.turnSeconds <= 0 {
		return 0, false
	}

	return s.turnRemaining(), true
}

// openMenuScreen runs a menu modal and refreshes the game view on return. Fullscreen screens replace bgWindow and its palette; windowed dialogs retain the game palette.
func (s *Screen) openMenuScreen(fn func(), fullscreen bool) {
	s.installCursor(cursors.Hand) // modals run their own loop; don't leak a context cursor
	// Only the FULLSCREEN menu screens (Files/Reports/FACBUILD/...) drop the
	// wide view to 4:3 while they run - their 640 art covers the whole screen.
	// Windowed dialogs (Preferences, Goal, Research, X-Fer, ...) draw over the
	// live game and KEEP the wide view, centered on it.
	wasWide := s.wideMode && fullscreen

	if wasWide {
		s.setWideMode(false)
	}

	s.withTurnClockPaused(fn) // pause the turn timer while the modal is up (real time passes in its own loop)

	if wasWide {
		s.setWideMode(true)
	}

	// The closing click (a button firing on press) is still down on return; eat
	// its release so it does not bleed into the map as a move/build/attack.
	s.orderInput.swallowRelease = true
	buf := s.sys.Buf(bgWindow)

	if fullscreen {
		s.plat.SetPalette(&s.pal)
		s.repaintBackground(buf)
	}

	s.draw(buf)
}

// screenHooks holds callbacks for menu screens, movie and music playback, and world loading. Voice and sound effect callbacks live with their own state.
type screenHooks struct {
	movieFn        func(tag string) error                                                                                                            // plays an MVE resource
	buildFn        func(builder assets.UnitType, team, storage int) (screendata.BuildSelection, bool, error)                                         // opens the construction menu
	factoryFn      func(factory assets.UnitType, team, storage int, queue []assets.UnitType, repeat bool) (screendata.FactorySelection, bool, error) // opens the production menu
	statusFn       func(u *game.UnitInfo, name string) error                                                                                         // opens unit status
	optionsFn      func() error                                                                                                                      // opens preferences
	saveLoadFn     func(saving bool) (quit bool, err error)                                                                                          // opens save or load
	researchFn     func(research *[game.ResearchTopicCount]game.ResearchTopic, centers int) error                                                    // opens research allocation
	allocFn        func(c *economy.Complex, rmap *economy.ResourceMap, difficulty int) error                                                         // opens resource allocation
	upgradeFn      func(units []*game.UpgradeUnit, gold *int) (bool, error)                                                                          // opens unit upgrades
	transferFn     func(info screendata.TransferInfo) (int, error)                                                                                   // opens unit transfer
	confirmFn      func(text string) bool                                                                                                            // asks for confirmation
	selfDestructFn func() (bool, error)                                                                                                              // handles self destruct confirmation
	repairFn       func(ops RepairShopOps) error                                                                                                     // opens the repair shop
	reportsFn      func(data screendata.ReportData) (uint16, error)                                                                                  // opens reports; returns a clicked unit's id (0 = none)
	helpFn         func(section string)                                                                                                              // shows contextual help
	messageFn      func(text string)                                                                                                                 // shows a modal message box with one OK (GOAL info, Alt+P pause)
	musicFn        func(tag string, loop bool)                                                                                                       // plays music
	worldReloader  func(mapFile string) (*world.Map, error)                                                                                          // reloads a world map
}
