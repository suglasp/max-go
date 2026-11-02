// settings_test.go : testing unit for settings.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadOrCreateWritesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	got, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Errorf("first LoadOrCreate = %+v, want defaults", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("settings file not created: %v", err)
	}
}

// TestSaveAppliesLaunchRestores: a registered launch restore edits only the copy
// being written, so the override never reaches the file while the running config
// keeps it; a value edited away from the override persists normally.
func TestSaveAppliesLaunchRestores(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.json")
	s := Default()
	s.Video.WindowScale = 2

	const override = 4
	pristine := s.Video.WindowScale
	s.Video.WindowScale = override
	s.RegisterLaunchRestore(func(out *Settings) {
		if out.Video.WindowScale == override {
			out.Video.WindowScale = pristine
		}
	})

	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	onDisk, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Video.WindowScale != pristine {
		t.Errorf("file window scale = %d, want the pristine %d", onDisk.Video.WindowScale, pristine)
	}
	if s.Video.WindowScale != override {
		t.Errorf("running config window scale = %d, the Save must not touch it", s.Video.WindowScale)
	}

	s.Video.WindowScale = 3 // an edit away from the override persists
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	if onDisk, err = Load(path); err != nil || onDisk.Video.WindowScale != 3 {
		t.Errorf("edited value must persist: scale=%d err=%v", onDisk.Video.WindowScale, err)
	}
}

func TestLoadOrCreateUpgradesSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	// This older schema omits newer sections.
	old := `{"data_dir":"D:\\max","video":{"fullscreen":true,"window_scale":3}}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if s.DataDir != `D:\max` || !s.Video.Fullscreen || s.Video.WindowScale != 3 {
		t.Errorf("existing values lost: %+v", s)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"debug"`, `"console"`, `"audio"`, `"keybinds"`, `"widescreen"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("rewritten settings.json missing %s:\n%s", key, raw)
		}
	}
	// A schema without widescreen receives the automatic default.
	if s.Video.Widescreen != WidescreenAuto {
		t.Errorf("Widescreen on schema upgrade = %q, want auto", s.Video.Widescreen)
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	s := Default()
	s.DataDir = `C:\games\max`
	s.Audio.Backend = BackendSoLoud
	s.Audio.MusicVolume = 33
	s.Video.Fullscreen = true
	s.Video.Widescreen = WidescreenOn
	s.Player.Name = "Commander"
	s.Debug.Console = true
	s.Keybinds["end_turn"] = "Space"

	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, s) {
		t.Errorf("round trip mismatch:\ngot  %+v\nwant %+v", got, s)
	}
}

func TestWidescreenModeJSONForms(t *testing.T) {
	// Forced modes use JSON booleans; automatic mode uses a string.
	cases := []struct {
		mode WidescreenMode
		want string
	}{
		{WidescreenOn, "true"},
		{WidescreenOff, "false"},
		{WidescreenAuto, `"auto"`},
	}
	for _, c := range cases {
		raw, err := c.mode.MarshalJSON()
		if err != nil || string(raw) != c.want {
			t.Errorf("Marshal(%q) = %s, %v; want %s", c.mode, raw, err, c.want)
		}
	}

	// Unmarshal accepts legacy booleans and strings. Invalid values fall back to automatic mode.
	reads := []struct {
		raw  string
		want WidescreenMode
	}{
		{"true", WidescreenOn},
		{"false", WidescreenOff},
		{`"auto"`, WidescreenAuto},
		{`"TRUE"`, WidescreenOn},
		{`"on"`, WidescreenOn},
		{`"off"`, WidescreenOff},
		{`"widest"`, WidescreenAuto},
		{"42", WidescreenAuto},
	}
	for _, c := range reads {
		s := Default()
		if err := (&s.Video.Widescreen).UnmarshalJSON([]byte(c.raw)); err != nil {
			t.Errorf("Unmarshal(%s): %v", c.raw, err)
			continue
		}
		s.Validate()
		if s.Video.Widescreen != c.want {
			t.Errorf("Unmarshal(%s) validated = %q, want %q", c.raw, s.Video.Widescreen, c.want)
		}
	}
}

func TestResolveDataDir(t *testing.T) {
	s := Default()

	t.Setenv("MAX_DATA_DIR", `X:\env\dir`)
	if dir, _ := s.ResolveDataDir(); dir != `X:\env\dir` {
		t.Errorf("env override: got %q", dir)
	}

	t.Setenv("MAX_DATA_DIR", "")
	s.DataDir = `Y:\setting\dir`
	if dir, _ := s.ResolveDataDir(); dir != `Y:\setting\dir` {
		t.Errorf("data_dir setting: got %q", dir)
	}

	s.DataDir = ""
	dir, err := s.ResolveDataDir()
	if err != nil || dir == "" {
		t.Errorf("fallback to base dir: got %q, %v", dir, err)
	}
}

func TestDataDirExplicit(t *testing.T) {
	s := Default()
	t.Setenv("MAX_DATA_DIR", "")

	if s.DataDirExplicit() {
		t.Error("default settings: explicit = true, want false (base dir fallback)")
	}

	s.SetDataDirOverride(`X:\flag\dir`)
	if !s.DataDirExplicit() {
		t.Error("-datadir override: explicit = false, want true")
	}
	s.SetDataDirOverride("")

	t.Setenv("MAX_DATA_DIR", `X:\env\dir`)
	if !s.DataDirExplicit() {
		t.Error("MAX_DATA_DIR: explicit = false, want true")
	}
	t.Setenv("MAX_DATA_DIR", "")

	s.DataDir = `Y:\setting\dir`
	if !s.DataDirExplicit() {
		t.Error("data_dir setting: explicit = false, want true")
	}
}

// TestResolvePath checks preferred, legacy, and new-install settings paths.
func TestResolvePath(t *testing.T) {
	dir := t.TempDir()
	preferred := filepath.Join(dir, "max-go.json")
	legacy := filepath.Join(dir, "settings.json")

	if got := ResolvePath(dir); got != preferred {
		t.Errorf("fresh dir: %q, want %q", got, preferred)
	}

	if err := os.WriteFile(legacy, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ResolvePath(dir); got != legacy {
		t.Errorf("legacy only: %q, want %q", got, legacy)
	}

	if err := os.WriteFile(preferred, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ResolvePath(dir); got != preferred {
		t.Errorf("both present: %q, want %q", got, preferred)
	}
}

func TestValidateNormalizesBadValues(t *testing.T) {
	s := Settings{}
	s.Audio.Backend = "winamp"
	s.Audio.MusicVolume = 999
	s.Audio.VoiceVolume = -5
	s.Video.WindowScale = 0

	s.Validate()

	if s.Audio.Backend != BackendDefault {
		t.Errorf("Backend = %q, want %q", s.Audio.Backend, BackendDefault)
	}
	if s.Audio.MusicVolume != 100 || s.Audio.VoiceVolume != 0 {
		t.Errorf("volumes not clamped: music=%d voice=%d", s.Audio.MusicVolume, s.Audio.VoiceVolume)
	}
	if s.Video.WindowScale != 2 {
		t.Errorf("WindowScale = %d, want 2", s.Video.WindowScale)
	}
	if s.Keybinds == nil {
		t.Error("Keybinds not defaulted")
	}
}

// TestValidateKeybinds fills missing actions and resolves the legacy Survey/Find collision without changing an intentional Survey binding.
func TestValidateKeybinds(t *testing.T) {
	// A five action settings file carries the legacy Survey binding to F.
	s := Settings{Keybinds: map[string]string{
		"end_turn": "Enter",
		"menu":     "Escape",
		"grid":     "G",
		"survey":   "F",
		"status":   "S",
	}}
	s.Validate()

	for _, action := range []string{"find", "help", "options", "zoom_in", "zoom_out"} {
		if s.Keybinds[action] == "" {
			t.Errorf("action %q not filled from the defaults", action)
		}
	}
	if s.Keybinds["survey"] != "Ctrl+R" {
		t.Errorf("survey = %q, want the F collision migrated to R and then on to Ctrl+R", s.Keybinds["survey"])
	}
	if s.Keybinds["status"] != "Ctrl+S" {
		t.Errorf("status = %q, want the old plain S moved to Ctrl+S", s.Keybinds["status"])
	}
	if s.Keybinds["grid"] != "G" {
		t.Errorf("grid = %q, want the user's value kept", s.Keybinds["grid"])
	}

	// The old plain letters move to Ctrl in any case; another letter stays.
	s3 := Settings{Keybinds: map[string]string{"survey": "r", "status": "Q", "options": "O"}}
	s3.Validate()

	if s3.Keybinds["survey"] != "Ctrl+R" || s3.Keybinds["options"] != "Ctrl+O" {
		t.Errorf("old letters not moved: survey=%q options=%q, want Ctrl+R / Ctrl+O", s3.Keybinds["survey"], s3.Keybinds["options"])
	}
	if s3.Keybinds["status"] != "Q" {
		t.Errorf("status = %q, want the deliberate Q kept", s3.Keybinds["status"])
	}

	// A deliberate survey on F stays because find uses another key.
	s2 := Settings{Keybinds: map[string]string{"survey": "F", "find": "C"}}
	s2.Validate()

	if s2.Keybinds["survey"] != "F" || s2.Keybinds["find"] != "C" {
		t.Errorf("deliberate survey=F/find=C rewritten to survey=%q find=%q", s2.Keybinds["survey"], s2.Keybinds["find"])
	}
}

// TestChannelsReversedKey checks the audio.channels_reversed key (from the original settings.ini): it defaults
// to false, an older file without it stays false, and a file that sets it loads it.
func TestChannelsReversedKey(t *testing.T) {
	if Default().Audio.ChannelsReversed {
		t.Error("channels_reversed must default to false")
	}
	path := filepath.Join(t.TempDir(), "max-go.json")
	if err := os.WriteFile(path, []byte(`{"audio": {"sfx_volume": 80, "channels_reversed": true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Audio.ChannelsReversed || s.Audio.SfxVolume != 80 {
		t.Errorf("loaded channels_reversed=%v sfx_volume=%d, want true and 80", s.Audio.ChannelsReversed, s.Audio.SfxVolume)
	}
}
