// chooseplayer_test.go : testing unit for chooseplayer.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/testutil"
)

// newTestChoosePlayerMenu builds a choosePlayerMenu with its real buttons on a headless
// gnw System, seeded with the assignment ChoosePlayer forces for a single player game.
func newTestChoosePlayerMenu(t *testing.T) *choosePlayerMenu {
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
	sys := gnw.New(gfx.New(platform.ScreenWidth, platform.ScreenHeight), fs)
	ctx := &Context{Sys: sys, Lib: lib, Assets: a, DataDir: dir}

	params := &game.StartParams{}
	params.Teams[0].Type = game.TeamPlayer
	params.Teams[1].Type = game.TeamComputer
	params.Teams[2].Type = game.TeamNone
	params.Teams[3].Type = game.TeamNone

	m := &choosePlayerMenu{ctx: ctx, params: params, isSinglePlayer: true}
	if err := m.buildWidgets(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, b := range m.buttons {
			sys.DeleteButton(b)
		}
	})
	return m
}

// checkedColumns lists the type columns showing pressed on a team's row.
func checkedColumns(t *testing.T, m *choosePlayerMenu, team int) []int {
	t.Helper()
	cols := []int{}
	for col := 0; col < len(cpColumns); col++ {
		if m.ctx.Sys.ButtonChecked(m.typeBtns[col*4+team]) {
			cols = append(cols, col)
		}
	}
	return cols
}

// wantRow fails unless exactly one column is pressed on the row and it is the one
// matching the team's stored type, so the buttons can never disagree with the state.
func wantRow(t *testing.T, m *choosePlayerMenu, team int, want game.TeamType) {
	t.Helper()
	if got := m.params.Teams[team].Type; got != want {
		t.Errorf("team %d type = %v, want %v", team, got, want)
	}
	cols := checkedColumns(t, m, team)
	if len(cols) != 1 || cols[0] != typeColumn(want) {
		t.Errorf("team %d has columns %v pressed, want exactly [%d] for %v",
			team, cols, typeColumn(want), want)
	}
}

// TestChoosePlayerRowsStartWithOneButtonEach pins the built rest state: a row shows
// exactly one pressed button, the one for the type it was seeded with.
func TestChoosePlayerRowsStartWithOneButtonEach(t *testing.T) {
	m := newTestChoosePlayerMenu(t)

	wantRow(t, m, 0, game.TeamPlayer)
	wantRow(t, m, 1, game.TeamComputer)
	wantRow(t, m, 2, game.TeamNone)
	wantRow(t, m, 3, game.TeamNone)
}

// TestChoosePlayerHumanSwapReleasesTheOldHumanButton is the regression for the displaced
// team keeping its Human button lit. Picking Human on another row moves the single human
// slot and hands the clicked row's old type to the previous human, which must leave BOTH
// rows showing one pressed button. The old code only pressed the displaced team's new
// button and never released its Human one, so that row lit two at once and its radio
// group counted two checked in a group that allows one.
func TestChoosePlayerHumanSwapReleasesTheOldHumanButton(t *testing.T) {
	m := newTestChoosePlayerMenu(t)

	// Team 1's Human button, driven through gnw first so the radio group does its own
	// work exactly as a real click would, then dispatched like the input loop does.
	humanOfTeam1 := m.typeBtns[typeColumn(game.TeamPlayer)*4+1]
	m.ctx.Sys.PressAndRelease(humanOfTeam1)
	if _, done, err := m.handleKey(cpTypeBase + typeColumn(game.TeamPlayer)*4 + 1); err != nil || done {
		t.Fatalf("handleKey(team 1 human) = done %v, err %v", done, err)
	}

	// Team 1 is the human now; team 0 inherited team 1's old computer type.
	wantRow(t, m, 1, game.TeamPlayer)
	wantRow(t, m, 0, game.TeamComputer)
	wantRow(t, m, 2, game.TeamNone)
	wantRow(t, m, 3, game.TeamNone)

	// Only the human's clan button may be reachable.
	if m.ctx.Sys.ButtonDisabled(m.clanBtns[1]) {
		t.Error("the human team's clan button stayed disabled")
	}
	if !m.ctx.Sys.ButtonDisabled(m.clanBtns[0]) {
		t.Error("the displaced computer team's clan button stayed enabled")
	}
}

// TestChoosePlayerRowStaysSingleAcrossTypeSwitches walks a row through all three types
// and back, checking the row never shows two pressed buttons. A stale rest state also
// corrupts the group's checked counter, which only shows up on a later click.
func TestChoosePlayerRowStaysSingleAcrossTypeSwitches(t *testing.T) {
	m := newTestChoosePlayerMenu(t)

	for _, step := range []struct {
		team int
		want game.TeamType
	}{
		{2, game.TeamComputer}, {2, game.TeamNone}, {2, game.TeamPlayer},
		{2, game.TeamComputer}, {0, game.TeamPlayer}, {0, game.TeamNone},
	} {
		col := typeColumn(step.want)
		m.ctx.Sys.PressAndRelease(m.typeBtns[col*4+step.team])
		if _, done, err := m.handleKey(cpTypeBase + col*4 + step.team); err != nil || done {
			t.Fatalf("handleKey(team %d -> %v) = done %v, err %v", step.team, step.want, done, err)
		}
		wantRow(t, m, step.team, step.want)

		// No other row may be showing two either, whatever the swap did to it.
		for team := 0; team < game.TeamCount; team++ {
			if cols := checkedColumns(t, m, team); len(cols) != 1 {
				t.Fatalf("after team %d -> %v, team %d has columns %v pressed, want exactly one",
					step.team, step.want, team, cols)
			}
		}
	}
}
