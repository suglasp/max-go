// gameover_render_test.go : guards the end of game score screen render: the title and
// per team stat columns must draw visible content over the ENDGAME background.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/winloss"
)

// TestGameOverRendersContent renders a two team win over ENDGAME5 and checks that the
// title and columns actually mark the buffer (the render path is intact). It does not
// assert exact pixels: it compares the buffer before and after the score content is
// drawn and requires a meaningful number of changed pixels.
func TestGameOverRendersContent(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	a, err := assets.Load(lib, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	fs, err := fonts.LoadSet(dir)
	if err != nil {
		t.Fatal(err)
	}
	screen := gfx.New(platform.ScreenWidth, platform.ScreenHeight)
	sys := gnw.New(screen, fs)
	ctx := &Context{Sys: sys, Lib: lib, Assets: a, DataDir: dir}

	data := screendata.GameOverData{
		TurnCounter: 40, // ENDGAME(1+40%9) = ENDGAME5
		Outcome:     winloss.PlayerWon,
		Teams: []screendata.GameOverTeam{
			{Name: "The Chosen", Points: 128, Rank: 1, FactoriesBuilt: 2, MinesBuilt: 3, BuildingsBuilt: 11, BuildingsLost: 1, UnitsBuilt: 9, UnitsLost: 2, GoldOnUpgrades: 60},
			{Name: "Crimson Path", Points: 47, Rank: 2, FactoriesBuilt: 1, MinesBuilt: 2, BuildingsBuilt: 6, BuildingsLost: 4, UnitsBuilt: 5, UnitsLost: 7, GoldOnUpgrades: 20},
		},
	}

	bg, err := ctx.loadBig("ENDGAME5")
	if err != nil {
		t.Fatalf("loadBig ENDGAME5: %v", err)
	}
	ctx.drawBigAt(bg, 0, 0)
	sys.Fonts().SetFont(fonts.Font5)
	f := sys.Fonts().Get(fonts.Font5)
	buf := sys.Buf(mainWindow)

	before := buf.Clone() // the bare background, before any score content

	sys.Lock()
	drawGameOverTitle(buf, f, ctx.text("71f1", "Game Over"))
	drawGameOverWin(ctx, buf, f, data)
	sys.Unlock()

	changed := 0
	for y := 0; y < buf.H; y++ {
		rb := before.Row(y)
		ra := buf.Row(y)
		for x := 0; x < buf.W; x++ {
			if ra[x] != rb[x] {
				changed++
			}
		}
	}

	// The title, two column tables, and left labels mark well over a thousand pixels; a near-zero count means score content did not render.
	if changed < 500 {
		t.Fatalf("score screen drew too little over the background: %d changed pixels (title/columns missing)", changed)
	}
}
