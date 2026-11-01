// gamescreen.go : implements the in-game layout, map viewport, minimap, turn loop, and unit interaction.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : game_manager.cpp - GameManager_GameLoop; window_manager.cpp - WindowManager_RunGame
package gamescreen

import (
	"fmt"
	"time"

	"github.com/suglasp/max-go/internal/ai"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/paths"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/winloss"
	"github.com/suglasp/max-go/internal/world"
)

// bgWindow is the full-screen background window created by GNW.
// Menus and the game screen draw into it.
const bgWindow gnw.WinID = 1

// mapArtState caches lazily built resource markers, status icons, and path arrows.
type mapArtState struct {
	resTileNative  map[string]*gfx.Buffer // loaded GOLDMK/FUELMK/RAWMSK marker images
	resTileScaled  map[string]*gfx.Buffer // markers scaled to zoom, keyed "TAG@zoom"
	statusIcons    map[string]*gfx.Buffer // STATUS overlay icons (IL_SPEED/IL_SHOTS/IL_DSBLD), keyed by tag
	pathArrowCache map[string]*gfx.Buffer // pre rendered move route arrows, keyed "color dir@zoom"
}

// Screen drives the in game view loop for one loaded map.
type Screen struct {
	sys         *gnw.System        // GNW window and input system
	plat        *platform.Platform // host platform services
	wmap        *world.Map         // terrain and water map
	r           *drawmap.Renderer  // terrain renderer
	bg          *gfx.Buffer        // active FRAMEPIC panel art (narrow or wide), or nil
	wideMode    bool               // active widescreen layout
	bgNarrow    *gfx.Buffer        // narrow FRAMEPIC art
	bgWide      *gfx.Buffer        // lazily built widescreen FRAMEPIC art
	lib         units.Reader       // game resource reader
	game        *sim.Game          // game simulation
	sprites     *units.SpriteCache // unit sprite cache
	lang        assets.Strings     // in game text strings
	pal         gfx.Palette        // active game palette
	colorOrange uint8              // attack reticle palette index
	ai          *ai.AI             // computer opponent
	makeUnit    sim.UnitFactory    // unit factory used by AI and production

	selected  *game.UnitInfo   // selected unit or group leader
	selection []*game.UnitInfo // selected units, leader first

	siteSelect  *siteSelectState // starting location picker
	cursorMgr   *cursors.Manager // software cursor manager
	cursorStamp uint64           // last cursor animation timestamp
	gameOver    bool             // locks input after the game ends
	outcome     winloss.Outcome  // win or loss result
	exitToMenu  bool             // returns Run to the menu

	baseDir string // save directory
	mapName string // map name stored in saves

	gameGoal          string // GOAL panel text
	missionTitle      string // a scripted mission's title, kept so a save can put its briefing back (v22)
	missionBriefing   string // a scripted mission's DESCR text, the other half of the same
	noMovie           bool   // skips win and loss movies
	winMovieTag       string // victory movie resource tag
	loseMovieTag      string // defeat movie resource tag
	landingMovieTag   string // landing movie resource tag
	alienDerelicts    int    // custom game derelict density
	customOpponents   int    // custom game opponent count
	resourceLevels    [3]int // raw, fuel, and gold density
	resourceLevelsSet bool   // custom resource levels are set
	startCredit       int    // the game's landing credit: the bench reserve (-startcredit) and every computer team's upgrade budget
	startCreditSet    bool   // a start credit was passed (the setup screen, or -startcredit)

	grid               bool                 // grid overlay toggle
	showNames          bool                 // unit name toggle
	surveyOverlay      bool                 // surveyed resource overlay toggle
	showColors         bool                 // team colour bracket toggle
	showStatus         bool                 // unit status icon toggle
	showHits           bool                 // hit point bar toggle
	showAmmo           bool                 // ammo bar toggle
	showRange          bool                 // weapon range toggle
	showScan           bool                 // scan range toggle
	effectsOff         bool                 // disables optional effects
	followUnit         bool                 // follows the selected unit
	clickScroll        bool                 // requires a click for edge scrolling
	autoSelect         bool                 // selects the next movable unit
	enemyHalt          bool                 // stops movement when an enemy appears
	autoSave           bool                 // saves after each player turn
	accessOverlay      bool                 // inaccessible cell overlay toggle
	fogTeam            int                  // team used for fog drawing
	fogShow            bool                 // fog overlay toggle
	maxSurvey          bool                 // reveals all resource markers
	showAStarPath      bool                 // active A* route toggle
	showAIPaths        bool                 // computer route toggle
	showSentryCoverage bool                 // guarded zones of the player's sentries (F2 toggle, no vanilla equivalent)
	cheatInput         string               // cheat code input buffer
	keybinds           [bindActionCount]int // remappable action keys
	unitGroups         [9][]uint16          // numbered unit control groups

	cycleStamps     []uint64 // terrain colour cycle timestamps
	pulse           uint8    // current pulse animation value
	turretStamp     uint64   // turret animation timestamp
	bridgeStamp     uint64   // bridge animation timestamp
	spinStamp       uint64   // spinning turret timestamp
	bodyStamp       uint64   // building body animation timestamp
	walkStamp       uint64   // soldier walk animation timestamp
	buildSlideStamp uint64   // build pose animation timestamp

	demoMode        bool   // attract demo mode
	demoTurnStart   int    // first attract demo turn
	demoLastButtons uint32 // previous attract demo mouse buttons

	debug         bool           // developer controls toggle
	walkAnimAll   bool           // animates all soldiers in place
	galleryLabels []galleryLabel // asset gallery labels
	spawnSeq      int            // debug unit sequence number

	announcer  announcerState  // announcer queue and memory
	sound      soundState      // sound effect state
	minimap    minimapState    // minimap state
	debugMenu  debugMenuState  // developer menu state
	dragBox    dragBoxState    // drag selection state
	portraitUI portraitState   // corner portrait state
	banner     bannerState     // message banner state
	effects    effectsState    // combat, build, and transport effects
	mapArt     mapArtState     // map overlay art cache
	hudArt     hudArtState     // HUD art and button state
	panelAnim  panelAnimState  // instrument bars unfolding at game start
	orderInput orderInputState // click and order input state
	buildUI    buildUIState    // build placement state
	hooks      screenHooks     // host callbacks
	camera     cameraState     // viewport state
	turnFlow   turnFlowState   // turn flow and watchdog state
	bench      benchState      // -emulate bench stops (Tab)
}

