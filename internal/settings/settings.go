// settings.go : the M.A.X. settings package  loads and stores game configuration in 'max-go.json' next to the binary.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : settings.cpp - settings.ini parsing and persistence
//
// Package settings stores game configuration in max-go.json beside the binary and accepts the legacy settings.json name through ResolvePath.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Audio backend names accepted in Settings.Audio.Backend.
const (
	// BackendDefault is an alias resolved to the native Ebitengine/Oto backend
	// (BackendEbiten); it is the value written for a fresh settings.json so the
	// file documents itself as using the default backend.
	BackendDefault   = "default"
	BackendEbiten    = "ebiten"
	BackendSoLoud    = "soloud"
	BackendMiniAudio = "miniaudio"
)

// Language names select which lang-<code>.ini strings file is loaded.
const (
	LangEnglish = "en-us"
)

// WidescreenMode is the video.widescreen value: a real JSON bool forces the
// wide (true) or classic 4:3 (false) game view, the string "auto" picks the
// aspect closest to the monitor at startup.
type WidescreenMode string

// Widescreen modes accepted in Settings.Video.Widescreen.
const (
	WidescreenOn   WidescreenMode = "true"
	WidescreenOff  WidescreenMode = "false"
	WidescreenAuto WidescreenMode = "auto"
)

// MarshalJSON writes the mode in its settings.json form: the forced modes
// stay real JSON booleans, auto is the string "auto".
func (m WidescreenMode) MarshalJSON() ([]byte, error) {
	switch m {
	case WidescreenOn:
		return []byte("true"), nil
	case WidescreenOff:
		return []byte("false"), nil
	default:
		return []byte(`"auto"`), nil
	}
}

// UnmarshalJSON accepts legacy JSON booleans or strings and falls back to automatic mode for other values.
func (m *WidescreenMode) UnmarshalJSON(data []byte) error {
	var forced bool
	if err := json.Unmarshal(data, &forced); err == nil {
		if forced {
			*m = WidescreenOn
		} else {
			*m = WidescreenOff
		}
		return nil
	}
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		*m = WidescreenMode(strings.ToLower(strings.TrimSpace(name)))
		return nil
	}
	*m = WidescreenAuto
	return nil
}

// SoLoudBackends lists the SoLoud device backends audio.soloud_backend may name, in the order max-settings offers
// them; "auto" lets SoLoud choose.
var SoLoudBackends = []string{"auto", "miniaudio", "wasapi", "winmm", "xaudio2", "sdl2", "portaudio", "openal", "alsa", "coreaudio"}

// SupportedLanguages lists the bundled UI languages. The loader still
// accepts lang-<code>.ini files dropped in through the command line hook.
var SupportedLanguages = []string{LangEnglish}

// IsSupportedLanguage reports whether name is an offered language.
func IsSupportedLanguage(name string) bool {
	for _, l := range SupportedLanguages {
		if l == name {
			return true
		}
	}
	return false
}

// Settings is the root of settings.json.
type Settings struct {
	DataDir  string            `json:"data_dir"`           // Original game-data directory; empty uses the binary directory.
	Language string            `json:"language,omitempty"` // Optional language code; absent or empty reads the texts from LANG.RES, an unsupported code counts as absent.
	Audio    Audio             `json:"audio"`
	Video    Video             `json:"video"`
	Player   Player            `json:"player"`
	Game     Game              `json:"game"`
	Prefs    Prefs             `json:"preferences"`
	Debug    Debug             `json:"debug"`
	Keybinds map[string]string `json:"keybinds"`

	dataDirOverride string // Per-run -datadir value; never serialized.

	// launchRestores back one-run launch override flags (-fullscreen, -audio, ...)
	// out of every Save, so they never flip the saved file. Registered at boot per
	// overridden field; see RegisterLaunchRestore. The slice header rides value
	// copies of Settings, so every save site is covered without wiring.
	launchRestores []func(*Settings)
}

// SetDataDirOverride points the game data lookup at dir for this run only;
// it beats MAX_DATA_DIR and the data_dir setting.
func (s *Settings) SetDataDirOverride(dir string) {
	s.dataDirOverride = dir
}

