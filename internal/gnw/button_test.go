// button_test.go : tests GNW button interaction and rendering.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gnw

import (
	"testing"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

func solid(w, h int, c uint8) *gfx.Buffer {
	b := gfx.New(w, h)
	b.Fill(c)
	return b
}

// mouse builds a MouseState for the helpers below.
func mouse(x, y int, buttons uint32) platform.MouseState {
	return platform.MouseState{X: x, Y: y, Buttons: buttons}
}

// click simulates press + release frames on (x, y), returning all
// emitted codes.
func click(s *System, x, y int) []int {
	var out []int
	out = append(out, s.CheckButtons(mouse(x, y, platform.MousePressLeft))...)
	out = append(out, s.CheckButtons(mouse(x, y, platform.MouseReleaseLeft))...)
	return out
}

func newButtonSys(t *testing.T) (*System, WinID) {
	t.Helper()
	s, _ := newSys(t, 320, 200)
	win := s.AddWindow(20, 20, 200, 120, 9, 0)
	return s, win
}

func TestButtonClick(t *testing.T) {
	s, win := newButtonSys(t)

	up, down := solid(40, 20, 50), solid(40, 20, 60)
	bid := s.AddButton(win, 10, 10, 40, 20, NoValue, NoValue, 100, 101, up, down, nil, 0)
	if bid < 0 {
		t.Fatal("AddButton failed")
	}

	// Button area is at screen 30..69 x 30..49.
	got := click(s, 35, 35)
	if len(got) != 2 || got[0] != 100 || got[1] != 101 {
		t.Errorf("click emitted %v, want [100 101]", got)
	}

	// Down image drawn while held.
	s.CheckButtons(mouse(35, 35, platform.MousePressLeft))
	if s.Buf(win).At(15, 15) != 60 {
		t.Errorf("held button shows %d, want down image 60", s.Buf(win).At(15, 15))
	}
	// Release outside cancels: only the press code fires.
	got = s.CheckButtons(mouse(200, 200, platform.MouseReleaseLeft))
	if len(got) != 0 {
		t.Errorf("release outside emitted %v", got)
	}
	if s.Buf(win).At(15, 15) != 50 {
		t.Error("canceled button not restored to up image")
	}

	// Click outside any button emits nothing.
	if got := click(s, 25, 25); len(got) != 0 {
		t.Errorf("empty area click emitted %v", got)
	}
}

func TestButtonHover(t *testing.T) {
	s, win := newButtonSys(t)
	up := solid(40, 20, 50)
	hover := solid(40, 20, 70)
	s.AddButton(win, 10, 10, 40, 20, 1, 2, NoValue, NoValue, up, solid(40, 20, 60), hover, 0)

	got := s.CheckButtons(mouse(35, 35, 0))
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("hover enter emitted %v, want [1]", got)
	}
	if s.Buf(win).At(15, 15) != 70 {
		t.Error("hover image not drawn")
	}
	got = s.CheckButtons(mouse(200, 200, 0))
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("hover leave emitted %v, want [2]", got)
	}
	if s.Buf(win).At(15, 15) != 50 {
		t.Error("up image not restored after hover leave")
	}
}

func TestStickyAndRadio(t *testing.T) {
	s, win := newButtonSys(t)

	// Sticky button toggles.
	bid := s.AddButton(win, 10, 10, 20, 10, NoValue, NoValue, NoValue, 7,
		solid(20, 10, 50), solid(20, 10, 60), nil, ButtonSticky)
	click(s, 35, 35)
	if !s.ButtonChecked(bid) {
		t.Fatal("sticky button not checked after click")
	}
	if s.Buf(win).At(15, 15) != 60 {
		t.Error("checked button must keep its down image")
	}
	click(s, 35, 35)
	if s.ButtonChecked(bid) {
		t.Fatal("sticky button still checked after second click")
	}

	// Radio group: one of three, clicking checked one is a no op.
	var changed []ButtonID
	ids := []ButtonID{
		s.AddButton(win, 10, 40, 20, 10, NoValue, NoValue, NoValue, 11, solid(20, 10, 50), solid(20, 10, 61), nil, 0),
		s.AddButton(win, 40, 40, 20, 10, NoValue, NoValue, NoValue, 12, solid(20, 10, 50), solid(20, 10, 62), nil, 0),
		s.AddButton(win, 70, 40, 20, 10, NoValue, NoValue, NoValue, 13, solid(20, 10, 50), solid(20, 10, 63), nil, 0),
	}
	s.GroupRadioButtons(ids, func(b ButtonID) { changed = append(changed, b) })

	click(s, 35, 65) // check first (screen: win+10..29 x win+40..49)
	if !s.ButtonChecked(ids[0]) {
		t.Fatal("radio 1 not checked")
	}
	click(s, 65, 65) // check second: first releases
	if s.ButtonChecked(ids[0]) || !s.ButtonChecked(ids[1]) {
		t.Fatal("radio switch failed")
	}
	click(s, 65, 65) // clicking checked radio keeps it checked
	if !s.ButtonChecked(ids[1]) {
		t.Fatal("radio unchecked by re-click despite NoUncheck")
	}
	if len(changed) == 0 {
		t.Error("group change callback never fired")
	}
}