// New builds the screen for a loaded map.
// basePal is the game UI palette from FRAMEPIC or COLOR.PAL, with map terrain entries 64 through 159 installed over it.
// bg is the optional 640x480 panel frame drawn behind the live windows.
func New(sys *gnw.System, plat *platform.Platform, m *world.Map, basePal gfx.Palette, bg *gfx.Buffer, lib units.Reader, lang assets.Strings) *Screen {
	pal := basePal
	m.ApplyTerrainPalette(&pal)
	r := drawmap.New(m)

	// New always starts in 4:3; Run switches to wide afterward.
	// The viewport floor keeps small maps filling the map view at minimum zoom.
	r.SetViewport(viewSize(MainMap))
	r.SetZoom(16)

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	s := &Screen{
		sys:      sys,
		plat:     plat,
		wmap:     m,
		r:        r,
		bg:       bg,
		bgNarrow: bg,
		pal:      pal,
		// Grid overlay defaults OFF, matching the original (GameManager_DisplayButtonGrid
		// is a zero initialised global); the GRID toggle button turns it on.
		grid:        false,
		cycleStamps: make([]uint64, len(world.ColorCycles)),
		game:        sim.New(p, m),
		sprites:     units.NewSpriteCache(lib, &pal),
		lang:        lang,
		colorOrange: pal.MapColor(255, 140, 0, true),
		turnFlow:    turnFlowState{handover: -1, handBackTeam: -1},
		// The remappable action keys start at the vanilla defaults; the host
		// overrides them from the settings file via SetKeybinds.
		keybinds: defaultKeybinds,
	}

	// Amphibious units choose their water frame from the modified surface, so a platform or bridge over water still reads as land.
	// C++: SetSpriteFrameForTerrain.
	s.sprites.SetSurfaceSource(func(x, y int) uint8 {
		if s.game != nil && s.game.Grid != nil {
			return s.game.Grid.ModifiedSurface(x, y, false)
		}

		return m.SurfaceAt(x, y) // fallback to the raw tile if the grid is unavailable
	})

	// Attach the task-based computer opponent for every game. It remains inert for non-computer teams.
	// The closure is the AI unit factory. C++: Ai_Init.
	makeUnit := func(team int, t assets.UnitType, flags game.Flags) *game.UnitInfo {
		s.spawnSeq++
		u := game.NewUnitInfo(t, team, s.spawnSeq, 0)
		u.Flags = flags
		u.Values = s.teamValues(team, t)
		u.InitState()

		return u
	}

	s.ai = ai.New(makeUnit)
	s.makeUnit = makeUnit

	// attachGameServices re applies AI + makeUnit after a load too (Load replaces s.game).
	s.attachGameServices()
	s.lib = lib
	s.effects.colRubble = pal.MapColor(72, 72, 72, true)
	s.effects.colHazard = pal.MapColor(255, 210, 0, true)

	s.loadHudArt()
	s.loadGauges()

	shadow := pal.MapColor(40, 8, 8, true)
	white := pal.MapColor(255, 255, 255, true)
	red := pal.MapColor(255, 40, 40, true)
	green := pal.MapColor(40, 220, 40, true)
	s.hudArt.capEnabled = fonts.Color{Base: white, Outline: white, Shadow: shadow}
	s.hudArt.capPressed = fonts.Color{Base: red, Outline: red, Shadow: shadow}
	s.hudArt.capDisabled = fonts.Color{Base: green, Outline: green, Shadow: shadow}
	s.turnFlow.turnSeconds = defaultTurnSeconds
	s.turnFlow.fullTurnSeconds = defaultTurnSeconds
	s.turnFlow.turnStart = platform.Ticks()
	s.announcer.lastBeginKey = -1                // no "Begin!" announced yet; the first live player turn triggers it
	s.announcer.pointsLeader = announcerNoLeader // the first round close seeds the points leader without announcing it
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = r.ClampScroll((m.W*r.Zoom()-vw)/2, (m.H*r.Zoom()-vh)/2, vw, vh)

	return s
}

