// launch_args_test.go : testing unit for launch_args.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/saveload"
	"github.com/suglasp/max-go/internal/settings"
)

// parseTestArgs runs parseLaunchArgs on a fresh flag set, as main does on the command line.
func parseTestArgs(args ...string) (launchFlags, error) {
	return parseLaunchArgs(flag.NewFlagSet("max-go", flag.ContinueOnError), args)
}

// TestParseLaunchArgs runs every command line check: a good line parses, a bad one returns a command line
// error (exit code 2) whose text names the problem.
func TestParseLaunchArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      string // substring of the error; "" means the line is good
		debugOnly bool   // -emulate exists in a DEBUG_GAME build only
	}{
		{name: "no flags"},
		{name: "npc 1", args: []string{"-npc", "1"}},
		{name: "npc 3", args: []string{"-random", "-npc", "3"}},
		{name: "npc 0", args: []string{"-npc", "0"}, want: "flag -npc must be an integer from 1 to 3, got 0"},
		{name: "npc 4", args: []string{"-npc", "4"}, want: "from 1 to 3, got 4"},
		{name: "npc -1", args: []string{"-npc", "-1"}, want: "from 1 to 3, got -1"},
		{name: "unknown flag", args: []string{"-bogus"}, want: "flag provided but not defined: -bogus"},
		{name: "bad int", args: []string{"-npc", "x"}, want: `invalid value "x" for flag -npc`},
		{name: "missing argument", args: []string{"-npc"}, want: "flag needs an argument: -npc"},
		{name: "flag name without dash", args: []string{"random"}, want: `unexpected argument "random"; did you mean -random?`},
		{name: "switch with a value", args: []string{"-random", "false"}, want: "a switch takes its value as -name=false"},
		{name: "stray argument", args: []string{"-npc", "2", "foo", "-random"}, want: `unexpected argument "foo"`},
		{name: "switch as -name=false", args: []string{"-random=false"}},
		{name: "audio default", args: []string{"-audio", "default"}},
		{name: "audio ebiten", args: []string{"-audio", "Ebiten"}},
		{name: "audio unknown", args: []string{"-audio", "sdl"}, want: "flag -audio must be one of native, default"},
		{name: "scale given as 0", args: []string{"-scale", "0"}, want: "flag -scale must be an integer from 1 to 8, got 0"},
		{name: "scale 1", args: []string{"-scale", "1"}},
		{name: "scale 8", args: []string{"-scale", "8"}},
		{name: "scale 9", args: []string{"-scale", "9"}, want: "from 1 to 8, got 9"},
		{name: "windowed and fullscreen", args: []string{"-windowed", "-fullscreen"}, want: "flags -windowed and -fullscreen cannot be combined"},
		{name: "difficulty unset", args: []string{"-difficulty", "-1"}},
		{name: "difficulty 6", args: []string{"-difficulty", "6"}, want: "flag -difficulty must be an integer from 0 to 5"},
		{name: "turnlimit empty is unset", args: []string{"-turnlimit="}},
		{name: "turnlimit 251", args: []string{"-turnlimit", "251"}, want: "flag -turnlimit must be empty or an integer from 60 to 250"},
		{name: "endturn 14", args: []string{"-endturn", "14"}, want: "flag -endturn must be empty or an integer from 15 to 90"},
		{name: "duration 10000", args: []string{"-duration", "10000"}, want: "flag -duration must be empty or an integer from 1 to 9999"},
		{name: "score and duration", args: []string{"-score", "High", "-duration", "200"}, want: "flags -score and -duration cannot be combined"},
		{name: "turnbased and simultane", args: []string{"-turnbased", "-simultane"}, want: "flags -turnbased and -simultane cannot be combined"},
		{name: "raw empty", args: []string{"-raw="}, want: "flag -raw must be one of Poor, Medium, Rich"},
		{name: "derelicts none", args: []string{"-derelicts", "none"}},
		{name: "emulate screen", args: []string{"-emulate", "random2"}, debugOnly: true},
		{name: "emulate sprite", args: []string{"-emulate", "sprite:APC1"}, debugOnly: true},
		{name: "emulate map editor", args: []string{"-emulate", "mapeditor"}, debugOnly: true},
		{name: "emulate unknown", args: []string{"-emulate", "xyz"}, want: `flag -emulate: unknown screen "xyz"; the screens are: planetselect`, debugOnly: true},
		{name: "emulate sprite without tag", args: []string{"-emulate", "sprite:"}, want: "needs a sprite tag", debugOnly: true},
		{name: "emulate case", args: []string{"-emulate", "Random"}, want: "unknown screen", debugOnly: true},
		{name: "save slot", args: []string{"-save", "3"}},
		{name: "save text", args: []string{"-save", "abc"}, want: `flag -save must be a slot number from 1 up, got "abc"`},
		{name: "save 0", args: []string{"-save", "0"}, want: "slot number from 1 up"},
		{name: "save negative", args: []string{"-save", "-3"}, want: "slot number from 1 up"},
		{name: "mission and save", args: []string{"-mission", "SAVE1.TRA", "-save", "1"}, want: "flags -mission and -save cannot be combined"},
		{name: "mve and flic", args: []string{"-mve", "a", "-flic", "b"}, want: "flags -mve and -flic cannot be combined"},
		{name: "demo and map editor", args: []string{"-demo", "-mapeditor"}, want: "flags -demo and -mapeditor cannot be combined"},
		{name: "rng and mission", args: []string{"-rng", "-mission", "x"}, want: "flags -mission and -rng cannot be combined"},
		{name: "random and emulate", args: []string{"-random", "-emulate", "game"}, want: "flags -emulate and -random cannot be combined", debugOnly: true},
		{name: "random and rng", args: []string{"-random", "-rng"}},
		{name: "demo off", args: []string{"-demo=false", "-random"}},
		{name: "boot switches together", args: []string{"-skipintro", "-nomovie", "-nosplash", "-random"}},
		{name: "screenshot with a start mode", args: []string{"-random", "-screenshot", "shot.png"}},
	}

	// Every value flag refuses an explicit empty value.
	for _, name := range valueFlags {
		if name == "emulate" && !DEBUG_GAME {
			continue
		}
		tests = append(tests, struct {
			name      string
			args      []string
			want      string
			debugOnly bool
		}{name: "empty " + name, args: []string{"-" + name + "="}, want: fmt.Sprintf("flag -%s needs a value", name)})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.debugOnly && !DEBUG_GAME {
				t.Skip("-emulate is registered in a DEBUG_GAME build only")
			}
			_, err := parseTestArgs(tt.args...)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("%v: unexpected error %v", tt.args, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%v: no error, want %q", tt.args, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%v: error %q, want it to contain %q", tt.args, err, tt.want)
			}
			if code := launchExitCode(err); code != exitCommandLine {
				t.Errorf("%v: exit code %d, want %d", tt.args, code, exitCommandLine)
			}
		})
	}
}

