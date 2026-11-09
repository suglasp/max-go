// platform.go : Ebitengine shell: 640x480 8-bit screen, RGBA conversion, GNW key input, mouse tracking, and
// screenshot support.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : svga.cpp, input.cpp - SDL3 display and input layer
package platform

import (
	"fmt"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/gfx"
)

// The 4:3 logical resolution of the menus - the startup size of the screen buffer.
const (
	ScreenWidth  = 640
	ScreenHeight = 480
)

// WideWidth is the 16:9 logical width the live game view switches to when
// video.widescreen is on (854x480 = true 16:9 at the 480 height; 214 extra columns).
const WideWidth = 854

// WidescreenMode selects the wide game view: forced off, forced on, or
// picked from the monitor aspect at startup (video.widescreen = auto).
type WidescreenMode int

// Widescreen modes, mapped from the video.widescreen setting.
const (
	WidescreenOff WidescreenMode = iota
	WidescreenOn
	WidescreenAuto
)

// String names the mode for the debug log.
func (m WidescreenMode) String() string {
	switch m {
	case WidescreenOn:
		return "on"
	case WidescreenAuto:
		return "auto"
	default:
		return "off"
	}
}

// monitorPrefersWide reports whether the monitor aspect sits closer to 16:9
// than to 4:3, deciding the auto widescreen mode. Integer cross multiplied
// distances (16/9 vs 12/9 at a common 9*h denominator), so a 16:10 or wider
// panel goes wide while 4:3 and 3:2 panels keep the classic view.
func monitorPrefersWide(width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	distanceWide := absInt(9*width - 16*height)
	distanceClassic := absInt(9*width - 12*height)
	return distanceWide < distanceClassic
}

// absInt returns the absolute value of an int.
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// GNW mouse button flags (mouse.h). Press flags are set while the
// button is held; release flags only on the frame the button goes up.
const (
	MousePressLeft    = 0x01
	MousePressRight   = 0x02
	MouseReleaseLeft  = 0x10
	MouseReleaseRight = 0x20
)

// MouseState is the polled mouse snapshot for the current frame, in
// logical 640x480 coordinates.
type MouseState struct {
	X, Y    int
	Buttons uint32
	/* // note: unused code in Go port
	// WheelX / WheelY were written every frame and never read; wheel input reaches the game through ConsumeWheelY.
	WheelX  int
	WheelY  int
	*/
}

// Config configures the platform window.
type Config struct {
	Title          string
	WindowScale    int
	Fullscreen     bool
	Brightness     int            // Startup mode 1..5; 0 selects normal.
	Widescreen     WidescreenMode // Forced or monitor-selected 854x480 game view.
	BaseDir        string         // Directory containing the binary.
	ScreenshotPath string         // One-shot -screenshot output path.
}

// Platform implements ebiten.Game. The frame hook runs game logic on the same
// goroutine as Update.
type Platform struct {
	cfg Config

	mu     sync.Mutex
	screen *gfx.Buffer
	lut    [256][4]uint8 // display RGBA, brightness applied
	rawLUT [256][3]uint8 // display RGB before brightness, so F10 can rebuild
	bright int           // brightness mode 1..5 (3 = normal); see brightnessFactors
	rgba   []uint8

	logicalW int  // Current logical width: 640 for menus or 854 for wide gameplay.
	logicalH int  // Current logical height.
	wide     bool // Resolved widescreen choice.

	inputs   []int
	keysDown []ebiten.Key

	mouse       MouseState
	prevLeft    bool
	prevRight   bool
	prevClosing bool // Previous window-close state for edge detection.

	wheelAccum     int     // Wheel delta drained by ConsumeWheelY.
	wheelRemainder float64 // Fraction of a notch carried to the next poll (trackpads).

	frameHook func(*Platform) error
	overlay   func(*ebiten.Image) // Drawn over the frame.

	frame     int
	shotTaken bool
	shotErr   error // Why the one-shot -screenshot could not be written; nil when it was.
	quit      bool
	fpsShow   bool // Whether the FPS overlay is visible.

	OnBrightnessChange func(mode int) // Persists an F10 brightness change.
}