// loadHudArt loads every HUD face drawn by the screen.
// This includes build markers, captioned FILES, PREF, and FLIC buttons, the lower strip, minimap chrome, control-row faces, and readout backgrounds.
// C++ ported : game_manager.cpp - the GameManager_MenuItems art loads.
func (s *Screen) loadHudArt() {
	lib, lang, sys := s.lib, s.lang, s.sys

	for _, tag := range []string{"BLDMRK1", "BLDMRK2", "BLDMRK3", "BLDMRK4", "BLDMRK5"} {
		if f := loadMarkerFrame(lib, tag); f != nil {
			s.effects.bldMrk = append(s.effects.bldMrk, f)
		}
	}

	s.effects.bulbFrame = loadMarkerFrame(lib, "BULB") // BULB "thinking" overlay; nil = never draws
	s.hudArt.filesArt = loadSimpleArt(lib, "FILES_OF")
	s.hudArt.prefsArt = loadSimpleArt(lib, "PREF_OFF")
	s.hudArt.filesOnArt = loadSimpleArt(lib, "FILES_ON")
	s.hudArt.prefsOnArt = loadSimpleArt(lib, "PREF_ON")

	// FILES/PREF art is a blank face; captions bake on top. C++: MenuItem.label + Button::SetCaption.
	lblFont := sys.Fonts().Get(fonts.Font2)
	upCol := fonts.Color{Base: 0xA2, Outline: 0xA7, Shadow: 0xE6}
	downCol := fonts.Color{Base: 0xA2, Outline: 0xA3, Shadow: 0x24}
	bakeButtonLabel(lblFont, s.hudArt.filesArt, lang.Get("abc7", "Files"), upCol)
	bakeButtonLabel(lblFont, s.hudArt.prefsArt, lang.Get("6696", "Preferences"), upCol)
	bakeButtonLabel(lblFont, s.hudArt.filesOnArt, lang.Get("abc7", "Files"), downCol)
	bakeButtonLabel(lblFont, s.hudArt.prefsOnArt, lang.Get("6696", "Preferences"), downCol)

	// Corner FLIC play/pause buttons; default to playing (GameManager_PlayFlic starts true).
	s.hudArt.playArt = loadSimpleArt(lib, "PLAY_OF")
	s.hudArt.playOnArt = loadSimpleArt(lib, "PLAY_ON")
	s.hudArt.pauseArt = loadSimpleArt(lib, "PAUSE_OF")
	s.hudArt.pauseOnArt = loadSimpleArt(lib, "PAUSE_ON")
	s.hudArt.playFlic = true

	// Lower strip Reports / Help buttons (the art carries its own label/icon).
	s.hudArt.reptArt = loadSimpleArt(lib, "REPT_OFF")
	s.hudArt.reptOnArt = loadSimpleArt(lib, "REPT_ON")
	s.hudArt.helpArt = loadSimpleArt(lib, "HELP_OF")
	s.hudArt.helpOnArt = loadSimpleArt(lib, "HELP_ON")
	s.hudArt.untbtnUp = loadSimpleArt(lib, "UNTBTN_U")
	s.hudArt.untbtnDown = loadSimpleArt(lib, "UNTBTN_D")
	s.hudArt.popupArt = loadPopupButtonArt(lib)
	s.hudArt.panelTopArt = loadSimpleArt(lib, "PANELTOP") // landing sequence minimal panels
	s.hudArt.panelBtmArt = loadSimpleArt(lib, "PANELBTM")
	s.panelAnim = loadPanelAnimArt(lib)                    // the instrument bars unfolding once the landing is done
	s.hudArt.pnlCancelArt = loadSimpleArt(lib, "PNLCAN_U") // Cancel / Help on the site select panel
	s.hudArt.pnlHelpArt = loadSimpleArt(lib, "PNLHLP_U")
	s.cursorMgr = newCursorManager(lib)
	s.minimap.mmX2Off = loadSimpleArt(lib, "MIN2X_OF")
	s.minimap.mmX2On = loadSimpleArt(lib, "MIN2X_ON")
	s.minimap.mmTntOff = loadSimpleArt(lib, "MINFL_OF")
	s.minimap.mmTntOn = loadSimpleArt(lib, "MINFL_ON")

	s.loadPanelFaces()

	// GOAL_OFF/ON faces for the CHAT slot in single player goal mode.
	s.hudArt.goalOffArt = loadSimpleArt(lib, "GOAL_OFF")
	s.hudArt.goalOnArt = loadSimpleArt(lib, "GOAL_ON")
	s.minimap.zoomTrack = loadSimpleArt(lib, "ZOOMPNL1")
	s.minimap.zoomKnob = loadSimpleArt(lib, "ZOOMPTR")

	if s.minimap.zoomKnob != nil {
		s.minimap.zoomKnob.KeyToZero(s.minimap.zoomKnob.At(0, 0)) // its corner is the transparent colour
	}

	// Minimap dots use the bright low UI palette indices (not the clan band colours),
	// so a team's dot stays readable even on a same coloured planet. C++: COLOR_RED/GREEN/BLUE.
	s.minimap.minimapDots = [4]uint8{gfx.ColorRed, gfx.ColorGreen, gfx.ColorBlue, 0xFF} // red, green, blue, gray(white)

	s.hudArt.endTurnUp = loadSimpleArt(lib, "ENDTRN_U")

	// Per team pressed faces, order matches team index: red, green, blue, grey.
	for i, tag := range []string{"R_ENDT_D", "G_ENDT_D", "B_ENDT_D", "W_ENDT_D"} {
		s.hudArt.endTurnDown[i] = loadSimpleArt(lib, tag)
	}

	s.hudArt.turnCounterBg = loadSimpleArt(lib, "TURNS")
	s.hudArt.turnTimerBg = loadSimpleArt(lib, "TIMER")
	s.hudArt.coordsBg = loadSimpleArt(lib, "XYPOS")
	s.hudArt.unitNameBg = loadSimpleArt(lib, "UNITNAME")
}

