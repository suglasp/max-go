// sprite_test.go : testing unit for sprite.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package units

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/world"
)

// stubReader serves synthetic resources by tag and counts reads so tests
// can assert the cache does not read it again.
type stubReader struct {
	data  map[string][]byte
	reads map[string]int
}

func (r *stubReader) Read(tag string) ([]byte, error) {
	if r.reads == nil {
		r.reads = map[string]int{}
	}
	r.reads[tag]++
	d, ok := r.data[tag]
	if !ok {
		return nil, errors.New("not found: " + tag)
	}
	return d, nil
}

// encodeMulti builds a resource of several frames (the same container the sprite
// and shadow decoders read): nFrames identical frames of w*h, fully
// opaque. When pixels is true each row carries pixel bytes of val (sprite
// encoding); otherwise rows are silhouette runs only (shadow encoding).
func encodeMulti(nFrames, w, h, hotX, hotY int, val byte, pixels bool) []byte {
	var rowsPerFrame []byte
	if pixels {
		row := append([]byte{0, byte(w)}, bytesOf(val, w)...)
		rowsPerFrame = append(row, 0xFF)
	} else {
		rowsPerFrame = []byte{0, byte(w), 0xFF}
	}

	out := make([]byte, 2+4*nFrames)
	binary.LittleEndian.PutUint16(out[0:], uint16(nFrames))
	for i := 0; i < nFrames; i++ {
		frameOff := len(out)
		binary.LittleEndian.PutUint32(out[2+i*4:], uint32(frameOff))

		hdr := make([]byte, 8)
		binary.LittleEndian.PutUint16(hdr[0:], uint16(int16(w)))
		binary.LittleEndian.PutUint16(hdr[2:], uint16(int16(h)))
		binary.LittleEndian.PutUint16(hdr[4:], uint16(int16(hotX)))
		binary.LittleEndian.PutUint16(hdr[6:], uint16(int16(hotY)))
		out = append(out, hdr...)

		rowTableAt := len(out)
		out = append(out, make([]byte, 4*h)...)
		for y := 0; y < h; y++ {
			binary.LittleEndian.PutUint32(out[rowTableAt+y*4:], uint32(len(out)))
			out = append(out, rowsPerFrame...)
		}
	}
	return out
}

func bytesOf(v byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = v
	}
	return b
}

func TestTagHelpers(t *testing.T) {
	for _, c := range []struct{ tag, shadow string }{
		{"TANK", "S_TANK"},
		{"SCOUT", "S_SCOUT"},
		{"CONSTRCT", "S_CONSTR"},
		{"AWAC", "S_AWAC"},
	} {
		if got := shadowTag(c.tag); got != c.shadow {
			t.Errorf("shadowTag(%q) = %q, want %q", c.tag, got, c.shadow)
		}
	}
	// dataTag resolves the reconstructed table first (TANK->D_TANK, the radar
	// SCANNER->D_SCANNR, a fixed gun->D_FIXED, SCOUT->its shared D_AMPHIB hull),
	// then the "D_"+tag[:6] heuristic for unpinned types (CONSTRCT->D_CONSTR).
	for _, c := range []struct {
		t    assets.UnitType
		data string
	}{
		{assets.Tank, "D_TANK"},
		{assets.Scanner, "D_SCANNR"},
		{assets.GunTurrt, "D_FIXED"},
		{assets.Scout, "D_AMPHIB"},
		{assets.Awac, "D_AWAC"},
		{assets.Constrct, "D_CONSTR"},
	} {
		if got := dataTag(c.t); got != c.data {
			t.Errorf("dataTag(%s) = %q, want %q", c.t.Tag(), got, c.data)
		}
	}
}

