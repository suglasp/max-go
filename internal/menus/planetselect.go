// planetselect.go : implements the world picker shown before a custom game.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: planetselectmenu.cpp - PlanetSelectMenu
package menus

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/world"
)

// WorldCount is the number of selectable planets; four groups of six
// (Snow, Crater, Green, Desert).
const WorldCount = 24

// worldFileTags are the MAX.RES filename reference tags of the .WRL
// maps, in world order.
var worldFileTags = [WorldCount]string{
	"SNOW_1", "SNOW_2", "SNOW_3", "SNOW_4", "SNOW_5", "SNOW_6",
	"CRATER_1", "CRATER_2", "CRATER_3", "CRATER_4", "CRATER_5", "CRATER_6",
	"GREEN_1", "GREEN_2", "GREEN_3", "GREEN_4", "GREEN_5", "GREEN_6",
	"DESERT_1", "DESERT_2", "DESERT_3", "DESERT_4", "DESERT_5", "DESERT_6",
}

// groupPics are the big image backdrops per world group.
var groupPics = [4]string{"SNOW_PIC", "CRTR_PIC", "GREN_PIC", "DSRT_PIC"}

// WorldTag returns the .WRL resource tag of a world index (0..23).
func WorldTag(i int) string {
	if i < 0 || i >= WorldCount {
		i = 0
	}
	return worldFileTags[i]
}

// planetNameKeys / planetDescKeys are the lang_english.ini ids, in
// world order (planetselectmenu.cpp DrawTexts).
var planetNameKeys = [WorldCount]string{
	"e43b", "f588", "c78b", "895d", "5f5f", "e7b2",
	"f3fe", "8524", "4bb8", "f408", "0935", "7303",
	"94ef", "c46c", "48ac", "275a", "ea47", "fcf0",
	"6426", "7ea8", "386d", "41e5", "bbcb", "ba99",
}

var planetDescKeys = [WorldCount]string{
	"1471", "c336", "f40f", "6c6c", "8d9a", "cb1d",
	"5195", "012c", "323e", "57a1", "6da3", "708f",
	"6903", "4d80", "ae4e", "d0a4", "fdd7", "0ac0",
	"5f4b", "8475", "cffe", "7842", "dfdd", "6517",
}

// The six 112x112 minimap slots (corners inclusive).
var slotRects = [6]gfx.Rect{
	gfx.NewRect(192, 51, 303, 162), gfx.NewRect(330, 51, 441, 162), gfx.NewRect(469, 51, 580, 162),
	gfx.NewRect(192, 189, 303, 300), gfx.NewRect(330, 189, 441, 300), gfx.NewRect(469, 189, 580, 300),
}

// Input codes of the screen's buttons.
const (
	psSlot0 = 1000 + iota // ..psSlot5
	psSlot1
	psSlot2
	psSlot3
	psSlot4
	psSlot5
	psPrev
	psNext
	psRandom
	psCancel
	psHelp
	psDone
)

// PlanetSelect shows the planet menu and returns a world index from 0 through 23, or -1 on cancel.
//
// C++: PlanetSelectMenu (planetselectmenu.cpp)
// planetButton is one arrow or bottom plate button on the planet select
// screen: position, up art tag (down = tag with U->D), caption, key value and
// press sound.
type planetButton struct {
	x, y  int
	tag   string
	label string
	value int
	sfx   string
}

func PlanetSelect(ctx *Context, startWorld int) (int, error) {
	pic, err := ctx.loadBig("PLANETSE")
	if err != nil {
		return -1, err
	}
	ctx.drawBigAt(mainWindow, pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)
	menuPal := pic.Palette

	ctx.Sys.Fonts().SetFont(fonts.Font5)
	ctx.printCentered(ctx.text("f7c8", "Planet Select Menu"), gfx.NewRect(230, 6, 410, 26), colorGreen)

	// Snapshot the pristine background under the text displays; it is
	// restored before each redraw so text does not stack up.
	//
	// C++: PlanetSelectMenu image1/image3 (Image::Copy / Image::Write)
	st := &planetScreenState{
		nameRect: gfx.NewRect(16, 61, 165, 80),
		descRect: gfx.NewRect(41, 350, 600, 409),
	}
	st.nameBg = ctx.snapshotArea(st.nameRect)
	st.descBg = ctx.snapshotArea(st.descRect)

	buttons, err := buildPlanetWidgets(ctx)
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()
	if err != nil {
		return -1, err
	}

	sel := startWorld
	if sel < 0 || sel >= WorldCount {
		sel = 0
	}
	if err := drawPlanetGroup(ctx, &menuPal, sel, st); err != nil {
		return -1, err
	}

	ctx.Sys.MouseShow()

	for {
		key := toUpper(ctx.Sys.GetInput())
		prev := sel
		switch {
		case key == -1:
			continue
		case key >= psSlot0 && key <= psSlot5:
			sel = (sel/6)*6 + (key - psSlot0)
		case key == psPrev:
			sel = (sel + WorldCount - 6) % WorldCount
		case key == psNext:
			sel = (sel + 6) % WorldCount
		case key == psRandom:
			sel = ctx.Rand.Intn(WorldCount)
		case key == psDone, key == platform.KeyReturn:
			return sel, nil
		case key == psCancel, key == platform.KeyEscape:
			return -1, nil
		case key == psHelp, key == '?':
			// The dialog lives in its own window; nothing to repaint.
			Help(ctx, "PLANET_SETUP")
			continue
		default:
			continue
		}
		if sel != prev {
			if err := drawPlanetGroup(ctx, &menuPal, sel, st); err != nil {
				return -1, err
			}
		}
	}
}

