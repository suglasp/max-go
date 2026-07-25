// debugopen.go : implements the Debug entry points the dev benches drive.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"
	"time"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/winloss"
	"github.com/suglasp/max-go/internal/world"
)

// DebugOpenPrefs paints the game frame then opens the Preferences screen, for
// screenshotting/inspecting the in game dialog (-menu prefs-game). Debug only.
func (s *Screen) DebugOpenPrefs() {
	s.plat.SetPalette(&s.pal)
	defer s.clearPortrait()
	buf := s.sys.Buf(bgWindow)
	s.repaintBackground(buf)
	s.draw(buf)
	s.openMenuScreen(s.openOptions, false)
}

// DebugOpenResearch spawns a few research centres for the player team then opens
// the Research screen (-menu research), so the screen has labs to allocate.
func (s *Screen) DebugOpenResearch() {
	team := s.game.State.PlayerTeam

	for i := 0; i < 3; i++ {
		s.deployUnit(assets.Research, game.FlagBuilding, team, 0, 6+2*i, 6)
	}

	s.plat.SetPalette(&s.pal)
	defer s.clearPortrait()
	buf := s.sys.Buf(bgWindow)
	s.repaintBackground(buf)
	s.draw(buf)
	s.openMenuScreen(s.openResearch, false)
}

// DebugOpenAlloc seeds deposits + a mining station for the player team (so a
// complex exists with capacity) then opens the Allocate screen (-menu alloc).
func (s *Screen) DebugOpenAlloc() {
	team := s.game.State.PlayerTeam

	if s.game.Resources == nil { // a plain (un landed) game has no resource map yet
		s.game.Resources = economy.NewResourceMap(s.wmap.W, s.wmap.H)
	}

	if s.game.MiningDifficulty == 0 { // a plain game leaves it 0; 4 = human normal
		s.game.MiningDifficulty = 4
	}

	gx, gy := s.wmap.W/2, s.wmap.H/2
	s.game.Resources.SetDeposit(gx, gy, economy.CargoRaw, 14)
	s.game.Resources.SetDeposit(gx+1, gy, economy.CargoFuel, 12)
	s.game.Resources.SetDeposit(gx, gy+1, economy.CargoGold, 8)
	u := game.NewUnitInfo(assets.MiningSt, team, s.spawnSeq+1, 0)
	u.Flags = game.FlagBuilding | game.FlagStationary
	u.Values = s.teamValues(team, assets.MiningSt)
	u.InitState()
	u.Order = game.OrderPowerOn             // powered, so NetProduction counts its mining
	u.State = game.OrderStateExecutingOrder // not Init, so it reads Order (not PriorOrder)
	s.spawnSeq++
	s.game.PlaceBuilding(u, gx, gy)
	s.game.Resources.SetInitialMining(u)
	s.plat.SetPalette(&s.pal)
	defer s.clearPortrait()
	buf := s.sys.Buf(bgWindow)
	s.repaintBackground(buf)
	s.draw(buf)
	s.openMenuScreen(func() { s.openAlloc(u) }, false)
}

// DebugOpenUpgrade gives the player team a gold reserve then opens the Upgrade
// screen (-menu upgrade), so the screen has something to spend.
func (s *Screen) DebugOpenUpgrade() {
	team := s.game.State.PlayerTeam

	if s.game.Gold[team] < 200 {
		s.game.Gold[team] = 500
	}

	s.plat.SetPalette(&s.pal)
	defer s.clearPortrait()
	buf := s.sys.Buf(bgWindow)
	s.repaintBackground(buf)
	s.draw(buf)
	s.openMenuScreen(s.openUpgrade, false)
}

// DebugOpenSelfDestruct opens the SELFDSTR arm/detonate dialog over a live game
// frame to inspect it; the -menu selfdestruct debug target. It does not destroy
// anything (the dialog result is ignored here).
func (s *Screen) DebugOpenSelfDestruct() {
	s.plat.SetPalette(&s.pal)
	buf := s.sys.Buf(bgWindow)
	s.repaintBackground(buf)
	s.draw(buf)
	s.openMenuScreen(func() {
		if s.hooks.selfDestructFn != nil {
			_, _ = s.hooks.selfDestructFn()
		}
	}, false)
}