// RegisterLaunchRestore records a fix up applied to the copy being written on
// every Save: a launch override flag folds a one-run value into the running
// config, and its restore puts the saved file's value back - typically only when
// the field still holds the override, so a value the player edited in a menu
// afterwards persists normally. Registered once per overridden field at boot,
// BEFORE the config is shared, so all copies carry the restores.
func (s *Settings) RegisterLaunchRestore(restore func(*Settings)) {
	if restore != nil {
		s.launchRestores = append(s.launchRestores, restore)
	}
}

// Debug holds developer/troubleshooting switches.
type Debug struct {
	// Menu enables the in game debug UI at runtime: the F2 developer flag
	// menu, the developer keys (U/A/V/B) and the on screen key hint. It is
	// independent of the console/log switches below (those only control
	// diagnostic output). A DEBUG_GAME dev build forces this on regardless;
	// in a release build it is the only way to reach the F2 menu. Default off.
	Menu bool `json:"menu"`
	// Console enables debugging output to the console. On Windows it
	// also keeps the console window visible (Ebitengine normally
	// hides it when the game is started by double click).
	Console bool `json:"console"`
	// LogFile enables debugging output to max-go.log next to the
	// binary. The file is overwritten at every game start.
	LogFile bool `json:"log_file"`
	// LogBufferLines is how many log lines are buffered in memory
	// before they are written to the log file in one go, so logging
	// does not stall the game with a disk write for every line.
	LogBufferLines int `json:"log_buffer_lines"`
}

// Audio selects the sound backend and mixing volumes.
type Audio struct {
	Backend       string `json:"backend"`        // "default" (==ebiten), "ebiten", "soloud", or "miniaudio"
	SoLoudBackend string `json:"soloud_backend"` // SoLoud device backend, one of SoLoudBackends; Validate turns any other value into auto.
	MusicVolume   int    `json:"music_volume"`   // 0..100
	SfxVolume     int    `json:"sfx_volume"`     // 0..100
	VoiceVolume   int    `json:"voice_volume"`   // 0..100
	MusicEnabled  bool   `json:"music_enabled"`
	SfxEnabled    bool   `json:"sfx_enabled"`
	VoiceEnabled  bool   `json:"voice_enabled"`
	// ChannelsReversed swaps left and right for positional unit sounds.
	// C++: settings.ini [AUDIO_SETTINGS] channels_reversed (SoundManager::GetPanning).
	ChannelsReversed bool `json:"channels_reversed"`
}

// Video controls the window and the look of the game UI. The game renders at a
// fixed logical 640x480; WindowScale multiplies that for the desktop window size.
type Video struct {
	Fullscreen       bool           `json:"fullscreen"`
	WindowScale      int            `json:"window_scale"`
	Brightness       int            `json:"brightness"`         // Startup mode 1..5; F10 cycles it.
	Widescreen       WidescreenMode `json:"widescreen"`         // Forced 854x480, forced 640x480, or monitor-selected.
	PopupVanillaLook bool           `json:"popup_vanilla_look"` // Order popup: the retail uniform face (true) or the unused bevel art (false).
}

// Player mirrors the [SETUP] player fields of settings.ini.
type Player struct {
	Name string `json:"name"` // Player name for Multiplayer; single player never displays it.
	Clan int    `json:"clan"`
}

// Game mirrors the [SETUP]/[OPTIONS] gameplay fields of settings.ini:
// startup behavior plus the defaults offered for a new custom game.
type Game struct {
	IntroMovie     bool `json:"intro_movie"`
	AutoSave       bool `json:"auto_save"`
	World          int  `json:"world"`
	TurnTimer      int  `json:"turn_timer"`
	EndTurn        int  `json:"end_turn"`
	StartGold      int  `json:"start_gold"`
	PlayMode       int  `json:"play_mode"`
	VictoryType    int  `json:"victory_type"`
	VictoryLimit   int  `json:"victory_limit"`
	Opponent       int  `json:"opponent"`
	RawResource    int  `json:"raw_resource"`
	FuelResource   int  `json:"fuel_resource"`
	GoldResource   int  `json:"gold_resource"`
	AlienDerelicts int  `json:"alien_derelicts"`
	LastCampaign   int  `json:"last_campaign"` // One-based number of unlocked campaign missions.
}

