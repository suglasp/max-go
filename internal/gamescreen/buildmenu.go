// buildmenu.go : implements the constructor build menu.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - build mode click and site selection
package gamescreen

import (
	"fmt"
	"strings"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/world"
)

// buildUIState holds constructor and engineer placement state along with the cached ATTRIBS and CLANATRB tables.
type buildUIState struct {
	buildIdx      int                     // B cycles a buildable type: 0 = off; 1..len(buildRoster) selects a type
	buildMenuOpen bool                    // build popup list is showing (legacy; replaced by CONBUILD)
	buildRoster   []buildableType         // armed placement roster (the single chosen type)
	buildRate     int                     // build rate (1/2/4) chosen in the Construction Menu
	buildPath     bool                    // engineer Path (line) build armed; next map click picks a destination (StartBuildPath)
	attribs       []assets.UnitAttributes // base ATTRIBS rows, for build costs
	clans         []assets.Clan           // CLANATRB definitions, per team clan stat bonuses
}

const (
	buildRowH    = 16
	buildMenuTop = 26 // first row offset below the title
)

// buildMenuRect is the popup list's screen rectangle, sized to the active
// builder's roster: a small list centred over the map, one clickable row per
// buildable type.
func (s *Screen) buildMenuRect() gfx.Rect {
	return gfx.Rect{ULX: 250, ULY: 110, LRX: 430, LRY: 130 + len(s.buildUI.buildRoster)*buildRowH}
}

// SetAttribs supplies the base ATTRIBS rows so the build menu can show each
// type's build cost in turns. Indexed by assets.UnitType.
func (s *Screen) SetAttribs(rows []assets.UnitAttributes) {
	s.buildUI.attribs = rows
}

// SetClans supplies the CLANATRB clan definitions so each team's units pick up
// their clan stat bonuses (applied per team in researchValues). Indexed 0..7 by
// clan id - 1.
func (s *Screen) SetClans(clans []assets.Clan) {
	s.buildUI.clans = clans
}

// buildCost returns a buildable type's build time in turns, or 0 if unknown.
func (s *Screen) buildCost(t assets.UnitType) int {
	if int(t) >= 0 && int(t) < len(s.buildUI.attribs) {
		return s.buildUI.attribs[t].TurnsToBuild
	}

	return 0
}

// buildRawCost is the total raw material to build product with builder at rate 1:
// the builder's own per turn raw draw times the product's (research adjusted) build
// turns. This is the figure klei refuses a start below and the same one the construct
// menu paints red when the builder cannot afford it.
// C++ ported : buildmenu.cpp - Cargo_GetRawConsumptionRate(builder, 1) * GetTurnsToBuild(product).
func (s *Screen) buildRawCost(builder *game.UnitInfo, product assets.UnitType) int {
	vals := s.teamValues(builder.Team, product)
	if vals == nil {
		return 0
	}

	return economy.RawConsumptionRate(builder.Type, 1) * vals.Get(game.AttribTurns)
}

// The Construction Menu's result is screendata.BuildSelection, shared with the
// menus package.

// SetBuildFunc installs the hook a builder's popup/B key calls to open the
// CONBUILD Construction Menu (wired by the host to menus.RunConstruction).
func (s *Screen) SetBuildFunc(fn func(builder assets.UnitType, team, storage int) (screendata.BuildSelection, bool, error)) {
	s.hooks.buildFn = fn
}

// The factory Production Menu's result is screendata.FactorySelection, shared with
// the menus package.

// SetFactoryBuildFunc installs the hook a factory's popup Build calls to open the
// FACBUILD Production Menu (wired by the host to menus.RunFactoryBuild).
func (s *Screen) SetFactoryBuildFunc(fn func(factory assets.UnitType, team, storage int, queue []assets.UnitType, repeat bool) (screendata.FactorySelection, bool, error)) {
	s.hooks.factoryFn = fn
}

