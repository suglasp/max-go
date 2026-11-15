// main_test.go : checks that max-settings exposes every configurable field.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/settings"
)

// settingsLeafNames returns the JSON path of every scalar leaf in Settings.
// Keybind rows come from the loaded map rather than the struct shape, so this
// walk skips them.
func settingsLeafNames() []string {
	var out []string

	var walk func(prefix string, rt reflect.Type)
	walk = func(prefix string, rt reflect.Type) {
		for i := 0; i < rt.NumField(); i++ {
			sf := rt.Field(i)
			tag := sf.Tag.Get("json")
			if tag == "" || tag == "-" {
				continue // unexported / untagged, e.g. the -datadir override
			}

			name := strings.Split(tag, ",")[0]
			if prefix != "" {
				name = prefix + "." + name
			}

			switch sf.Type.Kind() {
			case reflect.Struct:
				walk(name, sf.Type)
			case reflect.Map:
				// The loaded file supplies keybind rows dynamically.
			default:
				out = append(out, name)
			}
		}
	}

	walk("", reflect.TypeOf(settings.Settings{}))

	return out
}

// TestSettingFieldsCoverAllSettings requires an editor row for every setting.
func TestSettingFieldsCoverAllSettings(t *testing.T) {
	have := map[string]bool{}
	for _, f := range settingFields(settings.Default()) {
		have[f.name] = true
	}

	for _, want := range settingsLeafNames() {
		if !have[want] {
			t.Errorf("setting %q has no max-settings field row", want)
		}
	}
}

// runToolCapture runs the tool with piped input and returns the exit code and what it wrote to stderr.
func runToolCapture(t *testing.T, input string, args ...string) (int, string) {
	t.Helper()
	in, out := pipeInput(t, input)
	var errOut bytes.Buffer
	code := runTool(args, in, out, &errOut)
	return code, errOut.String()
}

// scratchFile returns a settings path inside the test folder.
func scratchFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "max-go.json")
}

// Every bad command line names the problem with the tool prefix and exits 2.
func TestRunToolCommandLineErrors(t *testing.T) {
	file := scratchFile(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"-bogus"}, "max-settings: flag provided but not defined: -bogus"},
		{"extra argument", []string{"-file", file, "extra"}, `max-settings: unexpected argument "extra"`},
		{"empty file", []string{"-file="}, "max-settings: flag -file needs a value"},
		{"blank file", []string{"-file", "  "}, "max-settings: flag -file needs a value"},
	}
	for _, tc := range cases {
		code, stderr := runToolCapture(t, "q\n", tc.args...)
		if code != 2 {
			t.Errorf("%s: exit %d, want 2", tc.name, code)
		}
		if !strings.HasPrefix(stderr, tc.want) {
			t.Errorf("%s: stderr %q does not start with %q", tc.name, stderr, tc.want)
		}
	}
	if _, err := os.Stat(file); err == nil {
		t.Errorf("a refused command line created %s", file)
	}
}

// -h prints the usage and exits 0.
func TestRunToolHelpExitsZero(t *testing.T) {
	code, stderr := runToolCapture(t, "", "-h")
	if code != 0 || !strings.Contains(stderr, "usage:") {
		t.Fatalf("exit %d, stderr %q, want 0 and the usage", code, stderr)
	}
}

// A missing file starts from the defaults and quitting writes nothing.
func TestRunToolMissingFileIsNotWritten(t *testing.T) {
	file := scratchFile(t)
	code, stderr := runToolCapture(t, "q\n", "-file", file)
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, stderr)
	}
	if !strings.Contains(stderr, "does not exist yet") {
		t.Errorf("stderr %q does not say the file is new", stderr)
	}
	if _, err := os.Stat(file); err == nil {
		t.Errorf("quitting created %s", file)
	}
}

// Save writes the file, so a new one is created only on s.
func TestRunToolSaveCreatesFile(t *testing.T) {
	file := scratchFile(t)
	if code, stderr := runToolCapture(t, "s\n\nq\n", "-file", file); code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, stderr)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("save did not create the file: %v", err)
	}
}