// buildPlanetWidgets creates the six minimap click regions, the group arrows and
// the bottom plates, and returns their ids for the caller to delete.
//
// C++: PlanetSelectMenu::Init - the button table (planetselectmenu.cpp)
func buildPlanetWidgets(ctx *Context) ([]gnw.ButtonID, error) {
	var buttons []gnw.ButtonID
	var err error

	addBtn := func(bid gnw.ButtonID, sfx string) {
		if bid >= 0 {
			ctx.registerButtonSfx(bid, sfx)
			buttons = append(buttons, bid)
		}
	}

	// Minimap slots are plain click regions.
	for i, r := range slotRects {
		bid := ctx.Sys.AddButton(mainWindow, r.ULX, r.ULY, r.Width()+1, r.Height()+1,
			gnw.NoValue, gnw.NoValue, gnw.NoValue, psSlot0+i, nil, nil, nil, 0)
		addBtn(bid, "PSELM0")
	}

	// Group arrows and bottom buttons (down image = tag with U->D).
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	for _, d := range []planetButton{
		{60, 248, "MNULAROU", "", psPrev, "PSELW0"},
		{92, 248, "MNURAROU", "", psNext, "PSELW0"},
		{243, 438, "MNUBTN4U", ctx.text("67bc", "Random"), psRandom, "PDONE0"},
		{354, 438, "MNUBTN4U", ctx.text("9d35", "Cancel"), psCancel, "PCANC0"},
		{465, 438, "MNUBTN5U", ctx.text("8a8d", "?"), psHelp, "PHELP0"},
		{514, 438, "MNUBTN6U", ctx.text("bf2c", "Done"), psDone, "PDONE0"},
	} {
		downTag := strings.TrimSuffix(d.tag, "U") + "D"
		var bid gnw.ButtonID
		if d.label == "" {
			bid, err = ctx.plateButton(mainWindow, d.x, d.y, d.tag, downTag, "", d.value)
		} else {
			bid, err = ctx.plateButton(mainWindow, d.x, d.y, d.tag, downTag, d.label, d.value)
		}
		if err != nil {
			return buttons, err
		}
		addBtn(bid, d.sfx)
	}

	return buttons, nil
}

// planetScreenState carries the saved backgrounds of the text
// displays between redraws.
type planetScreenState struct {
	nameRect, descRect gfx.Rect
	nameBg, descBg     *gfx.Buffer
}

// drawPlanetGroup repaints the group backdrop, previews, selection box, and selected-world text. It restores text backgrounds first.
//
// C++: PlanetSelectMenu::DrawMaps + DrawTexts
func drawPlanetGroup(ctx *Context, menuPal *gfx.Palette, sel int, st *planetScreenState) error {
	group := sel / 6

	// Group backdrop (big image positioned by its own ulx/uly).
	pic, err := ctx.loadBig(groupPics[group])
	if err != nil {
		return err
	}
	ctx.drawBigAt(mainWindow, pic, 0, 0)

	// Six world previews.
	for i := 0; i < 6; i++ {
		if err := drawWorldPreview(ctx, menuPal, group*6+i, slotRects[i]); err != nil {
			dbg.Printf("preview %s: %v", worldFileTags[group*6+i], err)
		}
	}

	// Selection box (double red frame).
	r := slotRects[sel%6]
	ctx.Sys.Box(mainWindow, r.ULX, r.ULY, r.LRX, r.LRY, colorRed)
	ctx.Sys.Box(mainWindow, r.ULX+1, r.ULY+1, r.LRX-1, r.LRY-1, colorRed)

	// Restore the pristine background under the texts, then print.
	ctx.restoreArea(st.nameRect, st.nameBg)
	ctx.restoreArea(st.descRect, st.descBg)
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	ctx.printCentered(ctx.text(planetNameKeys[sel], fmt.Sprintf("World %d", sel+1)),
		st.nameRect, colorGreen)
	ctx.printWrapped(ctx.text(planetDescKeys[sel], ""), st.descRect, colorGreen)
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	return nil
}

// drawWorldPreview loads a .WRL preview and blits it into the slot,
// remapping its palette onto the menu palette.
//
// C++: Menu_LoadPlanetMinimap (menu.cpp)
func drawWorldPreview(ctx *Context, menuPal *gfx.Palette, worldIdx int, slot gfx.Rect) error {
	name, err := ctx.Lib.ReadString(worldFileTags[worldIdx])
	if err != nil {
		return err
	}
	dir, err := ctx.dataDir()
	if err != nil {
		return err
	}
	prev, err := world.LoadPreview(filepath.Join(dir, strings.TrimSpace(name)))
	if err != nil {
		return err
	}

	// Remap the world palette to the nearest menu palette entries.
	var remap [256]uint8
	for i := 0; i < gfx.PaletteSize; i++ {
		r, g, b := prev.Palette.RGB(i)
		remap[i] = menuPal.MapColor(r, g, b, true)
	}

	mini := prev.MinimapBuffer().Clone()
	mini.Recolor(&remap)

	buf := ctx.Sys.Buf(mainWindow)
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	dst := buf.SubRect(slot)
	if mini.W == dst.W && mini.H == dst.H {
		dst.Copy(mini)
	} else {
		dst.Scale(mini) // not square maps stretch into the slot
	}
	return nil
}