// Prefs mirrors the [PREFERENCES] section of settings.ini.
type Prefs struct {
	Effects      bool `json:"effects"`
	ClickScroll  bool `json:"click_scroll"`
	QuickScroll  int  `json:"quick_scroll"`
	FastMovement bool `json:"fast_movement"`
	FollowUnit   bool `json:"follow_unit"`
	AutoSelect   bool `json:"auto_select"`
	EnemyHalt    bool `json:"enemy_halt"`
}

// Default returns the settings written on first run. Values follow the
// defaults of the original settings.ini.
func Default() Settings {
	return Settings{
		DataDir:  "",
		Language: "", // no language key in max-go.json: the texts come from LANG.RES
		Audio: Audio{
			Backend:       BackendDefault,
			SoLoudBackend: "auto",
			MusicVolume:   100,
			SfxVolume:     100,
			VoiceVolume:   50,
			MusicEnabled:  true,
			SfxEnabled:    true,
			VoiceEnabled:  true,
		},
		Video: Video{
			Fullscreen:       false,
			WindowScale:      2,
			Brightness:       3,
			Widescreen:       WidescreenAuto,
			PopupVanillaLook: true, // the retail look; the bevel art is opt in
		},
		Player: Player{
			Name: "Player 1",
			Clan: 7,
		},
		Game: Game{
			IntroMovie: true,
			AutoSave:   true,
			World:      0,
			TurnTimer:  180,
			EndTurn:    30,
			StartGold:  200,
			// New games default to turn based; simultaneous play is opt in.
			PlayMode:       0,
			VictoryType:    0,
			VictoryLimit:   100,
			Opponent:       2,
			RawResource:    1,
			FuelResource:   1,
			GoldResource:   1,
			AlienDerelicts: 0,
			LastCampaign:   1,
		},
		Prefs: Prefs{
			Effects:      true,
			ClickScroll:  false,
			QuickScroll:  16,
			FastMovement: true,
			FollowUnit:   false,
			AutoSelect:   false,
			EnemyHalt:    true,
		},
		Debug: Debug{
			Console:        false,
			LogFile:        false,
			LogBufferLines: 100,
		},
		// Remappable actions read by gamescreen.SetKeybinds. Defaults preserve the
		// original keys; survey, status, and options have no original key, so the
		// port assigns Ctrl combinations and leaves plain letters for popup hotkeys.
		Keybinds: map[string]string{
			"end_turn": "Enter",
			"menu":     "Escape",
			"grid":     "G",
			"survey":   "Ctrl+R",
			"status":   "Ctrl+S",
			"find":     "F",
			"help":     "?",
			"options":  "Ctrl+O",
			"zoom_in":  "=",
			"zoom_out": "-",
		},
	}
}

// Validate normalizes missing or invalid values to supported defaults.
func (s *Settings) Validate() {
	if s.Audio.Backend != BackendDefault && s.Audio.Backend != BackendEbiten && s.Audio.Backend != BackendSoLoud && s.Audio.Backend != BackendMiniAudio {
		s.Audio.Backend = BackendDefault
	}
	s.Audio.SoLoudBackend = strings.ToLower(strings.TrimSpace(s.Audio.SoLoudBackend))
	if !slices.Contains(SoLoudBackends, s.Audio.SoLoudBackend) {
		s.Audio.SoLoudBackend = "auto" // empty or unknown: SoLoud picks the device itself
	}
	s.Audio.MusicVolume = max(0, min(s.Audio.MusicVolume, 100))
	s.Audio.SfxVolume = max(0, min(s.Audio.SfxVolume, 100))
	s.Audio.VoiceVolume = max(0, min(s.Audio.VoiceVolume, 100))
	if s.Video.WindowScale < 1 || s.Video.WindowScale > 8 {
		s.Video.WindowScale = 2
	}
	if s.Video.Brightness < 1 || s.Video.Brightness > 5 {
		s.Video.Brightness = 3
	}
	switch s.Video.Widescreen {
	case WidescreenOn, WidescreenOff, WidescreenAuto:
	default:
		// Accept common hand typed spellings; anything else means auto.
		switch string(s.Video.Widescreen) {
		case "on", "yes", "wide", "1":
			s.Video.Widescreen = WidescreenOn
		case "off", "no", "0":
			s.Video.Widescreen = WidescreenOff
		default:
			s.Video.Widescreen = WidescreenAuto
		}
	}
	if s.Debug.LogBufferLines <= 0 {
		s.Debug.LogBufferLines = 100
	} else if s.Debug.LogBufferLines > 100000 {
		s.Debug.LogBufferLines = 100000
	}
	if s.Keybinds == nil {
		s.Keybinds = Default().Keybinds
	} else {
		// Add actions absent from older settings schemas.
		for action, key := range Default().Keybinds {
			if _, ok := s.Keybinds[action]; !ok {
				s.Keybinds[action] = key
			}
		}
		// Move only the legacy Survey/Find collision; preserve an intentional Survey=F binding.
		if s.Keybinds["survey"] == "F" && s.Keybinds["find"] == "F" {
			s.Keybinds["survey"] = "R"
		}
		// Port-pick defaults use Ctrl, leaving plain letters for popup hotkeys. Move
		// only the old default on its own action; preserve every other binding.
		for action, oldKey := range map[string]string{"survey": "R", "status": "S", "options": "O"} {
			if strings.EqualFold(strings.TrimSpace(s.Keybinds[action]), oldKey) {
				s.Keybinds[action] = Default().Keybinds[action]
			}
		}
	}
	if s.Language != "" && !IsSupportedLanguage(s.Language) {
		s.Language = "" // an unknown code counts as absent: the texts come from LANG.RES
	}
}

