// sprite.go : the M.A.X. unit sprite resolver - maps a unit's type, team, and state to the correct sprite resource tag and frame.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - sprite and animation frame selection
package units

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/world"
)

// isAmphibious reports whether a unit type swaps its sprite by surface, showing a boat
// form on water. Each amphibious type ships a water facing set (frames 8-15) beside the
// land set (0-7). Confirmed by decoding the retail sheets: the Personnel Carrier,
// Surveyor, Engineer and Scout each carry a distinct water set; the Constructor does
// NOT (its 8-15 are byte for byte the land set, and its 24-31 are a build phase, not a
// water variant), so it stays on its land look. C++: the AMPHIBIOUS image_base 0/8 swap.
func isAmphibious(t assets.UnitType) bool {
	switch t {
	case assets.ClnTrans, assets.Surveyor, assets.Engineer, assets.Scout:
		return true
	}
	return false
}

// amphibBase returns the body image_base for an amphibious unit on the given
// surface: the personnel carrier shows its boat at base 0 on water (land set at
// 8, pinned by bodyBaseOverride); the others use the land set at 0 and the water
// set at 8. C++: UnitInfo::SetSpriteFrameForTerrain (unitinfo.cpp).
func amphibBase(t assets.UnitType, water bool) int {
	if t == assets.ClnTrans {
		if water {
			return 0
		}
		return 8
	}
	if water {
		return 8
	}
	return 0
}

// amphibBodyBase returns the body image_base for the surface a unit stands on: an
// amphibious unit on full open water uses its water set, every other case uses
// imageBase. Build pose (+16) and facing frames are relative to this base, so an
// engineer building a bridge span over water shows its pontoon hull. Only the engineer
// of the amphibious types builds, so this only shifts the engineer's build frames.
func (c *SpriteCache) amphibBodyBase(u *game.UnitInfo, imageBase int) int {
	if c.surfaceAt != nil && isAmphibious(u.Type) && c.surfaceAt(u.GridX, u.GridY) == world.SurfaceWater {
		return amphibBase(u.Type, true)
	}
	return imageBase
}

// Reader loads a resource by its archive tag. *resfile.Library satisfies
// it; tests use a small stub held in memory. Keeping it an interface lets the
// units package stay free of the resource archive plumbing.
type Reader interface {
	Read(tag string) ([]byte, error)
}

// SpriteSet is a unit type's decoded art: its body/turret frames, the
// optional ground shadow silhouettes, and the frame layout that turns a
// facing into body and turret frame indices.
type SpriteSet struct {
	Body   *sprites.Multi // body + turret + firing frames
	Shadow *sprites.Multi // ground silhouettes, or nil
	Frame  game.FrameInfo // frame ranges + turret mount offsets
}

// shadowTag is the algorithmic shadow resource name: "S_" plus the first
// six characters of the unit tag (S_TANK, S_SCOUT, S_CONSTR...).
//
// C++: the shadow ResourceID per unit (units.json "shadow")
func shadowTag(tag string) string {
	if len(tag) > 6 {
		tag = tag[:6]
	}
	return "S_" + tag
}