// TestParseLaunchArgsHelp returns flag.ErrHelp for -help, which main answers with the full help and exit code 0.
func TestParseLaunchArgsHelp(t *testing.T) {
	for _, arg := range []string{"-help", "-h"} {
		if _, err := parseTestArgs(arg); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("%s: error %v, want flag.ErrHelp", arg, err)
		}
	}
}

// TestParseLaunchArgsRecordsProvided keeps the given flags, an alias under its own name, for the settings fold.
func TestParseLaunchArgsRecordsProvided(t *testing.T) {
	flags, err := parseTestArgs("-raw", "Rich", "-data", t.TempDir(), "-credit", "High")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"raw", "data", "credit"} {
		if !flags.provided[name] {
			t.Errorf("provided[%q] = false, want true", name)
		}
	}
	for _, name := range []string{"fuel", "gold", "datadir", "startcredit"} {
		if flags.provided[name] {
			t.Errorf("provided[%q] = true for a flag that was not given", name)
		}
	}
}

// TestMenuScreensAllPassEmulate keeps the help list and the -emulate check one list: every listed screen
// passes, and the help prints every one of them.
func TestMenuScreensAllPassEmulate(t *testing.T) {
	for _, name := range menuScreens {
		if name == "sprite:<tag>" {
			continue
		}
		if err := validateEmulate(name); err != nil {
			t.Errorf("listed screen %q refused: %v", name, err)
		}
		if !strings.Contains(menuScreensHelp, name) {
			t.Errorf("help list misses %q", name)
		}
	}
}

// TestNPCCountIsTheFlag: with -npc checked at startup, the count is the flag value as given.
func TestNPCCountIsTheFlag(t *testing.T) {
	saved := npcOverride
	t.Cleanup(func() { npcOverride = saved })
	for n := 1; n <= 3; n++ {
		npcOverride = n
		if got := npcCount(); got != n {
			t.Errorf("npcCount with -npc %d = %d", n, got)
		}
	}
}

// TestLaunchExitCode maps a command line error (wrapped or not) to 2 and anything else to 1.
func TestLaunchExitCode(t *testing.T) {
	bad := badCommandLine(errors.New("flag -npc must be an integer from 1 to 3, got 9"))
	if code := launchExitCode(bad); code != exitCommandLine {
		t.Errorf("command line error: exit code %d, want %d", code, exitCommandLine)
	}
	if code := launchExitCode(fmt.Errorf("launch: %w", bad)); code != exitCommandLine {
		t.Errorf("wrapped command line error: exit code %d, want %d", code, exitCommandLine)
	}
	if code := launchExitCode(errors.New("open SAVE9.TRA: no such file")); code != exitRuntime {
		t.Errorf("runtime error: exit code %d, want %d", code, exitRuntime)
	}
	if badCommandLine(nil) != nil {
		t.Error("badCommandLine(nil) is not nil")
	}
}