// openFactoryBuild runs the FACBUILD Production Menu for a factory and commits the
// edited build queue back to the sim: a queue that is not empty starts/continues
// production (the head unit rolls out over its turns), an empty queue halts the
// factory. C++: BuildMenu_Menu for a STATIONARY factory -> FactoryBuildMenu::Build.
func (s *Screen) openFactoryBuild(u *game.UnitInfo) {
	if u == nil || s.hooks.factoryFn == nil {
		return
	}

	queue, repeat := s.game.FactoryQueue(u)
	sel, ok, err := s.hooks.factoryFn(u.Type, u.Team, u.Storage, queue, repeat)

	if err != nil {
		dbg.Printf("factory build menu: %v", err)
		return
	}

	if !ok {
		return
	}

	rate := sel.Rate

	if rate < 1 {
		rate = 1
	}

	s.game.SetFactoryQueue(u, sel.Queue, rate, sel.Repeat)

	if len(sel.Queue) > 0 {
		s.playSfxForUnit(u, game.SfxExpand) // production begins
		s.showMessage(fmt.Sprintf("%s now producing %s.", displayName(u.Type), displayName(sel.Queue[0])))
	} else {
		s.showMessage(fmt.Sprintf("%s production halted.", displayName(u.Type)))
	}
}

// SetStatusFunc installs the hook that opens the STATS Unit Status Screen for a
// unit (wired by the host to menus.RunUnitStatus).
func (s *Screen) SetStatusFunc(fn func(u *game.UnitInfo, name string) error) {
	s.hooks.statusFn = fn
}

// openStatus shows the Unit Status Screen for u (a full screen palette swap, so
// it runs through openMenuScreen). C++: UnitStats_Menu (game_manager.cpp opens
// it on a right click of the selected unit).
func (s *Screen) openStatus(u *game.UnitInfo) {
	if u == nil || s.hooks.statusFn == nil {
		return
	}

	s.openMenuScreen(func() {
		if err := s.hooks.statusFn(u, displayName(u.Type)); err != nil {
			dbg.Printf("unit status screen: %v", err)
		}
	}, true)
}

// openBuild runs the Construction Menu for the selected builder and, on a Done
// choice, arms placement mode for the chosen building at the chosen rate (the
// next click on a valid green site places it). C++: BuildMenu_Menu then the
// ORDER_STATE_SELECT_SITE placement loop (buildmenu.cpp).
func (s *Screen) openBuild() {
	b := s.selectedBuilder()

	if b == nil || s.hooks.buildFn == nil {
		return
	}

	sel, ok, err := s.hooks.buildFn(b.Type, b.Team, b.Storage)

	if err != nil {
		dbg.Printf("construction menu: %v", err)
		return
	}

	if !ok {
		return
	}

	rate := sel.Rate

	if rate < 1 {
		rate = 1
	}

	// Refuse a build the mobile builder cannot pay for, the way klei does at the Build
	// button, before any placement is armed. A mobile builder spends its own storage
	// first, so a build it cannot afford would only stall out of material a few turns in
	// and strand the builder posed on the site (the construct menu already paints this
	// case red). This gates the engineer 1x1, the constructor 2x2, and the engineer path
	// build alike, since all three arm below.
	// C++ ported : buildmenu.cpp - MobileBuildMenu::Build (storage < rate1*turns -> _(c529), no ORDER_BUILD).
	if b.Storage < s.buildRawCost(b, sel.Type) {
		s.showAlert(s.lang.Get("c529", "WARNING: insufficient material in storage to start construction."))
		s.buildUI.buildIdx = 0
		s.buildUI.buildMenuOpen = false
		return
	}

	// Arm placement for the single chosen type. The constructor builds large
	// (2x2) structures, the engineer small (1x1) ones; the builder's type drives
	// the footprint (the rosters are split by builder).
	large := b.Type == assets.Constrct
	s.buildUI.buildRoster = []buildableType{{sel.Type, large}}
	s.buildUI.buildIdx = 1
	s.buildUI.buildRate = rate

	// Path (line) build: the engineer arms a destination pick instead of placing
	// at once. The next map click runs a straight run of the chosen structure from
	// the engineer's cell to the clicked cell (clickCell -> confirmBuildPath). The
	// MAP_PTR cursor shows; ESC / right click cancels. C++: GameManager_PathBuild.
	if sel.Path && b.Type == assets.Engineer {
		s.buildUI.buildPath = true
		s.playBuildPlacementPrompt() // V_F050, and only inside a training mission

		return
	}

	// C++ ported : buildmenu.cpp - MobileBuildMenu::Build. A 1x1 building (engineer,
	// no BUILDING flag) builds IMMEDIATELY on the builder's own cell - no site pick.
	// If that cell is an invalid square, refuse with the invalid square message
	// instead of leaving a SELECT_SITE placement cursor armed (the original's
	// UnitsManager_IssueBuildOrder fails -> _(1031) path). A 2x2 building
	// (constructor) instead enters SELECT_SITE below.
	if !large {
		if !s.placeBuildingAt(b.GridX, b.GridY) {
			// Invalid square: abort the build; do NOT arm a placement cursor. Read the
			// descriptor BEFORE clearing the selection, so the message can name the reason.
			d, _ := s.buildDesc()

			s.buildUI.buildIdx = 0
			s.buildUI.buildMenuOpen = false
			s.showMessage(s.buildRefusalText(d, b))
		}
	}

	// A constructor (large) is now in SELECT_SITE: the next map click positions
	// the footprint. The menu close release is eaten by openMenuScreen's
	// swallowRelease, so it does not place the site instantly.
	if large {
		s.playBuildPlacementPrompt() // "Position tape and click inside it..." - training only
	}
}