// dataResource maps each unit type to its frame info ("D_...") resource. The retail data
// ships no units.json (the unit table was baked into max.exe), so this is reconstructed
// from the frame info templates: each body sheet fits one data file's frame span, pinned
// by role (a tank uses D_TANK, a fixed gun D_FIXED, a radar D_SCANNR, a plain truck
// D_MOBILE). Several are shared templates (D_LRGBLD/D_SMLBLD, D_FIRING/D_SUB, D_MOBILE)
// the "D_"+tag heuristic cannot find. Only the turret/dish files change an idle unit's
// render; the no turret ones match the zero value fallback (body frame = facing) until
// their firing/swim/walk animations are ported, and are listed for that future work. A
// type left out falls back to the "D_"+tag heuristic. C++: the "data" ResourceID per
// unit (units.json), read by Unit::Unit.
var dataResource = map[assets.UnitType]string{
	// Turreted and spinning dish units; these drive the turret render.
	assets.Tank: "D_TANK", assets.AlnTank: "D_ALTANK", assets.Battlshp: "D_BATTLS",
	assets.Scanner: "D_SCANNR", assets.Awac: "D_AWAC", assets.AntiAir: "D_ANTIAI",
	assets.SpFlak: "D_SP_FLK", assets.FastBoat: "D_ESCORT",
	assets.GunTurrt: "D_FIXED", assets.ArtyTrrt: "D_FIXED", assets.AntiMssl: "D_FIXED",

	// Animated buildings (the dish/drill cycle is a future render branch).
	assets.Radar: "D_RADAR", assets.MiningSt: "D_MINING", assets.Bridge: "D_BRIDGE",

	// Large buildings (D_LRGBLD: 2 state + 8 connector frames).
	assets.CommTwr: "D_LRGBLD", assets.PowerStn: "D_LRGBLD", assets.Barracks: "D_LRGBLD",
	assets.ShieldGn: "D_LRGBLD", assets.Depot: "D_LRGBLD", assets.Hangar: "D_LRGBLD",
	assets.Dock: "D_LRGBLD", assets.Shipyard: "D_LRGBLD", assets.LightPlt: "D_LRGBLD",
	assets.LandPlt: "D_LRGBLD", assets.SuprtPlt: "D_LRGBLD", assets.AirPlt: "D_LRGBLD",
	assets.Habitat: "D_LRGBLD", assets.Research: "D_LRGBLD", assets.Greenhse: "D_LRGBLD",
	assets.RecCentr: "D_LRGBLD", assets.TrainHal: "D_LRGBLD",

	// Small buildings (D_SMLBLD: 2 state + 4 connector frames).
	assets.PowGen: "D_SMLBLD", assets.ADump: "D_SMLBLD", assets.FDump: "D_SMLBLD",
	assets.GoldSm: "D_SMLBLD", assets.Cnct4W: "D_SMLBLD", assets.Block: "D_SMLBLD",

	// Mobile units with no turret (D_MOBILE: 8 facings, no turret).
	assets.MineLayr: "D_MOBILE", assets.SplyTrck: "D_MOBILE", assets.GoldTrck: "D_MOBILE",
	assets.Repair: "D_MOBILE", assets.FuelTrck: "D_MOBILE", assets.SeaTrans: "D_MOBILE",
	assets.SeaMnLyr: "D_MOBILE", assets.CargoShp: "D_MOBILE", assets.AirTrans: "D_MOBILE",

	// Combat/action hulls (D_FIRING: 8 facings + 8 firing frames, no turret).
	assets.Artillry: "D_FIRING", assets.RocktLch: "D_FIRING", assets.MisslLch: "D_FIRING",
	assets.Corvette: "D_FIRING", assets.MsslBoat: "D_FIRING", assets.Fighter: "D_FIRING",
	assets.Bomber: "D_FIRING", assets.Juggrnt: "D_FIRING", assets.AlnAsGun: "D_FIRING",
	assets.AlnPlane: "D_FIRING", assets.Bulldozr: "D_FIRING", assets.Surveyor: "D_FIRING",
	assets.ClnTrans: "D_FIRING",
	assets.Submarne: "D_SUB", assets.Rocket: "D_TORPDO", assets.Torpedo: "D_TORPDO",

	// 32 frame movers with no turret + alien projectile/soldier layouts.
	assets.Scout: "D_AMPHIB", assets.Engineer: "D_AMPHIB", assets.AlnMissl: "D_ALNMSL",
	assets.Commando: "D_COMMAN", assets.Infantry: "D_INFANT",
	assets.AlnTBall: "D_ALNPBL", assets.AlnABall: "D_ALNPBL", assets.TrpBuble: "D_BUBBLE",

	// Props/markers with a single frame (D_DEFALT).
	assets.SmlSlab: "D_DEFALT", assets.LrgCones: "D_DEFALT", assets.SmlCones: "D_DEFALT",
	assets.Road: "D_DEFALT", assets.LandPad: "D_DEFALT", assets.WtrPltfm: "D_DEFALT",
	assets.LandMine: "D_DEFALT", assets.SeaMine: "D_DEFALT", assets.Harvster: "D_DEFALT",
	assets.Waldo: "D_DEFALT",

	// Ground cover / rubble.
	assets.LrgSlab: "D_LRGSLA", assets.LrgRuble: "D_LRGRBL", assets.SmlRuble: "D_SMLRBL",
}

// bodyBaseOverride pins the body frame for units whose sheet holds two facing sets: a
// magenta placeholder in 0-7 and the normal sprite in 8-15. The personnel carrier's
// body and the submarine's surfaced body are the second set (0-7 being the sub submerged
// state, not yet shown). Without this the facing index lands on the magenta frames and
// the unit renders see through. C++: the image_base per state (e.g. SUBMARNE surface/dive).
var bodyBaseOverride = map[assets.UnitType]int{
	assets.ClnTrans: 8,
	assets.Submarne: 8,
}