// shotFrame is the frame -screenshot captures: late enough for the
// logic goroutine to have drawn its first screen.
const shotFrame = 20

// New creates the platform with a black screen and a palette that is
// entirely black.
func New(cfg Config) *Platform {
	if cfg.WindowScale < 1 {
		cfg.WindowScale = 1
	}
	bright := cfg.Brightness
	if bright < 1 || bright >= len(brightnessFactors) {
		bright = brightnessNormal
	}
	p := &Platform{
		cfg:      cfg,
		screen:   gfx.New(ScreenWidth, ScreenHeight),
		rgba:     make([]uint8, ScreenWidth*ScreenHeight*4),
		bright:   bright,
		logicalW: ScreenWidth,
		logicalH: ScreenHeight,
		// The auto mode resolves against the monitor in Run; until then the
		// forced value stands (auto reads false, matching the 4:3 startup).
		wide: cfg.Widescreen == WidescreenOn,
	}
	for i := range p.lut {
		p.lut[i][3] = 0xFF
	}
	return p
}

// brightnessFactors is the RGB multiplier for each F10 brightness mode
// (1 very dark, 2 dark, 3 normal, 4 bright, 5 very bright); index 0 is unused.
//
// C++: the brightness control in game (PROJECT.txt F10).
var brightnessFactors = [6]float64{0, 0.55, 0.75, 1.0, 1.3, 1.6}

// brightnessLabels names each mode for the debug log; index 0 is unused.
var brightnessLabels = [6]string{0: "", 1: "very dark", 2: "dark", 3: "normal", 4: "bright", 5: "very bright"}

const brightnessNormal = 3

// scaleBright multiplies an 8-bit component by f and clamps to 255.
func scaleBright(c uint8, f float64) uint8 {
	v := float64(c) * f
	if v > 255 {
		v = 255
	}
	return uint8(v)
}

// rebuildLUT recomputes the display LUT from the raw palette at the current
// brightness mode. The caller holds p.mu.
func (p *Platform) rebuildLUT() {
	f := brightnessFactors[p.bright]
	for i := range p.rawLUT {
		p.lut[i] = [4]uint8{
			scaleBright(p.rawLUT[i][0], f),
			scaleBright(p.rawLUT[i][1], f),
			scaleBright(p.rawLUT[i][2], f),
			0xFF,
		}
	}
}

// cycleBrightness advances the F10 brightness mode (1->2->3->4->1) and
// rebuilds the display LUT so the change applies to the whole palette.
func (p *Platform) cycleBrightness() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bright++
	if p.bright > 5 {
		p.bright = 1
	}
	p.rebuildLUT()
	return p.bright
}

// Run opens the window and drives the game loop until the frame hook
// returns ebiten.Termination or the window is closed.
func (p *Platform) Run() error {
	ebiten.SetWindowTitle(p.cfg.Title)
	// Install the embedded icon or its procedural fallback.
	ebiten.SetWindowIcon(windowIcons())
	// Resolve automatic widescreen from the monitor. The 4:3 menus pillarbox inside a wide window.
	mw, mh := ebiten.Monitor().Size()
	if p.cfg.Widescreen == WidescreenAuto {
		p.wide = monitorPrefersWide(mw, mh)
	}
	windowWidth := ScreenWidth
	if p.wide {
		windowWidth = WideWidth
	}
	ebiten.SetWindowSize(windowWidth*p.cfg.WindowScale, ScreenHeight*p.cfg.WindowScale)
	ebiten.SetFullscreen(p.cfg.Fullscreen)
	dbg.Printf("display: logical %dx%d, window %dx%d (scale %d), fullscreen=%v, widescreen=%s (resolved %v), monitor %dx%d", p.logicalW, p.logicalH, windowWidth*p.cfg.WindowScale, ScreenHeight*p.cfg.WindowScale, p.cfg.WindowScale, p.cfg.Fullscreen, p.cfg.Widescreen, p.wide, mw, mh)
	ebiten.SetScreenClearedEveryFrame(false)
	// Handle the window 'X' (close) ourselves: route it to the active screen as an
	// ESC (in game opens the "OK to exit?" confirm; menus back out, the main menu
	// quits) instead of letting Ebitengine kill the process outright. See Update.
	ebiten.SetWindowClosingHandled(true)
	// GNW draws its own software cursor; the OS cursor stays hidden
	// over the game window.
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	return ebiten.RunGame(p)
}

