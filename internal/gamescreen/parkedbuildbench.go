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
)

// parkedBuildBenchZoom frames the three bench rows together.
const parkedBuildBenchZoom = 32

// parkedBuildBenchAttack kills the parked Engineer in one forced shot.
const parkedBuildBenchAttack = 200

// DeployParkedBuildBench lays out the -emulate parkedbuild bench for the October hand checks (PLAN-OKT2026),
// three rows on a snow block:
//   - row 1, H13: a Constructor parked on a finished Depot with an Air Transport beside it. Load the
//     Constructor: the Depot must unfold at once. H14: an Engineer parked on a finished Gun Turret with a one
//     shot player Tank beside it. Force fire (Shift or Ctrl click) the Engineer: the hidden turret goes with it.
//   - row 2, H2: a second Constructor parked on a finished Light Vehicle Plant and a second Engineer on a
//     finished Anti Aircraft gun. Sentry on the parked builder, then an exit marker: it drives off cleanly.
//   - row 3, H15: an idle Constructor and Engineer with full storage to open the Build menus (real unit
//     descriptions, one shared Description checkbox). H17: a running Habitat: select it and scroll away, its
//     hum pans and fades. H10: every unit here is a 2x2 minimap blip.
//
// C++: n/a (debug helper).
func (s *Screen) DeployParkedBuildBench() {
	const player = 0
	landFlags := game.FlagMobileLandUnit | game.FlagSelectable
	builderFlags := landFlags | game.FlagConstructorUnit
	bigFlags := game.FlagStationary | game.FlagBuilding | game.FlagRequiresSlab | game.FlagSelectable
	smallFlags := game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable

	originX, originY, ok := s.snowBlock(12, 14)
	if !ok {
		originX, originY = 6, 6
	}
	x, y := originX+2, originY+2
	engineerX := x + 6

	// Row 1. H13: Constructor parked on a finished 2x2 Depot, Air Transport hovering just north of it.
	constructor := s.deployUnit(assets.Constrct, builderFlags, player, 2, x, y)
	depot := s.parkOnFinishedBuild(constructor, assets.Depot, bigFlags, x, y)
	air := s.deployUnit(assets.AirTrans, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, player, 4, x, y-1)

	// H14: Engineer parked on a finished 1x1 Gun Turret, a one shot player Tank two cells south of it.
	engineer := s.deployUnit(assets.Engineer, builderFlags, player, 2, engineerX, y)
	turret := s.parkOnFinishedBuild(engineer, assets.GunTurrt, smallFlags, engineerX, y)
	tank := s.deployUnit(assets.Tank, landFlags, player, 0, engineerX, y+2)
	tank.Values.Set(game.AttribAttack, parkedBuildBenchAttack)

	// Row 2. H2: a second parked pair for the Sentry, then exit marker check.
	sentryY := y + 5
	sentryConstructor := s.deployUnit(assets.Constrct, builderFlags, player, 2, x, sentryY)
	plant := s.parkOnFinishedBuild(sentryConstructor, assets.LightPlt, bigFlags, x, sentryY)
	sentryEngineer := s.deployUnit(assets.Engineer, builderFlags, player, 2, engineerX, sentryY)
	antiAir := s.parkOnFinishedBuild(sentryEngineer, assets.AntiAir, smallFlags, engineerX, sentryY)

	// Row 3. H15: idle builders with a full load for the Build menus; H17: a running Habitat to hear its hum.
	menuY := y + 9
	for i, t := range []assets.UnitType{assets.Constrct, assets.Engineer} {
		u := s.deployUnit(t, builderFlags, player, 2, x+i*2, menuY)
		u.Storage = u.Values.Get(game.AttribStorage)
	}
	habitat := s.benchPlaceBuilding(assets.Habitat, player, engineerX, menuY, true)

	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.selected = air
	s.showNames = true
	s.fogShow = false
	s.game.RefreshVisibility()

	s.r.SetZoom(parkedBuildBenchZoom)
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(x+3, y+4, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
	s.showMessage("Parked builder bench (press F). Row 1 - H13: Air Transport Load, click the Constructor: the Depot unfolds at once. H14: select the Tank, Shift click the Engineer: its hidden turret vanishes too. Row 2 - H2: Sentry on a parked builder, then an exit marker: it drives off cleanly. Row 3 - H15: open Build on the idle Constructor / Engineer: real descriptions, one shared checkbox. H17: select the Habitat, scroll away: the hum pans and fades. H10: minimap blips 2x2.")

	dbg.Printf("parkedbuild bench: H13 constructor ready=%v depot hidden=%v; H14 engineer ready=%v turret hidden=%v; H2 constructor ready=%v plant hidden=%v, engineer ready=%v antiair hidden=%v; habitat order=%d; air (%d,%d) tank (%d,%d)",
		s.game.IsBuildReady(constructor), depot != nil && depot.HiddenUntilExpand,
		s.game.IsBuildReady(engineer), turret != nil && turret.HiddenUntilExpand,
		s.game.IsBuildReady(sentryConstructor), plant != nil && plant.HiddenUntilExpand,
		s.game.IsBuildReady(sentryEngineer), antiAir != nil && antiAir.HiddenUntilExpand,
		habitat.Order, air.GridX, air.GridY, tank.GridX, tank.GridY)
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
