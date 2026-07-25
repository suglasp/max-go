// construct_test.go : tests build rosters, rates, turns, and material costs.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/units"
)

// TestConstructRoster locks the engineer/constructor split and the Path flag.
func TestConstructRoster(t *testing.T) {
	con, conPath := constructRoster(assets.Constrct)
	eng, engPath := constructRoster(assets.Engineer)
	if len(con) == 0 || len(eng) == 0 {
		t.Fatalf("empty roster: con=%d eng=%d", len(con), len(eng))
	}
	if conPath {
		t.Error("constructor should not show the Path button")
	}
	if !engPath {
		t.Error("engineer should show the Path button")
	}
	if r, _ := constructRoster(assets.Tank); r != nil {
		t.Error("tank is not a builder, want nil roster")
	}
	// GOLDSM is the Gold Vault (engineer storage), not in the constructor list.
	for _, e := range con {
		if e.t == assets.GoldSm {
			t.Error("GOLDSM (Gold Vault) should be an engineer building")
		}
	}
}

// TestTurnsAndCost reproduces the vanilla Construction Menu numbers for a Mining
// Station (12 base turns) built by a constructor holding 60 material: the rate
// boxes read X1 12/24, X2 6/48, X4 5/56 (see compare/vanilla M.A.X.-constructor build.png).
// The cost climbs with the rate because faster building uses material less
// efficiently, and the depleting storage caps the speed each turn.
func TestTurnsAndCost(t *testing.T) {
	const base, storage = 12, 60
	cases := []struct{ speed, turns, cost int }{
		{1, 12, 24},
		{2, 6, 48},
		{4, 5, 56},
	}
	for _, c := range cases {
		gotT, gotC := turnsAndCost(assets.Constrct, assets.MiningSt, c.speed, storage, base)
		if gotT != c.turns || gotC != c.cost {
			t.Errorf("turnsAndCost speed %d = %d turns / %d cost, want %d / %d",
				c.speed, gotT, gotC, c.turns, c.cost)
		}
	}
}

// TestMaxBuildRate locks the build rate gate the Construction Menu uses to enable
// or disable the Build X1/X2/X4 buttons (a port of BuildMenu_GetMaxPossibleBuildRate):
// only rates sp <= max are live; the rest are disabled with blank turns/cost. The
// key case is a 1 turn item, which can only build at X1 (X2/X4 disabled) - a
// Only X1 is selectable when one turn remains.
func TestMaxBuildRate(t *testing.T) {
	cases := []struct {
		name             string
		buildTime, store int
		want             int
	}{
		{"one turn caps at X1", 1, 100000, 1},  // One turn permits only X1.
		{"two turns cap at X2", 2, 100000, 2},  // 2 turns can never reach X4
		{"ample storage allows X4", 12, 60, 4}, // matches TestTurnsAndCost's valid X4 row
		{"no storage floors at X1", 12, 0, 1},  // can't afford a faster rate
	}
	for _, c := range cases {
		if got := build.MaxBuildRate(assets.Constrct, c.buildTime, c.store); got != c.want {
			t.Errorf("%s: maxBuildRate(Constrct, %d, %d) = %d, want %d",
				c.name, c.buildTime, c.store, got, c.want)
		}
	}
	// The reachable rate must never decrease as storage grows (monotonic), and is
	// always one of 1/2/4 within [1,4].
	prev := 0
	for store := 0; store <= 200; store += 5 {
		m := build.MaxBuildRate(assets.Constrct, 12, store)
		if m != 1 && m != 2 && m != 4 {
			t.Fatalf("maxBuildRate gave a non-{1,2,4} rate %d at storage %d", m, store)
		}
		if m < prev {
			t.Fatalf("maxBuildRate dropped from %d to %d as storage grew to %d", prev, m, store)
		}
		prev = m
	}
}

// newTestConstructMenu builds a constructMenu on a headless gnw System with the
// CONBUILD art loaded, mirroring RunConstruction's setup minus buttons and
// palette (no platform needed). Without buttons the rate ids stay -1, which
// every gnw button call treats as a no-op.
func newTestConstructMenu(t *testing.T, storage int) *constructMenu {
	t.Helper()
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

	pic, err := ctx.loadBig("CONBUILD")
	if err != nil {
		t.Fatal(err)
	}
	ctx.drawBigAt(mainWindow, pic, 0, 0)
	sys.Fonts().SetFont(fonts.Font5)

	roster, _ := constructRoster(assets.Constrct)
	return &constructMenu{
		ctx: ctx, builder: assets.Constrct, storage: storage, roster: roster,
		rate: 1, maxRate: 1,
		pic: pic, f5: sys.Fonts().Get(fonts.Font5), buf: sys.Buf(mainWindow),
		sc: units.NewSpriteCache(lib, &pic.Palette), at: a.Attribs,
		listRows:  (cbListLRY - cbListY) / cbRowH,
		statIcons: map[string]*gfx.Buffer{}, portraitCache: map[assets.UnitType]*sprites.Big{},
		rateX1: -1, rateX2: -1, rateX4: -1,
	}
}

// TestConstructRateClampAndRedTurns verifies that sufficient storage permits X4 and empty storage clamps the rate to X1 with red turn counts.
func TestConstructRateClampAndRedTurns(t *testing.T) {
	m := newTestConstructMenu(t, 60)
	m.drawList()
	m.drawDetail()
	m.refreshRates()
	if m.maxRate != 4 {
		t.Fatalf("maxRate with 60 storage = %d, want 4 (TestMaxBuildRate's ample case)", m.maxRate)
	}
	m.rate = 4 // the input loop's Build X4 case
	m.drawRates()

	turnsCol := gfx.NewRect(cbTurnsRX-15, cbListY, cbTurnsRX, cbListY+m.listRows*cbRowH-1)
	redRich := countColor(t, m.ctx.Sys, turnsCol, colorRed)

	m.storage = 0
	m.refreshRates()
	if m.maxRate != 1 || m.rate != 1 {
		t.Errorf("empty storage: maxRate=%d rate=%d, want both clamped to 1", m.maxRate, m.rate)
	}
	m.drawList()
	if redPoor := countColor(t, m.ctx.Sys, turnsCol, colorRed); redPoor <= redRich {
		t.Errorf("red turns pixels with empty storage = %d, want more than the rich count %d", redPoor, redRich)
	}
}