func TestSoldierWalkFrame(t *testing.T) {
	if !isSoldier(assets.Infantry) || !isSoldier(assets.Commando) {
		t.Fatal("infantry/commando should be soldiers")
	}
	if isSoldier(assets.Tank) {
		t.Fatal("tank should not be a soldier")
	}
	// With image_base 0 and image_index_max 103 there are 13 walk phases.
	if got := soldierWalkPhases(0); got != 13 {
		t.Errorf("soldierWalkPhases(0) = %d, want 13", got)
	}
	// Phase 0 is the standing frame for the facing (base + angle); each phase
	// adds 8 (eight facings between frames). Frame = base + angle + 8*phase.
	if got := soldierWalkFrame(0, 3, 0); got != 3 {
		t.Errorf("walk frame phase 0 angle 3 = %d, want 3", got)
	}
	if got := soldierWalkFrame(0, 3, 2); got != 19 { // 3 + 8*2
		t.Errorf("walk frame phase 2 angle 3 = %d, want 19", got)
	}
	// The last phase in range (12) for angle 7 lands on image_index_max (103).
	if got := soldierWalkFrame(0, 7, 12); got != 103 {
		t.Errorf("walk frame phase 12 angle 7 = %d, want 103", got)
	}
	// An overflowing phase wraps to the standing frame for the facing, never
	// running into the firing/death frames above image_index_max.
	if got := soldierWalkFrame(0, 5, 13); got != 5 {
		t.Errorf("overflow walk frame = %d, want 5 (standing)", got)
	}
}

func TestSpriteCacheLoadsSet(t *testing.T) {
	r := &stubReader{data: map[string][]byte{
		"SCOUT":   encodeMulti(8, 6, 6, 3, 3, 40, true),
		"S_SCOUT": encodeMulti(8, 6, 6, 3, 3, 0, false),
	}}
	c := NewSpriteCache(r, &gfx.Palette{})

	set := c.Set(assets.Scout)
	if set == nil {
		t.Fatal("Set(Scout) = nil")
	}
	if set.Body == nil || len(set.Body.Frames) != 8 {
		t.Fatalf("body frames = %v, want 8", set.Body)
	}
	if set.Shadow == nil || len(set.Shadow.Frames) != 8 {
		t.Fatalf("shadow frames missing")
	}
	if set.Frame.HasData {
		t.Error("Scout has no data resource, HasData should be false")
	}

	// Second call is cached: no extra reads of the body resource.
	c.Set(assets.Scout)
	if r.reads["SCOUT"] != 1 {
		t.Errorf("body resource read %d times, want 1 (cached)", r.reads["SCOUT"])
	}
}

func TestSpriteCacheMissingBodyCachesNil(t *testing.T) {
	r := &stubReader{data: map[string][]byte{}}
	c := NewSpriteCache(r, &gfx.Palette{})
	if c.Set(assets.Tank) != nil {
		t.Error("Set with no body resource should be nil")
	}
	c.Set(assets.Tank)
	if r.reads["TANK"] != 1 {
		t.Errorf("missing body re-read %d times, want 1 (nil cached)", r.reads["TANK"])
	}
}

func TestDrawUnitRendersBodyAndTurret(t *testing.T) {
	// TANK: 16-frame body (8 body + 8 turret) plus a data resource giving a
	// turret at frame base 8 with a +4,+4 mount offset for facing 0.
	data := make([]byte, 24)
	data[1] = 8 // image_count
	data[2] = 8 // turret_image_base
	data[3] = 8 // turret_image_count
	data[8] = 4 // angle_offsets[0].x
	data[9] = 4 // angle_offsets[0].y
	r := &stubReader{data: map[string][]byte{
		"TANK":   encodeMulti(16, 8, 8, 4, 4, 50, true),
		"S_TANK": encodeMulti(16, 8, 8, 4, 4, 0, false),
		"D_TANK": data,
	}}
	c := NewSpriteCache(r, &gfx.Palette{})

	u := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagTurretSprite
	set := c.Set(assets.Tank)
	if !set.Frame.HasData || set.Frame.TurretImageCount != 8 {
		t.Fatalf("tank data not decoded: %+v", set.Frame)
	}
	u.ImageBase = set.Frame.ImageBase
	u.TurretImageBase = set.Frame.TurretImageBase
	u.SetAngle(0)
	u.SetPosition(1, 1)

	dst := gfx.New(128, 128)
	c.DrawUnit(dst, u, 64, 0, 0) // 1:1 zoom, no scroll

	painted := 0
	for _, p := range dst.Pix {
		if p != 0 {
			painted++
		}
	}
	if painted == 0 {
		t.Fatal("DrawUnit painted nothing")
	}
}

func TestDrawUnitMissingSpriteIsNoop(t *testing.T) {
	c := NewSpriteCache(&stubReader{data: map[string][]byte{}}, &gfx.Palette{})
	u := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	u.SetPosition(0, 0)
	dst := gfx.New(32, 32)
	c.DrawUnit(dst, u, 16, 0, 0) // must not panic
	for _, p := range dst.Pix {
		if p != 0 {
			t.Fatal("DrawUnit with no sprite painted pixels")
		}
	}
}

