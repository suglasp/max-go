// preset.go : lets max-settings choose from fixed values with the Left and Right arrow keys.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/suglasp/max-go/internal/settings"
)

// presetKey is one decoded keypress inside the raw terminal adjust loop.
type presetKey int

const (
	presetKeyLeft presetKey = iota
	presetKeyRight
	presetKeyAccept
	presetKeyCancel
)

// adjustPreset lets the user choose one of a field's fixed values. A terminal
// uses arrow keys, while redirected input accepts typed steps or a literal
// value. It reports whether the setting changed.
func adjustPreset(reader *bufio.Reader, in *os.File, out *os.File, cfg *settings.Settings, f field) bool {
	idx := presetIndex(f.presets, f.get(*cfg))
	start := idx

	if term.IsTerminal(int(in.Fd())) {
		idx = adjustPresetRaw(reader, in, out, f, idx)
	} else {
		idx = adjustPresetLine(reader, out, f, idx)
	}

	if idx < 0 || idx == start {
		return false
	}
	// Preset values always parse, so the setter error path never triggers.
	_ = f.set(cfg, f.presets[idx])
	return true
}

// adjustPresetRaw runs the arrow key loop in raw terminal mode. It returns the
// accepted index, or -1 when the user cancels or raw input fails.
func adjustPresetRaw(reader *bufio.Reader, in *os.File, out *os.File, f field, idx int) int {
	oldState, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return adjustPresetLine(reader, out, f, idx)
	}
	defer term.Restore(int(in.Fd()), oldState)

	for {
		// Redraw in place and pad over a longer previous value.
		fmt.Fprintf(out, "\r%s : <%-10s>  Left/Right = adjust, Enter = accept, Esc = cancel ", f.name, f.presets[idx])
		key, ok := readPresetKey(reader)
		if !ok {
			fmt.Fprint(out, "\r\n")
			return -1
		}
		switch key {
		case presetKeyLeft:
			idx = (idx + len(f.presets) - 1) % len(f.presets)
		case presetKeyRight:
			idx = (idx + 1) % len(f.presets)
		case presetKeyAccept:
			fmt.Fprint(out, "\r\n")
			return idx
		case presetKeyCancel:
			fmt.Fprint(out, "\r\n")
			return -1
		}
	}
}

// readPresetKey decodes VT arrow sequences, Enter, Esc, and the letter
// fallbacks from the raw input stream.
func readPresetKey(reader *bufio.Reader) (presetKey, bool) {
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, false
		}
		switch b {
		case 0x1b: // an arrow key's ESC [ C/D sequence, or a bare Esc = cancel
			if reader.Buffered() == 0 {
				return presetKeyCancel, true
			}
			next, err := reader.ReadByte()
			if err != nil {
				return 0, false
			}
			if next != '[' {
				return presetKeyCancel, true
			}
			final, err := reader.ReadByte()
			if err != nil {
				return 0, false
			}
			switch final {
			case 'C':
				return presetKeyRight, true
			case 'D':
				return presetKeyLeft, true
			}
			// Up/Down and other sequences are ignored.
		case '\r', '\n':
			return presetKeyAccept, true
		case 'a', 'A', '<':
			return presetKeyLeft, true
		case 'd', 'D', '>':
			return presetKeyRight, true
		case 'q', 'Q', 0x03: // q or Ctrl+C
			return presetKeyCancel, true
		}
	}
}

// adjustPresetLine handles redirected input one typed step per line. It returns
// the accepted index or -1 when the user cancels.
func adjustPresetLine(reader *bufio.Reader, out *os.File, f field, idx int) int {
	for {
		fmt.Fprintf(out, "%s : <%s>  (< or > = adjust, Enter = accept, q = cancel, or type a value): ", f.name, f.presets[idx])
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return -1
		}
		switch v := strings.TrimSpace(line); strings.ToLower(v) {
		case "":
			return idx
		case "<", "a":
			idx = (idx + len(f.presets) - 1) % len(f.presets)
		case ">", "d":
			idx = (idx + 1) % len(f.presets)
		case "q":
			return -1
		default:
			// A literal preset value picks and accepts it directly.
			if i := presetIndex(f.presets, v); i >= 0 && strings.EqualFold(f.presets[i], v) {
				return i
			}
			fmt.Fprintf(out, "expected one of: %s\n", strings.Join(f.presets, ", "))
		}
	}
}

// presetIndex finds a value without regard to case. Unknown values map to the
// first preset.
func presetIndex(presets []string, value string) int {
	for i, p := range presets {
		if strings.EqualFold(p, value) {
			return i
		}
	}
	return 0
}
