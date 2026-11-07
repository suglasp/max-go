// override_flags_test.go : tests temporary launch setting overrides.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/suglasp/max-go/internal/settings"
)

func stringPtr(v string) *string { return &v }
func intPtr(v int) *int          { return &v }
func boolPtr(v bool) *bool       { return &v }

func emptyOverrideFlags() *OverrideFlags {
	return &OverrideFlags{
		DataDir:    stringPtr(""),
		PlayerName: stringPtr(""),
		Audio:      stringPtr(""),
		Scale:      intPtr(0),
		Windowed:   boolPtr(false),
		Fullscreen: boolPtr(false),
		Debug:      boolPtr(false),
		Reset:      boolPtr(false),
	}
}

func TestValidateOverrideFlags(t *testing.T) {
	if err := ValidateOverrideFlags(nil); err != nil {
		t.Fatalf("nil flags: %v", err)
	}

	of := emptyOverrideFlags()
	if err := ValidateOverrideFlags(of); err != nil {
		t.Fatalf("all unset: %v", err)
	}

	of = emptyOverrideFlags()
	of.Audio = stringPtr("SoLoud")
	if err := ValidateOverrideFlags(of); err != nil {
		t.Fatalf("audio soloud: %v", err)
	}
	if *of.Audio != "soloud" {
		t.Errorf("audio not normalized: %q", *of.Audio)
	}

	of = emptyOverrideFlags()
	of.Audio = stringPtr("sdl")
	if err := ValidateOverrideFlags(of); err == nil {
		t.Error("unknown audio backend accepted")
	}

	of = emptyOverrideFlags()
	of.Scale = intPtr(9)
	if err := ValidateOverrideFlags(of); err == nil {
		t.Error("scale 9 accepted")
	}

	of = emptyOverrideFlags()
	of.Scale = intPtr(-1)
	if err := ValidateOverrideFlags(of); err == nil {
		t.Error("scale -1 accepted")
	}

	of = emptyOverrideFlags()
	of.Windowed = boolPtr(true)
	of.Fullscreen = boolPtr(true)
	if err := ValidateOverrideFlags(of); err == nil {
		t.Error("-windowed with -fullscreen accepted")
	}
}

func TestApplyLaunchOverrides(t *testing.T) {
	cfg := settings.Default()
	base := cfg

	// All flags unset: the settings stay untouched.
	applyLaunchOverrides(&cfg, emptyOverrideFlags())
	if !reflect.DeepEqual(cfg, base) {
		t.Fatalf("unset flags changed the settings: %+v", cfg)
	}

	of := emptyOverrideFlags()
	of.PlayerName = stringPtr("Commander")
	of.Audio = stringPtr("native")
	of.Scale = intPtr(4)
	of.Fullscreen = boolPtr(true)
	of.Debug = boolPtr(true)
	applyLaunchOverrides(&cfg, of)

	if cfg.Player.Name != "Commander" {
		t.Errorf("player name = %q, want Commander", cfg.Player.Name)
	}
	if cfg.Audio.Backend != settings.BackendEbiten {
		t.Errorf("audio backend = %q, want %q (native alias)", cfg.Audio.Backend, settings.BackendEbiten)
	}
	if cfg.Video.WindowScale != 4 {
		t.Errorf("window scale = %d, want 4", cfg.Video.WindowScale)
	}
	if !cfg.Video.Fullscreen {
		t.Error("fullscreen not applied")
	}
	if !cfg.Debug.Console || !cfg.Debug.LogFile {
		t.Errorf("debug not applied: console=%v logfile=%v", cfg.Debug.Console, cfg.Debug.LogFile)
	}

	// -windowed turns a saved fullscreen off.
	of = emptyOverrideFlags()
	of.Windowed = boolPtr(true)
	applyLaunchOverrides(&cfg, of)
	if cfg.Video.Fullscreen {
		t.Error("-windowed left fullscreen on")
	}
}

// TestLaunchOverridesNeverReachDisk: folded override flags run the game but a
// later whole config save keeps the FILE's own values - unless the player edited
// the field away from the override in a menu, which persists like any edit.
func TestLaunchOverridesNeverReachDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.json")
	saved := settings.Default()
	saved.Video.WindowScale = 2
	saved.Video.Fullscreen = false
	if err := saved.Save(path); err != nil {
		t.Fatal(err)
	}

	cfg, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	of := emptyOverrideFlags()
	of.Scale = intPtr(4)
	of.Fullscreen = boolPtr(true)
	of.Debug = boolPtr(true)
	applyLaunchOverrides(&cfg, of)

	// A menu save (any whole config save) happens while the overrides are live.
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	onDisk, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Video.WindowScale != 2 || onDisk.Video.Fullscreen || onDisk.Debug.Console || onDisk.Debug.LogFile {
		t.Errorf("launch overrides leaked into the file: scale=%d fullscreen=%v debug=%v/%v",
			onDisk.Video.WindowScale, onDisk.Video.Fullscreen, onDisk.Debug.Console, onDisk.Debug.LogFile)
	}
	if cfg.Video.WindowScale != 4 || !cfg.Video.Fullscreen {
		t.Error("the RUNNING config must keep the overrides after the save")
	}

	// The player edits the overridden field AWAY from the override in a menu:
	// that edit persists like any other.
	cfg.Video.WindowScale = 3
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	if onDisk, err = settings.Load(path); err != nil || onDisk.Video.WindowScale != 3 {
		t.Errorf("a real menu edit must persist: scale=%d err=%v", onDisk.Video.WindowScale, err)
	}
}

func TestApplyLaunchOverridesDataDirBeatsEnv(t *testing.T) {
	t.Setenv("MAX_DATA_DIR", "env-data")

	cfg := settings.Default()
	if dir, err := cfg.ResolveDataDir(); err != nil || dir != "env-data" {
		t.Fatalf("env precedence broken before the flag: %q, %v", dir, err)
	}

	of := emptyOverrideFlags()
	of.DataDir = stringPtr("flag-data")
	applyLaunchOverrides(&cfg, of)
	if dir, err := cfg.ResolveDataDir(); err != nil || dir != "flag-data" {
		t.Fatalf("-datadir must win over MAX_DATA_DIR: got %q, %v", dir, err)
	}
}

// TestLanguageFlagNeverReachesDisk checks that -language sets the running config only: a menu save keeps the
// language key out of max-go.json.
func TestLanguageFlagNeverReachesDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.json")
	if err := settings.Default().Save(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	applyLanguageFlag(&cfg, " en-us ")
	if cfg.Language != settings.LangEnglish {
		t.Fatalf("running language = %q, want en-us", cfg.Language)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	onDisk, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Language != "" {
		t.Errorf("-language leaked into the file: %q", onDisk.Language)
	}

	applyLanguageFlag(&cfg, "") // no flag: nothing changes
	if cfg.Language != settings.LangEnglish {
		t.Errorf("an empty flag changed the language to %q", cfg.Language)
	}
}

func TestAudioBackendName(t *testing.T) {
	cases := map[string]string{
		"native":    settings.BackendEbiten,
		"default":   settings.BackendEbiten,
		"Ebiten":    settings.BackendEbiten,
		"soloud":    settings.BackendSoLoud,
		"MINIAUDIO": settings.BackendMiniAudio,
	}
	for in, want := range cases {
		if got := audioBackendName(in); got != want {
			t.Errorf("audioBackendName(%q) = %q, want %q", in, got, want)
		}
	}
}