// loadPanelFaces loads the control-row and toggle-grid rest (_OFF) and pressed (_ON) faces by rectangle.
// Both maps use the same rectangles, so one table feeds both.
// C++ ported : game_manager.cpp - GameManager_MenuItems.
func (s *Screen) loadPanelFaces() {
	lib := s.lib
	panelFaces := []struct {
		rect    gfx.Rect
		off, on string
	}{
		{CenterButton, "FIND_OFF", "FIND_ON"},
		{LockButton, "LOCK_OF", "LOCK_ON"},
		{PrevButton, "PREV_OF", "PREV_ON"},
		{DoneButton, "UDONE_OF", "UDONE_ON"},
		{NextButton, "NEXT_OF", "NEXT_ON"},
		{ReportsButton, "REPT_OFF", "REPT_ON"},
		{ChatButton, "CHAT_OFF", "CHAT_ON"},
		{SurveyButton, "SURV_OFF", "SURV_ON"},
		{StatusButton, "STAT_OFF", "STAT_ON"},
		{ColorsButton, "COLOR_OF", "COLOR_ON"},
		{HitsButton, "HITS_OF", "HITS_ON"},
		{AmmoButton, "AMMO_OF", "AMMO_ON"},
		{RangeButton, "RANG_OFF", "RANG_ON"},
		{ScanButton, "VISN_OFF", "VISN_ON"},
		{GridButton, "GRID_OFF", "GRID_ON"},
		{NameButton, "NAMES_UP", "NAMES_DN"},
	}
	s.hudArt.panelOffArt = make(map[gfx.Rect]*gfx.Buffer, len(panelFaces))
	s.hudArt.panelOnArt = make(map[gfx.Rect]*gfx.Buffer, len(panelFaces))
	for _, pf := range panelFaces {
		s.hudArt.panelOffArt[pf.rect] = loadSimpleArt(lib, pf.off)
		s.hudArt.panelOnArt[pf.rect] = loadSimpleArt(lib, pf.on)
	}
}