// dataTag is the frame info resource name for a unit type: its dataResource entry, or
// the "D_"+tag[:6] heuristic for types not yet pinned (which for a no turret unit MISSes
// and falls back to the zero value layout of body frame as facing).
// C++: the "data" ResourceID per unit (units.json "data")
func dataTag(t assets.UnitType) string {
	if d, ok := dataResource[t]; ok {
		return d
	}
	tag := t.Tag()
	if len(tag) > 6 {
		tag = tag[:6]
	}
	return "D_" + tag
}

// SpriteCache decodes and memoizes a SpriteSet per unit type, building the team color
// remap tables and the shadow darkening LUT from the active palette once. Hand it the
// resource reader plus the in game palette and it draws units onto the map viewport.
// C++: ResourceManager_GetUnit sprite loading + Gfx_DecodeSprite/Shadow
type SpriteCache struct {
	r         Reader
	cache     map[assets.UnitType]*SpriteSet
	teams     [5][256]uint8
	shadow    *[256]uint8
	surfaceAt func(x, y int) uint8 // optional terrain lookup for the amphibious on water sprite; nil = land frames
}

// SetSurfaceSource lets DrawUnit pick the water frame for amphibious units
// from the cell they stand on. Pass the MODIFIED surface (a Water Platform or
// Bridge over water reads as land, so a unit on one shows its land frame), not the
// raw tile; leaving it nil (headless/tests) keeps every unit on its land frame.
func (c *SpriteCache) SetSurfaceSource(f func(x, y int) uint8) {
	c.surfaceAt = f
}

// NewSpriteCache returns a cache that loads art through r and recolors it
// with tables derived from pal (team ramps and the unit shadow LUT). The
// shadow uses brightness level 5 (~71%), matching Gfx_DecodeShadow; not the
// darker minimap darkening table.
func NewSpriteCache(r Reader, pal *gfx.Palette) *SpriteCache {
	c := &SpriteCache{r: r, cache: map[assets.UnitType]*SpriteSet{}, shadow: pal.BrightnessTable(5)}
	for i := range c.teams {
		c.teams[i] = sprites.TeamColorTable(sprites.TeamColor(i))
	}
	return c
}

// SetPalette rebuilds the palette derived shadow darkening LUT from pal. An in game
// load can swap to another world whose terrain palette (slots 64..159) differs; the
// shadow LUT maps each terrain index to a dimmer index of that same palette, so without
// a rebuild after the swap it still points at the PREVIOUS world's colours and unit
// ground shadows render in the wrong hue (brown on snow, blue over grass). The team
// colour tables are fixed slot index remaps, palette independent, so only the shadow
// LUT needs recomputing; the decoded body/turret frames are index data, unaffected.
// C++: World::ApplyPaletteAndTints regenerates BrightnessColorIndexTable on a palette change.
func (c *SpriteCache) SetPalette(pal *gfx.Palette) {
	c.shadow = pal.BrightnessTable(5)
}

// Set returns the decoded art for a unit type, loading it on first use.
// A unit whose body sprite cannot be read yields nil (the caller skips
// drawing it); a missing shadow or data resource is tolerated.
func (c *SpriteCache) Set(t assets.UnitType) *SpriteSet {
	if s, ok := c.cache[t]; ok {
		return s
	}
	s := c.load(t)
	c.cache[t] = s // cache nil too, so a missing sprite isn't retried each frame
	return s
}

// load resolves and caches the sprite set for one unit type.
func (c *SpriteCache) load(t assets.UnitType) *SpriteSet {
	tag := t.Tag()
	bodyData, err := c.r.Read(tag)
	if err != nil {
		return nil
	}
	body, err := sprites.DecodeMultiSprite(bodyData)
	if err != nil {
		return nil
	}
	s := &SpriteSet{Body: body}
	if sd, err := c.r.Read(shadowTag(tag)); err == nil {
		if shadow, err := sprites.DecodeMultiShadow(sd); err == nil {
			s.Shadow = shadow
		}
	}
	if dd, err := c.r.Read(dataTag(t)); err == nil {
		s.Frame = game.DecodeFrameInfo(dd)
	}
	if base, ok := bodyBaseOverride[t]; ok {
		s.Frame.ImageBase = base
		s.Frame.HasData = true
	}
	return s
}

