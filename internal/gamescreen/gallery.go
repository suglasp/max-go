// gallery.go : provides the unit gallery overlay.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/world"
)

// galleryKind classifies a unit type for the -menu assets gallery: how it is
// flagged and where it is placed (land grid vs water grid). Types that are not
// real buildings or units (rubble, slabs, tape, explosions, projectiles,
// markers) are skipped.
type galleryKind int

const (
	gSkip          galleryKind = iota
	gBuildingLarge             // 2x2 stationary building
	gBuildingSmall             // 1x1 stationary building / fixed turret
	gLand                      // mobile land unit
	gSea                       // mobile sea unit
	gAir                       // mobile air unit
)

// galleryClass returns how a unit type is shown in the asset gallery. The
// building/mobile split and the footprint of 2x2 vs 1x1 follow the reconstructed
// data resource layout (large D_LRGBLD/D_MINING/D_RADAR buildings are 2x2);
// the air and sea rosters are the canonical M.A.X. flying and naval units.
func galleryClass(t assets.UnitType) galleryKind {
	switch t {
	// Air.
	case assets.Fighter, assets.Bomber, assets.AirTrans, assets.Awac, assets.AlnPlane:
		return gAir

	// Sea.
	case assets.FastBoat, assets.Corvette, assets.Battlshp, assets.Submarne,
		assets.SeaTrans, assets.MsslBoat, assets.SeaMnLyr, assets.CargoShp:
		return gSea

	// 2x2 buildings.
	case assets.CommTwr, assets.PowerStn, assets.Barracks, assets.ShieldGn,
		assets.Depot, assets.Hangar, assets.Dock, assets.Shipyard, assets.LightPlt,
		assets.LandPlt, assets.SuprtPlt, assets.AirPlt, assets.Habitat, assets.Research,
		assets.Greenhse, assets.RecCentr, assets.TrainHal, assets.MiningSt:
		return gBuildingLarge

	// 1x1 buildings and fixed turrets. The radar is a 1x1 detector (its dish still
	// animates), not a 2x2 building.
	case assets.Radar, assets.PowGen, assets.ADump, assets.FDump, assets.GoldSm, assets.Cnct4W,
		assets.WtrPltfm, assets.GunTurrt, assets.AntiAir, assets.ArtyTrrt, assets.AntiMssl,
		assets.Block, assets.Bridge:
		return gBuildingSmall

	// Mobile land units.
	case assets.Constrct, assets.Scout, assets.Tank, assets.Artillry, assets.RocktLch,
		assets.MisslLch, assets.SpFlak, assets.MineLayr, assets.Surveyor, assets.Scanner,
		assets.SplyTrck, assets.GoldTrck, assets.Engineer, assets.Bulldozr, assets.Repair,
		assets.FuelTrck, assets.ClnTrans, assets.Commando, assets.Infantry, assets.Juggrnt,
		assets.AlnTank, assets.AlnAsGun:
		return gLand

	default:
		return gSkip
	}
}

// galleryFlags is the flag set a gallery unit of the given kind deploys with.
// The turret/dish/body animations are all driven by data (from the unit's loaded
// frame layout), so only the category/footprint flags matter here.
func galleryFlags(kind galleryKind) game.Flags {
	switch kind {
	case gBuildingLarge:
		return game.FlagStationary | game.FlagBuilding | game.FlagSelectable

	case gBuildingSmall:
		return game.FlagStationary | game.FlagSelectable

	case gSea:
		return game.FlagMobileSeaUnit | game.FlagSelectable

	case gAir:
		return game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable

	default: // gLand
		return game.FlagMobileLandUnit | game.FlagSelectable
	}
}

// galleryLabel is a unit's name drawn under its sprite in the gallery.
type galleryLabel struct {
	name   string
	gx, gy int // grid cell the label is centred under
}

// DeployGallery lays out every building and unit type across the loaded world
// for the -menu assets view: buildings then land units on a grid over the land
// (snow) terrain, and the naval units on a grid over the water, each labelled
// with its name. Every sprite animates through the normal screen ticks, so the
// radar dish spins, the mining pump cycles, and turreted units show their guns.
func (s *Screen) DeployGallery() {
	// Two layout grids, spaced wide enough that a 2x2 building footprint plus
	// its name label do not collide with the next column (at galleryZoom a
	// 6-cell step is ~96px, room for most unit names).
	const (
		landCols, landStep = 8, 6
		seaCols, seaStep   = 8, 6
	)

	taken := map[game.Point]bool{}

	var land, sea []assets.UnitType

	for t := assets.UnitType(0); int(t) < assets.UnitCount; t++ {
		switch galleryClass(t) {
		case gSkip:
			continue

		case gSea:
			sea = append(sea, t)

		default:
			land = append(land, t)
		}
	}

	// Land/building grid, anchored at a land cell near the top left interior.
	landX0, landY0, ok := s.galleryAnchor(world.SurfaceLand, landCols*landStep, len(land)/landCols+1, landStep)

	if !ok {
		landX0, landY0 = 4, 4
	}

	for i, t := range land {
		gx := landX0 + (i%landCols)*landStep
		gy := landY0 + (i/landCols)*landStep
		s.placeGallerySnapped(t, gx, gy, world.SurfaceLand, landStep, taken)
	}

	// Sea grid over the nearest water region; naval units snap onto water so
	// they never sit on the snow (and buildings never sit in the water).
	wx, wy, ok := s.galleryAnchor(world.SurfaceWater, seaCols*seaStep, len(sea)/seaCols+1, seaStep)

	if !ok {
		wx, wy = 2, 2
	}

	for i, t := range sea {
		gx := wx + (i%seaCols)*seaStep
		gy := wy + (i/seaCols)*seaStep
		s.placeGallerySnapped(t, gx, gy, world.SurfaceWater, seaStep, taken)
	}

	dbg.Printf("gallery: land anchor (%d,%d) land=%d sea=%d deployed=%d", landX0, landY0, len(land), len(sea), len(s.game.Units.Units()))

	s.fogShow = false
	s.r.SetZoom(galleryZoom)
	s.camera.scrollX = clampInt((landX0-1)*s.r.Zoom(), 0, 1<<30)
	s.camera.scrollY = clampInt((landY0-1)*s.r.Zoom(), 0, 1<<30)
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
	s.startBench("Assets gallery: every building and land unit on the snow, the naval units on the water, each animating with its name below. Scroll to see them all.")
}