// loadGauges loads stat gauge icon pairs with full SI_* and empty EI_* faces.
// HITS also has blue, yellow, and red faces selected by hit ratio.
// C++ ported : game_manager.cpp - GameManager_DrawInfoDisplayRowIcons.
func (s *Screen) loadGauges() {
	lib := s.lib

	// Gauge icons come in full (SI_*) / empty (EI_*) pairs; HITS also has blue/yellow/red
	// variants by hit ratio. C++: GameManager_DrawInfoDisplayRow(Icons).
	loadGauge := func(full, empty string) gaugePair {
		f := loadSimpleArt(lib, full)
		e := loadSimpleArt(lib, empty)

		if f != nil {
			f.KeyToZero(f.At(0, 0))
		}

		if e != nil {
			e.KeyToZero(e.At(0, 0))
		}

		return gaugePair{f, e}
	}

	s.hudArt.statGauges = map[string]gaugePair{
		"AMMO":  loadGauge("SI_AMMO", "EI_AMMO"),
		"SPEED": loadGauge("SI_SPEED", "EI_SPEED"),
		"SHOTS": loadGauge("SI_SHOTS", "EI_SHOTS"),
		"CARGO": loadGauge("SI_RAW", "EI_RAW"),
		"FUEL":  loadGauge("SI_FUEL", "EI_FUEL"),
		"GOLD":  loadGauge("SI_GOLD", "EI_GOLD"),
		"UNITS": loadGauge("SI_LAND", "EI_LAND"), // transport passenger slots (1 icon = 2 small units)
		"POWER": loadGauge("SI_POWER", "EI_POWER"),
		"WORK":  loadGauge("SI_WORK", "EI_WORK"),
	}

	s.hudArt.hitsGauge = [3]gaugePair{
		loadGauge("SI_HITSB", "EI_HITSB"), // > 1/2: blue/green
		loadGauge("SI_HITSY", "EI_HITSY"), // > 1/4: yellow
		loadGauge("SI_HITSR", "EI_HITSR"), // <=1/4: red
	}
}

