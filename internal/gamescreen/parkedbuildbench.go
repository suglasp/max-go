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

// parkedBuildBenchZoom frames both parked builders together.
const parkedBuildBenchZoom = 48

// parkedBuildBenchAttack kills the parked Engineer in one forced shot.
const parkedBuildBenchAttack = 200

// DeployParkedBuildBench lays out the -emulate parkedbuild bench for the step 1443 fixes (PLAN-OKT2026 H13,
// H14). Two builders sit parked on the buildings they just finished, exit markers not yet picked:
//   - H13: a Constructor on a finished Depot with an Air Transport beside it. Load the Constructor with the
//     Air Transport: the Depot must unfold at once and the Constructor ride along out of its build pose.
//   - H14: an Engineer on a finished Gun Turret with a strong player Tank beside it. Force fire (Shift or
//     Ctrl click) the Engineer: the hidden turret goes with it, no slab or invisible blocker is left.
//
// C++: n/a (debug helper).
func (s *Screen) DeployParkedBuildBench() {
	const player = 0
	landFlags := game.FlagMobileLandUnit | game.FlagSelectable

	originX, originY, ok := s.snowBlock(12, 8)
	if !ok {
		originX, originY = 6, 6
	}
	x, y := originX+2, originY+3

	// H13: Constructor parked on a finished 2x2 Depot, Air Transport hovering just north of it.
	constructor := s.deployUnit(assets.Constrct, landFlags|game.FlagConstructorUnit, player, 2, x, y)
	depot := s.parkOnFinishedBuild(constructor, assets.Depot, game.FlagStationary|game.FlagBuilding|game.FlagRequiresSlab|game.FlagSelectable, x, y)
	air := s.deployUnit(assets.AirTrans, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, player, 4, x, y-1)

	// H14: Engineer parked on a finished 1x1 Gun Turret, a one shot player Tank two cells south of it.
	engineerX := x + 6
	engineer := s.deployUnit(assets.Engineer, landFlags|game.FlagConstructorUnit, player, 2, engineerX, y)
	turret := s.parkOnFinishedBuild(engineer, assets.GunTurrt, game.FlagStationary|game.FlagRequiresSlab|game.FlagSelectable, engineerX, y)
	tank := s.deployUnit(assets.Tank, landFlags, player, 0, engineerX, y+2)
	tank.Values.Set(game.AttribAttack, parkedBuildBenchAttack)

	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.selected = air
	s.showNames = true
	s.fogShow = false
	s.game.RefreshVisibility()

	s.r.SetZoom(parkedBuildBenchZoom)
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(x+3, y, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
	s.showMessage("Parked builder bench (press F to find it). H13: Air Transport popup Load, click the Constructor - the Depot must unfold at once. H14: select the Tank, Shift click the Engineer - its hidden Gun Turret must vanish with it, no slab left.")

	dbg.Printf("parkedbuild bench: constructor (%d,%d) ready=%v depot hidden=%v; engineer (%d,%d) ready=%v turret hidden=%v; air (%d,%d), tank (%d,%d)",
		constructor.GridX, constructor.GridY, s.game.IsBuildReady(constructor), depot != nil && depot.HiddenUntilExpand,
		engineer.GridX, engineer.GridY, s.game.IsBuildReady(engineer), turret != nil && turret.HiddenUntilExpand,
		air.GridX, air.GridY, tank.GridX, tank.GridY)
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