// DebugOpenTransfer spawns two adjacent supply trucks (raw cargo, one full, one
// empty) for the player team then opens the Transfer screen between them
// (-menu transfer), so the screen has a real exchange to make.
func (s *Screen) DebugOpenTransfer() {
	team := s.game.State.PlayerTeam
	gx, gy := s.wmap.W/2, s.wmap.H/2
	src := s.deployUnit(assets.SplyTrck, game.FlagSelectable, team, 0, gx, gy)
	src.Storage = 30 // a loaded truck
	dst := s.deployUnit(assets.SplyTrck, game.FlagSelectable, team, 0, gx+1, gy)
	dst.Storage = 5 // a nearly empty truck
	s.selected = src
	s.plat.SetPalette(&s.pal)
	defer s.clearPortrait()
	buf := s.sys.Buf(bgWindow)
	s.repaintBackground(buf)
	s.draw(buf)
	s.openMenuScreen(func() { s.openTransfer(src, dst) }, false)
}

// DebugOpenRepairShop spawns a depot with a couple of damaged/empty adjacent
// units (and seeds its complex with raw), then opens the Repair Shop screen
// (-menu repairshop), so reload/repair/upgrade have something to act on.
func (s *Screen) DebugOpenRepairShop() {
	team := s.game.State.PlayerTeam
	gx, gy := s.wmap.W/2, s.wmap.H/2
	depot := game.NewUnitInfo(assets.Depot, team, s.spawnSeq+1, 0)
	depot.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	depot.Values = s.teamValues(team, assets.Depot)
	depot.InitState()
	s.spawnSeq++
	s.game.PlaceBuilding(depot, gx, gy)

	// An edge adjacent raw material dump so the depot's complex has raw to
	// service from (the gauge and reload/repair viability need a nonzero store).
	dump := game.NewUnitInfo(assets.ADump, team, s.spawnSeq+1, 0)
	dump.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	dump.Values = s.teamValues(team, assets.ADump)
	dump.InitState()
	dump.Storage = dump.Values.Get(game.AttribStorage) // full of raw
	s.spawnSeq++
	s.game.PlaceBuilding(dump, gx-2, gy)

	dmg := s.deployUnit(assets.Tank, game.FlagSelectable|game.FlagMobileLandUnit, team, 0, gx+2, gy)

	if dmg.Values != nil {
		dmg.Hits = dmg.Values.Get(game.AttribHits) / 3 // badly damaged
		dmg.Ammo = 0                                   // out of ammo
	}

	dmg2 := s.deployUnit(assets.Scout, game.FlagSelectable|game.FlagMobileLandUnit, team, 0, gx, gy+2)

	if dmg2.Values != nil {
		dmg2.Hits = dmg2.Values.Get(game.AttribHits) / 2
	}

	// The shop lists units STORED in the depot's bay, not units merely standing next
	// to it, so board both here or the screen opens on an empty list.
	s.game.LoadUnit(depot, dmg)
	s.game.LoadUnit(depot, dmg2)

	s.selected = depot

	s.plat.SetPalette(&s.pal)
	defer s.clearPortrait()
	buf := s.sys.Buf(bgWindow)
	s.repaintBackground(buf)
	s.draw(buf)
	s.openMenuScreen(func() { s.openRepairShop(depot) }, true)
}

// DebugOpenReports paints the game frame then opens the Reports screen (-menu reports).
// It seeds 50 test messages so the Messages tab's scrolling and icon/coords/text columns
// are testable: turn banners (I_CMPLX icon) and unit events (unit icon + red coordinate).
func (s *Screen) DebugOpenReports() {
	team := s.game.State.PlayerTeam
	cmplx := loadSimpleArt(s.lib, "I_CMPLX")
	evTypes := []assets.UnitType{assets.LandPlt, assets.GunTurrt, assets.Tank, assets.MiningSt, assets.Radar}
	evNames := []string{"Light Vehicle Plant", "Gun Turret", "Tank", "Mining Station", "Radar"}

	for i := 1; i <= 50; i++ {
		if i%3 == 0 {
			s.logEvent(cmplx, "", fmt.Sprintf("Begin turn %d.", i))
		} else {
			k := i % len(evTypes)
			s.logEvent(s.iconForType(evTypes[k], team),
				fmt.Sprintf("%d,%d", 20+i%30, 40+i%20),
				fmt.Sprintf("%s %d will be available in %d turns.", evNames[k], i, 1+i%5))
		}
	}

	s.plat.SetPalette(&s.pal)
	defer s.clearPortrait()
	buf := s.sys.Buf(bgWindow)
	s.repaintBackground(buf)
	s.draw(buf)
	s.openMenuScreen(s.openReports, true)
}

