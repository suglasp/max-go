// scrollbar.go : implements vertical, horizontal, and limited scrollbars.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gnw

import (
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// ScrollbarOrientation selects the axis used by Scrollbar click math and bar
// drawing.
type ScrollbarOrientation int

const (
	ScrollbarHorizontal ScrollbarOrientation = iota
	ScrollbarVertical
)

// ScrollbarOptions describes a GNW scrollbar. SliderBounds and AmountBounds
// follow the original's Rect convention here: LRX/ LRY are exclusive for the width/
// height calculation, matching scrollbar.cpp's lrx-ulx formulas.
type ScrollbarOptions struct {
	Window       WinID
	SliderBounds gfx.Rect
	AmountBounds gfx.Rect
	Bar          *gfx.Buffer
	Background   *gfx.Buffer
	IncreaseKey  int
	DecreaseKey  int
	ClickKey     int
	ScaleFactor  int
	Orientation  ScrollbarOrientation
	OnChange     func(oldValue, newValue int)
}

// Scrollbar is the reusable GNW transfer/scroll widget ported from
// scrollbar.cpp. It owns only the slider button and value math; consumers still
// supply the real M.A.X. art buffers and decide what list/menu the value drives.
//
// C++ ported : scrollbar.cpp - Scrollbar
type Scrollbar struct {
	sys *System
	win WinID

	sliderBounds gfx.Rect
	amountBounds gfx.Rect
	bar          *gfx.Buffer
	background   *gfx.Buffer
	button       ButtonID

	freeCapacity int
	zeroOffset   int
	value        int
	scaleFactor  int
	orientation  ScrollbarOrientation

	increaseKey int
	decreaseKey int
	clickKey    int

	onChange func(oldValue, newValue int)
}

// NewScrollbar creates and registers a scrollbar slider button, then draws the
// initial value.
func (s *System) NewScrollbar(opts ScrollbarOptions) *Scrollbar {
	if opts.ClickKey == 0 {
		opts.ClickKey = NoValue
	}
	sb := &Scrollbar{
		sys:          s,
		win:          opts.Window,
		sliderBounds: opts.SliderBounds,
		amountBounds: opts.AmountBounds,
		bar:          opts.Bar,
		background:   opts.Background,
		increaseKey:  opts.IncreaseKey,
		decreaseKey:  opts.DecreaseKey,
		clickKey:     opts.ClickKey,
		scaleFactor:  opts.ScaleFactor,
		orientation:  opts.Orientation,
		onChange:     opts.OnChange,
	}
	sb.registerButton()
	sb.Refresh()
	return sb
}

func (sb *Scrollbar) registerButton() {
	if sb.sys == nil || sb.win < 0 || sb.clickKey == NoValue {
		return
	}
	width, height := sb.sliderWidth(), sb.sliderHeight()
	x, y := sb.sliderBounds.ULX, sb.sliderBounds.ULY
	if sb.orientation == ScrollbarVertical {
		y -= 3
		height += 6
	} else {
		x -= 3
		width += 6
	}
	sb.button = sb.sys.AddButton(sb.win, x, y, width, height,
		NoValue, NoValue, sb.clickKey, NoValue, nil, nil, nil, 0)
}

// ButtonID returns the registered slider button id.
func (sb *Scrollbar) ButtonID() ButtonID {
	return sb.button
}

// SetValue sets the current value without firing OnChange, like the original's setter.
func (sb *Scrollbar) SetValue(value int) {
	sb.value = sb.clamp(value)
	sb.Refresh()
}

// Value returns the current scrollbar value.
func (sb *Scrollbar) Value() int {
	return sb.value
}

// SetZeroOffset sets the minimum value.
func (sb *Scrollbar) SetZeroOffset(offset int) {
	sb.zeroOffset = offset
	sb.value = sb.clamp(sb.value)
	sb.Refresh()
}

// SetFreeCapacity sets the maximum value.
func (sb *Scrollbar) SetFreeCapacity(freeCapacity int) {
	sb.freeCapacity = freeCapacity
	sb.value = sb.clamp(sb.value)
	sb.Refresh()
}

// SetBar replaces the art used for the filled bar.
func (sb *Scrollbar) SetBar(bar *gfx.Buffer) {
	sb.bar = bar
	sb.Refresh()
}

// Refresh redraws the slider background, filled bar, and optional amount text.
func (sb *Scrollbar) Refresh() {
	if sb.sys == nil || sb.win < 0 {
		return
	}
	buf := sb.sys.Buf(sb.win)
	if buf == nil {
		return
	}
	sb.sys.Lock()
	defer sb.sys.Unlock()

	if sb.background != nil && sb.amountWidth() > 0 && sb.amountHeight() > 0 {
		copyClipped(buf, sb.amountBounds.ULX, sb.amountBounds.ULY, sb.background)
	}
	sliderW, sliderH := sb.sliderWidth(), sb.sliderHeight()
	if sliderW <= 0 || sliderH <= 0 {
		return
	}
	if sb.background != nil {
		copyClipped(buf, sb.sliderBounds.ULX, sb.sliderBounds.ULY, sb.background)
	} else {
		buf.Sub(sb.sliderBounds.ULX, sb.sliderBounds.ULY, sliderW, sliderH).Fill(0)
	}

	capacity := sb.freeCapacity - sb.zeroOffset
	filled := 0
	if capacity != 0 {
		if sb.orientation == ScrollbarVertical {
			filled = ((sb.value - sb.zeroOffset) * sliderH) / capacity
		} else {
			filled = ((sb.value - sb.zeroOffset) * sliderW) / capacity
		}
	}
	if filled < 0 {
		filled = 0
	}
	if sb.orientation == ScrollbarVertical {
		if filled > sliderH {
			filled = sliderH
		}
		drawBar(buf, sb.sliderBounds.ULX, sb.sliderBounds.ULY+sliderH-filled, sliderW, filled, sb.bar)
	} else {
		if filled > sliderW {
			filled = sliderW
		}
		drawBar(buf, sb.sliderBounds.ULX, sb.sliderBounds.ULY, filled, sliderH, sb.bar)
	}
	if sb.scaleFactor != 0 && sb.amountWidth() > 0 && sb.amountHeight() > 0 {
		sb.sys.fonts.Font().Blit(buf.Sub(sb.amountBounds.ULX, sb.amountBounds.ULY, sb.amountWidth(), sb.amountHeight()),
			itoa(sb.scaleFactor*sb.value), sb.amountWidth(), 0xFF)
	}
}

// ProcessKey handles increase/decrease/click input. mouseX/mouseY are
// window relative cursor coordinates, matching the window->GetCursorPosition call.
//
// C++ ported : scrollbar.cpp - Scrollbar::ProcessKey
func (sb *Scrollbar) ProcessKey(key, mouseX, mouseY int) bool {
	switch key {
	case sb.increaseKey:
		if sb.value < sb.freeCapacity {
			sb.processValueChange(sb.value + 1)
		}
		return true
	case sb.decreaseKey:
		if sb.value > sb.zeroOffset {
			sb.processValueChange(sb.value - 1)
		}
		return true
	case sb.clickKey:
		sb.processValueChange(sb.valueFromClick(mouseX, mouseY))
		return true
	default:
		return false
	}
}

// ProcessInput handles a GNW key code using the current platform mouse.
func (sb *Scrollbar) ProcessInput(key int, mouse platform.MouseState) bool {
	if sb.sys == nil {
		return false
	}
	r, ok := sb.sys.WinRect(sb.win)
	if !ok {
		return false
	}
	return sb.ProcessKey(key, mouse.X-r.ULX, mouse.Y-r.ULY)
}

func (sb *Scrollbar) processValueChange(newValue int) {
	newValue = sb.clamp(newValue)
	oldValue := sb.value
	sb.value = newValue
	if oldValue != newValue {
		sb.Refresh()
		if sb.onChange != nil {
			sb.onChange(oldValue, newValue)
		}
	}
}

func (sb *Scrollbar) valueFromClick(mouseX, mouseY int) int {
	capacity := sb.freeCapacity - sb.zeroOffset
	var value int
	if sb.orientation == ScrollbarVertical {
		y := sb.sliderBounds.ULY + sb.sliderHeight() - mouseY
		value = (sb.sliderHeight()/2+capacity*y)/sb.sliderHeight() + sb.zeroOffset
	} else {
		x := mouseX - sb.sliderBounds.ULX
		value = (sb.sliderWidth()/2+capacity*x)/sb.sliderWidth() + sb.zeroOffset
	}
	return sb.clamp(value)
}

func (sb *Scrollbar) clamp(value int) int {
	if value < sb.zeroOffset {
		return sb.zeroOffset
	}
	if value > sb.freeCapacity {
		return sb.freeCapacity
	}
	return value
}

func (sb *Scrollbar) sliderWidth() int {
	return sb.sliderBounds.LRX - sb.sliderBounds.ULX
}
func (sb *Scrollbar) sliderHeight() int {
	return sb.sliderBounds.LRY - sb.sliderBounds.ULY
}
func (sb *Scrollbar) amountWidth() int {
	return sb.amountBounds.LRX - sb.amountBounds.ULX
}
func (sb *Scrollbar) amountHeight() int {
	return sb.amountBounds.LRY - sb.amountBounds.ULY
}

// LimitedScrollbar is Scrollbar with the transfer give/take clamps.
//
// C++ ported : scrollbar.cpp - LimitedScrollbar
type LimitedScrollbar struct {
	*Scrollbar
	xferGiveMax int
	xferTakeMax int
}

// NewLimitedScrollbar creates a limited scrollbar.
func (s *System) NewLimitedScrollbar(opts ScrollbarOptions) *LimitedScrollbar {
	return &LimitedScrollbar{
		Scrollbar:   s.NewScrollbar(opts),
		xferTakeMax: -28672,
	}
}

// SetXferGiveMax sets the maximum value this limited scrollbar can increase to.
func (ls *LimitedScrollbar) SetXferGiveMax(limit int) {
	ls.xferGiveMax = limit
}

// SetXferTakeMax sets the minimum value this limited scrollbar can decrease to.
func (ls *LimitedScrollbar) SetXferTakeMax(limit int) {
	ls.xferTakeMax = limit
}

// ProcessKey handles the limited scrollbar bounds before delegating.
//
// C++ ported : scrollbar.cpp - LimitedScrollbar::ProcessKey
func (ls *LimitedScrollbar) ProcessKey(key, mouseX, mouseY int) bool {
	switch key {
	case ls.increaseKey:
		if ls.value >= ls.xferGiveMax {
			return true
		}
	case ls.decreaseKey:
		if ls.value <= ls.xferTakeMax {
			return true
		}
	case ls.clickKey:
		newValue := ls.valueFromClick(mouseX, mouseY)
		if newValue > ls.xferGiveMax {
			newValue = ls.xferGiveMax
		}
		if newValue < ls.xferTakeMax {
			newValue = ls.xferTakeMax
		}
		ls.processValueChange(newValue)
		return true
	}
	return ls.Scrollbar.ProcessKey(key, mouseX, mouseY)
}

// ProcessInput handles a GNW key code using the current platform mouse.
func (ls *LimitedScrollbar) ProcessInput(key int, mouse platform.MouseState) bool {
	if ls == nil || ls.Scrollbar == nil || ls.sys == nil {
		return false
	}
	r, ok := ls.sys.WinRect(ls.win)
	if !ok {
		return false
	}
	return ls.ProcessKey(key, mouse.X-r.ULX, mouse.Y-r.ULY)
}

func copyClipped(dst *gfx.Buffer, x, y int, src *gfx.Buffer) {
	if src == nil || x < 0 || y < 0 || x >= dst.W || y >= dst.H {
		return
	}
	w, h := src.W, src.H
	if x+w > dst.W {
		w = dst.W - x
	}
	if y+h > dst.H {
		h = dst.H - y
	}
	if w <= 0 || h <= 0 {
		return
	}
	dst.Sub(x, y, w, h).Copy(src.Sub(0, 0, w, h))
}

/* // note: ported but unused code in Go port
// LoadHorizontalBar draws the left part of bar into dst, using color 0 as
// transparent. It mirrors the original's helper, with callers passing the decoded art.
//
// C++ ported : scrollbar.cpp - LoadHorizontalBar
func LoadHorizontalBar(dst *gfx.Buffer, capacity, height int, bar *gfx.Buffer) {
	drawBar(dst, 0, 0, capacity, height, bar)
}

// LoadVerticalBar draws a vertical bar segment into dst, using color 0 as
// transparent. It mirrors the original's helper, with callers passing the decoded art.
//
// C++ ported : scrollbar.cpp - LoadVerticalBar
func LoadVerticalBar(dst *gfx.Buffer, capacity, width int, bar *gfx.Buffer) {
	drawBar(dst, 0, 0, width, capacity, bar)
}
*/

// drawBar blits the top-left wExtent x hExtent rectangle of bar into dst at x,y,
// using color 0 as transparent. Both extents clamp to the bar art size. The
// horizontal and vertical scrollbar fills are the same blit, only the axis that
// grows with the filled amount differs, so both share this one helper.
func drawBar(dst *gfx.Buffer, x, y, wExtent, hExtent int, bar *gfx.Buffer) {
	if bar == nil || wExtent <= 0 || hExtent <= 0 {
		return
	}
	if wExtent > bar.W {
		wExtent = bar.W
	}
	if hExtent > bar.H {
		hExtent = bar.H
	}
	for yy := 0; yy < hExtent; yy++ {
		for xx := 0; xx < wExtent; xx++ {
			c := bar.At(xx, yy)
			if c != 0 && x+xx >= 0 && y+yy >= 0 && x+xx < dst.W && y+yy < dst.H {
				dst.Set(x+xx, y+yy, c)
			}
		}
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