// FrameCount returns how many body frames a unit type's sheet has, or 0 when
// the type has no sprite. Used to pace effect animations (explosions, rubble).
func (c *SpriteCache) FrameCount(t assets.UnitType) int {
	set := c.Set(t)
	if set == nil || set.Body == nil {
		return 0
	}
	return len(set.Body.Frames)
}

// TurretCount returns how many turret frames a unit type's data resource
// declares, or 0 when the type has no turret/dish art. Used to wrap a
// spinning turret's animation frame. C++: FrameInfo::turret_image_count.
func (c *SpriteCache) TurretCount(t assets.UnitType) int {
	set := c.Set(t)
	if set == nil {
		return 0
	}
	return set.Frame.TurretImageCount
}

// DrawFrameAtPixel composites one body frame at an arbitrary PIXEL position (tile*64
// world space, like UnitInfo.X/Y), neutral coloured. Used for a travelling missile +
// its trail, which move smoothly between cells rather than snapping to one.
// C++: Paths_DrawMissile blits the ROCKET/TORPEDO + RKTSMOKE at interpolated positions.
func (c *SpriteCache) DrawFrameAtPixel(dst *gfx.Buffer, t assets.UnitType, frame, pixelX, pixelY, zoom, scrollX, scrollY int) {
	set := c.Set(t)
	if set == nil || set.Body == nil {
		return
	}
	const tile = 64
	sx := pixelX*zoom/tile - scrollX
	sy := pixelY*zoom/tile - scrollY
	if f := frameAt(set.Body, frame); f != nil {
		f.DrawScaled(dst, sx, sy, zoom, tile, &c.teams[0])
	}
}

// DrawEffect composites one frame of a teamless effect sprite (explosion or rubble)
// centred on an n x n cell footprint, neutral coloured. footprint 1 centres on a single
// cell; 2 on a 2x2 building footprint so a large blast sits over the whole structure.
// A frame out of range draws nothing. C++: the blits for a hit explosion or rubble.
func (c *SpriteCache) DrawEffect(dst *gfx.Buffer, t assets.UnitType, frame, cellX, cellY, footprint, zoom, scrollX, scrollY int) {
	set := c.Set(t)
	if set == nil || set.Body == nil {
		return
	}
	const tile = 64
	sx := cellX*zoom + footprint*zoom/2 - scrollX
	sy := cellY*zoom + footprint*zoom/2 - scrollY
	if f := frameAt(set.Body, frame); f != nil {
		// Centre by dimensions (not the baked hotspot): explosion frames carry
		// hotspots off centre that track the blast origin, which would throw a
		// 2x2 building blast a cell down and to the right of its footprint.
		f.DrawScaledCentered(dst, sx, sy, zoom, tile, &c.teams[0])
	}
}

// iconAngle is the facing used by icons of mobile units: angle 2 = east
// (pointing right), the catalog orientation. Buildings have no facing frames,
// so their icon uses frame 0.
const iconAngle = 2

// iconFit is the target cell size the icon is scaled down to fit.
const iconFit = 30

// DrawIcon composites a unit type's icon (body facing east plus turret for a mobile
// unit, or frame 0 for a building), team coloured, scaled down by a clean integer
// divisor to fit ~iconFit px and centred at (cx, cy). An integer divisor avoids the
// muddy colours of an arbitrary resample, and fitting handles the wildly different
// native sizes. C++: ReportStats_DrawListItemIcon / ReportStats_RenderSprite.
func (c *SpriteCache) DrawIcon(dst *gfx.Buffer, t assets.UnitType, team, cx, cy int) {
	set := c.Set(t)
	if set == nil || set.Body == nil {
		return
	}
	mobile := t >= assets.Constrct // units from Constructor on have facing frames
	bodyFrame := set.Frame.ImageBase
	if mobile {
		bodyFrame += iconAngle
	}
	bf := frameAt(set.Body, bodyFrame)
	if bf == nil {
		bf = frameAt(set.Body, set.Frame.ImageBase)
	}
	if bf == nil {
		return
	}
	// Pick the smallest integer divisor that fits the (larger) body dimension.
	span := bf.W
	if bf.H > span {
		span = bf.H
	}
	div := 1
	for span/div > iconFit {
		div++
	}
	remap := &c.teams[teamIndex(team)]
	// drawScaled blits frame fi at 1/div, its centre at (hx, hy).
	drawScaled := func(fi, hx, hy int) {
		f := frameAt(set.Body, fi)
		if f == nil {
			return
		}
		dw, dh := f.W/div, f.H/div
		ox, oy := hx-dw/2, hy-dh/2
		for y := 0; y < dh; y++ {
			ty := oy + y
			if ty < 0 || ty >= dst.H {
				continue
			}
			srow := (y * div) * f.W
			for x := 0; x < dw; x++ {
				sx := x * div
				if !f.Opaque[srow+sx] {
					continue
				}
				if tx := ox + x; tx >= 0 && tx < dst.W {
					dst.Set(tx, ty, remap[f.Pix[srow+sx]])
				}
			}
		}
	}
	drawScaled(bodyFrame, cx, cy)
	if mobile && set.Frame.TurretImageCount > 0 {
		off := set.Frame.AngleOffsets[iconAngle]
		drawScaled(set.Frame.TurretImageBase+iconAngle, cx+off.X/div, cy+off.Y/div)
	}
}

