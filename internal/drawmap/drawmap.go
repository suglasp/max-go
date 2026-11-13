// drawmap.go : renders tactical map tiles, zoom, grids, and the minimap.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package drawmap blits 64x64 terrain tiles into a scrolled viewport at the
// selected zoom. It redraws the visible area each frame and can overlay a tile
// grid and minimap.
package drawmap

import (
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/world"
)

// Zoom is expressed in screen pixels per 64-pixel map tile. The game
// steps through these presets; any value in [MinZoom, MaxZoom] renders.
//
// C++: GameManager_ZoomMainMapView clamps to [MainmapZoomLimit, 64];
// the zoom buttons double from 4 to 64.
const (
	MinZoom = 4
	MaxZoom = world.TileSize // 64, one tile at 1:1
)

// DefaultGridColor is the land grid overlay color used when a world
// supplies none (the original's fallback in DrawMap_RenderSimpleGridOverlay).
const DefaultGridColor uint8 = 37

// Renderer draws one map at one zoom, caching each tile scaled ahead of time
// to the current zoom so repeated frames only blit.
type Renderer struct {
	m     *world.Map
	zoom  int
	cache map[uint16]*gfx.Buffer

	// viewW/viewH is the viewport SetViewport declared; 0 = no floor beyond
	// MinZoom (library users that render into arbitrary buffers).
	viewW, viewH int
}

// New makes a renderer for m at full zoom (1:1).
func New(m *world.Map) *Renderer {
	return &Renderer{m: m, zoom: MaxZoom, cache: map[uint16]*gfx.Buffer{}}
}

// SetMap points the renderer at a different map, dropping the scaled tile cache so the
// next frame rebuilds it for the new terrain. The zoom and viewport are kept, then the
// zoom is clamped again against the new map's dimensions. Used when an in game load swaps
// to a saved game's own world.
func (r *Renderer) SetMap(m *world.Map) {
	r.m = m
	r.cache = map[uint16]*gfx.Buffer{}
	r.SetZoom(r.zoom) // clamp again against the new map's viewport floor
}

// Zoom returns the current zoom (screen pixels per tile).
func (r *Renderer) Zoom() int {
	return r.zoom
}

// SetViewport tells the renderer the viewport SetZoom must be able to fill,
// raising the effective minimum zoom so the scaled map always covers it (no
// dead band right of or under a small map). The current zoom clamps again. At
// the 4:3 viewport (448px) a 112 cell map keeps today's floor of 4; the wide
// 662px viewport lifts it to 6.
func (r *Renderer) SetViewport(vw, vh int) {
	r.viewW, r.viewH = vw, vh
	r.SetZoom(r.zoom)
}

// minZoom is the viewport aware zoom floor: the map may never render
// smaller than the viewport the game screen shows it in.
func (r *Renderer) minZoom() int {
	z := MinZoom
	if r.viewW > 0 && r.m.W > 0 {
		z = max(z, (r.viewW+r.m.W-1)/r.m.W)
	}
	if r.viewH > 0 && r.m.H > 0 {
		z = max(z, (r.viewH+r.m.H-1)/r.m.H)
	}
	return z
}

// SetZoom changes the zoom, clamped to [minZoom, MaxZoom], and drops the
// scaled tile cache.
func (r *Renderer) SetZoom(z int) {
	if low := r.minZoom(); z < low {
		z = low
	}
	if z > MaxZoom {
		z = MaxZoom
	}
	if z != r.zoom {
		r.zoom = z
		r.cache = map[uint16]*gfx.Buffer{}
	}
}

// MapPixels returns the scaled map size in screen pixels.
func (r *Renderer) MapPixels() (w, h int) {
	return r.m.W * r.zoom, r.m.H * r.zoom
}

// ClampScroll keeps a viewport of size (vw, vh) inside the scaled map.
func (r *Renderer) ClampScroll(scrollX, scrollY, vw, vh int) (int, int) {
	mw, mh := r.MapPixels()
	return max(0, min(scrollX, max(0, mw-vw))), max(0, min(scrollY, max(0, mh-vh)))
}

// scaledTile returns tile id scaled to zoom x zoom, building and caching
// it on first use.
//
// C++: Gfx_DecodeMapTile nearest neighbor sampling (gfx.cpp)
func (r *Renderer) scaledTile(id uint16) *gfx.Buffer {
	if b, ok := r.cache[id]; ok {
		return b
	}
	src := r.m.Tile(int(id))
	z := r.zoom
	dst := gfx.New(z, z)
	if z == world.TileSize {
		dst.Copy(src)
	} else {
		// Source column/row for destination i: (i*step)>>16, with the
		// original's +8 bias so the last pixel lands on tile edge 63.
		step := ((world.TileSize-1)<<16)/(z-1) + 8
		for j := 0; j < z; j++ {
			srcY := (j * step) >> 16
			row := dst.Row(j)
			for i := 0; i < z; i++ {
				row[i] = src.At((i*step)>>16, srcY)
			}
		}
	}
	r.cache[id] = dst
	return dst
}

