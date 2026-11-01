// parkedbuildbench.go : provides the parked builder bench for the O7 hand checks.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// parkedBuildBenchZoom frames one scenario.
const parkedBuildBenchZoom = 32

// parkedBuildBenchAttack kills the parked Engineer in one forced shot.
const parkedBuildBenchAttack = 200

// parkedBuildScenario is one stop of the bench: the land block it needs, how it deploys, and its banner.
type parkedBuildScenario struct {
	w, h   int
	title  string
	text   string
	deploy func(x, y int) (game.Point, *game.UnitInfo) // fills the block at (x, y); returns the stop cell and the selected unit
}

// DeployParkedBuildBench lays out the -emulate parkedbuild bench for the October hand checks (PLAN-OKT2026),
// five scenarios, each on its own land block near its own compass anchor so only one is on screen at a time
// (Tab moves on):
//   - H13: a Constructor parked on a finished Depot with an Air Transport beside it. The Air Transport's Load
//     must refuse it: the select pointer appears over it and a click only selects it.
//   - H14: an Engineer parked on a finished Gun Turret with a one shot player Tank beside it. Force fire (Shift
//     or Ctrl click) the Engineer: the hidden turret goes with it.
//   - H2: a Constructor parked on a finished Light Vehicle Plant and an Engineer on a finished Anti Aircraft
//     gun. Their popup has no Sentry button; an exit marker drives each off cleanly.
//   - H15: an idle Constructor and Engineer with full storage to open the Build menus (real unit descriptions,
//     one shared Description checkbox).
//   - H17: a running Habitat on its own: select it and scroll away, its hum pans and fades.
//
// H10: every unit here is a 2x2 minimap blip.
//
// C++: n/a (debug helper).
func (s *Screen) DeployParkedBuildBench() {
	const player = 0
	landFlags := game.FlagMobileLandUnit | game.FlagSelectable
	builderFlags := landFlags | game.FlagConstructorUnit
	bigFlags := game.FlagStationary | game.FlagBuilding | game.FlagRequiresSlab | game.FlagSelectable
	smallFlags := game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable

	var constructor, engineer, sentryConstructor, sentryEngineer *game.UnitInfo
	var depot, turret, plant, antiAir, habitat, air, tank *game.UnitInfo

	scenarios := []parkedBuildScenario{
		{w: 4, h: 5, title: "H13 Air Transport cannot load a parked builder",
			text: "Air Transport popup Load, point at the Constructor: the select pointer (no load pointer), a click only selects it, nothing is picked up",
			deploy: func(x, y int) (game.Point, *game.UnitInfo) {
				constructor = s.deployUnit(assets.Constrct, builderFlags, player, 2, x+1, y+2)
				depot = s.parkOnFinishedBuild(constructor, assets.Depot, bigFlags, x+1, y+2)
				air = s.deployUnit(assets.AirTrans, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, player, 4, x+1, y+1)
				return game.Point{X: x + 2, Y: y + 2}, air
			}},
		{w: 3, h: 5, title: "H14 a parked builder is killed",
			text: "the Tank is selected: Shift click the Engineer, its hidden Gun Turret is lost too, no slab left",
			deploy: func(x, y int) (game.Point, *game.UnitInfo) {
				engineer = s.deployUnit(assets.Engineer, builderFlags, player, 2, x+1, y+1)
				turret = s.parkOnFinishedBuild(engineer, assets.GunTurrt, smallFlags, x+1, y+1)
				tank = s.deployUnit(assets.Tank, landFlags, player, 0, x+1, y+3)
				tank.Values.Set(game.AttribAttack, parkedBuildBenchAttack)
				return game.Point{X: x + 1, Y: y + 2}, tank
			}},
		{w: 7, h: 4, title: "H2 a parked builder has no Sentry",
			text: "open the parked Constructor's (and Engineer's) popup: no Sentry button, Done is there; click an exit marker: it drives off cleanly",
			deploy: func(x, y int) (game.Point, *game.UnitInfo) {
				sentryConstructor = s.deployUnit(assets.Constrct, builderFlags, player, 2, x+1, y+1)
				plant = s.parkOnFinishedBuild(sentryConstructor, assets.LightPlt, bigFlags, x+1, y+1)
				sentryEngineer = s.deployUnit(assets.Engineer, builderFlags, player, 2, x+5, y+1)
				antiAir = s.parkOnFinishedBuild(sentryEngineer, assets.AntiAir, smallFlags, x+5, y+1)
				return game.Point{X: x + 3, Y: y + 2}, sentryConstructor
			}},
		{w: 6, h: 4, title: "H15 unit descriptions in the Build menus",
			text: "open Build on the Constructor and on the Engineer: real descriptions, the box ticked; untick it in one, it stays off in the other",
			deploy: func(x, y int) (game.Point, *game.UnitInfo) {
				var first *game.UnitInfo
				for i, t := range []assets.UnitType{assets.Constrct, assets.Engineer} {
					u := s.deployUnit(t, builderFlags, player, 2, x+1+i*3, y+1)
					u.Storage = u.Values.Get(game.AttribStorage)
					if first == nil {
						first = u
					}
				}
				return game.Point{X: x + 3, Y: y + 2}, first
			}},
		{w: 4, h: 4, title: "H17 positional sound",
			text: "the Habitat hums: scroll left and right (the hum pans), far away (it fades), zoom out (quieter)",
			deploy: func(x, y int) (game.Point, *game.UnitInfo) {
				habitat = s.benchPlaceBuilding(assets.Habitat, player, x+1, y+1, true)
				return game.Point{X: x + 2, Y: y + 2}, habitat
			}},
	}

	for _, sc := range scenarios {
		var blockX, blockY int

		compass, ok := s.claimAnchor(func(anchor game.Point) bool {
			x, y, fits := s.landBlockNear(anchor, sc.w, sc.h, benchAnchorRadius)
			blockX, blockY = x, y
			return fits
		})
		if !ok {
			dbg.Printf("parkedbuild bench: no free compass anchor has a %dx%d land block for %s", sc.w, sc.h, sc.title)
			continue
		}

		cell, sel := sc.deploy(blockX, blockY)
		s.addBenchStop(benchStop{Compass: compass, Title: sc.title, Text: sc.text, Cell: cell, Select: sel})
		dbg.Printf("parkedbuild bench: %s %s block (%d,%d)", compass, sc.title, blockX, blockY)
	}

	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.showNames = true
	s.fogShow = false
	s.game.RefreshVisibility()

	s.r.SetZoom(parkedBuildBenchZoom)
	s.startBench("Parked builder bench. H10: every unit is a 2x2 minimap blip (1 pixel for mobile units with 2X on).")

	dbg.Printf("parkedbuild bench: H13 constructor ready=%v depot hidden=%v; H14 engineer ready=%v turret hidden=%v; H2 constructor ready=%v plant hidden=%v, engineer ready=%v antiair hidden=%v; habitat running=%v",
		constructor != nil && s.game.IsBuildReady(constructor), depot != nil && depot.HiddenUntilExpand,
		engineer != nil && s.game.IsBuildReady(engineer), turret != nil && turret.HiddenUntilExpand,
		sentryConstructor != nil && s.game.IsBuildReady(sentryConstructor), plant != nil && plant.HiddenUntilExpand,
		sentryEngineer != nil && s.game.IsBuildReady(sentryEngineer), antiAir != nil && antiAir.HiddenUntilExpand,
		habitat != nil && habitat.Order == game.OrderPowerOn)
}

