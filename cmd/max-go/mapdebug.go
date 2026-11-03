// mapdebug.go : provides tactical map rendering and map view debug screens.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/menus"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/settings"
	"github.com/suglasp/max-go/internal/world"
)

// drawMapDebug renders the selected world through the drawmap tactical
// renderer: scroll with the arrow keys, zoom with PageUp/PageDown, toggle
// the grid with G, and shows the minimap viewport box.
func drawMapDebug(ctx *menus.Context, cfg *settings.Settings) error {
	name, err := ctx.Lib.ReadString(menus.WorldTag(cfg.Game.World))

	if err != nil {
		return err
	}

	m, err := world.LoadMap(resfile.DataPath(ctx.DataDir, strings.TrimSpace(name)))
	if err != nil {
		return err
	}

	var pal gfx.Palette
	if data, err := os.ReadFile(resfile.DataPath(ctx.DataDir, "COLOR.PAL")); err == nil {
		if pf, err := gfx.ParsePalFile(data); err == nil {
			pal = pf.Palette
		}
	}

	m.ApplyTerrainPalette(&pal)
	ctx.Plat.SetPalette(&pal)

	r := drawmap.New(m)
	r.SetZoom(16)
	buf := ctx.Sys.Buf(1)
	mw, mh := r.MapPixels()
	scrollX, scrollY := max(0, (mw-buf.W)/2), max(0, (mh-buf.H)/2)
	grid := true
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	ctx.Sys.MouseShow()

	for {
		switch key := ctx.Sys.GetInput(); key {
		case platform.KeyEscape, platform.KeyReturn:
			return nil

		case platform.KeyLeft:
			scrollX -= r.Zoom() * 2

		case platform.KeyRight:
			scrollX += r.Zoom() * 2

		case platform.KeyUp:
			scrollY -= r.Zoom() * 2

		case platform.KeyDown:
			scrollY += r.Zoom() * 2

		case platform.KeyPageUp, '=', '+':
			r.SetZoom(r.Zoom() * 2)

		case platform.KeyPageDown, '-':
			r.SetZoom(r.Zoom() / 2)

		case 'g', 'G':
			grid = !grid
		}

		scrollX, scrollY = r.ClampScroll(scrollX, scrollY, buf.W, buf.H)

		ctx.Sys.Lock()
		buf.Fill(0)
		r.Render(buf, scrollX, scrollY)
		if grid {
			r.RenderGrid(buf, scrollX, scrollY, drawmap.DefaultGridColor)
		}
		const mmSize = 120
		r.Minimap(buf.Sub(buf.W-mmSize-4, 4, mmSize, mmSize), scrollX, scrollY, buf.W, buf.H, 0xFF)
		ctx.Sys.Unlock()

		ctx.Sys.Print(1, fmt.Sprintf("%s  zoom %d  scroll %d,%d  (arrows scroll, PgUp/PgDn zoom, G grid, ESC)", strings.TrimSpace(name), r.Zoom(), scrollX, scrollY), 620, 8, buf.H-16, 0xFF)
	}
}

// mapViewDebug draws the top left corner of a fully loaded map with
// live palette cycling. drawmap provides the scrolling renderer.
func mapViewDebug(ctx *menus.Context, cfg *settings.Settings) error {
	name, err := ctx.Lib.ReadString(menus.WorldTag(cfg.Game.World))

	if err != nil {
		return err
	}

	m, err := world.LoadMap(resfile.DataPath(ctx.DataDir, strings.TrimSpace(name)))
	if err != nil {
		return err
	}

	dbg.Printf("mapview: %s %dx%d cells, %d tiles", strings.TrimSpace(name), m.W, m.H, m.TileCount)

	// Keep COLOR.PAL as the base and install map colors in entries 64 through 159.
	var pal gfx.Palette
	if data, err := os.ReadFile(resfile.DataPath(ctx.DataDir, "COLOR.PAL")); err == nil {
		if pf, err := gfx.ParsePalFile(data); err == nil {
			pal = pf.Palette
		}
	}

	m.ApplyTerrainPalette(&pal)
	ctx.Plat.SetPalette(&pal)

	// Aim the view at the first coast cell so the screenshot shows
	// terrain variety, not just open sea.
	offX, offY := 0, 0
	for i, s := range m.Surface {
		if s == world.SurfaceCoast {
			offX = max(0, min(i%m.W-4, m.W-10))
			offY = max(0, min(i/m.W-3, m.H-8))
			break
		}
	}

	buf := ctx.Sys.Buf(1)
	ctx.Sys.Lock()
	for ty := 0; ty*world.TileSize < buf.H; ty++ {
		h := min(world.TileSize, buf.H-ty*world.TileSize)
		for tx := 0; tx*world.TileSize < buf.W; tx++ {
			tile := m.Tile(int(m.TileIDs[(offY+ty)*m.W+offX+tx]))
			buf.Sub(tx*world.TileSize, ty*world.TileSize, world.TileSize, h).
				Copy(tile.Sub(0, 0, world.TileSize, h))
		}
	}
	ctx.Sys.Unlock()

	ctx.Sys.MouseShow()

	// Advance each color cycle at its own rate, like
	// World::UpdateColorAnimations driven from the game tick.
	stamps := make([]uint64, len(world.ColorCycles))
	var pulse uint8
	for {
		now := platform.Ticks()
		changed := false

		for i, c := range world.ColorCycles {
			if now-stamps[i] >= uint64(1000/c.FPS) {
				stamps[i] = now
				world.CycleStep(&pal, c, &pulse)
				changed = true
			}
		}

		if changed {
			ctx.Plat.SetPalette(&pal)
		}

		switch ctx.Sys.GetInput() {
		case platform.KeyEscape, platform.KeyReturn:
			return nil
		}
	}
}