func TestDisableAndRestState(t *testing.T) {
	s, win := newButtonSys(t)
	bid := s.AddButton(win, 10, 10, 20, 10, NoValue, NoValue, 1, 2,
		solid(20, 10, 50), solid(20, 10, 60), nil, 0)

	s.DisableButton(bid)
	if got := click(s, 35, 35); len(got) != 0 {
		t.Errorf("disabled button emitted %v", got)
	}
	s.EnableButton(bid)
	if got := click(s, 35, 35); len(got) != 2 {
		t.Errorf("re-enabled button emitted %v", got)
	}

	sticky := s.AddButton(win, 10, 40, 20, 10, NoValue, NoValue, NoValue, 3,
		solid(20, 10, 50), solid(20, 10, 60), nil, ButtonSticky)
	s.SetRestState(sticky, true)
	if !s.ButtonChecked(sticky) {
		t.Error("SetRestState(true) did not check")
	}
	if s.Buf(win).At(15, 45) != 60 {
		t.Error("SetRestState did not draw down image")
	}
}

func TestTopWindowWinsClicks(t *testing.T) {
	s, win := newButtonSys(t)
	up := solid(40, 20, 50)
	s.AddButton(win, 10, 10, 40, 20, NoValue, NoValue, NoValue, 100, up, nil, nil, 0)

	// A window covering the button steals the click.
	cover := s.AddWindow(20, 20, 200, 120, 8, 0)
	if got := click(s, 35, 35); len(got) != 0 {
		t.Errorf("covered button emitted %v", got)
	}
	s.DeleteWindow(cover)
	if got := click(s, 35, 35); len(got) != 1 || got[0] != 100 {
		t.Errorf("uncovered button emitted %v", got)
	}
}

func TestPressAndRelease(t *testing.T) {
	s, win := newButtonSys(t)
	var clicks []ButtonID
	s.SetButtonSoundFunc(func(b ButtonID, down bool) {
		if !down {
			clicks = append(clicks, b)
		}
	})
	bid := s.AddButton(win, 10, 10, 20, 10, NoValue, NoValue, NoValue, 42,
		solid(20, 10, 50), nil, nil, 0)

	s.PressAndRelease(bid)
	if got := s.GetInputNonBlocking(); got != 42 {
		t.Errorf("PressAndRelease queued %d, want 42", got)
	}
	if len(clicks) != 1 || clicks[0] != bid {
		t.Errorf("sound hook calls = %v", clicks)
	}
}

func TestTextButton(t *testing.T) {
	s, win := newButtonSys(t)
	bid := s.AddTextButton(win, 10, 10, NoValue, NoValue, NoValue, 55, "Done", 0)
	if bid < 0 {
		t.Fatal("AddTextButton failed")
	}
	r := s.findButton(bid).rect
	if r.Width() < 10 || r.Height() < 5 {
		t.Errorf("text button rect %+v too small", r)
	}
	got := click(s, 20+10+2, 20+10+2)
	if len(got) != 1 || got[0] != 55 {
		t.Errorf("text button click emitted %v, want [55]", got)
	}
}

func TestTextEdit(t *testing.T) {
	s, win := newButtonSys(t)
	te := s.NewTextEdit(win, 10, 80, 150, 12, 0x04, fonts.Font1)
	te.SetActive(true)

	for _, c := range "Player 1" {
		if !te.HandleKey(int(c)) {
			t.Fatalf("HandleKey(%c) not consumed", c)
		}
	}
	if te.Text() != "Player 1" {
		t.Errorf("Text = %q", te.Text())
	}
	te.HandleKey(platform.KeyBackspace)
	if te.Text() != "Player " {
		t.Errorf("after backspace: %q", te.Text())
	}
	if te.HandleKey(platform.KeyEscape) {
		t.Error("Escape must not be consumed")
	}

	// An actively edited field highlights white (0xFF), per the C++
	// TextEdit::DrawFullText (is_being_edited => 0xFF), regardless of its
	// configured idle color (0x04 here).
	countColor := func(want uint8) int {
		n := 0
		buf := s.Buf(win)
		s.Lock()
		defer s.Unlock()
		for y := 80; y < 90; y++ {
			for x := 10; x < 160; x++ {
				if buf.At(x, y) == want {
					n++
				}
			}
		}
		return n
	}
	if countColor(0xFF) == 0 {
		t.Error("active text edit drew nothing white")
	}

	// Inactive, it draws in its configured color (0x04).
	te.SetActive(false)
	if countColor(4) == 0 {
		t.Error("inactive text edit did not draw its configured color")
	}
}
