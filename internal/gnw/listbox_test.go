// listbox_test.go : tests list selection, scrolling, and navigation.
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

func listEntries(names ...string) []ListBoxEntry {
	entries := make([]ListBoxEntry, len(names))
	for i, name := range names {
		entries[i] = ListBoxEntry{Text: name, Data: i}
	}
	return entries
}

func TestListBoxSelectionClampAndCallbacks(t *testing.T) {
	s, win := newButtonSys(t)
	var selects [][2]int
	lb := s.NewListBox(ListBoxOptions{
		Window:    win,
		Bounds:    gfx.NewRect(10, 10, 109, 39),
		RowHeight: 10,
		Entries:   listEntries("A", "B", "C", "D", "E"),
		OnSelect: func(oldIndex, newIndex int) {
			selects = append(selects, [2]int{oldIndex, newIndex})
		},
	})

	if lb.Selected() != 0 || lb.ScrollOffset() != 0 || lb.VisibleRows() != 3 {
		t.Fatalf("initial selected/scroll/rows = %d/%d/%d, want 0/0/3", lb.Selected(), lb.ScrollOffset(), lb.VisibleRows())
	}
	lb.SetSelected(4)
	if lb.Selected() != 4 || lb.ScrollOffset() != 2 {
		t.Fatalf("select 4 selected/scroll = %d/%d, want 4/2", lb.Selected(), lb.ScrollOffset())
	}
	lb.SetSelected(99)
	if lb.Selected() != 4 || lb.ScrollOffset() != 2 {
		t.Fatalf("clamped high selected/scroll = %d/%d, want 4/2", lb.Selected(), lb.ScrollOffset())
	}
	lb.SetSelected(-99)
	if lb.Selected() != 0 || lb.ScrollOffset() != 0 {
		t.Fatalf("clamped low selected/scroll = %d/%d, want 0/0", lb.Selected(), lb.ScrollOffset())
	}
	if !reflect.DeepEqual(selects, [][2]int{{0, 4}, {4, 0}}) {
		t.Fatalf("select callbacks = %v", selects)
	}
}

func TestListBoxKeyboardNavigation(t *testing.T) {
	s, win := newButtonSys(t)
	lb := s.NewListBox(ListBoxOptions{
		Window:    win,
		Bounds:    gfx.NewRect(0, 0, 99, 29),
		RowHeight: 10,
		Entries:   listEntries("A", "B", "C", "D", "E", "F"),
	})

	lb.ProcessKey(platform.KeyDown, 0, 0)
	if lb.Selected() != 1 {
		t.Fatalf("down selected = %d, want 1", lb.Selected())
	}
	lb.ProcessKey(platform.KeyPageDown, 0, 0)
	if lb.Selected() != 4 || lb.ScrollOffset() != 2 {
		t.Fatalf("page down selected/scroll = %d/%d, want 4/2", lb.Selected(), lb.ScrollOffset())
	}
	lb.ProcessKey(platform.KeyPageUp, 0, 0)
	if lb.Selected() != 1 || lb.ScrollOffset() != 1 {
		t.Fatalf("page up selected/scroll = %d/%d, want 1/1", lb.Selected(), lb.ScrollOffset())
	}
	lb.ProcessKey(platform.KeyHome, 0, 0)
	if lb.Selected() != 0 || lb.ScrollOffset() != 0 {
		t.Fatalf("home selected/scroll = %d/%d, want 0/0", lb.Selected(), lb.ScrollOffset())
	}
	lb.ProcessKey(platform.KeyEnd, 0, 0)
	if lb.Selected() != 5 || lb.ScrollOffset() != 3 {
		t.Fatalf("end selected/scroll = %d/%d, want 5/3", lb.Selected(), lb.ScrollOffset())
	}
	if lb.ProcessKey('x', 0, 0) {
		t.Fatal("unknown key consumed")
	}
}

