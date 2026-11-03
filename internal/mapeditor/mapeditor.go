// mapeditor.go : implements the -mapeditor map and scenario editor viewport.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package mapeditor

import (
	"fmt"
	"os"
	"strings"

	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/menus"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/settings"
	"github.com/suglasp/max-go/internal/world"
)

const (
	MINIMAP_SIZE = 120 // pixel size of the viewport minimap overlay
	START_ZOOM   = 32  // initial cell size in pixels when the editor opens
)

// Editor holds the loaded world, tactical renderer, viewport, and toolbar state for one map editor session.
type Editor struct {
	ctx       *menus.Context
	worldMap  *world.Map
	worldName string
	renderer  *drawmap.Renderer
	palette   gfx.Palette
	font      *fonts.Font // the compact button label font (cached from the gnw set)
	tabFont   *fonts.Font // the narrow font used for the rotated left-edge tabs
	scrollX   int
	scrollY   int
	showGrid  bool
	tab       int  // active mode tab (index into editorTabs)
	barFolded bool // the left tool bar is folded away to a corner button
	rowH      int  // cached height of a chrome button row (font derived)
	unfoldW   int  // cached width of the folded-state unfold button
}

// Run is the -mapeditor mode entry: pick a world on the planet select screen,
// then open the editor viewport over it. Returns to the main menu on exit.
func Run(ctx *menus.Context, cfg *settings.Settings) error {
	// A key held over from the boot (Enter that launched the command, or a
	// skipped intro's ESC) must not confirm or cancel the picker instantly.
	ctx.Sys.FlushInput()
	worldIndex, err := menus.PlanetSelect(ctx, cfg.Game.World)

	if err != nil {
		return err
	}
	if worldIndex < 0 {
		return nil // cancelled on the picker, back to the main menu
	}

	editor, err := open(ctx, worldIndex)
	if err != nil {
		return err
	}

	return editor.loop()
}

// open loads the picked world plus the base palette and builds the editor
// state with the view centered on the map.
func open(ctx *menus.Context, worldIndex int) (*Editor, error) {
	fileName, err := ctx.Lib.ReadString(menus.WorldTag(worldIndex))

	if err != nil {
		return nil, err
	}
	fileName = strings.TrimSpace(fileName)

	worldMap, err := world.LoadMap(resfile.DataPath(ctx.DataDir, fileName))
	if err != nil {
		return nil, err
	}

	// Cache the fonts + button metrics so the overlay geometry helpers stay
	// free of the font/context and remain testable. The buttons use the small
	// FONT0; the far-left rotated tabs use the even narrower FONT2 so the five
	// full words stack above the minimap without clipping.
	f := ctx.Sys.Fonts().Get(fonts.Font0)
	editor := &Editor{
		ctx:       ctx,
		worldMap:  worldMap,
		worldName: fileName,
		showGrid:  true,
		font:      f,
		tabFont:   ctx.Sys.Fonts().Get(fonts.Font2),
		rowH:      f.Height + 6,
		unfoldW:   f.Width("Unfold >>") + 12,
	}

	// COLOR.PAL stays the base palette; the map installs the terrain slots 64..159.
	if data, err := os.ReadFile(resfile.DataPath(ctx.DataDir, "COLOR.PAL")); err == nil {
		if palFile, err := gfx.ParsePalFile(data); err == nil {
			editor.palette = palFile.Palette
		}
	}
	worldMap.ApplyTerrainPalette(&editor.palette)
	ctx.Plat.SetPalette(&editor.palette)

	buf := ctx.Sys.Buf(1)
	editor.renderer = drawmap.New(worldMap)
	editor.renderer.SetViewport(buf.W, buf.H)
	editor.renderer.SetZoom(START_ZOOM)

	mapW, mapH := editor.renderer.MapPixels()
	editor.scrollX = max(0, (mapW-buf.W)/2)
	editor.scrollY = max(0, (mapH-buf.H)/2)

	return editor, nil
}