// Run installs the palette, paints the static frame once, then loops the map view until Escape.
// It returns when the player leaves the screen.
func (s *Screen) Run() error {
	s.plat.SetPalette(&s.pal)
	defer s.clearPortrait()
	defer s.stopUnitSfx() // don't leave a drive/idle sound playing after leaving

	// The live game view runs at wide 16:9. Leaving for the always-4:3 menus restores narrow mode and its logical resolution.
	if s.plat.Widescreen() {
		s.setWideMode(true)
		defer s.setWideMode(false)
	}

	buf := s.sys.Buf(bgWindow)

	// The frame art is static, and the compositor keeps window buffers between frames, so it only needs painting once.
	// Only the map and minimap rectangles redraw each frame.
	s.repaintBackground(buf)

	s.sys.Fonts().SetFont(fonts.Font1)
	s.sys.MouseShow()

	// Restore the shared Hand cursor when leaving the game, so the menu the
	// player returns to is not left showing a stale context cursor (e.g. a
	// scroll arrow). The global default is installed once at boot.
	defer s.installCursor(cursors.Hand)

	// Debug only: a watchdog goroutine that dumps the stuck goroutine stack if a
	// computer-turn frame ever freezes the loop. It stops
	// when Run returns. Idle cost is one atomic load per poll; off entirely when
	// debugging is disabled.
	wdDone := make(chan struct{})
	defer close(wdDone)
	go s.runStallWatchdog(wdDone)    // Spawn patrolling thread
	paths.EnableStats(dbg.Enabled()) // attribute the AI turn slowdown to pathfinding

	for {
		// A Quit from the FILES screen / a confirmed ESC abandons the game to
		// the main menu: leave the screen.
		if s.exitToMenu {
			return nil
		}

		// Heartbeat for the stall watchdog: mark this frame's start and whether it
		// is animating a computer turn (the suspect window for the hang).
		frameStart := time.Now().UnixNano()
		s.turnFlow.heartbeat.Store(frameStart)
		aiFrame := s.turnFlow.aiTurn
		s.turnFlow.wdInAITurn.Store(aiFrame)

		var pfCalls0, pfNanos0 int64
		if aiFrame {
			pfCalls0, pfNanos0 = paths.StatsSnapshot() // pathfinding done this frame
		}

		key := s.sys.GetInput()

		if key != -1 {
			s.handleGameKey(key, buf)

			continue // drain remaining queued events before redrawing
		}

		s.tickIdlePhase()

		s.tickFrameEvents()

		s.stepAnimations()

		s.tickTurnClock()

		vw, vh := viewSize(MainMap)
		s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
		s.animatePalette()
		s.updateCursor()

		// Debug census of the AI turn slowdown that precedes the hang: log a frame
		// that took too long, with live order counts (queue growth vs a stuck order).
		if aiFrame && dbg.Enabled() {
			if ms := (time.Now().UnixNano() - frameStart) / 1e6; ms >= aiFrameSlowMS {
				pfCalls, pfNanos := paths.StatsSnapshot()
				dbg.Printf("ai: slow AI frame %dms (pathfind %dms over %d finds) activeOrders=%d movers=%d turn=%d", ms, (pfNanos-pfNanos0)/1e6, pfCalls-pfCalls0, s.game.ActiveOrders(), len(s.game.MoveOrders()), s.game.State.TurnCounter)
			}
		}

		s.draw(buf)
	}
}