// Screen returns the 8-bit indexed screen buffer the game draws into.
func (p *Platform) Screen() *gfx.Buffer {
	return p.screen
}

// Widescreen reports whether the wide 16:9 game view is active for this run:
// the forced setting, or the monitor resolved choice under auto.
func (p *Platform) Widescreen() bool {
	return p.wide
}

// SetLogicalSize switches between the 640x480 menu and 854x480 game resolutions. It preserves the buffer
// pointer, replaces its pixels, and does nothing
// when the requested size is already active.
func (p *Platform) SetLogicalSize(width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if width == p.logicalW && height == p.logicalH {
		return
	}
	p.logicalW = width
	p.logicalH = height
	// Keep the buffer pointer shared by GNW and the active screens.
	p.screen.W = width
	p.screen.H = height
	p.screen.Stride = width
	p.screen.Pix = make([]uint8, width*height)
	p.rgba = make([]uint8, width*height*4)
	dbg.Printf("logical resolution: %dx%d", width, height)
}

// BaseDir returns the directory next to the binary (where saves/ and
// screenshots/ folders are created).
func (p *Platform) BaseDir() string {
	return p.cfg.BaseDir
}

// SetFrameHook installs a function called once per Update tick.
func (p *Platform) SetFrameHook(hook func(*Platform) error) {
	p.frameHook = hook
}

// SetOverlay installs a function drawn over the finished frame each Draw.
// The data bootstrap uses it to paint text and a progress bar before any
// game font exists. Pass nil to remove it.
func (p *Platform) SetOverlay(fn func(*ebiten.Image)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.overlay = fn
}

// SetPalette installs a 6-bit game palette as the display palette.
//
// C++: Svga_SetPalette (svga.cpp)
func (p *Platform) SetPalette(pal *gfx.Palette) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < gfx.PaletteSize; i++ {
		r, g, b := pal.RGBA8(i)
		p.rawLUT[i] = [3]uint8{r, g, b}
	}
	p.rebuildLUT()
}

// SetPaletteColor updates a single display palette entry (6-bit
// components). This is what palette cycling animations call.
//
// C++: Svga_SetPaletteColor via Color_SetSystemPaletteEntry
func (p *Platform) SetPaletteColor(i int, r, g, b uint8) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rawLUT[i] = [3]uint8{r * 4, g * 4, b * 4}
	f := brightnessFactors[p.bright]
	p.lut[i] = [4]uint8{scaleBright(r*4, f), scaleBright(g*4, f), scaleBright(b*4, f), 0xFF}
}

// GetInput returns the next queued GNW input code, or -1 when the
// queue is empty.
//
// C++: get_input (input.cpp)
func (p *Platform) GetInput() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.inputs) == 0 {
		return -1
	}
	code := p.inputs[0]
	p.inputs = p.inputs[1:]
	return code
}

// Mouse returns the mouse snapshot of the current frame (position and buttons).
// Scroll wheel input is read through ConsumeWheelY.
//
// C++: mouse_get_position / mouse_get_buttons (mouse.c)
func (p *Platform) Mouse() MouseState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mouse
}

// quantizeWheel adds a wheel delta to the carried remainder and returns the whole notches, so trackpad
// fractions add up instead of truncating to zero.
//
// C++ ported : mouse.c - mouse_quantize_wheel_axis
func quantizeWheel(delta float64, remainder *float64) int {
	*remainder += delta
	notches := int(*remainder)
	*remainder -= float64(notches)
	return notches
}