// buildRefusalText explains why a 1x1 build on the builder's own cell was refused. The
// original has ONE generic line for every reason (_(1031) "in an invalid square"), which
// leaves the player guessing. The port keeps that line, but when the SURFACE is the
// blocker it says so and names both surfaces, because the map's art does not tell you
// what the pass table calls a tile: a shoreline tile can be drawn as water meeting land
// and still be classified LAND, and a Bridge takes water or coastal shallows only. That
// is a real retail map trap (DESERT_1 has a single land tile sitting in a coast strip).
//
// C++: buildmenu.cpp - MobileBuildMenu::Build's _(1031) message, with the reason added.
func (s *Screen) buildRefusalText(d access.UnitDesc, builder *game.UnitInfo) string {
	surface := s.game.Grid.ModifiedSurface(builder.GridX, builder.GridY, false)

	if d.LandType != 0 && d.LandType&surface == 0 {
		return fmt.Sprintf("A %s can only be built on %s. The %s is standing on %s.",
			displayName(d.Type), surfaceWords(d.LandType),
			displayName(builder.Type), surfaceWords(surface))
	}

	return fmt.Sprintf(s.lang.Get("1031", "The %s is in an invalid square to build the selected unit."),
		displayName(builder.Type))
}

// surfaceWords spells a surface mask out for the player ("water or coastal shallows"),
// where surfaceName spells it out for the debug log ("water+coast").
func surfaceWords(mask uint8) string {
	var parts []string

	for _, s := range []struct {
		bit  uint8
		name string
	}{
		{world.SurfaceLand, "land"},
		{world.SurfaceCoast, "coastal shallows"},
		{world.SurfaceWater, "water"},
		{world.SurfaceAir, "air"},
	} {
		if mask&s.bit != 0 {
			parts = append(parts, s.name)
		}
	}

	switch len(parts) {
	case 0:
		return "nothing"

	case 1:
		return parts[0]

	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " or " + parts[len(parts)-1]
	}
}

/* // note: unused code in Go port
// requestBuildMenu opens the Construction Menu for the selected builder via the
// host hook (a full screen palette swap, so it runs through openMenuScreen).
func (s *Screen) requestBuildMenu() {
	if s.selectedBuilder() == nil {
		return
	}

	s.openMenuScreen(s.openBuild, true)
}
*/

// buildMenuRowAt returns the buildable index under a screen point, or -1.
func (s *Screen) buildMenuRowAt(mx, my int) int {
	r := s.buildMenuRect()

	if !s.buildUI.buildMenuOpen || mx < r.ULX || mx > r.LRX {
		return -1
	}

	row := (my - (r.ULY + buildMenuTop)) / buildRowH

	if row < 0 || row >= len(s.buildUI.buildRoster) {
		return -1
	}

	return row
}