// DrawUnit composites one unit onto the map view: ground shadow, body, and (for
// turreted types) turret, scaled to zoom pixels per 64 pixel tile, anchored so the
// hotspot lands on the unit's cell center. scrollX/scrollY are the view's top left
// corner in scaled map pixels. C++: UnitInfo::Draw shadow/sprite/turret passes.
func (c *SpriteCache) DrawUnit(dst *gfx.Buffer, u *game.UnitInfo, zoom, scrollX, scrollY int) {
	set := c.Set(u.Type)
	if set == nil {
		return
	}
	team := &c.teams[teamIndex(u.Team)]
	const tile = 64

	// Pixel position (cell center at rest, interpolated while a MoveOrder
	// is driving the unit between cells) scaled to the current zoom.
	sx := u.X*zoom/tile - scrollX
	sy := u.Y*zoom/tile - scrollY

	// Landing unfold: a structure freshly landed scales up to full over a few
	// steps (Unfold 4 -> 0). The sprite is drawn at a reduced zoom but anchored
	// on the same centre (DrawScaled pins the hotspot), so it grows in place.
	// C++: scaler_adjust shrinks the sprite during ORDER_STATE_EXPAND.
	drawZoom := zoom
	if u.Unfold > 0 {
		drawZoom = zoom * int(5-u.Unfold) / 5
		if drawZoom < 1 {
			drawZoom = 1
		}
	}

	bodyFrame := c.bodyFrameFor(u, set)

	// A submerged submarine renders as a dark underwater silhouette (drawn through
	// the shadow LUT, no team colour and no ground shadow) so it reads as "below
	// the surface"; surfacing shows the full sprite.
	submerged := u.Type == assets.Submarne && !u.Surfaced

	d := unitDraw{
		sx: sx, sy: sy, zoom: zoom, drawZoom: drawZoom,
		bodyFrame: bodyFrame, submerged: submerged, team: team,
	}

	c.drawUnitShadow(dst, u, set, d)
	c.drawUnitBody(dst, set, d)
	c.drawUnitConnectors(dst, u, set, d)
	c.drawUnitTurret(dst, u, set, d)
}

// unitDraw carries what DrawUnit resolves once and each draw pass then needs: where the
// sprite lands, at what zoom, which body frame, whether it renders as a submerged
// silhouette, and the owning team's colour ramp. Passed by value, so the passes stay
// allocation free on a per frame path.
type unitDraw struct {
	sx, sy    int
	zoom      int
	drawZoom  int
	bodyFrame int
	submerged bool
	team      *[256]uint8
}