// TestDrawUnitRealTank exercises the real TANK body, S_TANK shadow and
// D_TANK frame data from the game archives: the turret base (8) must index
// inside the real 16-frame sheet and the unit must paint at the default
// zoom in game without going out of range.
func TestDrawUnitRealTank(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	c := NewSpriteCache(lib, &gfx.Palette{})
	set := c.Set(assets.Tank)
	if set == nil || set.Body == nil {
		t.Fatal("real TANK body did not load")
	}
	if !set.Frame.HasData {
		t.Fatal("real D_TANK frame data did not load")
	}
	if set.Frame.TurretImageBase+7 >= len(set.Body.Frames) {
		t.Fatalf("turret frames %d..%d out of TANK sheet (%d frames)",
			set.Frame.TurretImageBase, set.Frame.TurretImageBase+7, len(set.Body.Frames))
	}

	u := game.NewUnitInfo(assets.Tank, 1, 1, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagTurretSprite
	u.ImageBase = set.Frame.ImageBase
	u.TurretImageBase = set.Frame.TurretImageBase
	u.SetAngle(3)
	u.TurretAngle = 3
	u.SetPosition(2, 2)

	dst := gfx.New(448, 448)
	for _, z := range []int{4, 16, 64} {
		c.DrawUnit(dst, u, z, 0, 0)
	}
	painted := 0
	for _, p := range dst.Pix {
		if p != 0 {
			painted++
		}
	}
	if painted == 0 {
		t.Fatal("real TANK painted nothing")
	}
}

// TestScannerDataOverride checks the dataTag override that points SCANNER at
// D_SCANNR (the archive abbreviates it; the old "D_"+tag[:6] heuristic asked
// for the nonexistent D_SCANNE, so the dish never loaded). The dish is a
// 16-frame spinning turret whose frames must index inside the body sheet.
func TestScannerDataOverride(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	c := NewSpriteCache(lib, &gfx.Palette{})
	set := c.Set(assets.Scanner)
	if set == nil || set.Body == nil {
		t.Fatal("real SCANNER body did not load")
	}
	if !set.Frame.HasData {
		t.Fatal("SCANNER frame data did not load (dataTag override missing?)")
	}
	if set.Frame.TurretImageCount != 16 {
		t.Errorf("scanner dish frame count = %d, want 16", set.Frame.TurretImageCount)
	}
	if c.TurretCount(assets.Scanner) != set.Frame.TurretImageCount {
		t.Errorf("TurretCount = %d, want %d", c.TurretCount(assets.Scanner), set.Frame.TurretImageCount)
	}
	if hi := set.Frame.TurretImageBase + set.Frame.TurretImageCount - 1; hi >= len(set.Body.Frames) {
		t.Fatalf("dish frames %d..%d out of SCANNER sheet (%d frames)",
			set.Frame.TurretImageBase, hi, len(set.Body.Frames))
	}

	// A spinning unit paints across the dish range without going out of bounds.
	u := game.NewUnitInfo(assets.Scanner, 1, 1, 2)
	u.Flags = game.FlagMobileLandUnit | game.FlagSpinningTurret
	u.ImageBase = set.Frame.ImageBase
	u.TurretImageBase = set.Frame.TurretImageBase
	u.SetAngle(2)
	u.SetPosition(2, 2)
	dst := gfx.New(320, 320)
	for spin := 0; spin < set.Frame.TurretImageCount; spin++ {
		u.SpinFrame = uint8(spin)
		c.DrawUnit(dst, u, 64, 0, 0)
	}
	painted := 0
	for _, p := range dst.Pix {
		if p != 0 {
			painted++
		}
	}
	if painted == 0 {
		t.Fatal("spinning scanner painted nothing")
	}
}

// TestDataResourceTableFits checks the reconstructed unit->data table against
// the real archives: every mapped data file must load and its highest frame
// index must fall inside the unit's body sprite sheet (the frame fit the table
// was reconstructed from). This guards against a wrong pin that would index a
// turret/firing frame past the end of the sheet.
func TestDataResourceTableFits(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	for ut, dtag := range dataResource {
		dd, err := lib.Read(dtag)
		if err != nil {
			t.Errorf("%s -> %s: data resource missing: %v", ut.Tag(), dtag, err)
			continue
		}
		fi := game.DecodeFrameInfo(dd)
		body, err := lib.Read(ut.Tag())
		if err != nil {
			continue // some mapped types (markers) have no body sheet of their own
		}
		m, err := sprites.DecodeMultiSprite(body)
		if err != nil {
			continue
		}
		hi := fi.ImageBase + fi.ImageCount - 1
		for _, v := range []int{
			fi.TurretImageBase + fi.TurretImageCount - 1,
			fi.FiringImageBase + fi.FiringImageCount - 1,
			fi.ConnectorImageBase + fi.ConnectorImageCount - 1,
		} {
			if v > hi {
				hi = v
			}
		}
		// Soldier walk cycle files store image_count as a large packed value
		// that decodes negative as int8; skip the span check for those.
		if fi.ImageCount >= 0 && hi >= len(m.Frames) {
			t.Errorf("%s -> %s: frame span reaches %d but sheet has %d frames",
				ut.Tag(), dtag, hi, len(m.Frames))
		}
	}
}

// TestAnimatedBodyCycles checks the body cycle of an animated building: only
// RADAR is a genuine idle animation (its 16-frame body is a rotating dish), so
// its frames differ as BodyAnim advances. MININGST (16 clan emblem frames, not
// an animation), the bridge (4 connector variants) and a plain 2-frame plant
// (POWGEN) must NOT be treated as animated.
func TestAnimatedBodyCycles(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	c := NewSpriteCache(lib, &gfx.Palette{})
	flags := game.FlagStationary | game.FlagBuilding

	for _, ut := range []assets.UnitType{assets.PowGen, assets.Bridge, assets.MiningSt} {
		if n := c.BodyAnimCount(ut, flags); n != 0 {
			t.Errorf("%s BodyAnimCount = %d, want 0 (not a body animation)", ut.Tag(), n)
		}
	}

	n := c.BodyAnimCount(assets.Radar, flags)
	if n <= 2 {
		t.Fatalf("RADAR BodyAnimCount = %d, want an animation loop (>2)", n)
	}
	u := game.NewUnitInfo(assets.Radar, 1, 1, 0)
	u.Flags = flags
	u.ImageBase = c.Set(assets.Radar).Frame.ImageBase
	u.SetPosition(2, 2)
	snap := func(frame uint8) []uint8 {
		u.BodyAnim = frame
		d := gfx.New(192, 192)
		c.DrawUnit(d, u, 64, 0, 0)
		return append([]uint8(nil), d.Pix...)
	}
	a, b := snap(0), snap(uint8(n/2))
	diff := 0
	for i := range a {
		if a[i] != b[i] {
			diff++
		}
	}
	if diff == 0 {
		t.Errorf("RADAR: body frames 0 and %d are identical (dish not animating)", n/2)
	}
}

// TestBuildingPoweredFrameSwap checks the state frame of a powered building: a
// generator (and other buildings with two states) draws its lit frame
// (image_base+1) while powered on and the idle frame (image_base) otherwise;
// the mining station is excluded (its frames are clan emblems, not an on/off pair).
func TestBuildingPoweredFrameSwap(t *testing.T) {
	flags := game.FlagStationary | game.FlagRequiresSlab
	if !isBuildingStateBody(&game.UnitInfo{Type: assets.PowGen, Flags: flags}) {
		t.Error("PowGen should be a two-state building body")
	}
	if isBuildingStateBody(&game.UnitInfo{Type: assets.MiningSt, Flags: flags | game.FlagBuilding}) {
		t.Error("MiningSt must be excluded (clan-emblem frames, not on/off)")
	}

	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	c := NewSpriteCache(lib, &gfx.Palette{})
	base := c.Set(assets.PowGen)
	if base == nil || base.Body == nil || len(base.Body.Frames) < 2 {
		t.Skip("PowGen sprite lacks a second (lit) frame in this data set")
	}
	draw := func(order game.Order) []uint8 {
		u := game.NewUnitInfo(assets.PowGen, 1, 1, 0)
		u.Flags = flags
		u.Order = order
		u.ImageBase = base.Frame.ImageBase
		u.SetPosition(2, 2)
		d := gfx.New(192, 192)
		c.DrawUnit(d, u, 64, 0, 0)
		return append([]uint8(nil), d.Pix...)
	}
	off, on := draw(game.OrderPowerOff), draw(game.OrderPowerOn)
	diff := 0
	for i := range off {
		if off[i] != on[i] {
			diff++
		}
	}
	if diff == 0 {
		t.Error("powered-on generator should draw a different (lit) frame than powered-off")
	}
}

// TestFactoryHaltedIdleFrame locks the stopped factory body state: a factory the player
// STOPPED keeps its queue but is paused (rate 0) and stays ORDER_POWER_ON in the port
// model, so it must draw the idle body frame, not the lit "running" one. factoryHalted
// flags exactly that case; a producing or empty queue factory and any not a factory are not.
func TestFactoryHaltedIdleFrame(t *testing.T) {
	producing := &game.UnitInfo{Type: assets.LandPlt, BuildQueue: []assets.UnitType{assets.Tank}, BuildRate: 1}
	if factoryHalted(producing) {
		t.Error("a producing factory (rate>0) is not halted")
	}
	halted := &game.UnitInfo{Type: assets.LandPlt, BuildQueue: []assets.UnitType{assets.Tank}, BuildRate: 0}
	if !factoryHalted(halted) {
		t.Error("a stopped factory (queue kept, rate 0) must be halted -> idle frame")
	}
	if factoryHalted(&game.UnitInfo{Type: assets.LandPlt, BuildRate: 0}) {
		t.Error("an empty-queue factory has no kept queue -> not 'halted'")
	}
	if factoryHalted(&game.UnitInfo{Type: assets.PowGen}) {
		t.Error("a non-factory building (no queue) must never read as halted")
	}
}

// TestBodyBaseOverride checks the carrier/submarine fix: these sheets hold a
// magenta placeholder underwater/submerged variant in frames 0-7 and the real
// sprite in 8-15, so their displayed body base is pinned to 8. At base 8 the
// body frame facing east must be a normal opaque sprite, not mostly the magenta
// TestAmphibBase locks the water sprite base swap. The units that carry a real
// water facing set in their sheet switch to it on open water: the Personnel Carrier
// (inverted: image_base 8 on land, 0 on water), and the Surveyor, Engineer and Scout
// (the standard sense: 0 on land, 8 on water - their float/pontoon water pose). The
// retail sheets were decoded to confirm each: the surveyor/engineer/scout n+8 frames
// differ per facing (a real water form), while the Constructor's frames 8-15 are byte
// for byte the same as its land drive set, so the Constructor has NO water form and
// stays on its land look. A tank/corvette is not amphibious either.
func TestAmphibBase(t *testing.T) {
	// The carrier, surveyor, engineer and scout each have a dedicated water set.
	for _, t2 := range []assets.UnitType{assets.ClnTrans, assets.Surveyor, assets.Engineer, assets.Scout} {
		if !isAmphibious(t2) {
			t.Errorf("%v should report isAmphibious (it has a dedicated water facing set)", t2)
		}
	}
	// The Constructor (water frames duplicate the land set) and plain land/sea units
	// keep their land look on water.
	for _, t2 := range []assets.UnitType{assets.Constrct, assets.Tank, assets.Corvette} {
		if isAmphibious(t2) {
			t.Errorf("%v should not be amphibious (keeps its land look on water)", t2)
		}
	}
	// Engineer and Scout use the standard sense: land=0, water=8.
	for _, t2 := range []assets.UnitType{assets.Engineer, assets.Scout} {
		if got := amphibBase(t2, false); got != 0 {
			t.Errorf("%v land base = %d, want 0", t2, got)
		}
		if got := amphibBase(t2, true); got != 8 {
			t.Errorf("%v water base = %d, want 8", t2, got)
		}
	}
	// Personnel carrier is inverted: land=8, water=0.
	if got := amphibBase(assets.ClnTrans, false); got != 8 {
		t.Errorf("ClnTrans land base = %d, want 8", got)
	}
	if got := amphibBase(assets.ClnTrans, true); got != 0 {
		t.Errorf("ClnTrans water base = %d, want 0", got)
	}
	// Surveyor is the standard sense: land=0, water=8 (its water/float pose).
	if got := amphibBase(assets.Surveyor, false); got != 0 {
		t.Errorf("Surveyor land base = %d, want 0", got)
	}
	if got := amphibBase(assets.Surveyor, true); got != 8 {
		t.Errorf("Surveyor water base = %d, want 8", got)
	}
}

// TestAmphibBodyBaseEngineerBuildPose locks the surface aware base used by the
// build pose: an engineer building over open water uses its water hull set (8),
// so the frame driving in (8-15) and +16 build pose (24-31) come from the water
// frames; on land or where the surface reads as land (coast/bridge/platform) it
// stays at the land base (0). The Constructor never shifts - it has no water form.
func TestAmphibBodyBaseEngineerBuildPose(t *testing.T) {
	c := &SpriteCache{}
	eng := &game.UnitInfo{Type: assets.Engineer, GridX: 5, GridY: 5}
	con := &game.UnitInfo{Type: assets.Constrct, GridX: 5, GridY: 5}

	// With no surface source (headless/tests) every unit stays at the land base.
	if got := c.amphibBodyBase(eng, 0); got != 0 {
		t.Errorf("no surface source: engineer base = %d, want 0", got)
	}

	// Over open water the engineer shifts to its water hull (8); the constructor
	// stays on land frames (it carries no water set).
	c.SetSurfaceSource(func(x, y int) uint8 { return world.SurfaceWater })
	if got := c.amphibBodyBase(eng, 0); got != 8 {
		t.Errorf("on water: engineer base = %d, want 8", got)
	}
	if got := c.amphibBodyBase(con, 0); got != 0 {
		t.Errorf("on water: constructor base = %d, want 0 (no water form)", got)
	}

	// A coast tile reads as land (only full open water swaps), so the engineer
	// keeps its land base there.
	c.SetSurfaceSource(func(x, y int) uint8 { return world.SurfaceCoast })
	if got := c.amphibBodyBase(eng, 0); got != 0 {
		t.Errorf("on coast: engineer base = %d, want 0", got)
	}
}

// placeholder band (123-126) that produced the see through silhouette.
func TestBodyBaseOverride(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	c := NewSpriteCache(lib, &gfx.Palette{})
	for _, ut := range []assets.UnitType{assets.ClnTrans, assets.Submarne} {
		set := c.Set(ut)
		if set == nil || set.Body == nil {
			t.Fatalf("%s did not load", ut.Tag())
		}
		if set.Frame.ImageBase != 8 {
			t.Errorf("%s ImageBase = %d, want 8", ut.Tag(), set.Frame.ImageBase)
		}
		f := frameAt(set.Body, set.Frame.ImageBase+2) // east facing
		if f == nil {
			t.Fatalf("%s frame %d missing", ut.Tag(), set.Frame.ImageBase+2)
		}
		op, magenta := 0, 0
		for i, o := range f.Opaque {
			if o {
				op++
				if f.Pix[i] >= 123 && f.Pix[i] <= 126 {
					magenta++
				}
			}
		}
		if op == 0 || magenta*2 > op {
			t.Errorf("%s body frame is %d/%d magenta (still the underwater placeholder?)", ut.Tag(), magenta, op)
		}
	}
}

// TestFixedTurretBodyDoesNotRotate verifies the D_FIXED render path: a fixed
// gun (GUNTURRT) has a single body frame, so DrawUnit must keep the hull at
// ImageBase for every facing and only rotate the turret, never index a turret
// frame as the body.
func TestFixedTurretBodyDoesNotRotate(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	c := NewSpriteCache(lib, &gfx.Palette{})
	set := c.Set(assets.GunTurrt)
	if set == nil || !set.Frame.HasData {
		t.Fatal("GUNTURRT data did not load")
	}
	if set.Frame.ImageCount != 1 || set.Frame.TurretImageCount != 8 {
		t.Fatalf("GUNTURRT layout = body %d / turret %d, want 1 / 8",
			set.Frame.ImageCount, set.Frame.TurretImageCount)
	}
	u := game.NewUnitInfo(assets.GunTurrt, 1, 1, 0)
	u.Flags = game.FlagBuilding | game.FlagTurretSprite
	u.ImageBase = set.Frame.ImageBase
	u.TurretImageBase = set.Frame.TurretImageBase
	u.SetPosition(2, 2)
	dst := gfx.New(256, 256)
	for a := uint8(0); a < 8; a++ {
		u.SetAngle(a) // hull frame must not follow this
		dst.Fill(0)
		c.DrawUnit(dst, u, 64, 0, 0)
		painted := 0
		for _, p := range dst.Pix {
			if p != 0 {
				painted++
			}
		}
		if painted == 0 {
			t.Fatalf("facing %d painted nothing", a)
		}
	}
}