// landBlockNear returns the top left of a w x h block of land with no unit on it, nearest to anchor and at
// most radius cells from it, or false.
//
// C++: n/a (debug helper).
func (s *Screen) landBlockNear(anchor game.Point, w, h, radius int) (int, int, bool) {
	free := func(x, y int) bool {
		if x+w > s.wmap.W || y+h > s.wmap.H {
			return false
		}

		for fy := y; fy < y+h; fy++ {
			for fx := x; fx < x+w; fx++ {
				if s.wmap.SurfaceAt(fx, fy)&world.SurfaceLand == 0 || len(s.game.Units.Cells.At(fx, fy)) > 0 {
					return false
				}
			}
		}

		return true
	}

	return s.nearestCell(anchor, radius, free)
}

// parkOnFinishedBuild has builder raise a product at (gx, gy) and runs the build to completion, so the
// builder ends parked build ready on the still hidden building. It returns that building, or nil.
func (s *Screen) parkOnFinishedBuild(builder *game.UnitInfo, t assets.UnitType, flags game.Flags, gx, gy int) *game.UnitInfo {
	product := game.NewUnitInfo(t, builder.Team, s.spawnSeq+1, 0)
	product.Flags = flags
	product.Values = s.teamValues(builder.Team, t)
	product.Values.Set(game.AttribTurns, 1) // finish on the first advance
	product.InitState()
	s.spawnSeq++

	builder.Storage = builder.Values.Get(game.AttribStorage) // a full load pays for the build
	s.game.StartBuild(builder, product, gx, gy, 1)

	for turn := 0; turn < 8 && s.game.BuildOrderFor(builder) != nil; turn++ {
		s.game.AdvanceBuilds(builder.Team)
	}

	if br := s.game.BuildReadyInfo(builder); br != nil {
		return br.Building
	}

	return nil
}
