// drawmap_test.go : tests map scaling, scrolling, grids, and minimap rendering.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package drawmap

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/world"
)

// synthMap is a 2x2-cell world with two recognizable tiles: tile 0 is a
// solid color, tile 1 is a horizontal gradient (pixel value == column).
func synthMap() *world.Map {
	tiles := make([]uint8, 2*world.TileSize*world.TileSize)
	for i := 0; i < world.TileSize*world.TileSize; i++ {
		tiles[i] = 100
	}
	for y := 0; y < world.TileSize; y++ {
		for x := 0; x < world.TileSize; x++ {
			tiles[world.TileSize*world.TileSize+y*world.TileSize+x] = uint8(x)
		}
	}
	return &world.Map{
		Preview:   world.Preview{W: 2, H: 2, Minimap: []uint8{10, 20, 30, 40}},
		TileCount: 2,
		TileIDs:   []uint16{0, 1, 1, 0}, // (0,0)=0 (1,0)=1 (0,1)=1 (1,1)=0
		Tiles:     tiles,
	}
}

func TestScaledTileIdentity(t *testing.T) {
	r := New(synthMap()) // zoom 64
	tile := r.scaledTile(1)
	if tile.W != 64 || tile.H != 64 {
		t.Fatalf("scaled tile %dx%d, want 64x64", tile.W, tile.H)
	}
	for x := 0; x < 64; x++ {
		if got := tile.At(x, 0); got != uint8(x) {
			t.Fatalf("1:1 scale altered pixel %d -> %d", x, got)
		}
	}
}

func TestScaledTileDownsample(t *testing.T) {
	r := New(synthMap())
	r.SetZoom(32)
	tile := r.scaledTile(1)
	if tile.W != 32 {
		t.Fatalf("scaled tile width %d, want 32", tile.W)
	}
	// Nearest neighbor sampling: first column samples source 0, last
	// column lands on the tile's right edge (63), and columns rise
	// monotonically.
	if tile.At(0, 0) != 0 {
		t.Errorf("first column = %d, want 0", tile.At(0, 0))
	}
	if tile.At(31, 0) != 63 {
		t.Errorf("last column = %d, want 63", tile.At(31, 0))
	}
	prev := -1
	for x := 0; x < 32; x++ {
		v := int(tile.At(x, 0))
		if v < prev {
			t.Fatalf("sampling not monotonic at %d: %d < %d", x, v, prev)
		}
		prev = v
	}
}

func TestRenderPlacementAndScroll(t *testing.T) {
	r := New(synthMap()) // zoom 64, map is 128x128 scaled
	dst := gfx.New(128, 128)
	r.Render(dst, 0, 0)
	// Tile (0,0) is solid 100; tile (1,0) is the gradient.
	if dst.At(0, 0) != 100 {
		t.Errorf("top-left tile = %d, want 100", dst.At(0, 0))
	}
	if dst.At(64+5, 0) != 5 {
		t.Errorf("gradient tile at x=69 = %d, want 5", dst.At(64+5, 0))
	}
	if dst.At(0, 64) != 0 { // tile (0,1) is the gradient, column 0
		t.Errorf("bottom-left tile = %d, want 0", dst.At(0, 64))
	}

	// Scrolling 32px right shows column 32 of the top left tile at x=0.
	dst.Fill(255)
	r.Render(dst, 32, 0)
	if dst.At(0, 0) != 100 {
		t.Errorf("scrolled top-left = %d, want 100", dst.At(0, 0))
	}
	if dst.At(32, 0) != 0 { // tile (1,0) gradient column 0 now at x=32
		t.Errorf("scrolled tile boundary = %d, want 0", dst.At(32, 0))
	}
}

func TestClampScroll(t *testing.T) {
	r := New(synthMap()) // 128x128 scaled
	if x, y := r.ClampScroll(-10, -10, 64, 64); x != 0 || y != 0 {
		t.Errorf("negative scroll = %d,%d, want 0,0", x, y)
	}
	if x, y := r.ClampScroll(1000, 1000, 64, 64); x != 64 || y != 64 {
		t.Errorf("overscroll = %d,%d, want 64,64 (128-64)", x, y)
	}
}

func TestRenderGrid(t *testing.T) {
	r := New(synthMap())
	dst := gfx.New(128, 128)
	dst.Fill(50)
	r.RenderGrid(dst, 0, 0, 9)
	// Tile boundaries at x=0 and x=64 are vertical grid lines.
	if dst.At(0, 10) != 9 || dst.At(64, 10) != 9 {
		t.Errorf("vertical grid lines missing: x0=%d x64=%d", dst.At(0, 10), dst.At(64, 10))
	}
	if dst.At(10, 0) != 9 || dst.At(10, 64) != 9 {
		t.Errorf("horizontal grid lines missing: y0=%d y64=%d", dst.At(10, 0), dst.At(10, 64))
	}
	// A cell interior is untouched.
	if dst.At(30, 30) != 50 {
		t.Errorf("grid overwrote a cell interior: %d", dst.At(30, 30))
	}
}

func TestMinimapViewportBox(t *testing.T) {
	r := New(synthMap())
	dst := gfx.New(2, 2) // one minimap pixel per cell
	r.Minimap(dst, 0, 0, 64, 64, 200)
	// Viewport (0,0)+64x64 of the 128x128 scaled map covers the top left
	// quarter -> minimap box corners at (0,0) painted in the box color.
	if dst.At(0, 0) != 200 {
		t.Errorf("minimap box not drawn: (0,0)=%d", dst.At(0, 0))
	}
}

func TestZoomClampAndPresets(t *testing.T) {
	r := New(synthMap())
	r.SetZoom(1)
	if r.Zoom() != MinZoom {
		t.Errorf("zoom clamped low = %d, want %d", r.Zoom(), MinZoom)
	}
	r.SetZoom(999)
	if r.Zoom() != MaxZoom {
		t.Errorf("zoom clamped high = %d, want %d", r.Zoom(), MaxZoom)
	}
}

// TestRenderGoldenGreen1 renders a fixed GREEN_1 region at two zoom levels and
// compares the results with recorded hashes. These hashes cover this renderer's
// full redraw path rather than the original dirty rectangle path.
func TestRenderGoldenGreen1(t *testing.T) {
	dir := testutil.DataDir(t)
	m, err := world.LoadMap(filepath.Join(dir, "GREEN_1.WRL"))
	if err != nil {
		t.Fatal(err)
	}
	r := New(m)
	for _, tc := range []struct {
		zoom, sx, sy int
		want         string
	}{
		{64, 1024, 1024, "80456c88c42b94244b852433170adde279234c288ab55babeda518a161853b67"},
		{16, 256, 256, "e0e5cfa62143488953d0f34db01cd44473a6d13b8b08696e7d3882faa3ab7f4e"},
	} {
		r.SetZoom(tc.zoom)
		dst := gfx.New(640, 480)
		sx, sy := r.ClampScroll(tc.sx, tc.sy, dst.W, dst.H)
		r.Render(dst, sx, sy)
		sum := sha256.Sum256(dst.Pix)
		if got := hex.EncodeToString(sum[:]); got != tc.want {
			t.Errorf("zoom %d render hash = %s, want %s", tc.zoom, got, tc.want)
		}
	}
}