func TestListBoxClickRowAndConfirm(t *testing.T) {
	s, win := newButtonSys(t)
	var confirms []int
	lb := s.NewListBox(ListBoxOptions{
		Window:         win,
		Bounds:         gfx.NewRect(20, 20, 119, 49),
		RowHeight:      10,
		Entries:        listEntries("A", "B", "C", "D", "E"),
		ClickKey:       1000,
		DoubleClickKey: 1001,
		OnConfirm: func(index int) {
			confirms = append(confirms, index)
		},
	})
	lb.SetScrollOffset(1)

	index, ok := lb.RowAt(25, 41)
	if !ok || index != 3 {
		t.Fatalf("RowAt = %d/%v, want 3/true", index, ok)
	}
	if index, ok := lb.RowAt(25, 60); ok || index != -1 {
		t.Fatalf("RowAt outside = %d/%v, want -1/false", index, ok)
	}
	lb.ProcessKey(1000, 25, 41)
	if lb.Selected() != 3 {
		t.Fatalf("click selected = %d, want 3", lb.Selected())
	}
	lb.ProcessKey(platform.KeyReturn, 0, 0)
	lb.ProcessKey(1001, 25, 31)
	if !reflect.DeepEqual(confirms, []int{3, 2}) {
		t.Fatalf("confirm callbacks = %v, want [3 2]", confirms)
	}
}

func TestListBoxEmptySingleAndSetEntries(t *testing.T) {
	s, win := newButtonSys(t)
	var confirms []int
	lb := s.NewListBox(ListBoxOptions{
		Window:    win,
		Bounds:    gfx.NewRect(0, 0, 99, 29),
		RowHeight: 10,
		OnConfirm: func(index int) {
			confirms = append(confirms, index)
		},
	})

	if lb.Selected() != -1 || lb.ScrollOffset() != 0 {
		t.Fatalf("empty selected/scroll = %d/%d, want -1/0", lb.Selected(), lb.ScrollOffset())
	}
	lb.ProcessKey(platform.KeyReturn, 0, 0)
	if len(confirms) != 0 {
		t.Fatalf("empty confirm fired: %v", confirms)
	}
	lb.SetEntries(listEntries("Only"))
	if lb.Selected() != 0 || lb.ScrollOffset() != 0 {
		t.Fatalf("single selected/scroll = %d/%d, want 0/0", lb.Selected(), lb.ScrollOffset())
	}
	lb.SetSelected(10)
	if lb.Selected() != 0 {
		t.Fatalf("single clamp selected = %d, want 0", lb.Selected())
	}
	got := lb.Entries()
	got[0].Text = "changed"
	if lb.Entries()[0].Text != "Only" {
		t.Fatal("Entries exposed internal slice")
	}
}

func TestListBoxScrollbarSyncAndRegisteredButton(t *testing.T) {
	s, win := newButtonSys(t)
	sb := s.NewScrollbar(ScrollbarOptions{
		Window:       win,
		SliderBounds: gfx.NewRect(130, 10, 138, 50),
		Bar:          testBar(8, 40, 70),
		Orientation:  ScrollbarVertical,
	})
	lb := s.NewListBox(ListBoxOptions{
		Window:    win,
		Bounds:    gfx.NewRect(10, 10, 109, 39),
		RowHeight: 10,
		Entries:   listEntries("A", "B", "C", "D", "E", "F"),
		ClickKey:  1000,
		Scrollbar: sb,
	})

	if lb.ButtonID() <= 0 {
		t.Fatal("listbox did not register a click button")
	}
	if got := click(s, 20+10, 20+10); len(got) == 0 || got[0] != 1000 {
		t.Fatalf("listbox click emitted %v, want 1000", got)
	}
	lb.SetSelected(5)
	if lb.ScrollOffset() != 3 || sb.Value() != 3 {
		t.Fatalf("scroll/listbar = %d/%d, want 3/3", lb.ScrollOffset(), sb.Value())
	}
	lb.SetScrollOffset(99)
	if lb.ScrollOffset() != 3 || sb.Value() != 3 {
		t.Fatalf("clamped scroll/listbar = %d/%d, want 3/3", lb.ScrollOffset(), sb.Value())
	}
	lb.SetEntries(listEntries("A"))
	if lb.ScrollOffset() != 0 || sb.Value() != 0 {
		t.Fatalf("after shrink scroll/listbar = %d/%d, want 0/0", lb.ScrollOffset(), sb.Value())
	}
}