// DebugColonyStart spawns a powered Eco Sphere (Greenhouse) for the player team
// and one enemy team, then fires the colony started announce (voice + banner +
// message log) so it can be heard/seen (-menu colony, which then runs the game).
func (s *Screen) DebugColonyStart() {
	gx, gy := s.wmap.W/2, s.wmap.H/2
	spawnEco := func(team, x, y int) {
		g := game.NewUnitInfo(assets.Greenhse, team, s.spawnSeq+1, 0)
		g.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
		g.Values = s.teamValues(team, assets.Greenhse)
		g.InitState()
		g.Order = game.OrderPowerOn // operational
		g.State = game.OrderStateExecutingOrder
		s.spawnSeq++
		s.game.PlaceBuilding(g, x, y)
	}

	player := s.game.State.PlayerTeam
	enemy := (player + 1) % game.TeamCount
	spawnEco(player, gx, gy)
	spawnEco(enemy, gx+3, gy)
	s.checkColonyStarted() // play the announce(s) + log/banner now
}

// DebugOpenHelp paints the game frame then shows the game screen help blurb
// (-menu help).
func (s *Screen) DebugOpenHelp() {
	s.plat.SetPalette(&s.pal)
	defer s.clearPortrait()
	buf := s.sys.Buf(bgWindow)
	s.repaintBackground(buf)
	s.draw(buf)
	s.openMenuScreen(func() { s.openHelp("GAME_SCREEN_SETUP") }, false)
}

// DebugOpenLoad paints the game frame then opens the Load screen through the
// real in game ops (-menu loadgame), for verifying it lists saves. Debug only.
func (s *Screen) DebugOpenLoad() {
	s.plat.SetPalette(&s.pal)
	defer s.clearPortrait()
	buf := s.sys.Buf(bgWindow)
	s.repaintBackground(buf)
	s.draw(buf)
	s.openMenuScreen(s.openLoadMenu, true)
}

// DebugSpawn deploys one of each demo unit type across a small block of
// cells around the view center, for the -menu game screenshot view; the
// real New Game flow never calls it, so live games start empty.
func (s *Screen) DebugSpawn() {
	vw, vh := viewSize(MainMap)
	z := s.r.Zoom()

	cellX := clampInt((s.camera.scrollX+vw/2)/z, 0, s.wmap.W-2)
	cellY := clampInt((s.camera.scrollY+vh/2)/z, 0, s.wmap.H-2)

	for i := range debugUnitTypes {
		s.selected = s.deployAt(cellX+i%3, cellY+i/3)
	}

	// Hot seat demo: teams 0/1 are both human so RETURN rotates and shows the pass banner.
	s.game.State.Teams[1].Type = game.TeamPlayer
	s.game.State.ActiveTeam = s.game.State.FirstActiveTeam()
	s.game.State.PlayerTeam = s.game.State.ActiveTeam
	s.fogShow = true // show fog immediately; A toggles access
}

// DebugPopup spawns the demo units then force opens the order popup for the
// selected unit, so the unit/building popup menu can be screenshotted
// (-menu popup). Debug only.
func (s *Screen) DebugPopup() {
	s.DebugSpawn()
	u := s.selected

	if u == nil {
		return
	}

	u.Team = s.game.State.ActiveTeam // so the popup logic accepts it
	u.Flags |= game.FlagSelectable
	z := s.r.Zoom()
	mx := MainMap.ULX + u.GridX*z - s.camera.scrollX + z/2
	my := MainMap.ULY + u.GridY*z - s.camera.scrollY + z/2

	if !s.openUnitPopup(mx, my) { // fall back to a direct open if the cell lookup misses
		s.orderInput.popupUnit = u
		s.orderInput.popupItems = s.popupItemsFor(u)
		s.orderInput.popupX = mx + z
		s.orderInput.popupY = my - len(s.orderInput.popupItems)*popupPitch/2
		s.orderInput.popupOpen = len(s.orderInput.popupItems) > 0
	}
}

