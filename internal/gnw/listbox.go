// listbox.go : implements reusable scrollable list selection.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gnw

/* // note: unused code in Go port
// ListBox was only built by the -emulate listbox bench; no menu uses a list box and the reference has no list box widget.

import (
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// ListBoxEntry is one selectable row in a GNW list box. Data lets callers keep
// the menu specific payload without coupling the widget to a game type.
type ListBoxEntry struct {
	Text string
	Data any
}

// ListBoxOptions describes a widget only GNW list box. Bounds are
// window relative and inclusive, matching the rest of the GNW layer.
type ListBoxOptions struct {
	Window         WinID
	Bounds         gfx.Rect
	Entries        []ListBoxEntry
	RowHeight      int
	Background     *gfx.Buffer
	Scrollbar      *Scrollbar
	ClickKey       int
	DoubleClickKey int
	ConfirmKey     int
	UpKey          int
	DownKey        int
	PageUpKey      int
	PageDownKey    int
	TextColor      uint8
	SelectedColor  uint8
	OnSelect       func(oldIndex, newIndex int)
	OnConfirm      func(index int)
}

// ListBox is the headless testable list/selection part of the original's scrolling
// menus. It owns row math, selected index, scroll offset, and callbacks; callers
// provide art and wire it into a concrete screen.
//
// The click, selection, confirmation, and scrollbar offset behavior follows local Scrollbar conventions because no listbox.cpp source is available.
type ListBox struct {
	sys *System
	win WinID

	bounds     gfx.Rect
	rowHeight  int
	background *gfx.Buffer
	scrollbar  *Scrollbar
	button     ButtonID

	entries     []ListBoxEntry
	selected    int
	scroll      int
	textColor   uint8
	selectColor uint8

	clickKey       int
	doubleClickKey int
	confirmKey     int
	upKey          int
	downKey        int
	pageUpKey      int
	pageDownKey    int

	onSelect  func(oldIndex, newIndex int)
	onConfirm func(index int)
}

// NewListBox creates a list box and registers a click button when ClickKey is
// supplied.
func (s *System) NewListBox(opts ListBoxOptions) *ListBox {
	if opts.ClickKey == 0 {
		opts.ClickKey = NoValue
	}
	if opts.DoubleClickKey == 0 {
		opts.DoubleClickKey = NoValue
	}
	if opts.ConfirmKey == 0 {
		opts.ConfirmKey = platform.KeyReturn
	}
	if opts.UpKey == 0 {
		opts.UpKey = platform.KeyUp
	}
	if opts.DownKey == 0 {
		opts.DownKey = platform.KeyDown
	}
	if opts.PageUpKey == 0 {
		opts.PageUpKey = platform.KeyPageUp
	}
	if opts.PageDownKey == 0 {
		opts.PageDownKey = platform.KeyPageDown
	}
	if opts.RowHeight <= 0 {
		opts.RowHeight = 10
	}
	if opts.TextColor == 0 {
		opts.TextColor = 0xFF
	}
	if opts.SelectedColor == 0 {
		opts.SelectedColor = 0x38
	}
	lb := &ListBox{
		sys:            s,
		win:            opts.Window,
		bounds:         opts.Bounds,
		rowHeight:      opts.RowHeight,
		background:     opts.Background,
		scrollbar:      opts.Scrollbar,
		selected:       -1,
		textColor:      opts.TextColor,
		selectColor:    opts.SelectedColor,
		clickKey:       opts.ClickKey,
		doubleClickKey: opts.DoubleClickKey,
		confirmKey:     opts.ConfirmKey,
		upKey:          opts.UpKey,
		downKey:        opts.DownKey,
		pageUpKey:      opts.PageUpKey,
		pageDownKey:    opts.PageDownKey,
		onSelect:       opts.OnSelect,
		onConfirm:      opts.OnConfirm,
	}
	lb.entries = append(lb.entries, opts.Entries...)
	if len(lb.entries) > 0 {
		lb.selected = 0
	}
	lb.syncScrollbarBounds()
	lb.registerButton()
	lb.Refresh()
	return lb
}

func (lb *ListBox) registerButton() {
	if lb.sys == nil || lb.win < 0 || lb.clickKey == NoValue {
		return
	}
	lb.button = lb.sys.AddButton(lb.win, lb.bounds.ULX, lb.bounds.ULY, lb.width(), lb.height(),
		NoValue, NoValue, lb.clickKey, NoValue, nil, nil, nil, 0)
}

// ButtonID returns the list area's registered click button id.
func (lb *ListBox) ButtonID() ButtonID {
	return lb.button
}

// SetEntries replaces all rows and clamps selection/scroll offset.
func (lb *ListBox) SetEntries(entries []ListBoxEntry) {
	lb.entries = append(lb.entries[:0], entries...)
	if len(lb.entries) == 0 {
		lb.selected = -1
		lb.scroll = 0
	} else if lb.selected < 0 {
		lb.selected = 0
	} else if lb.selected >= len(lb.entries) {
		lb.selected = len(lb.entries) - 1
	}
	lb.scroll = lb.clampScroll(lb.scroll)
	lb.ensureSelectedVisible()
	lb.syncScrollbarBounds()
	lb.Refresh()
}

// Entries returns the current rows.
func (lb *ListBox) Entries() []ListBoxEntry {
	out := make([]ListBoxEntry, len(lb.entries))
	copy(out, lb.entries)
	return out
}

// Selected returns the selected row index, or -1 when the list is empty.
func (lb *ListBox) Selected() int {
	return lb.selected
}

// ScrollOffset returns the first visible row index.
func (lb *ListBox) ScrollOffset() int {
	return lb.scroll
}

// VisibleRows returns how many whole rows fit inside the list bounds.
func (lb *ListBox) VisibleRows() int {
	if lb.rowHeight <= 0 {
		return 0
	}
	rows := lb.height() / lb.rowHeight
	if rows < 0 {
		return 0
	}
	return rows
}

// SetSelected selects a row, clamps it, and fires OnSelect on real changes.
func (lb *ListBox) SetSelected(index int) {
	if len(lb.entries) == 0 {
		lb.setSelectedNoCallback(-1)
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(lb.entries) {
		index = len(lb.entries) - 1
	}
	old := lb.selected
	lb.selected = index
	lb.ensureSelectedVisible()
	lb.syncScrollbarValue()
	lb.Refresh()
	if old != index && lb.onSelect != nil {
		lb.onSelect(old, index)
	}
}

// SetScrollOffset sets the first visible row without changing selection.
func (lb *ListBox) SetScrollOffset(offset int) {
	lb.scroll = lb.clampScroll(offset)
	lb.syncScrollbarValue()
	lb.Refresh()
}

// RowAt maps window relative mouse coordinates to an entry index.
func (lb *ListBox) RowAt(mouseX, mouseY int) (int, bool) {
	if mouseX < lb.bounds.ULX || mouseX > lb.bounds.LRX || mouseY < lb.bounds.ULY || mouseY > lb.bounds.LRY {
		return -1, false
	}
	row := (mouseY - lb.bounds.ULY) / lb.rowHeight
	if row < 0 || row >= lb.VisibleRows() {
		return -1, false
	}
	index := lb.scroll + row
	if index < 0 || index >= len(lb.entries) {
		return -1, false
	}
	return index, true
}

// ProcessKey handles keyboard/list click input. mouseX/mouseY are
// window relative cursor coordinates, matching the Scrollbar API.
func (lb *ListBox) ProcessKey(key, mouseX, mouseY int) bool {
	switch key {
	case lb.upKey:
		lb.SetSelected(lb.selected - 1)
		return true
	case lb.downKey:
		lb.SetSelected(lb.selected + 1)
		return true
	case lb.pageUpKey:
		lb.SetSelected(lb.selected - maxInt(1, lb.VisibleRows()))
		return true
	case lb.pageDownKey:
		lb.SetSelected(lb.selected + maxInt(1, lb.VisibleRows()))
		return true
	case platform.KeyHome:
		lb.SetSelected(0)
		return true
	case platform.KeyEnd:
		lb.SetSelected(len(lb.entries) - 1)
		return true
	case lb.clickKey:
		if index, ok := lb.RowAt(mouseX, mouseY); ok {
			lb.SetSelected(index)
		}
		return true
	case lb.doubleClickKey:
		if index, ok := lb.RowAt(mouseX, mouseY); ok {
			lb.SetSelected(index)
			lb.confirm(index)
		}
		return true
	case lb.confirmKey:
		lb.confirm(lb.selected)
		return true
	default:
		return false
	}
}

// ProcessInput handles a GNW key code using the current platform mouse.
func (lb *ListBox) ProcessInput(key int, mouse platform.MouseState) bool {
	if lb == nil || lb.sys == nil {
		return false
	}
	r, ok := lb.sys.WinRect(lb.win)
	if !ok {
		return false
	}
	return lb.ProcessKey(key, mouse.X-r.ULX, mouse.Y-r.ULY)
}

// Refresh redraws the optional background, visible row text, and selection bar.
func (lb *ListBox) Refresh() {
	if lb.sys == nil || lb.win < 0 {
		return
	}
	buf := lb.sys.Buf(lb.win)
	if buf == nil {
		return
	}
	lb.sys.Lock()
	defer lb.sys.Unlock()

	dst := buf.Sub(lb.bounds.ULX, lb.bounds.ULY, lb.width(), lb.height())
	if lb.background != nil {
		copyClipped(buf, lb.bounds.ULX, lb.bounds.ULY, lb.background)
	} else {
		dst.Fill(0)
	}
	if lb.sys.fonts == nil || lb.sys.fonts.Font() == nil {
		return
	}
	for row := 0; row < lb.VisibleRows(); row++ {
		index := lb.scroll + row
		if index >= len(lb.entries) {
			break
		}
		y := row * lb.rowHeight
		if index == lb.selected {
			dst.Sub(0, y, lb.width(), minInt(lb.rowHeight, lb.height()-y)).Fill(lb.selectColor)
		}
		lb.sys.fonts.Font().Blit(dst.Sub(2, y, maxInt(0, lb.width()-2), minInt(lb.rowHeight, lb.height()-y)),
			lb.entries[index].Text, maxInt(0, lb.width()-2), int(lb.textColor))
	}
}

func (lb *ListBox) confirm(index int) {
	if index >= 0 && index < len(lb.entries) && lb.onConfirm != nil {
		lb.onConfirm(index)
	}
}

func (lb *ListBox) setSelectedNoCallback(index int) {
	lb.selected = index
	lb.ensureSelectedVisible()
	lb.syncScrollbarValue()
	lb.Refresh()
}

func (lb *ListBox) ensureSelectedVisible() {
	if lb.selected < 0 {
		lb.scroll = lb.clampScroll(lb.scroll)
		return
	}
	rows := lb.VisibleRows()
	if rows <= 0 {
		lb.scroll = 0
		return
	}
	if lb.selected < lb.scroll {
		lb.scroll = lb.selected
	} else if lb.selected >= lb.scroll+rows {
		lb.scroll = lb.selected - rows + 1
	}
	lb.scroll = lb.clampScroll(lb.scroll)
}

func (lb *ListBox) clampScroll(offset int) int {
	maxScroll := lb.maxScroll()
	if offset < 0 {
		return 0
	}
	if offset > maxScroll {
		return maxScroll
	}
	return offset
}

func (lb *ListBox) maxScroll() int {
	rows := lb.VisibleRows()
	if rows <= 0 || len(lb.entries) <= rows {
		return 0
	}
	return len(lb.entries) - rows
}

func (lb *ListBox) syncScrollbarBounds() {
	if lb.scrollbar == nil {
		return
	}
	lb.scrollbar.SetZeroOffset(0)
	lb.scrollbar.SetFreeCapacity(lb.maxScroll())
	lb.syncScrollbarValue()
}

func (lb *ListBox) syncScrollbarValue() {
	if lb.scrollbar != nil {
		lb.scrollbar.SetValue(lb.scroll)
	}
}

func (lb *ListBox) width() int {
	return lb.bounds.LRX - lb.bounds.ULX + 1
}
func (lb *ListBox) height() int {
	return lb.bounds.LRY - lb.bounds.ULY + 1
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
*/
