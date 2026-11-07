// override_flags.go : defines launch options that override saved settings for one run.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/suglasp/max-go/internal/settings"
)

// OverrideFlags holds temporary replacements for loaded settings. The -reset
// option is the exception: it writes defaults before loading the file.
type OverrideFlags struct {
	DataDir    *string
	PlayerName *string
	Audio      *string
	Scale      *int
	Windowed   *bool
	Fullscreen *bool
	Debug      *bool
	Reset      *bool
}

// registerOverrideFlags registers the settings override flags.
func registerOverrideFlags(fs *flag.FlagSet) *OverrideFlags {
	flags := &OverrideFlags{}
	flags.DataDir = fs.String("datadir", "", "game data directory for this run; wins over MAX_DATA_DIR and the data_dir setting")
	fs.StringVar(flags.DataDir, "data", "", "alias of -datadir")
	flags.PlayerName = fs.String("playername", "", "player name for this run (overrides player.name)")
	flags.Audio = fs.String("audio", "", "audio backend for this run: native, ebiten, soloud, miniaudio (overrides audio.backend)")
	flags.Scale = fs.Int("scale", 0, "desktop window scale for this run: 1..8 (overrides video.window_scale)")
	flags.Windowed = fs.Bool("windowed", false, "start in a desktop window for this run (overrides video.fullscreen)")
	flags.Fullscreen = fs.Bool("fullscreen", false, "start fullscreen for this run (overrides video.fullscreen)")
	flags.Debug = fs.Bool("debug", false, "enable the debug console and max-go.log for this run (overrides debug.console and debug.log_file)")
	flags.Reset = fs.Bool("reset", false, "rewrite the settings file (max-go.json) to its defaults and boot on them; the saves folder stays untouched")
	return flags
}

// ValidateOverrideFlags checks the override flag values and normalizes named
// values to their canonical spelling.
func ValidateOverrideFlags(flags *OverrideFlags) error {
	if flags == nil {
		return nil
	}
	if err := validateChoice(flags.Audio, "audio", "", "native", "default", "ebiten", "soloud", "miniaudio"); err != nil {
		return err
	}
	if n := intValue(flags.Scale); n != 0 && (n < 1 || n > 8) {
		return fmt.Errorf("flag -scale must be an integer from 1 to 8")
	}
	if boolValue(flags.Windowed) && boolValue(flags.Fullscreen) {
		return fmt.Errorf("flags -windowed and -fullscreen are mutually exclusive")
	}
	return nil
}

// applyLaunchOverrides updates settings for the current boot. Each override registers a save-time restore, unless the player later changes that field in a menu.
func applyLaunchOverrides(cfg *settings.Settings, of *OverrideFlags) {
	if cfg == nil || of == nil {
		return
	}
	if dir := strings.TrimSpace(stringValue(of.DataDir)); dir != "" {
		cfg.SetDataDirOverride(dir) // unexported field, never serialized: no restore needed
	}
	if name := strings.TrimSpace(stringValue(of.PlayerName)); name != "" {
		overrideForRun(cfg, func(s *settings.Settings) *string { return &s.Player.Name }, name)
	}
	if backend := strings.TrimSpace(stringValue(of.Audio)); backend != "" {
		overrideForRun(cfg, func(s *settings.Settings) *string { return &s.Audio.Backend }, audioBackendName(backend))
	}
	if n := intValue(of.Scale); n >= 1 && n <= 8 {
		overrideForRun(cfg, func(s *settings.Settings) *int { return &s.Video.WindowScale }, n)
	}
	if boolValue(of.Windowed) {
		overrideForRun(cfg, func(s *settings.Settings) *bool { return &s.Video.Fullscreen }, false)
	}
	if boolValue(of.Fullscreen) {
		overrideForRun(cfg, func(s *settings.Settings) *bool { return &s.Video.Fullscreen }, true)
	}
	if boolValue(of.Debug) {
		overrideForRun(cfg, func(s *settings.Settings) *bool { return &s.Debug.Console }, true)
		overrideForRun(cfg, func(s *settings.Settings) *bool { return &s.Debug.LogFile }, true)
	}
}

// overrideForRun applies a one-run override and registers its save-time restore. A later menu edit to the field persists normally.
func overrideForRun[T comparable](cfg *settings.Settings, field func(*settings.Settings) *T, value T) {
	pristine := *field(cfg)
	*field(cfg) = value
	cfg.RegisterLaunchRestore(func(s *settings.Settings) {
		if *field(s) == value {
			*field(s) = pristine
		}
	})
}

// validateDifficulty checks the -difficulty value: -1 (the default) leaves the tier unset, 0..5 picks
// Clueless..God. Anything else is refused, since there is no tier past God.
func validateDifficulty(n int) error {
	if n < -1 || n > 5 {
		return fmt.Errorf("flag -difficulty must be an integer from 0 to 5 (Clueless..God), got %d", n)
	}
	return nil
}

// applyLanguageFlag sets the -language code for this run only, so a menu save never writes it to max-go.json.
func applyLanguageFlag(cfg *settings.Settings, language string) {
	if language = strings.TrimSpace(language); language != "" {
		overrideForRun(cfg, func(s *settings.Settings) *string { return &s.Language }, language)
	}
}

// audioBackendName maps -audio values to settings backend names. Both native
// and default select Ebitengine's built in backend.
func audioBackendName(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "native", "default", "ebiten":
		return settings.BackendEbiten
	case "soloud":
		return settings.BackendSoLoud
	case "miniaudio":
		return settings.BackendMiniAudio
	}
	return settings.BackendDefault
}

// intValue reads an int flag pointer, treating nil as zero.
func intValue(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