// Render draws the visible map into dst. scrollX, scrollY are the
// viewport's top left corner in scaled map pixels.
func (r *Renderer) Render(dst *gfx.Buffer, scrollX, scrollY int) {
	z := r.zoom
	tx0, ty0 := max(0, min(scrollX/z, r.m.W-1)), max(0, min(scrollY/z, r.m.H-1))
	tx1 := max(0, min((scrollX+dst.W-1)/z, r.m.W-1))
	ty1 := max(0, min((scrollY+dst.H-1)/z, r.m.H-1))
	for ty := ty0; ty <= ty1; ty++ {
		for tx := tx0; tx <= tx1; tx++ {
			tile := r.scaledTile(r.m.TileIDs[ty*r.m.W+tx])
			blit(dst, tile, tx*z-scrollX, ty*z-scrollY)
		}
	}
}

// RenderGrid overlays the tile grid (vertical and horizontal lines at
// every tile boundary) onto dst at the same scroll as Render.
//
// C++: DrawMap_RenderSimpleGridOverlay (drawmap.cpp)
func (r *Renderer) RenderGrid(dst *gfx.Buffer, scrollX, scrollY int, color uint8) {
	z := r.zoom
	for tx := scrollX / z; tx <= (scrollX+dst.W-1)/z; tx++ {
		if x := tx*z - scrollX; x >= 0 && x < dst.W {
			vline(dst, x, 0, dst.H-1, color)
		}
	}
	for ty := scrollY / z; ty <= (scrollY+dst.H-1)/z; ty++ {
		if y := ty*z - scrollY; y >= 0 && y < dst.H {
			hline(dst, y, 0, dst.W-1, color)
		}
	}
}

// Minimap draws the map's overview (one pixel per cell) scaled to fill
// dst, then outlines the current viewport rectangle in boxColor.
// scroll/vw/vh describe the main view in scaled map pixels.
//
// C++: DrawMap_RenderMiniMap + the white viewport box (drawmap.cpp)
func (r *Renderer) Minimap(dst *gfx.Buffer, scrollX, scrollY, vw, vh int, boxColor uint8) {
	r.MinimapTerrain(dst)
	r.MinimapBox(dst, scrollX, scrollY, vw, vh, boxColor)
}

// MinimapTerrain fills dst with the terrain overview (one source cell per
// minimap pixel), without the viewport box. Callers can overlay unit dots
// before drawing the box.
func (r *Renderer) MinimapTerrain(dst *gfx.Buffer) {
	mini := r.m.MinimapBuffer() // W x H, one pixel per cell
	for dy := 0; dy < dst.H; dy++ {
		srcY := dy * r.m.H / dst.H
		row := dst.Row(dy)
		for dx := 0; dx < dst.W; dx++ {
			row[dx] = mini.At(dx*r.m.W/dst.W, srcY)
		}
	}
}

// MinimapBox draws the current viewport rectangle into dst, in minimap space.
func (r *Renderer) MinimapBox(dst *gfx.Buffer, scrollX, scrollY, vw, vh int, boxColor uint8) {
	mw, mh := r.MapPixels()
	bx0 := scrollX * dst.W / mw
	by0 := scrollY * dst.H / mh
	bx1 := max(0, min((scrollX+vw-1)*dst.W/mw, dst.W-1))
	by1 := max(0, min((scrollY+vh-1)*dst.H/mh, dst.H-1))
	hline(dst, by0, bx0, bx1, boxColor)
	hline(dst, by1, bx0, bx1, boxColor)
	vline(dst, bx0, by0, by1, boxColor)
	vline(dst, bx1, by0, by1, boxColor)
}

// blit copies src into dst at (x, y), clipping to dst's bounds.
func blit(dst, src *gfx.Buffer, x, y int) {
	srcX, srcY, w, h := 0, 0, src.W, src.H
	if x < 0 {
		srcX, w, x = -x, w+x, 0
	}
	if y < 0 {
		srcY, h, y = -y, h+y, 0
	}
	if x+w > dst.W {
		w = dst.W - x
	}
	if y+h > dst.H {
		h = dst.H - y
	}
	if w <= 0 || h <= 0 {
		return
	}
	dst.Sub(x, y, w, h).Copy(src.Sub(srcX, srcY, w, h))
}

// hline draws a clipped horizontal line into the map buffer.
func hline(b *gfx.Buffer, y, startX, endX int, c uint8) {
	row := b.Row(y)
	for x := startX; x <= endX; x++ {
		row[x] = c
	}
}

// vline draws a clipped vertical line into the map buffer.
func vline(b *gfx.Buffer, x, startY, endY int, c uint8) {
	for y := startY; y <= endY; y++ {
		b.Set(x, y, c)
	}
}