// tickFrameEvents drains the simulation events queued for the screen once per frame.
// It handles followed-unit, announcer, production, planning, build, transport, mine, projectile, and combat events in order.
// A reveal fires reactions before the resulting casualties and damage draw.
func (s *Screen) tickFrameEvents() {
	s.followLockedUnit()     // LOCK: keep the view centred on the followed unit
	s.tickVoiceQueue()       // play "<colour> team ended turn" lines one at a time, no overlap
	s.tickPowerBounce()      // cut a failed power on's rev up + show the "needs power" message
	s.tickProductionEvents() // notify on factory/colony start/stop transitions
	s.tickPlanningBulb()     // clear the "thinking" bulb dwell and start the deferred move

	s.tickBuildExpansions()      // a just finished building unfolds once its builder drives off
	s.tickLoads()                // a unit loading into a Depot/Hangar/Dock scales down + is stored
	s.tickAirUnload()            // air transport unload: descend, drop the unit, rise
	s.tickAirLoad()              // air transport load: fly over, descend, pick up, rise
	s.tickMineBlasts()           // a triggered mine detonates: blast + sound
	s.tickMinesFound()           // a detector uncovers one: say so before someone drives into it
	s.tickProjectiles()          // a missile that reached its target explodes on arrival
	s.game.FireRevealReactions() // a defender fires on an enemy the fog refresh just revealed
	s.tickCasualties()           // a unit killed by an instant attack blows up
	s.tickDamage()               // a unit that SURVIVED an instant attack: bloom, hit sound, under attack banner
	s.tickReactions()            // reaction (opportunity) fire: the weapon report + banner
	s.tickCommandoEvents()       // an enemy commando seized / shut down / was caught raiding a player unit
	s.tickBenchBanner()          // -emulate benches: the stop banner returns once a turn start alert is gone
}