// bodyFrameFor resolves which body frame a unit shows.
//
// A unit with a body of several frames rotates with its facing; a body of a single
// frame (a fixed turret mounted on a static base) never rotates the hull; an animated
// building (radar dish, mining drill) cycles its body frames in place, independent of
// facing.
func (c *SpriteCache) bodyFrameFor(u *game.UnitInfo, set *SpriteSet) int {
	bodyFrame := u.BodyFrameIndex()
	switch {
	case u.Building:
		// A builder faces diagonally inward. While gliding onto the centre it shows the
		// plain facing frame; once in place it switches to the build pose (facing + 16).
		// An amphibious builder (the engineer) over open water uses its water hull set
		// (base 8), so both frames come from 8-15 / 24-31 not the land ones. C++:
		// TAPE_POSITIONING_ENTER drives in, then DrawSpriteFrame(image_index + 16).
		base := c.amphibBodyBase(u, set.Frame.ImageBase) + int(u.Angle&7)
		bodyFrame = base
		if u.X == u.BuildToX && u.Y == u.BuildToY {
			if bf := base + 16; set.Body != nil && bf < len(set.Body.Frames) {
				bodyFrame = bf
			}
		}
	case u.Clearing:
		// A bulldozer faces diagonally inward and glides onto the rubble site
		// (plain facing frame); once centred it shows its +8 "working, blade down"
		// pose while it clears. C++: PositionInTape slide then DrawSpriteFrame(+8).
		base := set.Frame.ImageBase + int(u.Angle&7)
		bodyFrame = base
		if u.X == u.BuildToX && u.Y == u.BuildToY {
			if bf := base + 8; set.Body != nil && bf < len(set.Body.Frames) {
				bodyFrame = bf
			}
		}
	case isSoldier(u.Type):
		// A marching soldier cycles walk frames (image_base + angle + 8*phase);
		// its body sheet also holds firing/death frames past the walk range, so
		// the standing/walk render must come before the ImageCount<=1 branch
		// (these types decode a negative image_count and would wrongly match it).
		bodyFrame = soldierWalkFrame(set.Frame.ImageBase, u.Angle, u.WalkFrame)
	case c.surfaceAt != nil && isAmphibious(u.Type):
		// An amphibious unit shows its water (boat/hover) facing set only on FULL
		// open water; a coast/shore tile keeps the land set (the unit is grounded
		// on the shallows). C++: UnitInfo::SetSpriteFrameForTerrain swaps image_base
		// (0 vs 8, inverted for the personnel carrier) on surface == SURFACE_TYPE_WATER.
		water := c.surfaceAt(u.GridX, u.GridY) == world.SurfaceWater
		if bf := amphibBase(u.Type, water) + int(u.Angle&7); set.Body != nil && bf < len(set.Body.Frames) {
			bodyFrame = bf
		}
	case u.Type == assets.Submarne:
		// A submarine always uses its clean surfaced facing set (base 8); when
		// submerged it is drawn below as a dark underwater silhouette (the raw
		// submerged frames 0-7 are a magenta placeholder that the renderer cannot blend yet).
		// It surfaces (u.Surfaced) only when it attacks, is attacked, or is spotted
		// by a Corvette. C++: the SUBMARNE image_base 0/8 swap (SpotByTeam).
		if bf := 8 + int(u.Angle&7); set.Body != nil && bf < len(set.Body.Frames) {
			bodyFrame = bf
		}
	case u.Type == assets.Bridge:
		// A bridge shows its raise frame: image_base+0 is flat (land units cross over)
		// up through image_base+BridgeRaiseMax fully raised (a sea unit passes under).
		// C++: BRIDGE image_index = image_base + the elevate step (unitinfo.cpp).
		if bf := set.Frame.ImageBase + u.BridgeRaise; set.Body != nil && bf >= 0 && bf < len(set.Body.Frames) {
			bodyFrame = bf
		}
	case set.Frame.HasData && set.Frame.ImageCount <= 1:
		bodyFrame = set.Frame.ImageBase
	case isAnimatedBody(u.Type, u.Flags):
		bodyFrame = set.Frame.ImageBase + int(u.BodyAnim)%set.Frame.ImageCount
	case isBuildingStateBody(u):
		// A building body with two states: image_base is idle/unpowered, image_base+1
		// the lit "running" frame while powered on (the generator/plant animation). C++:
		// ProcessOrderPowerOn -> DrawSpriteFrame(image_base + 1); PowerDown -> image_base.
		// A factory the player STOPPED keeps its queue but is paused (rate 0); the port
		// leaves it ORDER_POWER_ON, so exclude a halted factory here or a stopped plant
		// would read as still lit/running.
		bodyFrame = set.Frame.ImageBase
		if u.Order == game.OrderPowerOn && !factoryHalted(u) {
			if bf := set.Frame.ImageBase + 1; set.Body != nil && bf < len(set.Body.Frames) {
				bodyFrame = bf
			}
		}
	}
	return bodyFrame
}