// DebugGameOver seeds the demo teams then forces the end of game screen with a
// PlayerLost outcome, for the -emulate death debug target.
func (s *Screen) DebugGameOver() {
	s.DebugGameOverOutcome(winloss.PlayerLost)
}

// DebugGameOverOutcome seeds a small two team game with 40 turns of fake score
// history, then immediately shows the game over screen with the given outcome.
// Used by -emulate win / -emulate lose so the score graph has visible data.
func (s *Screen) DebugGameOverOutcome(o winloss.Outcome) {
	s.RandomStart(time.Now().UnixNano(), 1)

	// Seed 40 turns of fake history so the score graph has visible lines.
	// Player grows faster (3 pts/turn) than the opponent (2 pts/turn).
	for i := 0; i < 40; i++ {
		for t := 0; t < game.TeamCount; t++ {
			if s.game.State.Teams[t].Type == game.TeamNone {
				continue
			}

			if t == s.game.State.PlayerTeam {
				s.game.TeamPoints[t] = (i + 1) * 3
			} else {
				s.game.TeamPoints[t] = (i + 1) * 2
			}
		}

		s.game.SnapshotScoreHistory()
	}

	s.enterGameOver(o)
}

// DebugSpawnShadow is the unit shadow inspection view (-menu shadow): one
// constructor at 1:1 zoom on land, no grid/fog, centered, so the ground
// shadow renders full size for comparison against the original.
func (s *Screen) DebugSpawnShadow() {
	s.r.SetZoom(64)
	s.grid = false
	s.fogShow = false
	vw, vh := viewSize(MainMap)
	cellX, cellY := s.wmap.W/2, s.wmap.H/2

	for y := 0; y < s.wmap.H; y++ { // find a land cell to spawn on

		for x := 0; x < s.wmap.W; x++ {
			if s.wmap.SurfaceAt(x, y)&world.SurfaceLand != 0 {
				cellX, cellY = x, y
				y, x = s.wmap.H, s.wmap.W
			}
		}
	}

	s.spawnSeq = 4 // Constrct is index 4 in debugUnitTypes
	s.selected = s.deployAt(cellX, cellY)
	s.camera.scrollX, s.camera.scrollY = centerScroll(cellX, cellY, 64, vw, vh)
}