// ConsumeWheelY returns the accumulated vertical scroll wheel delta since the
// last call and resets the accumulator, so each notch is acted on exactly
// once regardless of the relative rates of the logic loop and the input poll.
func (p *Platform) ConsumeWheelY() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	v := p.wheelAccum
	p.wheelAccum = 0
	return v
}

// ShiftOrCtrlHeld reports whether either Shift or Control is currently held. The
// in game screen uses it for the FORCE ATTACK modifier (fire on a friendly unit).
func (p *Platform) ShiftOrCtrlHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyControl)
}

// ShiftHeld reports whether Shift alone is held. The in game screen uses it for
// the additive multi select modifier (shift click a friendly unit to add or drop
// it from the group), keeping Control free for the force attack on a friendly.
func (p *Platform) ShiftHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShift)
}

// Update implements ebiten.Game.
func (p *Platform) Update() error {
	if p.quit {
		return ebiten.Termination
	}

	p.pollKeyboard()
	p.pollMouse()
	p.pollWindowClose()

	if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		path, err := p.SaveScreenshot()
		if err != nil {
			dbg.Printf("screenshot failed: %v", err)
		} else {
			dbg.Printf("screenshot saved: %s", path)
		}
	}

	// F3 toggles the FPS overlay, F10 cycles brightness, and F11 toggles fullscreen. These keys do not enter the
	// game input queue.
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		p.fpsShow = !p.fpsShow
		dbg.Printf("fps overlay: %v", p.fpsShow)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF10) && !ebiten.IsKeyPressed(ebiten.KeyShift) {
		mode := p.cycleBrightness()
		dbg.Printf("brightness mode: %d [%s]", mode, brightnessLabels[mode])
		if p.OnBrightnessChange != nil {
			p.OnBrightnessChange(mode)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		// Toggle the intended state directly because ebiten.IsFullscreen may lag immediately after a window operation.
		p.cfg.Fullscreen = !p.cfg.Fullscreen
		ebiten.SetFullscreen(p.cfg.Fullscreen)
		ww, wh := ebiten.WindowSize()
		mw, mh := ebiten.Monitor().Size()
		p.mu.Lock()
		logicalW, logicalH := p.logicalW, p.logicalH
		p.mu.Unlock()
		dbg.Printf("fullscreen: %v (window %dx%d, monitor %dx%d, logical %dx%d)", p.cfg.Fullscreen, ww, wh, mw, mh, logicalW, logicalH)
	}

	if p.frameHook != nil {
		if err := p.frameHook(p); err != nil {
			return err
		}
	}
	return nil
}

// pollKeyboard refreshes keyboard edge and held key state from Ebitengine.
func (p *Platform) pollKeyboard() {
	shift := ebiten.IsKeyPressed(ebiten.KeyShift)
	alt := ebiten.IsKeyPressed(ebiten.KeyAlt)
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl)
	p.keysDown = inpututil.AppendJustPressedKeys(p.keysDown[:0])

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range p.keysDown {
		if k == ebiten.KeyF3 || k == ebiten.KeyF11 || k == ebiten.KeyF12 || (k == ebiten.KeyF10 && !shift) {
			continue // reserved for fps / fullscreen / screenshots / brightness
		}
		if code, ok := translateKeyWithCtrl(k, shift, alt, ctrl); ok {
			p.inputs = append(p.inputs, code)
		}
	}
}

// pollWindowClose converts each window-close edge into one Escape input, allowing the active screen to
// confirm, back out, or quit normally.
func (p *Platform) pollWindowClose() {
	closing := ebiten.IsWindowBeingClosed()
	if closing && !p.prevClosing {
		p.mu.Lock()
		p.inputs = append(p.inputs, KeyEscape)
		p.mu.Unlock()
	}
	p.prevClosing = closing
}