// TestCheckLaunchTargets checks the targets once the settings are loaded: a missing directory, file or save
// slot is a runtime failure (exit code 1), an unknown language a bad value (exit code 2).
func TestCheckLaunchTargets(t *testing.T) {
	base := t.TempDir()
	data := t.TempDir()
	missionFile := filepath.Join(t.TempDir(), "OWN.SCE")
	for _, path := range []string{
		filepath.Join(data, "MAX.RES"),
		filepath.Join(data, "SAVE1.TRA"),
		filepath.Join(data, "INTRO.MVE"),
		filepath.Join(data, "lang-xx-test.ini"),
		missionFile,
	} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	savePath, err := saveload.Path(base, "save2")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(savePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A data dir without MAX.RES: the first run bootstrap still has to find the game.
	noGame := t.TempDir()

	tests := []struct {
		name    string
		dataDir string
		args    []string
		want    string // substring of the error; "" means the targets are fine
		code    int
	}{
		{name: "no targets", dataDir: data},
		{name: "datadir given", dataDir: data, args: []string{"-datadir", data}},
		{name: "datadir missing", dataDir: data, args: []string{"-datadir", filepath.Join(data, "nope")}, want: "flag -datadir: no such directory", code: exitRuntime},
		{name: "datadir is a file", dataDir: data, args: []string{"-data", filepath.Join(data, "MAX.RES")}, want: "no such directory", code: exitRuntime},
		{name: "mission in the data dir", dataDir: data, args: []string{"-mission", "save1.tra"}},
		{name: "mission by path", dataDir: data, args: []string{"-mission", missionFile}},
		{name: "mission missing", dataDir: data, args: []string{"-mission", "NOPE.TRA"}, want: `flag -mission: no mission file "NOPE.TRA"`, code: exitRuntime},
		{name: "mission before the bootstrap", dataDir: noGame, args: []string{"-mission", "NOPE.TRA"}},
		{name: "save slot", dataDir: data, args: []string{"-save", "2"}},
		{name: "save slot missing", dataDir: data, args: []string{"-save", "7"}, want: "flag -save: no save in slot 7", code: exitRuntime},
		{name: "movie", dataDir: data, args: []string{"-mve", "INTRO"}},
		{name: "movie missing", dataDir: data, args: []string{"-mve", "NOPE"}, want: `flag -mve: no such file "NOPE"`, code: exitRuntime},
		{name: "flic missing", dataDir: data, args: []string{"-flic", "NOPE"}, want: `flag -flic: no such file "NOPE"`, code: exitRuntime},
		{name: "language bundled", dataDir: data, args: []string{"-language", "en-us"}},
		{name: "language loose ini", dataDir: data, args: []string{"-language", "xx-test"}},
		{name: "language unknown", dataDir: data, args: []string{"-language", "zz-test"}, want: `flag -language: no strings for "zz-test"`, code: exitCommandLine},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := parseTestArgs(tt.args...)
			if err != nil {
				t.Fatalf("parse %v: %v", tt.args, err)
			}
			cfg := settings.Default()
			cfg.SetDataDirOverride(tt.dataDir) // wins over MAX_DATA_DIR in the test environment
			applyLaunchOverrides(&cfg, flags.override)

			err = checkLaunchTargets(cfg, base, flags)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("%v: unexpected error %v", tt.args, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%v: no error, want %q", tt.args, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%v: error %q, want it to contain %q", tt.args, err, tt.want)
			}
			if code := launchExitCode(err); code != tt.code {
				t.Errorf("%v: exit code %d, want %d", tt.args, code, tt.code)
			}
		})
	}
}

// launchExitChildEnv makes the test binary end the way a launch error ends max-go.
const launchExitChildEnv = "MAX_GO_LAUNCH_EXIT_CHILD"

// TestLaunchExitChild is the failing run itself; it only acts when started by TestLaunchErrorsExit.
func TestLaunchExitChild(t *testing.T) {
	switch os.Getenv(launchExitChildEnv) {
	case "":
		t.Skip("only runs as the child of TestLaunchErrorsExit")
	case "commandline":
		exitLaunchError(badCommandLine(errors.New("flag -npc must be an integer from 1 to 3, got 9")))
	case "runtime":
		exitLaunchError(errors.New("flag -save: no save in slot 7"))
	}
}

// TestLaunchErrorsExit runs both launch error kinds in a child process: one "max-go: ..." line on stderr
// without a log date prefix, the hint for a bad command line only, and the exit codes 2 and 1.
func TestLaunchErrorsExit(t *testing.T) {
	tests := []struct {
		kind     string
		code     int
		wantText string
	}{
		{"commandline", exitCommandLine, "max-go: flag -npc must be an integer from 1 to 3, got 9\nRun max-go -help for the list of flags.\n"},
		{"runtime", exitRuntime, "max-go: flag -save: no save in slot 7\n"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestLaunchExitChild$")
			cmd.Env = append(os.Environ(), launchExitChildEnv+"="+tt.kind)
			var stderr strings.Builder
			cmd.Stderr = &stderr
			err := cmd.Run()

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("child run: %v, want an exit error", err)
			}
			if got := exitErr.ExitCode(); got != tt.code {
				t.Errorf("exit code %d, want %d", got, tt.code)
			}
			if got := strings.ReplaceAll(stderr.String(), "\r\n", "\n"); got != tt.wantText {
				t.Errorf("stderr = %q, want %q", got, tt.wantText)
			}
		})
	}
}