// drawBuildMenu paints the build popup: a bordered panel, a "Build" title, and
// one row per buildable type with the hovered row highlighted.
//
// C++: BuildMenu_Init / the build unit list (buildmenu.cpp), reduced to the
// current buildable roster.
func (s *Screen) drawBuildMenu(buf *gfx.Buffer) {
	r := s.buildMenuRect()
	w, h := r.LRX-r.ULX+1, r.LRY-r.ULY+1
	buf.Sub(r.ULX, r.ULY, w, h).Fill(0) // dark backing
	buf.DrawBox(r.ULX, r.ULY, r.LRX, r.LRY, 0xFF)

	f := s.sys.Fonts().Get(fonts.Font2)
	s.blitStatText(buf, r.ULX, r.ULY+4, w, f.Height+2, f, s.lang.Get("4ee9", "Build"), colorTurnText, true)

	mouse := s.plat.Mouse()
	hover := s.buildMenuRowAt(mouse.X, mouse.Y)

	for i, b := range s.buildUI.buildRoster {
		y := r.ULY + buildMenuTop + i*buildRowH
		color := 0xFF

		if i == hover {
			buf.Sub(r.ULX+2, y, w-4, buildRowH-1).Fill(s.colorOrange) // highlight bar
			color = 0
		}

		s.blitStatText(buf, r.ULX+8, y, w-60, buildRowH, f, displayName(b.t), color, false)

		if cost := s.buildCost(b.t); cost > 0 {
			s.blitStatText(buf, r.LRX-50, y, 46, buildRowH, f, fmt.Sprintf("%d turns", cost), color, false)
		}
	}
}

// buildableType is one construction roster entry: the unit type and whether it is
// a large (2x2, FlagBuilding) building rather than 1x1, driving the footprint rule.
type buildableType struct {
	t     assets.UnitType // unit type offered for construction
	large bool            // whether the unit occupies a 2x2 footprint
}

// constructorBuildables is the Constructor's roster: large (2x2) structures, in the
// manual's "Constructors can build:" order (page 28). Row 7 "Gold Refinery" is unit
// type COMMTWR (legacy enum misnomer); the 1x1 "Gold Vault" (GOLDSM) is the separate
// engineer building. SUPRTPLT/RECCENTR/SHIELDGN are alien derelict artifacts, not
// buildable. C++: the constructor's Builder_GetBuildableUnits list (builder.cpp).
var constructorBuildables = []buildableType{
	{assets.MiningSt, true}, {assets.PowerStn, true}, {assets.LightPlt, true},
	{assets.LandPlt, true}, {assets.AirPlt, true}, {assets.Shipyard, true},
	{assets.CommTwr, true},
	{assets.Depot, true}, {assets.Hangar, true}, {assets.Dock, true},
	{assets.Habitat, true}, {assets.Research, true}, {assets.Greenhse, true},
	{assets.TrainHal, true}, {assets.Barracks, true},
}

// engineerBuildables is the Engineer's roster: small (1x1) structures - power,
// storage, stationary defenses, Road, Water Platform, and Bridge (the amphibious
// engineer drives onto water to lay either). Connector/landing pad need their own
// adjacency rules and are not yet listed. C++: the engineer's Builder_GetBuildableUnits.
var engineerBuildables = []buildableType{
	{assets.PowGen, false}, {assets.ADump, false}, {assets.FDump, false},
	{assets.GoldSm, false}, {assets.Radar, false}, {assets.GunTurrt, false},
	{assets.AntiAir, false}, {assets.ArtyTrrt, false}, {assets.AntiMssl, false},
	{assets.Block, false}, {assets.Road, false}, {assets.WtrPltfm, false}, {assets.Bridge, false},
}

// buildableTypes is the combined roster (constructor + engineer), used where a
// single list of every buildable type is needed (e.g. the Upgrade menu).
var buildableTypes = append(append([]buildableType{}, constructorBuildables...), engineerBuildables...)

// rosterFor returns the buildable roster for a builder unit type, or nil for a
// non builder. C++: Builder_GetBuildableUnits keyed by the builder's type.
func rosterFor(t assets.UnitType) []buildableType {
	switch t {
	case assets.Constrct:
		return constructorBuildables

	case assets.Engineer:
		return engineerBuildables

	default:
		return nil
	}
}

// selectedBuilder returns the selected unit when it is a builder (constructor or
// engineer) of the active team, else nil; the unit whose roster the build menu
// shows. C++: the GameManager_SelectedUnit builder check before BuildMenu_Menu.
func (s *Screen) selectedBuilder() *game.UnitInfo {
	u := s.selected

	if u == nil || u.Team != s.game.State.ActiveTeam || !u.Flags.Has(game.FlagConstructorUnit) {
		return nil
	}

	return u
}

