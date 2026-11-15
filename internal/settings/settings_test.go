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

// TestValidateSoLoudBackend keeps a listed device backend (any case or spacing, stored lower case) and turns an
// empty or unknown one into auto, as the field comment says.
func TestValidateSoLoudBackend(t *testing.T) {
	for in, want := range map[string]string{
		"":          "auto",
		"auto":      "auto",
		"wasapi":    "wasapi",
		" WASAPI ":  "wasapi",
		"MiniAudio": "miniaudio",
		"winamp":    "auto",
		"sdl3":      "auto",
	} {
		s := Default()
		s.Audio.SoLoudBackend = in
		s.Validate()
		if s.Audio.SoLoudBackend != want {
			t.Errorf("soloud_backend %q -> %q, want %q", in, s.Audio.SoLoudBackend, want)
		}
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

// Every integer setting is bounded: a value under or over its range is repaired by the table, and a value on
// each edge of the range stays.
func TestValidateBoundsEveryInt(t *testing.T) {
	for _, r := range IntRanges {
		cfg := Default()
		p := r.value(&cfg)

		*p = r.Min - 1
		cfg.Validate()
		if *p != r.Below {
			t.Errorf("%s: %d repaired to %d, want %d", r.Field, r.Min-1, *p, r.Below)
		}
		*p = r.Max + 1
		cfg.Validate()
		if *p != r.Above {
			t.Errorf("%s: %d repaired to %d, want %d", r.Field, r.Max+1, *p, r.Above)
		}
		for _, edge := range []int{r.Min, r.Max} {
			*p = edge
			cfg.Validate()
			if *p != edge {
				t.Errorf("%s: the valid value %d was repaired to %d", r.Field, edge, *p)
			}
		}
	}
}

// The defaults sit inside every range, so a fresh file reports no repairs.
func TestDefaultsAreInRange(t *testing.T) {
	cfg := Default()
	if repairs := cfg.ValidateReport(); len(repairs) != 0 {
		t.Fatalf("the defaults needed repairs: %v", repairs)
	}
}

// The ranges the game documents: the retail clan numbers, the Options slider and the volumes.
func TestValidateKnownRanges(t *testing.T) {
	cases := []struct {
		field string
		min   int
		max   int
	}{
		{"player.clan", 0, 8},
		{"preferences.quick_scroll", 4, 128},
		{"audio.music_volume", 0, 100},
		{"video.window_scale", 1, 8},
		{"game.opponent", 0, 5},
	}
	for _, tc := range cases {
		r, ok := IntRangeFor(tc.field)
		if !ok || r.Min != tc.min || r.Max != tc.max {
			t.Errorf("%s: range %+v (found %v), want %d..%d", tc.field, r, ok, tc.min, tc.max)
		}
	}
	if _, ok := IntRangeFor("no.such.field"); ok {
		t.Errorf("IntRangeFor found a field that does not exist")
	}
}

// ValidateReport lists each repair with its field, old and new value, and Validate makes the same changes.
func TestValidateReportListsRepairs(t *testing.T) {
	cfg := Default()
	cfg.Video.WindowScale = 99
	cfg.Player.Clan = -3
	cfg.Audio.MusicVolume = 250
	cfg.Audio.Backend = "bogus"
	cfg.Video.Widescreen = "on"
	cfg.Language = "xx"
	cfg.Prefs.QuickScroll = 2

	twin := cfg
	repairs := cfg.ValidateReport()
	var got []string
	for _, r := range repairs {
		got = append(got, r.String())
	}
	want := []string{
		`audio.backend was "bogus", now "default"`,
		"audio.music_volume was 250, now 100",
		"video.window_scale was 99, now 2",
		"player.clan was -3, now 7",
		"preferences.quick_scroll was 2, now 16",
		`video.widescreen was "on", now "true"`,
		`language was "xx", now ""`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("repairs\n got %q\nwant %q", got, want)
	}

	twin.Validate()
	if !reflect.DeepEqual(twin, cfg) {
		t.Errorf("Validate and ValidateReport differ:\n%+v\n%+v", twin, cfg)
	}
}

// A second pass finds nothing left to repair.
func TestValidateReportIsIdempotent(t *testing.T) {
	cfg := Default()
	cfg.Game.Opponent = 77
	cfg.Game.World = -1
	cfg.ValidateReport()
	if repairs := cfg.ValidateReport(); len(repairs) != 0 {
		t.Errorf("second pass repaired again: %v", repairs)
	}
}

// LoadReport hands back what Load repaired, and Load still returns the repaired settings.
func TestLoadReportListsRepairs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.json")
	body := `{"video": {"window_scale": 99}, "game": {"opponent": 9, "last_campaign": 0}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, repairs, err := LoadReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Video.WindowScale != 2 || cfg.Game.Opponent != 2 || cfg.Game.LastCampaign != 1 {
		t.Errorf("loaded %+v, want the repaired values", cfg)
	}
	if len(repairs) != 3 {
		t.Errorf("repairs %v, want 3", repairs)
	}
	plain, err := Load(path)
	if err != nil || !reflect.DeepEqual(plain, cfg) {
		t.Errorf("Load differs from LoadReport: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != body {
		t.Errorf("Load rewrote the file: %q", after)
	}
}

// Repairs land on the value the game falls back to: a slider speed caps, a volume clamps, the rest reset.
func TestValidateRepairTargets(t *testing.T) {
	cfg := Default()
	cfg.Prefs.QuickScroll = 500
	cfg.Audio.SfxVolume = -1
	cfg.Player.Clan = 42
	cfg.Game.VictoryLimit = 0
	cfg.Debug.LogBufferLines = 1 << 20
	cfg.Validate()
	if cfg.Prefs.QuickScroll != 128 || cfg.Audio.SfxVolume != 0 || cfg.Player.Clan != 7 ||
		cfg.Game.VictoryLimit != 100 || cfg.Debug.LogBufferLines != 100000 {
		t.Errorf("repair targets wrong: %+v", cfg)
	}
	cfg.Prefs.QuickScroll = 2
	cfg.Validate()
	if cfg.Prefs.QuickScroll != 16 {
		t.Errorf("quick_scroll 2 repaired to %d, want 16", cfg.Prefs.QuickScroll)
	}
}