// loop runs the editor viewport: arrow keys scroll, PageUp/PageDown or +/-
// zoom, G toggles the cell grid, a left click or drag on the minimap
// recenters the view, and ESC asks to exit. The terrain palette cycles at
// the in game animation rates.
func (e *Editor) loop() error {
	ctx := e.ctx
	buf := ctx.Sys.Buf(1)
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	ctx.Sys.MouseShow()
	ctx.Sys.FlushInput()

	cycleStamps := make([]uint64, len(world.ColorCycles))
	var cyclePulse uint8

	for {
		switch key := ctx.Sys.GetInput(); key {
		case platform.KeyEscape:
			if menus.Confirm(ctx, "Exit the map editor?") {
				return nil
			}
			ctx.Sys.FlushInput()

		case platform.KeyLeft:
			e.scrollX -= e.renderer.Zoom() * 2

		case platform.KeyRight:
			e.scrollX += e.renderer.Zoom() * 2

		case platform.KeyUp:
			e.scrollY -= e.renderer.Zoom() * 2

		case platform.KeyDown:
			e.scrollY += e.renderer.Zoom() * 2

		case platform.KeyPageUp, '=', '+':
			e.renderer.SetZoom(e.renderer.Zoom() * 2)

		case platform.KeyPageDown, '-':
			e.renderer.SetZoom(e.renderer.Zoom() / 2)

		case 'g', 'G':
			e.showGrid = !e.showGrid
		}

		mouse := ctx.Plat.Mouse()
		e.edgeScroll(buf, mouse)
		e.recenterOnMinimap(buf, mouse)

		// Advance each terrain color cycle at its own rate, like the game tick.
		now := platform.Ticks()
		paletteChanged := false

		for i, cycle := range world.ColorCycles {
			if now-cycleStamps[i] >= uint64(1000/cycle.FPS) {
				cycleStamps[i] = now
				world.CycleStep(&e.palette, cycle, &cyclePulse)
				paletteChanged = true
			}
		}

		if paletteChanged {
			ctx.Plat.SetPalette(&e.palette)
		}

		e.scrollX, e.scrollY = e.renderer.ClampScroll(e.scrollX, e.scrollY, buf.W, buf.H)

		ctx.Sys.Lock()
		buf.Fill(0)
		e.renderer.Render(buf, e.scrollX, e.scrollY)

		if e.showGrid {
			e.renderer.RenderGrid(buf, e.scrollX, e.scrollY, drawmap.DefaultGridColor)
		}

		// drawChrome paints the bar (and the minimap over its band) and reports
		// any file action clicked this frame.
		action := e.drawChrome(buf, mouse)
		ctx.Sys.Unlock()

		// File actions may open a modal, so run them after the frame composes.
		switch action {
		case actExit:
			if menus.Confirm(ctx, "Exit the map editor?") {
				return nil
			}
			ctx.Sys.FlushInput()
		case actNew, actOpen, actSave, actSaveAs, actPlayTest, actSettings:
			menus.Message(ctx, "This tool arrives in a later editor step.")
			ctx.Sys.FlushInput()
		}

		e.printStatus(buf, mouse)
	}
}

// recenterOnMinimap scrolls the main view to the map cell under the cursor
// while the left button is held over the minimap square.
func (e *Editor) recenterOnMinimap(buf *gfx.Buffer, mouse platform.MouseState) {
	if e.barFolded || mouse.Buttons&platform.MousePressLeft == 0 {
		return
	}

	mm := e.minimapRect(buf)
	if !mm.Contains(mouse.X, mouse.Y) {
		return
	}

	// Invert the minimap scale (one source cell per minimap pixel) back to a
	// map cell, then center the viewport on that cell.
	cellX := (mouse.X - mm.ULX) * e.worldMap.W / MINIMAP_SIZE
	cellY := (mouse.Y - mm.ULY) * e.worldMap.H / MINIMAP_SIZE
	zoom := e.renderer.Zoom()
	e.scrollX = cellX*zoom + zoom/2 - buf.W/2
	e.scrollY = cellY*zoom + zoom/2 - buf.H/2
}

// printStatus draws the one line editor status: world name, zoom, the map
// cell under the mouse and the key help.
func (e *Editor) printStatus(buf *gfx.Buffer, mouse platform.MouseState) {
	zoom := e.renderer.Zoom()
	cellX := (e.scrollX + mouse.X) / zoom
	cellY := (e.scrollY + mouse.Y) / zoom

	if cellX < 0 || cellX >= e.worldMap.W || cellY < 0 || cellY >= e.worldMap.H {
		cellX, cellY = -1, -1
	}

	status := fmt.Sprintf("MAP EDITOR  %s  [%s]  zoom %d  cell %d,%d  (edge/arrow scroll, PgUp/PgDn zoom, G grid, ESC exit)", e.worldName, editorTabs[e.tab], zoom, cellX, cellY)
	e.ctx.Sys.Print(1, status, 620, 8, buf.H-16, 0xFF)
}