// Piped input that ends without a quit command ends cleanly.
func TestRunToolEndOfInputIsClean(t *testing.T) {
	file := scratchFile(t)
	for _, input := range []string{"", "1\n"} {
		code, stderr := runToolCapture(t, input, "-file", file)
		if code != 0 || strings.Contains(stderr, "EOF") {
			t.Errorf("input %q: exit %d, stderr %q, want 0 and no EOF", input, code, stderr)
		}
	}
}

// A repaired value is listed, leaves the file alone, and makes the session modified.
func TestRunToolListsRepairsAndKeepsFile(t *testing.T) {
	file := scratchFile(t)
	body := []byte(`{"video": {"window_scale": 99}, "player": {"clan": 42}}`)
	if err := os.WriteFile(file, body, 0o644); err != nil {
		t.Fatal(err)
	}
	// The session starts modified, so quit asks before it discards.
	code, stderr := runToolCapture(t, "q\ny\n", "-file", file)
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, stderr)
	}
	for _, want := range []string{
		"max-settings: repaired on load: video.window_scale was 99, now 2",
		"max-settings: repaired on load: player.clan was 42, now 7",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q lacks %q", stderr, want)
		}
	}
	after, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(after, body) {
		t.Errorf("the settings file changed: %q (%v)", after, err)
	}
}

// A file that is not JSON is a runtime failure.
func TestRunToolBadJSONExitsOne(t *testing.T) {
	file := scratchFile(t)
	if err := os.WriteFile(file, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stderr := runToolCapture(t, "q\n", "-file", file)
	if code != 1 || !strings.HasPrefix(stderr, "max-settings: ") {
		t.Fatalf("exit %d, stderr %q, want 1 and the tool prefix", code, stderr)
	}
}

// A typed integer outside its range is refused with the range, and a valid one is taken.
func TestIntFieldRefusesOutOfRange(t *testing.T) {
	fieldByName := func(name string) field {
		for _, f := range settingFields(settings.Default()) {
			if f.name == name {
				return f
			}
		}
		t.Fatalf("no field %q", name)
		return field{}
	}
	cases := []struct {
		name  string
		value string
		want  string // "" means accepted
	}{
		{"video.window_scale", "99", "video.window_scale expects a number from 1 to 8, got 99"},
		{"video.window_scale", "0", "video.window_scale expects a number from 1 to 8, got 0"},
		{"video.window_scale", "4", ""},
		{"audio.music_volume", "150", "audio.music_volume expects a number from 0 to 100, got 150"},
		{"player.clan", "9", "player.clan expects a number from 0 to 8, got 9"},
		{"preferences.quick_scroll", "3", "preferences.quick_scroll expects a number from 4 to 128, got 3"},
		{"game.opponent", "6", "game.opponent expects a number from 0 to 5, got 6"},
		{"game.opponent", "5", ""},
		{"video.window_scale", "abc", "video.window_scale expects an integer"},
	}
	for _, tc := range cases {
		cfg := settings.Default()
		err := fieldByName(tc.name).set(&cfg, tc.value)
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s=%s refused: %v", tc.name, tc.value, err)
		case tc.want != "" && (err == nil || err.Error() != tc.want):
			t.Errorf("%s=%s: error %v, want %q", tc.name, tc.value, err, tc.want)
		}
	}
}

// Every ranged setting has an editor row, so the typed check covers the table.
func TestEveryIntRangeHasAField(t *testing.T) {
	have := map[string]bool{}
	for _, f := range settingFields(settings.Default()) {
		have[f.name] = true
	}
	for _, r := range settings.IntRanges {
		if !have[r.Field] {
			t.Errorf("range %q has no max-settings field row", r.Field)
		}
	}
}

// After a repair the session counts as modified: quit asks first, and a save writes the repaired values.
func TestRunToolRepairedSessionIsModified(t *testing.T) {
	file := scratchFile(t)
	if err := os.WriteFile(file, []byte(`{"video": {"window_scale": 99}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// q asks, n keeps editing, s saves (and waits), the second q finds the session clean.
	if code, stderr := runToolCapture(t, "q\nn\ns\n\nq\n", "-file", file); code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, stderr)
	}
	cfg, repairs, err := settings.LoadReport(file)
	if err != nil || len(repairs) != 0 || cfg.Video.WindowScale != 2 {
		t.Fatalf("saved file: scale %d, repairs %v, err %v; want the repaired value", cfg.Video.WindowScale, repairs, err)
	}
}