// pollMouse refreshes cursor position and button edge state from Ebitengine.
func (p *Platform) pollMouse() {
	x, y := ebiten.CursorPosition()
	left := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	right := ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	_, wy := ebiten.Wheel() /* wx: note: unused code in Go port, the horizontal wheel only fed MouseState.WheelX */

	var buttons uint32
	if left {
		buttons |= MousePressLeft
	}
	if right {
		buttons |= MousePressRight
	}
	if p.prevLeft && !left {
		buttons |= MouseReleaseLeft
	}
	if p.prevRight && !right {
		buttons |= MouseReleaseRight
	}
	p.prevLeft = left
	p.prevRight = right

	p.mu.Lock()
	p.wheelAccum += quantizeWheel(wy, &p.wheelRemainder)
	p.mouse = MouseState{X: x, Y: y, Buttons: buttons /* , WheelX: int(wx), WheelY: int(wy): note: unused code in Go port, see MouseState */}
	p.mu.Unlock()
}

// Draw implements ebiten.Game: one conversion from indexed to RGBA of the
// whole logical screen per frame.
//
// C++: Svga_Blit (svga.cpp), simplified to converting the full frame
func (p *Platform) Draw(screen *ebiten.Image) {
	p.mu.Lock()
	blitRGBA(p.screen, &p.lut, p.rgba)
	rgba := p.rgba
	overlay := p.overlay
	p.mu.Unlock()

	// On the frame a logical resolution switch lands, the ebiten screen can
	// still have the previous Layout size; WritePixels would panic on the
	// length mismatch, so skip that one frame (the next Layout catches up).
	bounds := screen.Bounds()
	if len(rgba) == bounds.Dx()*bounds.Dy()*4 {
		screen.WritePixels(rgba)
	}

	if p.fpsShow {
		// Top right corner, small debug font, ~5px inset. The debug font advances
		// 6px per glyph, so align it to the right by the text length.
		txt := fmt.Sprintf("FPS %.0f", ebiten.ActualFPS())
		ebitenutil.DebugPrintAt(screen, txt, bounds.Dx()-len(txt)*6-5, 5)
	}

	if overlay != nil {
		overlay(screen)
	}

	p.frame++
	p.takeLaunchScreenshot()
}

// takeLaunchScreenshot writes the one-shot -screenshot file once the frame count reaches shotFrame and asks the
// run to quit. A write failure is kept for ScreenshotErr.
func (p *Platform) takeLaunchScreenshot() {
	if p.cfg.ScreenshotPath == "" || p.shotTaken || p.frame < shotFrame {
		return
	}

	p.shotTaken = true
	if err := p.WriteScreenshot(p.cfg.ScreenshotPath); err != nil {
		p.shotErr = err
		dbg.Printf("-screenshot failed: %v", err)
	} else {
		dbg.Printf("-screenshot saved: %s", p.cfg.ScreenshotPath)
	}
	p.quit = true
}

// Layout implements ebiten.Game: the current logical resolution, Ebitengine
// scales (and letterboxes) it to the window and remaps the mouse to match.
func (p *Platform) Layout(outsideWidth, outsideHeight int) (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.logicalW, p.logicalH
}

// blitRGBA expands an indexed buffer through the palette LUT into a
// RGBA byte slice of len w*h*4.
func blitRGBA(src *gfx.Buffer, lut *[256][4]uint8, dst []uint8) {
	di := 0
	for y := 0; y < src.H; y++ {
		row := src.Row(y)
		for _, c := range row {
			copy(dst[di:di+4], lut[c][:])
			di += 4
		}
	}
}

/* // note: unused code in Go port
// String was a diagnostic Stringer; nothing calls it and no *Platform is ever passed to fmt.
// String returns a compact diagnostic description of the platform state.
func (p *Platform) String() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return fmt.Sprintf("platform %dx%d scale %d", p.logicalW, p.logicalH, p.cfg.WindowScale)
}
*/
