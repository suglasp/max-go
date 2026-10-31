// main.go : is the max-settings entry point for editing max-go settings in a terminal.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/suglasp/max-go/internal/settings"
)

type field struct {
	name    string
	get     func(settings.Settings) string
	set     func(*settings.Settings, string) error
	presets []string // fixed value list for bool/choice settings. non nil switches the editor from typed input to the Left/Right adjuster.
}

// main runs the max-settings tool.
func main() {
	pathFlag := flag.String("file", "", "settings file path (default: max-go.json next to the binary, else a legacy settings.json)")
	flag.Parse()

	path, err := settingsPath(*pathFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cfg, err := settings.LoadOrCreate(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := run(path, cfg, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Prefer max-go.json, but continue to recognize the legacy settings.json path.
func settingsPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	baseDir, err := settings.BaseDir()
	if err != nil {
		return "", err
	}

	return settings.ResolvePath(baseDir), nil
}

func run(path string, cfg settings.Settings, in *os.File, out *os.File) error {
	reader := bufio.NewReader(in)
	fields := settingFields(cfg)

	dirty := false

	for {
		printMenu(out, path, cfg, fields, dirty)
		fmt.Fprint(out, "> ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return err
		}

		cmd := strings.TrimSpace(line)

		switch strings.ToLower(cmd) {
		case "":
			continue

		case "q", "quit":
			// Quit without saving.
			if dirty && !confirm(reader, out, "Discard unsaved changes?") {
				continue
			}

			return nil

		case "s", "save":
			// Save the current settings.
			cfg.Validate()
			if err := cfg.Save(path); err != nil {
				return err
			}

			dirty = false
			fmt.Fprintln(out, "Saved.")
			wait(reader, out)

		default:
			i, err := strconv.Atoi(cmd)
			if err != nil || i < 1 || i > len(fields) {
				fmt.Fprintln(out, "Enter a field number, s to save, or q to quit.")
				wait(reader, out)
				continue
			}

			f := fields[i-1]
			// Preset settings use the Left and Right adjuster.
			if len(f.presets) > 0 {
				if adjustPreset(reader, in, out, &cfg, f) {
					dirty = true
					fields = settingFields(cfg)
				}
				continue
			}

			fmt.Fprintf(out, "%s [%s]: ", f.name, f.get(cfg))
			value, err := reader.ReadString('\n')

			if err != nil && len(value) == 0 {
				return err
			}

			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}

			if err := f.set(&cfg, value); err != nil {
				fmt.Fprintln(out, err)
				wait(reader, out)
				continue
			}

			dirty = true
			fields = settingFields(cfg)
		}
	}
}

func printMenu(out *os.File, path string, cfg settings.Settings, fields []field, dirty bool) {
	fmt.Fprint(out, "\n\n")
	fmt.Fprintln(out, "M.A.X. Settings")
	fmt.Fprintln(out, strings.Repeat("=", 64))
	fmt.Fprintf(out, "File: %s\n", path)

	if dirty {
		fmt.Fprintln(out, "Status: modified")
	} else {
		fmt.Fprintln(out, "Status: clean")
	}

	fmt.Fprintln(out)

	for i, f := range fields {
		// Angle brackets mark fields that open the Left and Right adjuster.
		value := f.get(cfg)
		if len(f.presets) > 0 {
			value = "<" + value + ">"
		}

		fmt.Fprintf(out, "%2d. %-28s %s\n", i+1, f.name, value)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Commands: number = edit (<value> fields adjust with Left/Right), s = save, q = quit")
}

func wait(reader *bufio.Reader, out *os.File) {
	fmt.Fprint(out, "Press Enter to continue...")
	_, _ = reader.ReadString('\n')
}

func confirm(reader *bufio.Reader, out *os.File, prompt string) bool {
	fmt.Fprintf(out, "%s [y/N] ", prompt)
	line, _ := reader.ReadString('\n')

	return strings.EqualFold(strings.TrimSpace(line), "y")
}

func settingFields(cfg settings.Settings) []field {
	fields := []field{
		stringField("data_dir", func(s settings.Settings) string { return s.DataDir }, func(s *settings.Settings, v string) { s.DataDir = v }),
		selectField("language", settings.SupportedLanguages, func(s settings.Settings) string { return s.Language }, func(s *settings.Settings, v string) { s.Language = v }),
		selectField("audio.backend", []string{settings.BackendDefault, settings.BackendEbiten, settings.BackendSoLoud, settings.BackendMiniAudio}, func(s settings.Settings) string { return s.Audio.Backend }, func(s *settings.Settings, v string) { s.Audio.Backend = v }),
		selectField("audio.soloud_backend", []string{"auto", "miniaudio", "wasapi", "winmm", "xaudio2", "sdl2", "portaudio", "openal", "alsa", "coreaudio"}, func(s settings.Settings) string { return s.Audio.SoLoudBackend }, func(s *settings.Settings, v string) { s.Audio.SoLoudBackend = v }),
		intField("audio.music_volume", func(s settings.Settings) int { return s.Audio.MusicVolume }, func(s *settings.Settings, v int) { s.Audio.MusicVolume = v }),
		intField("audio.sfx_volume", func(s settings.Settings) int { return s.Audio.SfxVolume }, func(s *settings.Settings, v int) { s.Audio.SfxVolume = v }),
		intField("audio.voice_volume", func(s settings.Settings) int { return s.Audio.VoiceVolume }, func(s *settings.Settings, v int) { s.Audio.VoiceVolume = v }),
		boolField("audio.music_enabled", func(s settings.Settings) bool { return s.Audio.MusicEnabled }, func(s *settings.Settings, v bool) { s.Audio.MusicEnabled = v }),
		boolField("audio.sfx_enabled", func(s settings.Settings) bool { return s.Audio.SfxEnabled }, func(s *settings.Settings, v bool) { s.Audio.SfxEnabled = v }),
		boolField("audio.voice_enabled", func(s settings.Settings) bool { return s.Audio.VoiceEnabled }, func(s *settings.Settings, v bool) { s.Audio.VoiceEnabled = v }),
		boolField("audio.channels_reversed", func(s settings.Settings) bool { return s.Audio.ChannelsReversed }, func(s *settings.Settings, v bool) { s.Audio.ChannelsReversed = v }),
		boolField("video.fullscreen", func(s settings.Settings) bool { return s.Video.Fullscreen }, func(s *settings.Settings, v bool) { s.Video.Fullscreen = v }),
		intField("video.window_scale", func(s settings.Settings) int { return s.Video.WindowScale }, func(s *settings.Settings, v int) { s.Video.WindowScale = v }),
		intField("video.brightness", func(s settings.Settings) int { return s.Video.Brightness }, func(s *settings.Settings, v int) { s.Video.Brightness = v }),
		selectField("video.widescreen", []string{"auto", "true", "false"}, func(s settings.Settings) string { return string(s.Video.Widescreen) }, func(s *settings.Settings, v string) { s.Video.Widescreen = settings.WidescreenMode(v) }),
		boolField("video.popup_vanilla_look", func(s settings.Settings) bool { return s.Video.PopupVanillaLook }, func(s *settings.Settings, v bool) { s.Video.PopupVanillaLook = v }),
		stringField("player.name", func(s settings.Settings) string { return s.Player.Name }, func(s *settings.Settings, v string) { s.Player.Name = v }),
		intField("player.clan", func(s settings.Settings) int { return s.Player.Clan }, func(s *settings.Settings, v int) { s.Player.Clan = v }),
		boolField("game.intro_movie", func(s settings.Settings) bool { return s.Game.IntroMovie }, func(s *settings.Settings, v bool) { s.Game.IntroMovie = v }),
		boolField("game.auto_save", func(s settings.Settings) bool { return s.Game.AutoSave }, func(s *settings.Settings, v bool) { s.Game.AutoSave = v }),
		intField("game.world", func(s settings.Settings) int { return s.Game.World }, func(s *settings.Settings, v int) { s.Game.World = v }),
		intField("game.turn_timer", func(s settings.Settings) int { return s.Game.TurnTimer }, func(s *settings.Settings, v int) { s.Game.TurnTimer = v }),
		intField("game.end_turn", func(s settings.Settings) int { return s.Game.EndTurn }, func(s *settings.Settings, v int) { s.Game.EndTurn = v }),
		intField("game.start_gold", func(s settings.Settings) int { return s.Game.StartGold }, func(s *settings.Settings, v int) { s.Game.StartGold = v }),
		intField("game.play_mode", func(s settings.Settings) int { return s.Game.PlayMode }, func(s *settings.Settings, v int) { s.Game.PlayMode = v }),
		intField("game.victory_type", func(s settings.Settings) int { return s.Game.VictoryType }, func(s *settings.Settings, v int) { s.Game.VictoryType = v }),
		intField("game.victory_limit", func(s settings.Settings) int { return s.Game.VictoryLimit }, func(s *settings.Settings, v int) { s.Game.VictoryLimit = v }),
		intField("game.opponent", func(s settings.Settings) int { return s.Game.Opponent }, func(s *settings.Settings, v int) { s.Game.Opponent = v }),
		intField("game.raw_resource", func(s settings.Settings) int { return s.Game.RawResource }, func(s *settings.Settings, v int) { s.Game.RawResource = v }),
		intField("game.fuel_resource", func(s settings.Settings) int { return s.Game.FuelResource }, func(s *settings.Settings, v int) { s.Game.FuelResource = v }),
		intField("game.gold_resource", func(s settings.Settings) int { return s.Game.GoldResource }, func(s *settings.Settings, v int) { s.Game.GoldResource = v }),
		intField("game.alien_derelicts", func(s settings.Settings) int { return s.Game.AlienDerelicts }, func(s *settings.Settings, v int) { s.Game.AlienDerelicts = v }),
		intField("game.last_campaign", func(s settings.Settings) int { return s.Game.LastCampaign }, func(s *settings.Settings, v int) { s.Game.LastCampaign = v }),
		boolField("preferences.effects", func(s settings.Settings) bool { return s.Prefs.Effects }, func(s *settings.Settings, v bool) { s.Prefs.Effects = v }),
		boolField("preferences.click_scroll", func(s settings.Settings) bool { return s.Prefs.ClickScroll }, func(s *settings.Settings, v bool) { s.Prefs.ClickScroll = v }),
		intField("preferences.quick_scroll", func(s settings.Settings) int { return s.Prefs.QuickScroll }, func(s *settings.Settings, v int) { s.Prefs.QuickScroll = v }),
		boolField("preferences.fast_movement", func(s settings.Settings) bool { return s.Prefs.FastMovement }, func(s *settings.Settings, v bool) { s.Prefs.FastMovement = v }),
		boolField("preferences.follow_unit", func(s settings.Settings) bool { return s.Prefs.FollowUnit }, func(s *settings.Settings, v bool) { s.Prefs.FollowUnit = v }),
		boolField("preferences.auto_select", func(s settings.Settings) bool { return s.Prefs.AutoSelect }, func(s *settings.Settings, v bool) { s.Prefs.AutoSelect = v }),
		boolField("preferences.enemy_halt", func(s settings.Settings) bool { return s.Prefs.EnemyHalt }, func(s *settings.Settings, v bool) { s.Prefs.EnemyHalt = v }),
		boolField("debug.menu", func(s settings.Settings) bool { return s.Debug.Menu }, func(s *settings.Settings, v bool) { s.Debug.Menu = v }),
		boolField("debug.console", func(s settings.Settings) bool { return s.Debug.Console }, func(s *settings.Settings, v bool) { s.Debug.Console = v }),
		boolField("debug.log_file", func(s settings.Settings) bool { return s.Debug.LogFile }, func(s *settings.Settings, v bool) { s.Debug.LogFile = v }),
		intField("debug.log_buffer_lines", func(s settings.Settings) int { return s.Debug.LogBufferLines }, func(s *settings.Settings, v int) { s.Debug.LogBufferLines = v }),
	}

	keys := make([]string, 0, len(cfg.Keybinds))
	for key := range cfg.Keybinds {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	for _, key := range keys {
		k := key
		fields = append(fields, stringField("keybinds."+k, func(s settings.Settings) string { return s.Keybinds[k] }, func(s *settings.Settings, v string) {
			if s.Keybinds == nil {
				s.Keybinds = map[string]string{}
			}
			s.Keybinds[k] = v
		}))
	}

	return fields
}

func stringField(name string, get func(settings.Settings) string, set func(*settings.Settings, string)) field {
	return field{name: name, get: get, set: func(s *settings.Settings, v string) error {
		set(s, v)

		return nil
	}}
}

func intField(name string, get func(settings.Settings) int, set func(*settings.Settings, int)) field {
	return field{name: name, get: func(s settings.Settings) string { return strconv.Itoa(get(s)) }, set: func(s *settings.Settings, v string) error {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s expects an integer", name)
		}
		set(s, n)

		return nil
	}}
}

func boolField(name string, get func(settings.Settings) bool, set func(*settings.Settings, bool)) field {
	return field{name: name, presets: []string{"true", "false"}, get: func(s settings.Settings) string { return strconv.FormatBool(get(s)) }, set: func(s *settings.Settings, v string) error {
		b, err := parseBool(v)
		if err != nil {
			return fmt.Errorf("%s expects true/false, yes/no, on/off, or 1/0", name)
		}
		set(s, b)

		return nil
	}}
}

func selectField(name string, allowed []string, get func(settings.Settings) string, set func(*settings.Settings, string)) field {
	return field{name: name, presets: allowed, get: get, set: func(s *settings.Settings, v string) error {
		for _, option := range allowed {
			if strings.EqualFold(v, option) {
				set(s, option)
				return nil
			}
		}

		return fmt.Errorf("%s expects one of: %s", name, strings.Join(allowed, ", "))
	}}
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true, nil

	case "0", "f", "false", "n", "no", "off":
		return false, nil

	default:
		return false, fmt.Errorf("invalid bool")
	}
}