// Load reads settings from path.
func Load(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Settings{}, err
	}

	// Decode over the defaults, not over a zero Settings. A key the file predates
	// keeps its default instead of reading back as 0/false/"".
	s := Default()
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("settings: parse %s: %w", path, err)
	}
	s.Validate()
	return s, nil
}

// Save writes settings to path as indented JSON. One-run launch overrides are
// backed out first (see RegisterLaunchRestore); the value receiver means the
// restores edit only the copy being written, never the running config.
func (s Settings) Save(path string) error {
	for _, restore := range s.launchRestores {
		restore(&s)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// LoadOrCreate reads settings from path, creating the file with
// defaults if it does not exist yet. A successfully loaded file is
// written back so it always shows the full current schema, including
// keys added since its creation.
func LoadOrCreate(path string) (Settings, error) {
	s, err := Load(path)
	if errors.Is(err, os.ErrNotExist) {
		s = Default()
	} else if err != nil {
		return s, err
	}
	if err := s.Save(path); err != nil {
		return s, fmt.Errorf("settings: write %s: %w", path, err)
	}
	return s, nil
}

// ResolveDataDir returns the directory holding the original game data
// files. Resolution order: the -datadir launch flag, the MAX_DATA_DIR
// environment variable, the data_dir setting, then the directory of the
// binary itself.
func (s Settings) ResolveDataDir() (string, error) {
	if s.dataDirOverride != "" {
		return s.dataDirOverride, nil
	}
	if dir := os.Getenv("MAX_DATA_DIR"); dir != "" {
		return dir, nil
	}
	if s.DataDir != "" {
		return s.DataDir, nil
	}
	return BaseDir()
}

// DataDirExplicit reports whether the data dir points somewhere
// explicitly (the -datadir flag, MAX_DATA_DIR, or the data_dir setting).
// When false, ResolveDataDir falls back to the binary's own directory,
// which is the only case where the first run bootstrap may go hunting
// for an installed M.A.X.; an explicit path errors without a hunt.
func (s Settings) DataDirExplicit() bool {
	return s.dataDirOverride != "" || os.Getenv("MAX_DATA_DIR") != "" || s.DataDir != ""
}

// BaseDir returns the directory the binary lives in, which is where
// the settings file and the saves folder are kept. Under `go run` the
// executable sits in a temporary build directory, in which case the
// current working directory is used instead.
func BaseDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return os.Getwd()
	}
	dir := filepath.Dir(exe)
	if strings.HasPrefix(dir, os.TempDir()) {
		return os.Getwd()
	}
	return dir, nil
}

// ResolvePath returns the settings file to use in dir: max-go.json when
// present, else the legacy settings.json when present, else max-go.json
// so a fresh install creates the new name.
func ResolvePath(dir string) string {
	preferred := filepath.Join(dir, "max-go.json")
	if _, err := os.Stat(preferred); err == nil {
		return preferred
	}

	legacy := filepath.Join(dir, "settings.json")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}

	return preferred
}