// draw renders the map viewport and minimap into the background window, then prints the status line.
func (s *Screen) draw(buf *gfx.Buffer) {
	vw, vh := viewSize(MainMap)

	s.sys.Lock()

	// While the landing panels slide off, repaint the full frame first so the
	// vacated area shows the real UI, not stale pixels the overlay left behind.
	if (s.siteSelect != nil && s.siteSelect.slideStart != 0) || s.camera.leavingStart != 0 {
		s.paintFrame(buf)
	}

	view := buf.Sub(MainMap.ULX, MainMap.ULY, vw, vh)
	s.r.Render(view, s.camera.scrollX, s.camera.scrollY)

	if s.grid && s.siteSelect == nil {
		// The grid is hidden while choosing a landing site (vanilla shows a clean
		// map there); it returns once the game proper begins.
		s.r.RenderGrid(view, s.camera.scrollX, s.camera.scrollY, drawmap.DefaultGridColor)
	}

	if s.accessOverlay && s.selected != nil {
		s.drawAccessOverlay(view)
	}

	// Rubble is ground cover and draws below units.
	// drawUnits places the construction overlay between floor layer 2 and the remaining units, so slabs on water platforms remain visible.
	s.drawRubble(view)
	s.drawUnits(view)

	// Move route arrows draw after the world, so they stay visible over roads and structures but below transient combat and UI overlays.
	// C++: GroundPath::Draw is a late pass over the terrain/structures.
	s.drawRoutes(view)         // white A* path (debug, -set-a-star only)
	s.drawAllGroundPaths(view) // green/blue arrows: active + queued routes, deduped per cell
	s.drawEffects(view)        // explosion animations, over the units (a blast covers all)
	s.drawProjectiles(view)    // missiles in flight + their smoke/bubble trail
	s.drawBuildReady(view)     // "New X available" flash + BLDMRK exit markers
	s.drawPlanningBulb(view)   // brief BULB "thinking" flash over the selected mover
	s.drawNameOverlay(view)    // the Name panel toggle
	s.drawGalleryLabels(view)

	if s.buildUI.buildIdx > 0 {
		s.drawBuildCursor(view)
	}

	s.drawSiteSelect(view)      // landing site bracket while choosing a start location
	s.drawResourceOverlay(view) // surveyed resource markers, above units / under fog
	s.drawScanRange(view)       // selected recon unit's scan range ring
	s.drawUnitDisplays(view)    // COLORS/STATUS/HITS/AMMO panel overlays over units
	s.drawUnitRangeRings(view)  // RANGE (red) + SCAN (yellow) panel range rings
	s.drawSentryCoverage(view)  // guarded zones of the player's sentries (F2 toggle)

	if s.fogShow {
		s.drawFog(view)
	}

	s.drawMinimap(buf)
	s.drawMinimapButtons(buf)
	s.drawZoomSlider(buf)

	// Top/bottom bars are hidden while choosing a landing site or during the descent.
	// Once the landing is over they do not just appear: they unfold on the retail
	// flipbook first, and the live chrome takes over when it ends.
	// C++: GameManager_MenuAnimateDisplayControls runs only after landing.
	if s.siteSelect == nil && s.camera.landingStep == 0 && s.camera.leavingStart == 0 {
		if s.panelAnimPlaying() {
			s.drawPanelAnim(buf)
		} else {
			s.drawTopBar(buf)
			s.drawBottomBar(buf)
		}
	}

	s.drawCornerFlic(buf)
	s.drawStatWindow(buf)

	if s.buildUI.buildMenuOpen {
		s.drawBuildMenu(buf)
	}

	if s.orderInput.popupOpen {
		s.drawPopup(buf)
	}

	s.drawBanner(buf)

	s.drawLandingPanels(buf) // minimal panels cover the left column, then slide apart
	s.drawDebugMenu(buf)     // F2 developer flag overlay, on top of everything
	s.sys.Unlock()

	// Print locks the system itself, so the status line must run outside Lock (not reentrant).
	s.sys.Print(bgWindow, fmt.Sprintf("turn %d  %s's turn  units %d  (%s)", s.game.State.TurnCounter, s.teamName(s.game.State.ActiveTeam), len(s.game.Units.Units()), s.statusHint()), MainMap.Width()+1, MainMap.ULX, MainMap.LRY+6, 0xFF)
}

// statusHint returns the key list printed under the map for the current mode.
// Armed debug placement, the build menu, and build or path picks each show their own keys instead of the general list.
func (s *Screen) statusHint() string {
	hint := "scroll, zoom, G grid, F find, ? help, Alt+S save, Alt+L load, Alt+1-4 set group, 1-4 recall, RETURN end turn, ESC"

	if s.debug {
		hint = "G grid, F find, Alt+S/L save/load, Alt+D explode, Alt+1-4/1-4 groups, F2 debug menu, A access, V fog, L move/attack, R info, RETURN end, ESC"
	}

	if s.debugPlaceArmed() {
		hint = "DEBUG PLACE: click a cell to drop the unit/building, ESC cancel"
	} else if s.buildUI.buildMenuOpen {
		hint = "BUILD MENU: click a type to select, or B / ESC to close"
	} else if s.buildUI.buildPath && s.buildUI.buildIdx > 0 && s.buildUI.buildIdx <= len(s.buildUI.buildRoster) {
		// C++: "Position tape and click inside it to begin building." (the engineer Path pick).
		hint = "PATH BUILD: click a destination to lay a line of " + displayName(s.buildUI.buildRoster[s.buildUI.buildIdx-1].t) + " - ESC / right-click cancel"
	} else if s.buildUI.buildIdx > 0 && s.buildUI.buildIdx <= len(s.buildUI.buildRoster) {
		hint = "BUILD: " + displayName(s.buildUI.buildRoster[s.buildUI.buildIdx-1].t) + " - click a green site to place, B menu, ESC cancel"
	}

	return hint
}
