// keys.go : the M.A.X. keyboard key constants - maps Ebitengine key codes to GNW key values used throughout the game.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : input.h - GNW key code definitions
package platform

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// GNW keyboard codes from input.h. Printable keys use ASCII, while special keys use DOS-style values at or above 256.
const (
	KeyBackspace = 8
	KeyTab       = 9
	KeyReturn    = 13
	KeyEscape    = 27
	KeySpace     = 32

	KeyF1  = 315
	KeyF2  = 316
	KeyF3  = 317
	KeyF4  = 318
	KeyF5  = 319
	KeyF6  = 320
	KeyF7  = 321
	KeyF8  = 322
	KeyF9  = 323
	KeyF10 = 324

	KeyHome     = 327
	KeyUp       = 328
	KeyPageUp   = 329
	KeyLeft     = 331
	KeyRight    = 333
	KeyEnd      = 335
	KeyDown     = 336
	KeyPageDown = 337
	KeyInsert   = 338
	KeyDelete   = 339

	KeyF11 = 389
	KeyF12 = 390

	KeyShiftF1  = 340
	KeyShiftF10 = 349
	KeyShiftF11 = 391
	KeyShiftF12 = 392

	// InputPress separates GNW button events from keyboard codes.
	InputPress = 0x7000

	// AltMod distinguishes Alt combinations without overlapping special keys or InputPress. C++: GNW_KB_KEY_LALT_*.
	AltMod = 0x800

	// CtrlMod marks a Ctrl+letter combination. It sits above the menu button
	// codes (1000-1099) and below InputPress, so it overlaps neither.
	CtrlMod = 0x1000
)

// specialKeys maps Ebitengine keys to GNW codes that do not depend on
// the shift state.
//
// C++: input_key_map (input.cpp)
var specialKeys = map[ebiten.Key]int{
	ebiten.KeyBackspace:   KeyBackspace,
	ebiten.KeyTab:         KeyTab,
	ebiten.KeyEnter:       KeyReturn,
	ebiten.KeyNumpadEnter: KeyReturn,
	ebiten.KeyEscape:      KeyEscape,
	ebiten.KeySpace:       KeySpace,
	ebiten.KeyF1:          KeyF1,
	ebiten.KeyF2:          KeyF2,
	ebiten.KeyF3:          KeyF3,
	ebiten.KeyF4:          KeyF4,
	ebiten.KeyF5:          KeyF5,
	ebiten.KeyF6:          KeyF6,
	ebiten.KeyF7:          KeyF7,
	ebiten.KeyF8:          KeyF8,
	ebiten.KeyF9:          KeyF9,
	ebiten.KeyF10:         KeyF10,
	ebiten.KeyF11:         KeyF11,
	ebiten.KeyF12:         KeyF12,
	ebiten.KeyHome:        KeyHome,
	ebiten.KeyArrowUp:     KeyUp,
	ebiten.KeyPageUp:      KeyPageUp,
	ebiten.KeyArrowLeft:   KeyLeft,
	ebiten.KeyArrowRight:  KeyRight,
	ebiten.KeyEnd:         KeyEnd,
	ebiten.KeyArrowDown:   KeyDown,
	ebiten.KeyPageDown:    KeyPageDown,
	ebiten.KeyInsert:      KeyInsert,
	ebiten.KeyDelete:      KeyDelete,
}

// asciiKeys maps Ebitengine keys to their unshifted ASCII character.
var asciiKeys = map[ebiten.Key]byte{
	ebiten.KeyA: 'a', ebiten.KeyB: 'b', ebiten.KeyC: 'c', ebiten.KeyD: 'd',
	ebiten.KeyE: 'e', ebiten.KeyF: 'f', ebiten.KeyG: 'g', ebiten.KeyH: 'h',
	ebiten.KeyI: 'i', ebiten.KeyJ: 'j', ebiten.KeyK: 'k', ebiten.KeyL: 'l',
	ebiten.KeyM: 'm', ebiten.KeyN: 'n', ebiten.KeyO: 'o', ebiten.KeyP: 'p',
	ebiten.KeyQ: 'q', ebiten.KeyR: 'r', ebiten.KeyS: 's', ebiten.KeyT: 't',
	ebiten.KeyU: 'u', ebiten.KeyV: 'v', ebiten.KeyW: 'w', ebiten.KeyX: 'x',
	ebiten.KeyY: 'y', ebiten.KeyZ: 'z',
	ebiten.KeyDigit0: '0', ebiten.KeyDigit1: '1', ebiten.KeyDigit2: '2',
	ebiten.KeyDigit3: '3', ebiten.KeyDigit4: '4', ebiten.KeyDigit5: '5',
	ebiten.KeyDigit6: '6', ebiten.KeyDigit7: '7', ebiten.KeyDigit8: '8',
	ebiten.KeyDigit9: '9',
	ebiten.KeyMinus:  '-', ebiten.KeyEqual: '=',
	ebiten.KeyBracketLeft: '[', ebiten.KeyBracketRight: ']',
	ebiten.KeySemicolon: ';', ebiten.KeyQuote: '\'', ebiten.KeyBackquote: '`',
	ebiten.KeyBackslash: '\\', ebiten.KeyComma: ',', ebiten.KeyPeriod: '.',
	ebiten.KeySlash:   '/',
	ebiten.KeyNumpad0: '0', ebiten.KeyNumpad1: '1', ebiten.KeyNumpad2: '2',
	ebiten.KeyNumpad3: '3', ebiten.KeyNumpad4: '4', ebiten.KeyNumpad5: '5',
	ebiten.KeyNumpad6: '6', ebiten.KeyNumpad7: '7', ebiten.KeyNumpad8: '8',
	ebiten.KeyNumpad9:   '9',
	ebiten.KeyNumpadAdd: '+', ebiten.KeyNumpadSubtract: '-',
	ebiten.KeyNumpadMultiply: '*', ebiten.KeyNumpadDivide: '/',
}