// buildDesc describes the currently selected build type for site checks.
func (s *Screen) buildDesc() (access.UnitDesc, bool) {
	if s.buildUI.buildIdx == 0 || s.buildUI.buildIdx > len(s.buildUI.buildRoster) {
		return access.UnitDesc{}, false
	}

	bt := s.buildUI.buildRoster[s.buildUI.buildIdx-1]
	t := bt.t
	if t == assets.Road { // drivable GROUND COVER, not a structure: no slab, no complex
		flags := game.FlagGroundCover
		return access.UnitDesc{Type: t, Flags: flags, LandType: access.LandTypeForUnit(t, flags)}, true
	}

	flags := game.FlagStationary | game.FlagSelectable
	// A connector/Water Platform/Bridge lays no slab. C++: excluded from REQUIRES_SLAB.
	if t != assets.Cnct4W && t != assets.WtrPltfm && t != assets.Bridge {
		flags |= game.FlagRequiresSlab
	}

	if bt.large {
		flags |= game.FlagBuilding // 2x2 footprint + large slab; 1x1 types omit it
	}

	if game.CanHeadRotateType(t) {
		flags |= game.FlagTurretSprite | game.FlagCanHeadRotate
	}

	return access.UnitDesc{Type: t, Flags: flags, LandType: access.LandTypeForUnit(t, flags)}, true
}

// placeBuildingAt raises the selected building type with its footprint top left
// at (cellX, cellY). A selected builder of the active team raises it over several
// turns drawing material; without one (demo/debug) it lands finished at once.
// Returns true when a build started (or a demo building landed).
// C++: MobileBuildMenu::Build -> ORDER_BUILD.
func (s *Screen) placeBuildingAt(cellX, cellY int) bool {
	d, ok := s.buildDesc()

	if !ok {
		return false
	}

	builder := s.selected
	isBuilder := builder != nil && builder.Flags.Has(game.FlagConstructorUnit) && builder.Team == s.game.State.ActiveTeam

	var ignore *game.UnitInfo // the builder occupies a cell in its own footprint; ignore it in the site check
	if isBuilder {
		ignore = builder
	}

	if !s.buildSiteValid(ignore, d, cellX, cellY) {
		// Say WHY, but only for a real placement attempt: the build CURSOR asks the same
		// question every frame while it hovers, and logging there would bury the log.
		s.dbgBuildSiteRefused(d, ignore, cellX, cellY)

		return false
	}

	u := game.NewUnitInfo(d.Type, s.game.State.ActiveTeam, s.spawnSeq+1, 0)
	u.Flags = d.Flags
	u.Values = s.teamValues(s.game.State.ActiveTeam, d.Type) // base ATTRIBS + team research
	u.InitState()                                            // Hits etc. so the stat window/portrait have data
	s.spawnSeq++

	if isBuilder {
		rate := s.buildUI.buildRate

		if rate < 1 {
			rate = 1
		}

		s.game.CancelBuildPath(builder) // a new single build supersedes any running Path run
		order := s.game.StartBuild(builder, u, cellX, cellY, rate)
		s.playSfxForUnit(builder, game.SfxExpand) // the slab deploying onto the site
		s.announceBuildStarted(builder, d.Type, order.TurnsLeft(), cellX, cellY)
		dbg.Printf("%s: start build %s at (%d,%d) rate x%d", displayName(builder.Type), displayName(d.Type), cellX, cellY, rate)
		s.buildUI.buildIdx = 0 // leave build mode; the builder now works the site
		s.buildUI.buildMenuOpen = false

		return true
	}

	s.game.PlaceBuilding(u, cellX, cellY)
	s.setSelected(u)

	return true
}

// cancelBuildArm clears any armed build placement / path pick and closes the build
// popup (ESC or right click). It does not touch a build already in progress.
func (s *Screen) cancelBuildArm() {
	s.orderInput.popupOpen = false
	s.buildUI.buildMenuOpen = false
	s.buildUI.buildIdx = 0
	s.buildUI.buildPath = false
}

