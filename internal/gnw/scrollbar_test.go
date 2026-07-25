// scrollbar_test.go : tests scrollbars and limited transfer bounds.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gnw

import (
	"reflect"
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

func testBar(w, h int, color uint8) *gfx.Buffer {
	b := gfx.New(w, h)
	b.Fill(color)
	return b
}

func TestScrollbarHorizontalMathAndCallback(t *testing.T) {
	s, win := newButtonSys(t)
	var changes [][2]int
	sb := s.NewScrollbar(ScrollbarOptions{
		Window:       win,
		SliderBounds: gfx.NewRect(10, 10, 110, 18),
		Bar:          testBar(100, 8, 70),
		IncreaseKey:  '+',
		DecreaseKey:  '-',
		ClickKey:     1000,
		OnChange: func(oldValue, newValue int) {
			changes = append(changes, [2]int{oldValue, newValue})
		},
	})
	sb.SetZeroOffset(10)
	sb.SetFreeCapacity(30)
	sb.SetValue(10)

	if !sb.ProcessKey('+', 0, 0) || sb.Value() != 11 {
		t.Fatalf("increase got value %d, want 11", sb.Value())
	}
	if !sb.ProcessKey('-', 0, 0) || sb.Value() != 10 {
		t.Fatalf("decrease got value %d, want 10", sb.Value())
	}
	if !sb.ProcessKey(1000, 60, 12) || sb.Value() != 20 {
		t.Fatalf("mid-track click value = %d, want 20", sb.Value())
	}
	if !reflect.DeepEqual(changes, [][2]int{{10, 11}, {11, 10}, {10, 20}}) {
		t.Fatalf("changes = %v", changes)
	}
	if sb.ProcessKey(12345, 0, 0) {
		t.Fatal("unknown key consumed")
	}
}

func TestScrollbarVerticalMathAndClamp(t *testing.T) {
	s, win := newButtonSys(t)
	sb := s.NewScrollbar(ScrollbarOptions{
		Window:       win,
		SliderBounds: gfx.NewRect(20, 20, 28, 120),
		Bar:          testBar(8, 100, 80),
		ClickKey:     1000,
		Orientation:  ScrollbarVertical,
	})
	sb.SetFreeCapacity(20)

	sb.ProcessKey(1000, 24, 120)
	if sb.Value() != 0 {
		t.Fatalf("bottom click value = %d, want 0", sb.Value())
	}
	sb.ProcessKey(1000, 24, 70)
	if sb.Value() != 10 {
		t.Fatalf("middle click value = %d, want 10", sb.Value())
	}
	sb.ProcessKey(1000, 24, 20)
	if sb.Value() != 20 {
		t.Fatalf("top click value = %d, want 20", sb.Value())
	}
	sb.ProcessKey(1000, 24, -200)
	if sb.Value() != 20 {
		t.Fatalf("above-track clamp = %d, want 20", sb.Value())
	}
}

func TestLimitedScrollbarClamp(t *testing.T) {
	s, win := newButtonSys(t)
	ls := s.NewLimitedScrollbar(ScrollbarOptions{
		Window:       win,
		SliderBounds: gfx.NewRect(10, 10, 110, 18),
		Bar:          testBar(100, 8, 70),
		IncreaseKey:  '+',
		DecreaseKey:  '-',
		ClickKey:     1000,
	})
	ls.SetZeroOffset(-10)
	ls.SetFreeCapacity(30)
	ls.SetXferGiveMax(12)
	ls.SetXferTakeMax(-4)
	ls.SetValue(12)

	if !ls.ProcessKey('+', 0, 0) || ls.Value() != 12 {
		t.Fatalf("increase beyond give max = %d, want 12", ls.Value())
	}
	ls.SetValue(-4)
	if !ls.ProcessKey('-', 0, 0) || ls.Value() != -4 {
		t.Fatalf("decrease beyond take max = %d, want -4", ls.Value())
	}
	if !ls.ProcessKey(1000, 110, 12) || ls.Value() != 12 {
		t.Fatalf("click beyond give max = %d, want 12", ls.Value())
	}
	if !ls.ProcessKey(1000, 0, 12) || ls.Value() != -4 {
		t.Fatalf("click below take max = %d, want -4", ls.Value())
	}
}

func TestScrollbarButtonAndProcessInput(t *testing.T) {
	s, win := newButtonSys(t)
	sb := s.NewScrollbar(ScrollbarOptions{
		Window:       win,
		SliderBounds: gfx.NewRect(10, 10, 110, 18),
		Bar:          testBar(100, 8, 70),
		ClickKey:     1000,
	})
	sb.SetFreeCapacity(20)
	if sb.ButtonID() <= 0 {
		t.Fatal("scrollbar did not register a slider button")
	}
	if got := click(s, 20+10, 20+10); len(got) == 0 || got[0] != 1000 {
		t.Fatalf("slider click emitted %v, want 1000", got)
	}
	if !sb.ProcessInput(1000, platform.MouseState{X: 20 + 60, Y: 20 + 12}) || sb.Value() != 10 {
		t.Fatalf("ProcessInput value = %d, want 10", sb.Value())
	}
}

func TestScrollbarDrawsRealSuppliedBar(t *testing.T) {
	s, win := newButtonSys(t)
	bar := gfx.New(10, 3)
	bar.Fill(0)
	for x := 0; x < 10; x++ {
		bar.Set(x, 1, 90)
	}
	sb := s.NewScrollbar(ScrollbarOptions{
		Window:       win,
		SliderBounds: gfx.NewRect(5, 5, 15, 8),
		Bar:          bar,
	})
	sb.SetFreeCapacity(10)
	sb.SetValue(5)

	buf := s.Buf(win)
	if got := buf.At(5, 6); got != 90 {
		t.Fatalf("bar pixel = %d, want supplied art color 90", got)
	}
	if got := buf.At(5, 5); got == 90 {
		t.Fatalf("transparent bar pixel drew color %d", got)
	}
}