// DebugBuild is the construction/economy view (-emulate build): one connected
// base complex (mining, power gen, 3 storage types, Heavy Vehicle Plant + Power
// Station, Research Centre) plus economy test props below (cargo trucks, an
// empty missile launcher for Reload, two idle builders) so the player can
// exercise build/X-Fer/Reload/research run gating by hand.
func (s *Screen) DebugBuild() {
	s.r.SetZoom(32)
	s.grid = true
	cellX, cellY := s.debugBuildSite()

	// Base strip (row cellY+3), each building abutting the next: ONE connected complex.
	s.debugPlaceBuilding(assets.MiningSt, cellX+0, cellY+3, true, game.OrderPowerOn) // 2x2 (cols 0,1)
	s.debugPlaceBuilding(assets.PowGen, cellX+2, cellY+3, false, game.OrderPowerOn)  // 1x1 (col 2)
	s.debugPlaceBuilding(assets.FDump, cellX+3, cellY+3, false, game.OrderPowerOn)   // fuel tank (col 3)
	s.debugPlaceBuilding(assets.GoldSm, cellX+4, cellY+3, false, game.OrderPowerOn)  // gold vault (col 4)
	s.debugPlaceBuilding(assets.ADump, cellX+5, cellY+3, false, game.OrderPowerOn)   // material storage (col 5)
	s.debugPlaceBuilding(assets.LandPlt, cellX+6, cellY+3, true, game.OrderPowerOn)  // Heavy Vehicle Plant (cols 6,7)
	s.debugPlaceBuilding(assets.Research, cellX+8, cellY+3, true, game.OrderPowerOn) // Research Centre (cols 8,9)
	// A large Power Station directly NORTH of the Heavy Vehicle Plant, abutting it so
	// it joins the complex and powers the factory.
	s.debugPlaceBuilding(assets.PowerStn, cellX+6, cellY+1, true, game.OrderPowerOn)

	// Allocate the research centre to a topic so it RUNS (hums + advances) at once;
	// the player can open Research and deallocate to see it fall idle (the gating).
	team := s.game.State.ActiveTeam
	s.game.State.Teams[team].Research[game.ResearchAttack].SetAllocation(game.ResearchFactors[game.ResearchAttack], 1)
	s.syncResearchOrders(team)

	// Economy props below the base (row cellY+6): one cargo truck per cargo kind +
	// an empty missile launcher to reload.
	s.debugPlaceCargoTruck(assets.SplyTrck, cellX+0, cellY+6) // material (raw)
	s.debugPlaceCargoTruck(assets.FuelTrck, cellX+2, cellY+6) // fuel
	s.debugPlaceCargoTruck(assets.GoldTrck, cellX+4, cellY+6) // gold
	// Missile Launcher with EMPTY ammo (a mobile combat unit carries ammo, not fuel):
	// park it by the supply truck and use Reload to refill it.
	rl := game.NewUnitInfo(assets.RocktLch, s.game.State.ActiveTeam, s.spawnSeq+1, 0)
	rl.Flags = flagsLand
	rl.Values = s.teamValues(s.game.State.ActiveTeam, assets.RocktLch)
	rl.InitState()
	rl.Speed = rl.Values.Get(game.AttribSpeed)
	rl.Ammo, rl.Shots = 0, 0 // empty: nothing to fire until reloaded
	s.spawnSeq++
	s.game.Units.Deploy(rl, cellX+6, cellY+6)

	// The two idle builders (NOT building): select one and start a build through the
	// real Construction Menu. deployIdleBuilder gives each full storage + movement.
	con := s.deployIdleBuilder(assets.Constrct, cellX+0, cellY+7)
	s.deployIdleBuilder(assets.Engineer, cellX+2, cellY+7)
	s.selected = con // start with the constructor selected so the Build button is ready

	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(cellX+4, cellY+3, 32, vw, vh)
}

// debugBuildSite finds the top left of a land block big enough for the build bench
// (11 wide x 9 tall), falling back to the map centre when the map has none.
func (s *Screen) debugBuildSite() (int, int) {
	cellX, cellY := s.wmap.W/2, s.wmap.H/2
	landBlock := func(x, y int) bool { // enough contiguous land for the bench (11 wide x 9 tall)
		for dy := 0; dy < 9; dy++ {
			for dx := 0; dx < 11; dx++ {
				if s.wmap.SurfaceAt(x+dx, y+dy)&world.SurfaceLand == 0 {
					return false
				}
			}
		}

		return true
	}

	for y := 2; y < s.wmap.H-10 && !landBlock(cellX, cellY); y++ {
		for x := 2; x < s.wmap.W-12; x++ {
			if landBlock(x, y) {
				cellX, cellY = x, y
				break
			}
		}
	}

	return cellX, cellY
}

// debugPlaceBuilding lands a finished building into the team's complex network; big = 2x2.
// order lets the Research Centre start powered OFF (test the run gating).
func (s *Screen) debugPlaceBuilding(t assets.UnitType, gx, gy int, big bool, order game.Order) *game.UnitInfo {
	flags := game.FlagStationary | game.FlagSelectable | game.FlagRequiresSlab

	if big {
		flags |= game.FlagBuilding
	}

	if game.CanHeadRotateType(t) {
		flags |= game.FlagTurretSprite | game.FlagCanHeadRotate
	}

	u := game.NewUnitInfo(t, s.game.State.ActiveTeam, s.spawnSeq+1, 0)
	u.Flags = flags
	u.Values = s.teamValues(s.game.State.ActiveTeam, t)
	u.InitState()
	u.Order = order
	u.State = game.OrderStateExecutingOrder
	s.spawnSeq++
	s.game.PlaceBuilding(u, gx, gy)

	return u
}