// confirmBuildPath runs the engineer's Path (line) build to (cellX, cellY), laying
// the armed structure one per turn along the run; keeps the engineer selected. A
// run that collapses to one cell falls back to a normal build. C++: GameManager_PathBuild.
func (s *Screen) confirmBuildPath(cellX, cellY int) {
	b := s.selectedBuilder()
	d, hasDesc := s.buildDesc()
	rate := s.buildUI.buildRate

	if rate < 1 {
		rate = 1
	}

	s.buildUI.buildPath = false
	s.buildUI.buildIdx = 0
	s.buildUI.buildMenuOpen = false

	if b == nil || !hasDesc {
		return
	}

	// Clone a template product (Type/Flags/Values); StartBuildPath replicates it per cell.
	template := game.NewUnitInfo(d.Type, s.game.State.ActiveTeam, s.spawnSeq+1, 0)
	template.Flags = d.Flags
	template.Values = s.teamValues(s.game.State.ActiveTeam, d.Type)
	template.InitState()
	s.spawnSeq++

	if s.game.StartBuildPath(b, template, rate, cellX, cellY) {
		s.playSfxForUnit(b, game.SfxExpand) // the confirm + first slab going down
		s.setSelected(b)                    // the engineer stays selected and starts the run
		s.showMessage(displayName(d.Type) + " path started.")

		dbg.Printf("%s: path build %s to (%d,%d) rate x%d", displayName(b.Type), displayName(d.Type), cellX, cellY, rate)

		return
	}

	s.buildUI.buildRoster = []buildableType{{d.Type, false}} // single cell run: fall back to a plain build
	s.buildUI.buildIdx = 1
	s.buildUI.buildRate = rate
	s.placeBuildingAt(b.GridX, b.GridY)
}

// buildSiteValid reports whether the structure described by d fits at (cellX, cellY),
// ignoring the builder itself. C++: UnitsManager_IssueBuildOrder removes the builder
// from the map hash before the site test, then re adds it.
func (s *Screen) buildSiteValid(ignore *game.UnitInfo, d access.UnitDesc, cellX, cellY int) bool {
	if ignore != nil {
		s.game.Units.Cells.Remove(ignore)

		defer s.game.Units.Cells.Add(ignore)
	}

	return build.SiteValid(s.game.Grid, d, s.game.State.ActiveTeam, cellX, cellY)
}

// dbgBuildSiteRefused logs WHY a build attempt was refused, one line per footprint cell:
// the raw map surface, the surface as the access layer sees it (a water platform or a
// bridge turns water into land), what the structure being placed accepts, and every unit
// sitting on the cell (the builder itself is marked, since the site check ignores it).
// Nothing without -debug. This is the answer to "why can I not build HERE" without
// guessing: a bridge takes water or coast only and never land, a stationary structure is
// refused outright on rubble or a mine, and a building blocks everything.
func (s *Screen) dbgBuildSiteRefused(d access.UnitDesc, ignore *game.UnitInfo, cellX, cellY int) {
	if !dbg.Enabled() {
		return
	}

	n := build.Footprint(d.Flags)

	for dy := 0; dy < n; dy++ {
		for dx := 0; dx < n; dx++ {
			x, y := cellX+dx, cellY+dy

			var occupants []string

			for _, u := range s.game.Units.Cells.At(x, y) {
				name := fmt.Sprintf("%s(team %d)", displayName(u.Type), u.Team)

				if u == ignore {
					name += "[the builder, ignored]"
				}

				occupants = append(occupants, name)
			}

			dbg.Printf("build site refused: %s at (%d,%d): map surface=%s, access surface=%s, accepts=%s, occupants=%v",
				displayName(d.Type), x, y,
				surfaceName(s.wmap.SurfaceAt(x, y)),
				surfaceName(s.game.Grid.ModifiedSurface(x, y, false)),
				surfaceName(d.LandType), occupants)
		}
	}
}

// surfaceName spells out a surface mask (land / coast / water / air) for the debug log.
func surfaceName(mask uint8) string {
	var parts []string

	for _, s := range []struct {
		bit  uint8
		name string
	}{
		{world.SurfaceLand, "land"},
		{world.SurfaceCoast, "coast"},
		{world.SurfaceWater, "water"},
		{world.SurfaceAir, "air"},
	} {
		if mask&s.bit != 0 {
			parts = append(parts, s.name)
		}
	}

	if len(parts) == 0 {
		return "none"
	}

	return strings.Join(parts, "+")
}

