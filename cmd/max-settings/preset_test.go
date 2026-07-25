// preset_test.go : tests the max-settings preset adjuster.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/settings"
)

// widescreenField pulls the video.widescreen preset field from the live table.
func widescreenField(t *testing.T, cfg settings.Settings) field {
	t.Helper()
	for _, f := range settingFields(cfg) {
		if f.name == "video.widescreen" {
			if len(f.presets) == 0 {
				t.Fatal("video.widescreen is not a preset field")
			}
			return f
		}
	}
	t.Fatal("video.widescreen field not found")
	return field{}
}

// pipeInput returns redirected input and a discard output file so adjustPreset
// exercises its line based fallback.
func pipeInput(t *testing.T, input string) (*os.File, *os.File) {
	t.Helper()
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		writeEnd.WriteString(input)
		writeEnd.Close()
	}()
	out, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { readEnd.Close(); out.Close() })
	return readEnd, out
}

func TestAdjustPresetStepsRight(t *testing.T) {
	cfg := settings.Default() // widescreen = auto
	f := widescreenField(t, cfg)

	// ">" walks auto -> true, the empty line accepts.
	in, out := pipeInput(t, ">\n\n")
	if !adjustPreset(bufio.NewReader(in), in, out, &cfg, f) {
		t.Fatal("adjustPreset reported no change")
	}
	if cfg.Video.Widescreen != settings.WidescreenOn {
		t.Fatalf("widescreen = %q, want true", cfg.Video.Widescreen)
	}
}

func TestAdjustPresetWrapsLeft(t *testing.T) {
	cfg := settings.Default() // auto = first preset
	f := widescreenField(t, cfg)

	// "<" wraps auto -> false (the list end), then accept.
	in, out := pipeInput(t, "<\n\n")
	if !adjustPreset(bufio.NewReader(in), in, out, &cfg, f) {
		t.Fatal("adjustPreset reported no change")
	}
	if cfg.Video.Widescreen != settings.WidescreenOff {
		t.Fatalf("widescreen = %q, want false", cfg.Video.Widescreen)
	}
}

func TestAdjustPresetCancelKeepsValue(t *testing.T) {
	cfg := settings.Default()
	f := widescreenField(t, cfg)

	in, out := pipeInput(t, ">\nq\n")
	if adjustPreset(bufio.NewReader(in), in, out, &cfg, f) {
		t.Fatal("cancel reported a change")
	}
	if cfg.Video.Widescreen != settings.WidescreenAuto {
		t.Fatalf("widescreen after cancel = %q, want auto", cfg.Video.Widescreen)
	}
}

func TestAdjustPresetLiteralValue(t *testing.T) {
	cfg := settings.Default()
	f := widescreenField(t, cfg)

	// Typing a literal preset picks and accepts it in one step.
	in, out := pipeInput(t, "false\n")
	if !adjustPreset(bufio.NewReader(in), in, out, &cfg, f) {
		t.Fatal("literal value reported no change")
	}
	if cfg.Video.Widescreen != settings.WidescreenOff {
		t.Fatalf("widescreen = %q, want false", cfg.Video.Widescreen)
	}
}

func TestReadPresetKeyDecodesArrows(t *testing.T) {
	// The raw loop's decoder: VT arrow sequences, Enter, Esc, letter steps.
	reader := bufio.NewReader(strings.NewReader("\x1b[C\x1b[Da d\r"))
	want := []presetKey{presetKeyRight, presetKeyLeft, presetKeyLeft, presetKeyRight, presetKeyAccept}
	for i, w := range want {
		got, ok := readPresetKey(reader)
		if !ok || got != w {
			t.Fatalf("key %d = %v, %v; want %v", i, got, ok, w)
		}
	}
}

func TestBoolFieldsArePresets(t *testing.T) {
	// Boolean settings use presets, while names and integers accept free text.
	for _, f := range settingFields(settings.Default()) {
		switch f.name {
		case "video.fullscreen", "audio.music_enabled", "debug.console":
			if len(f.presets) != 2 {
				t.Errorf("%s presets = %v, want true/false", f.name, f.presets)
			}
		case "player.name", "video.window_scale":
			if len(f.presets) != 0 {
				t.Errorf("%s unexpectedly has presets %v", f.name, f.presets)
			}
		}
	}
}