// shiftedASCII translates an unshifted ASCII character to its shifted
// counterpart on a US layout, matching the GNW_KB_KEY_SHIFT_ values.
var shiftedASCII = map[byte]byte{
	'1': '!', '2': '@', '3': '#', '4': '$', '5': '%', '6': '^',
	'7': '&', '8': '*', '9': '(', '0': ')',
	'-': '_', '=': '+', '[': '{', ']': '}', ';': ':', '\'': '"',
	'`': '~', '\\': '|', ',': '<', '.': '>', '/': '?',
}

// keyNames maps a lowercased keybind value from the settings file to its GNW
// code. Single characters are handled by KeyFromName itself.
var keyNames = map[string]int{
	"enter": KeyReturn, "return": KeyReturn,
	"escape": KeyEscape, "esc": KeyEscape,
	"space": KeySpace, "tab": KeyTab, "backspace": KeyBackspace,
	"f1": KeyF1, "f2": KeyF2, "f3": KeyF3, "f4": KeyF4, "f5": KeyF5,
	"f6": KeyF6, "f7": KeyF7, "f8": KeyF8, "f9": KeyF9, "f10": KeyF10,
	"f11": KeyF11, "f12": KeyF12,
	"pageup": KeyPageUp, "pagedown": KeyPageDown,
	"home": KeyHome, "end": KeyEnd, "insert": KeyInsert, "delete": KeyDelete,
	"up": KeyUp, "down": KeyDown, "left": KeyLeft, "right": KeyRight,
}

// KeyFromName resolves a case-insensitive key name or one printable ASCII character. Letters use their lowercase, unshifted GNW code.
// A "ctrl+" or "alt+" prefix before a letter gives that combination ("Ctrl+S"); Alt also takes a digit.
func KeyFromName(name string) (code int, ok bool) {
	name = strings.TrimSpace(name)
	lower := strings.ToLower(name)

	for _, mod := range []struct {
		prefix string
		bit    int
		digits bool
	}{{"ctrl+", CtrlMod, false}, {"alt+", AltMod, true}} {
		rest, found := strings.CutPrefix(lower, mod.prefix)

		if !found {
			continue
		}

		if len(rest) == 1 && ((rest[0] >= 'a' && rest[0] <= 'z') || (mod.digits && rest[0] >= '0' && rest[0] <= '9')) {
			return mod.bit | int(rest[0]), true
		}

		return 0, false
	}

	if n, ok := keyNames[lower]; ok {
		return n, true
	}

	if len(name) == 1 {
		c := name[0]

		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}

		if c > ' ' && c < 127 {
			return int(c), true
		}
	}

	return 0, false
}

// translateKeyWithCtrl is translateKey plus Ctrl: with Ctrl held and Alt not, a letter becomes its Ctrl combo
// code (CtrlMod | lowercase ascii). Every other key, and any key with Alt held, goes through translateKey.
func translateKeyWithCtrl(k ebiten.Key, shift, alt, ctrl bool) (code int, ok bool) {
	if ctrl && !alt {
		if c, found := asciiKeys[k]; found && c >= 'a' && c <= 'z' {
			return CtrlMod | int(c), true
		}
	}

	return translateKey(k, shift, alt)
}

// translateKey converts an Ebitengine key event to a GNW input code,
// returning ok=false for keys GNW has no code for (modifiers etc.). With alt
// held, a letter or digit becomes its Alt combo code (AltMod | ascii), taking
// precedence over shift (Alt+S, Alt+1...).
func translateKey(k ebiten.Key, shift, alt bool) (code int, ok bool) {
	if code, ok := specialKeys[k]; ok {
		if shift {
			switch {
			case code >= KeyF1 && code <= KeyF10:
				return code - KeyF1 + KeyShiftF1, true
			case code == KeyF11:
				return KeyShiftF11, true
			case code == KeyF12:
				return KeyShiftF12, true
			}
		}
		// Function keys retain AltMod, while navigation keys keep their plain code so Alt+scroll continues to work. C++: GNW_KB_KEY_LALT_F*.
		if alt && ((code >= KeyF1 && code <= KeyF10) || code == KeyF11 || code == KeyF12) {
			return AltMod | code, true
		}
		return code, true
	}
	c, ok := asciiKeys[k]
	if !ok {
		return 0, false
	}
	if alt && ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
		return AltMod | int(c), true // Alt+letter / Alt+digit (unshifted)
	}
	if shift {
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		} else if s, ok := shiftedASCII[c]; ok {
			c = s
		}
	}
	return int(c), true
}