// clampBuildSite anchors an n x n build footprint on the builder at (bx, by): a 1x1
// sits on the builder; a 2x2 can only nudge one cell toward the cursor cell (cx, cy),
// so the footprint ALWAYS overlaps the builder's own cell. A player cannot place the
// building away from the constructor and have it drive there - that far site drive is
// the AI TaskCreateBuilding path, not the manual build. C++ ported : game_manager.cpp
// - the ORDER_STATE_SELECT_SITE target_grid_x/y clamp (game_manager.cpp:5743).
func clampBuildSite(bx, by, cx, cy, n int) (int, int) {
	if n <= 1 {
		return bx, by
	}

	tx, ty := bx, by
	if cx < bx {
		tx = bx - 1
	}

	if cy < by {
		ty = by - 1
	}

	return tx, ty
}

// buildSiteCell maps the mouse to a builder's footprint top left via clampBuildSite,
// so a constructor's 2x2 stays anchored on the builder (the quadrant toward the mouse).
// C++: GameManager's SELECT_SITE target_grid_x/y (game_manager.cpp:5743).
func (s *Screen) buildSiteCell(builder *game.UnitInfo, n, mx, my int) (int, int) {
	bx, by := builder.GridX, builder.GridY
	if mc, mr, ok := s.mapCell(mx, my); ok {
		return clampBuildSite(bx, by, mc, mr, n)
	}

	return clampBuildSite(bx, by, bx, by, n)
}

// buildCell converts a screen pixel to the top left cell of an n cell building
// footprint centred on the cursor (so a 2x2 outline straddles the pointer
// rather than hanging down right of it). For n=1 it is identical to mapCell.
func (s *Screen) buildCell(px, py, n int) (int, int, bool) {
	if px < MainMap.ULX || px > MainMap.LRX || py < MainMap.ULY || py > MainMap.LRY {
		return 0, 0, false
	}

	z := s.r.Zoom()
	off := (n - 1) * z / 2 // shift so the footprint centre lands on the cursor
	cellX := (s.camera.scrollX + px - MainMap.ULX - off) / z
	cellY := (s.camera.scrollY + py - MainMap.ULY - off) / z

	if cellX < 0 || cellY < 0 || cellX+n > s.wmap.W || cellY+n > s.wmap.H {
		return 0, 0, false
	}

	return cellX, cellY, true
}

// drawBuildCursor outlines the selected building's footprint under the mouse:
// green where valid, red where not. C++: GameManager_UpdateDrawBounds.
func (s *Screen) drawBuildCursor(view *gfx.Buffer) {
	d, ok := s.buildDesc()

	if !ok {
		return
	}

	mouse := s.plat.Mouse()
	n := build.Footprint(d.Flags)
	var cellX, cellY int
	valid := false
	builder := s.selectedBuilder()

	if s.buildUI.buildPath && builder != nil {
		// Path pick: the hazard tape follows the cursor to the destination (1x1),
		// not pinned to the builder. C++: GameManager_TempTape under the MAP_PTR.
		mc, mr, in := s.mapCell(mouse.X, mouse.Y)

		if !in {
			return
		}

		cellX, cellY, n = mc, mr, 1
		valid = s.buildSiteValid(builder, d, cellX, cellY)
	} else if builder != nil { // anchored on the builder, snaps to the quadrant toward the mouse
		cellX, cellY = s.buildSiteCell(builder, n, mouse.X, mouse.Y)
		valid = s.buildSiteValid(builder, d, cellX, cellY)
	} else {
		var in bool

		if cellX, cellY, in = s.buildCell(mouse.X, mouse.Y, n); !in {
			return
		}

		valid = build.SiteValid(s.game.Grid, d, 0, cellX, cellY)
	}

	z := s.r.Zoom()

	// Hazard tape sprite marks the footprint (not a flat box); a blocked site adds
	// a red outline. C++: GameManager_TempTape during ORDER_STATE_SELECT_SITE.
	if builder != nil {
		tape := assets.SmlTape

		if n >= 2 {
			tape = assets.LrgTape
		}

		s.sprites.DrawEffect(view, tape, 0, cellX, cellY, n, z, s.camera.scrollX, s.camera.scrollY)

		if valid {
			return // the tape alone signals a good site
		}
	}

	startX := cellX*z - s.camera.scrollX
	startY := cellY*z - s.camera.scrollY
	endX := startX + n*z - 1
	endY := startY + n*z - 1
	color := gfx.ColorRed

	if valid {
		color = gfx.ColorGreen
	}

	view.DrawBox(clampInt(startX, 0, view.W-1), clampInt(startY, 0, view.H-1),
		clampInt(endX, 0, view.W-1), clampInt(endY, 0, view.H-1), color)
}