// drawUnitShadow lays the ground shadow under the body. A hovering air unit's shadow
// sits one tile down and to the right of the unit (it flies above its shadow).
func (c *SpriteCache) drawUnitShadow(dst *gfx.Buffer, u *game.UnitInfo, set *SpriteSet, d unitDraw) {
	const tile = 64

	if set.Shadow != nil && !d.submerged {
		shx, shy := d.sx, d.sy
		if u.Flags.Has(game.FlagMobileAirUnit) && game.AirAltMax > 0 {
			// Shadow offset scales with altitude: a full tile down and to the right
			// while flying (AirAlt == AirAltMax), under the body when landed, gliding
			// between the two while taking off or landing. C++: the HOVERING shadow_offset.
			off := d.zoom * u.AirAlt / game.AirAltMax
			shx += off
			shy += off
		}
		if f := frameAt(set.Shadow, d.bodyFrame); f != nil {
			f.DrawShadowScaled(dst, shx, shy, d.drawZoom, tile, c.shadow)
		}
	}
}

// drawUnitBody paints the hull itself, as a dark silhouette while submerged.
func (c *SpriteCache) drawUnitBody(dst *gfx.Buffer, set *SpriteSet, d unitDraw) {
	const tile = 64

	if f := frameAt(set.Body, d.bodyFrame); f != nil {
		if d.submerged {
			f.DrawShadowScaled(dst, d.sx, d.sy, d.drawZoom, tile, c.shadow)
		} else {
			f.DrawScaled(dst, d.sx, d.sy, d.drawZoom, tile, d.team)
		}
	}
}

// drawUnitConnectors overlays a connector frame on each edge that abuts a building or
// connector on the same team (the tube for auto rigging). The bit index is the frame
// offset from ConnectorImageBase, matching the original's draw per direction.
//
// C++: UnitInfo::RenderWithConnectors (unitinfo.cpp).
func (c *SpriteCache) drawUnitConnectors(dst *gfx.Buffer, u *game.UnitInfo, set *SpriteSet, d unitDraw) {
	const tile = 64

	if set.Frame.ConnectorImageCount > 0 && u.Connectors != 0 {
		for bit := 0; bit < 8; bit++ {
			if u.Connectors&(1<<bit) == 0 {
				continue
			}
			cf := set.Frame.ConnectorImageBase + bit
			if set.Body == nil || cf >= len(set.Body.Frames) {
				continue
			}
			if f := frameAt(set.Body, cf); f != nil {
				f.DrawScaled(dst, d.sx, d.sy, d.drawZoom, tile, d.team)
			}
		}
	}
}

// drawUnitTurret mounts the turret at the offset from the body center for its facing. A
// spinning turret (radar dish, turret frame count > 8) cycles its own frame range
// continuously, independent of the body facing; an aiming turret tracks turret_angle.
func (c *SpriteCache) drawUnitTurret(dst *gfx.Buffer, u *game.UnitInfo, set *SpriteSet, d unitDraw) {
	const tile = 64

	if set.Frame.TurretImageCount > 0 {
		off := set.Frame.AngleOffsets[u.Angle&7]
		tx := d.sx + off.X*d.zoom/tile
		ty := d.sy + off.Y*d.zoom/tile
		ti := u.TurretFrameIndex()
		if isSpinning(set.Frame, u.Flags) {
			ti = set.Frame.TurretImageBase + int(u.SpinFrame)%set.Frame.TurretImageCount
		}
		if f := frameAt(set.Body, ti); f != nil {
			f.DrawScaled(dst, tx, ty, d.zoom, tile, d.team)
		}
	}
}

// animatedBodyTypes are the buildings that cycle their body frames as an idle
// animation. Only the radar qualifies in retail data (16 frames = a rotating dish).
// MININGST is NOT animated: its 16 frames are the eight clan emblems, one per owning
// clan, so cycling them would roll every clan's logo past. Plants only animate while
// powered (not gated yet), so they stay static.
// C++: the ANIMATED flag (units.json), absent from the retail data here.
var animatedBodyTypes = map[assets.UnitType]bool{
	assets.Radar: true,
}

// isAnimatedBody reports whether a unit cycles its body frames in place; a
// stationary building with a moving part (the radar's dish). FlagAnimated
// forces it when set. C++: the ANIMATED flag.
func isAnimatedBody(t assets.UnitType, flags game.Flags) bool {
	return flags.Has(game.FlagAnimated) || animatedBodyTypes[t]
}

// factoryHalted reports whether u is a factory the player STOPPED: it keeps a build
// queue but its rate is 0 (paused). Such a factory stays ORDER_POWER_ON in the live
// model, so the lit "running" body frame must exclude it or a stopped plant reads as
// still producing. A not a factory building never has a queue, so this is always false.
func factoryHalted(u *game.UnitInfo) bool {
	return len(u.BuildQueue) > 0 && u.BuildRate == 0
}