// galleryAnchor finds the top left of a wxh cell block (probed at the given
// step) whose corners all carry the wanted surface, scanning from near the map
// origin outward. It is a light placement helper, not an exact fit.
func (s *Screen) galleryAnchor(surface uint8, w, rows, step int) (int, int, bool) {
	h := rows * step

	for y := 2; y+h < s.wmap.H; y += 2 {
		for x := 2; x+w < s.wmap.W; x += 2 {
			if s.wmap.SurfaceAt(x, y)&surface != 0 &&
				s.wmap.SurfaceAt(x+w-1, y)&surface != 0 &&
				s.wmap.SurfaceAt(x, y+h-1)&surface != 0 {
				return x, y, true
			}
		}
	}

	return 0, 0, false
}

// placeGallerySnapped places a gallery unit near its grid slot, pulling it onto
// the nearest free cell whose whole footprint carries the wanted surface; so a
// building/land unit lands on snow and a naval unit on water even where the
// island's coastline is ragged. Falls back to the raw slot if no such cell is
// near. C++: n/a (a debug layout helper).
func (s *Screen) placeGallerySnapped(t assets.UnitType, gx, gy int, surface uint8, radius int, taken map[game.Point]bool) {
	kind := galleryClass(t)
	flags := galleryFlags(kind)
	n := build.Footprint(flags)

	fits := func(x, y int) bool {
		for fy := y; fy < y+n; fy++ {
			for fx := x; fx < x+n; fx++ {
				if fx < 0 || fy < 0 || fx >= s.wmap.W || fy >= s.wmap.H {
					return false
				}

				if s.wmap.SurfaceAt(fx, fy)&surface == 0 || taken[game.Point{X: fx, Y: fy}] {
					return false
				}
			}
		}

		return true
	}

	x, y, ok := s.nearestCell(game.Point{X: gx, Y: gy}, radius, fits)

	if !ok {
		x, y = gx, gy // fallback off terrain: place at the raw slot anyway
	}

	if x < 0 || y < 0 || x >= s.wmap.W || y >= s.wmap.H {
		return
	}

	if kind == gBuildingLarge || kind == gBuildingSmall {
		s.placeStartBuilding(galleryTeam, t, flags, x, y)
	} else {
		s.deployUnit(t, flags, galleryTeam, 2, x, y) // angle 2 = east
	}

	for fy := y; fy < y+n; fy++ {
		for fx := x; fx < x+n; fx++ {
			taken[game.Point{X: fx, Y: fy}] = true
		}
	}

	s.galleryLabels = append(s.galleryLabels, galleryLabel{name: displayName(t), gx: x, gy: y})
}

// galleryTeam is the owning team for gallery units; red reads well on snow.
const galleryTeam = 1

// galleryZoom is the pixels per tile the gallery opens at: big enough that
// every sprite (and its 2x2 footprint) reads clearly while still showing a
// block of the grid at once. The view scrolls with the arrow keys for the rest.
const galleryZoom = 24

// drawGalleryLabels writes each gallery unit's name under its cell, in the map
// view. Only drawn in the -menu assets gallery (galleryLabels is otherwise nil).
func (s *Screen) drawGalleryLabels(view *gfx.Buffer) {
	if len(s.galleryLabels) == 0 {
		return
	}

	z := s.r.Zoom()
	f := s.sys.Fonts().Get(fonts.Font1)
	col := fonts.Color{Base: 0xFF, Outline: 0xFF, Shadow: 199}

	for _, l := range s.galleryLabels {
		w := f.Width(l.name)
		tx := l.gx*z + z/2 - s.camera.scrollX - w/2
		ty := l.gy*z + 2*z - s.camera.scrollY + 1 // two tiles down: clears a 2x2 building

		if tx < 0 || ty < 0 || tx >= view.W || ty >= view.H {
			continue
		}

		f.BlitShadowed(view.Sub(tx, ty, view.W-tx, view.H-ty), l.name, w+1, col)
	}
}