// debugPlaceCargoTruck deploys a full load cargo truck, for X-Fer.
func (s *Screen) debugPlaceCargoTruck(t assets.UnitType, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(t, s.game.State.ActiveTeam, s.spawnSeq+1, 0)
	u.Flags = flagsLand
	u.Values = s.teamValues(s.game.State.ActiveTeam, t)
	u.InitState()
	u.Speed = u.Values.Get(game.AttribSpeed)
	cap := u.Values.Get(game.AttribStorage)

	if cap <= 0 {
		cap = 40 // defensive default if the ATTRIBS lack a capacity
	}

	u.Storage = cap // full, so there is cargo to transfer out
	s.spawnSeq++
	s.game.Units.Deploy(u, gx, gy)

	return u
}

// deployIdleBuilder drops a selectable builder (constructor / engineer) of the
// active team at a land cell, idle and ready to be ordered, holding its FULL
// material capacity. Used by the -emulate build bench so the player initiates the
// build by hand. C++: n/a (a debug helper).
func (s *Screen) deployIdleBuilder(t assets.UnitType, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(t, s.game.State.ActiveTeam, s.spawnSeq+1, 0)
	u.Flags = flagsLand | game.FlagConstructorUnit
	u.Values = s.teamValues(s.game.State.ActiveTeam, t)
	u.InitState()

	// Start full at the unit's REAL material capacity (Constructor 60 / Engineer 40),
	// not an over cap stock: the CARGO gauge maxes at ATTRIB_STORAGE, so a stock of 200
	// would pin the bar full and hide the per turn material each build turn consumes.
	storeCap := u.Values.Get(game.AttribStorage)

	if storeCap <= 0 {
		storeCap = 40 // defensive default if the ATTRIBS lack a capacity
	}

	u.Storage = storeCap
	u.Speed = u.Values.Get(game.AttribSpeed) // movement budget so it can build and drive off
	s.spawnSeq++
	s.game.Units.Deploy(u, gx, gy)

	return u
}

// DebugBlowUp places a 2x2 mining station (laying its large slab) and a mobile
// Tank, then destroys both through the real destruction path, to verify the
// aftermath shows rubble but NO slab and that each death blast plays - the 2x2
// BLDEXPLD over the building and the small LNDEXPLD over the unit. -emulate blewup.
// (The blasts are aged so the frame-20 screenshot catches a mid animation frame.)
func (s *Screen) DebugBlowUp() {
	s.r.SetZoom(32)
	s.grid = true
	cellX, cellY := s.wmap.W/2, s.wmap.H/2
	land2 := func(x, y int) bool {
		for dy := 0; dy < 2; dy++ {
			for dx := 0; dx < 2; dx++ {
				if s.wmap.SurfaceAt(x+dx, y+dy)&world.SurfaceLand == 0 {
					return false
				}
			}
		}

		return true
	}

	for y := 2; y < s.wmap.H-3 && !land2(cellX, cellY); y++ {
		for x := 2; x < s.wmap.W-3; x++ {
			if land2(x, y) {
				cellX, cellY = x, y
				break
			}
		}
	}

	u := game.NewUnitInfo(assets.MiningSt, 0, s.spawnSeq+1, 0)
	u.Flags = game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable | game.FlagBuilding
	u.Values = s.teamValues(0, assets.MiningSt)
	u.InitState()
	s.spawnSeq++
	s.game.PlaceBuilding(u, cellX, cellY) // lays the large slab
	s.selected = u
	s.destroyUnit(u) // real path: explosion + rubble + remove building & slab

	// Blow up a mobile Tank a few cells west of the building too, so the bench shows
	// BOTH death blasts side by side: the 2x2 BLDEXPLD over the building and the
	// small LNDEXPLD over the unit (each leaving its own rubble).
	tank := s.deployUnit(assets.Tank, game.FlagMobileLandUnit|game.FlagSelectable|game.FlagTurretSprite, 0, 2, cellX-4, cellY)
	s.destroyUnit(tank)

	// Age each blast to ~40% of its OWN duration so the frame-20 screenshot catches
	// a mid animation frame for both the long building sheet and the shorter unit one
	// (a fixed offset would outlast the short LNDEXPLD and drop it before capture).
	for i := range s.effects.explosions {
		s.effects.explosions[i].start = platform.Ticks() - s.effects.explosions[i].dur*2/5
	}

	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(cellX, cellY, 32, vw, vh)
}