// isBuildingStateBody reports whether a unit is a building whose body has two states
// (image_base = idle, image_base+1 = lit/running). Excludes the mining station (its 16
// frames are clan emblems, not an off/on pair) and animated buildings (the radar cycles
// its dish). C++: the BUILDING power state frame swap in ProcessOrderPowerOn / PowerDown.
func isBuildingStateBody(u *game.UnitInfo) bool {
	if !u.Flags.Has(game.FlagStationary) || u.Type == assets.MiningSt {
		return false
	}
	return !isAnimatedBody(u.Type, u.Flags)
}

// BodyAnimCount returns the length of the body cycle for an animated unit
// type given its flags, or 0 when the type does not cycle its body. The caller
// advances AdvanceBodyAnim by this each tick. C++: image_count for an ANIMATED unit.
func (c *SpriteCache) BodyAnimCount(t assets.UnitType, flags game.Flags) int {
	set := c.Set(t)
	if set == nil || !isAnimatedBody(t, flags) {
		return 0
	}
	return set.Frame.ImageCount
}

// soldierImageIndexMax is the highest body frame a marching soldier (infantry or
// commando) walks through. The original hardcodes image_index_max = 103; their sheets
// hold firing/death frames past that, which the walk must not run into. With image_base
// 0 and 8 facings that is 13 walk phases (a standing phase plus 12 step frames at
// image_base + angle + 8*phase). C++: UnitInfo::Init image_index_max = 103.
const soldierImageIndexMax = 103

// isSoldier reports whether a unit type is a marching infantry/commando; the
// only types with the walk cycle that steps 8 per frame.
//
// C++: unit_type == COMMANDO || unit_type == INFANTRY (unitinfo.cpp).
func isSoldier(t assets.UnitType) bool {
	return t == assets.Infantry || t == assets.Commando
}

// soldierWalkPhases is how many walk cycle phases a soldier has for a body image
// base: phase k uses frame base+angle+8k while that stays at or below
// soldierImageIndexMax. C++: derived from image_index_max = 103.
func soldierWalkPhases(base int) int {
	if base > soldierImageIndexMax {
		return 1
	}
	return (soldierImageIndexMax-base)/8 + 1
}

// soldierWalkFrame is the body frame for a soldier at the given facing and walk phase:
// image_base + angle + 8*phase, never above image_index_max; it wraps to the standing
// frame for the facing when image_index+8 overflows, as the original does.
// C++: the image_index+8 vs image_base+angle branch in UnitInfo::Move.
func soldierWalkFrame(base int, angle, phase uint8) int {
	idx := base + int(angle&0x07) + 8*int(phase)
	if idx > soldierImageIndexMax {
		idx = base + int(angle&0x07)
	}
	return idx
}

// WalkPhaseCount returns how many walk cycle phases a soldier type has, or 0 for
// any unit that is not a soldier. The caller advances AdvanceWalk by this each step while the
// soldier moves. C++: the COMMANDO/INFANTRY walk advance (image_index_max=103).
func (c *SpriteCache) WalkPhaseCount(t assets.UnitType) int {
	if !isSoldier(t) {
		return 0
	}
	set := c.Set(t)
	if set == nil {
		return 0
	}
	return soldierWalkPhases(set.Frame.ImageBase)
}

// isSpinning reports whether a unit's turret is a dish that rotates continuously rather
// than an aiming gun. The retail data has no SPINNING_TURRET flag on the instance, so
// the dish is detected from its frame layout: only the radar SCANNER (16) and AWAC (30)
// carry more than the 8 turret facings of an aiming gun. FlagSpinningTurret still forces
// it when set. C++: the SPINNING_TURRET flag.
func isSpinning(fi game.FrameInfo, flags game.Flags) bool {
	return flags.Has(game.FlagSpinningTurret) || fi.TurretImageCount > 8
}

// frameAt returns frame i of m, or nil when i is out of range (a unit may
// reference a turret/firing frame that a stripped sprite sheet lacks).
func frameAt(m *sprites.Multi, i int) *sprites.MultiFrame {
	if m == nil || i < 0 || i >= len(m.Frames) {
		return nil
	}
	return &m.Frames[i]
}

// teamIndex clamps a team number to the five color tables.
func teamIndex(team int) int {
	if team < 0 || team > 4 {
		return 0
	}
	return team
}
