// launch_args.go : parses the launch flags and stops a bad command line before the window opens.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/bootstrap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/saveload"
	"github.com/suglasp/max-go/internal/settings"
)

// Exit codes, the same in every max-go binary: 0 for success (help included), 1 for a runtime failure
// (a missing file, a load or save error), 2 for a bad command line.
const (
	exitRuntime     = 1
	exitCommandLine = 2
)

// valueFlags take a value that cannot be empty, so an explicit -name= is refused. The custom game flags are
// not listed: an empty value is their documented "unset".
var valueFlags = []string{"screenshot", "mve", "flic", "emulate", "mission", "save", "language", "playername", "datadir", "data", "audio"}

// startModeFlags each pick what the run starts, and a run takes one of them. -rng is the alias of -random.
var startModeFlags = []string{"demo", "mve", "flic", "emulate", "mission", "save", "random", "rng", "mapeditor"}

// commandLineError is a bad command line: an unknown flag, a bad or empty value, a conflict or a stray
// argument. The run stops with exit code 2.
type commandLineError struct {
	err error
}

func (e commandLineError) Error() string { return e.err.Error() }

func (e commandLineError) Unwrap() error { return e.err }

// badCommandLine marks err as a command line error; nil stays nil.
func badCommandLine(err error) error {
	if err == nil {
		return nil
	}
	return commandLineError{err: err}
}

// launchExitCode maps a launch error to its exit code: 2 for a bad command line, 1 for anything else.
func launchExitCode(err error) int {
	var cle commandLineError
	if errors.As(err, &cle) {
		return exitCommandLine
	}
	return exitRuntime
}

// exitLaunchError ends the run on a launch error. A bad command line prints its reason and a hint and exits
// with code 2; any other error goes through fatalf (exit code 1).
func exitLaunchError(err error) {
	if launchExitCode(err) == exitCommandLine {
		fmt.Fprintf(os.Stderr, "max-go: %v\nRun max-go -help for the list of flags.\n", err)
		os.Exit(exitCommandLine)
	}
	fatalf("max-go: %v", err)
}

// parseLaunchArgs registers the launch flags on fs, parses args and checks every value that needs no file
// system: the parse itself, stray arguments, empty values, the ranges and names, and the start mode
// conflicts. -help comes back as flag.ErrHelp; every other error is a commandLineError.
func parseLaunchArgs(fs *flag.FlagSet, args []string) (launchFlags, error) {
	flags := defineLaunchFlags(fs)
	fs.Init(fs.Name(), flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller prints the reason in the max-go form, or the full help for -help

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return flags, err
		}
		return flags, badCommandLine(err)
	}

	flags.provided = map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		flags.provided[f.Name] = true
	})

	if fs.NArg() > 0 {
		return flags, badCommandLine(unexpectedArgument(fs, fs.Arg(0)))
	}

	for _, name := range valueFlags {
		if flags.provided[name] && strings.TrimSpace(fs.Lookup(name).Value.String()) == "" {
			return flags, badCommandLine(fmt.Errorf("flag -%s needs a value", name))
		}
	}

	// 0 is the unset default of -scale; given on the command line it is out of range.
	if flags.provided["scale"] && intValue(flags.override.Scale) == 0 {
		return flags, badCommandLine(fmt.Errorf("flag -scale must be an integer from 1 to 8, got 0"))
	}

	for _, err := range []error{
		ValidateCustomFlags(flags.custom),
		ValidateOverrideFlags(flags.override),
		validateDifficulty(*flags.difficulty),
		validateNPC(*flags.npc),
		validateEmulate(*flags.emulate),
		validateSaveSlot(*flags.save),
		startModeConflict(fs, flags.provided),
	} {
		if err != nil {
			return flags, badCommandLine(err)
		}
	}

	return flags, nil
}

// unexpectedArgument explains a stray argument: a flag name typed without its dash, or a true / false
// after a switch (a bool flag only takes its value as -name=false).
func unexpectedArgument(fs *flag.FlagSet, arg string) error {
	if strings.EqualFold(arg, "true") || strings.EqualFold(arg, "false") {
		return fmt.Errorf("unexpected argument %q; a switch takes its value as -name=%s", arg, strings.ToLower(arg))
	}
	if fs.Lookup(arg) != nil {
		return fmt.Errorf("unexpected argument %q; did you mean -%s?", arg, arg)
	}
	return fmt.Errorf("unexpected argument %q", arg)
}

// validateNPC checks the -npc count: one to three computer opponents (four team slots, one is the player).
func validateNPC(n int) error {
	if n < 1 || n > game.TeamCount-1 {
		return fmt.Errorf("flag -npc must be an integer from 1 to %d, got %d", game.TeamCount-1, n)
	}
	return nil
}

// validateEmulate checks an -emulate screen name: one of menuScreens, or sprite:<tag> with a tag. mapeditor is
// accepted too, unlisted like the -mapeditor flag while the editor is work in progress.
func validateEmulate(name string) error {
	if tag, ok := strings.CutPrefix(name, "sprite:"); ok {
		if strings.TrimSpace(tag) == "" {
			return fmt.Errorf("flag -emulate sprite: needs a sprite tag, for example sprite:APC1")
		}
		return nil
	}
	if name == "" || name == "mapeditor" || slices.Contains(menuScreens, name) {
		return nil
	}
	return fmt.Errorf("flag -emulate: unknown screen %q; the screens are: %s", name, menuScreensHelp)
}

// validateSaveSlot checks the -save value is a slot number: an integer from 1 up.
func validateSaveSlot(ref string) error {
	if ref == "" {
		return nil
	}
	if slot, err := strconv.Atoi(strings.TrimSpace(ref)); err != nil || slot < 1 {
		return fmt.Errorf("flag -save must be a slot number from 1 up, got %q", ref)
	}
	return nil
}

// startModeConflict refuses two start mode flags in one run. A switch given as -name=false starts nothing.
func startModeConflict(fs *flag.FlagSet, provided map[string]bool) error {
	var given []string
	for _, name := range startModeFlags {
		if !provided[name] || (name == "rng" && provided["random"]) {
			continue
		}
		if v := fs.Lookup(name).Value.String(); v == "" || v == "false" {
			continue
		}
		given = append(given, name)
	}
	if len(given) > 1 {
		return fmt.Errorf("flags -%s and -%s cannot be combined; a run starts one of them", given[0], given[1])
	}
	return nil
}

// checkLaunchTargets checks what the flags point at, once the settings name the data and base directories
// and before the window opens: the -datadir directory, the -mve / -flic / -mission files, the -save slot and
// the -language strings. A missing target is a runtime failure (exit code 1); an unknown language code is a
// bad value (exit code 2).
func checkLaunchTargets(cfg settings.Settings, baseDir string, flags launchFlags) error {
	if dir := strings.TrimSpace(stringValue(flags.override.DataDir)); dir != "" {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			return fmt.Errorf("flag -datadir: no such directory %q", dir)
		}
	}

	dataDir, err := cfg.ResolveDataDir()
	if err != nil {
		return err
	}

	if err := checkMediaFile(cfg, "mve", *flags.movie, ".MVE"); err != nil {
		return err
	}
	if err := checkMediaFile(cfg, "flic", *flags.flic, ".FLC"); err != nil {
		return err
	}

	// A data dir without MAX.RES starts the first run bootstrap, which may find the data elsewhere and keeps
	// the -mission jump, so a base name is only looked up in a data dir that holds the game.
	if ref := *flags.mission; ref != "" && !isFile(ref) && bootstrap.HasMaxRes(dataDir) && !isFile(resfile.DataPath(dataDir, ref)) {
		return fmt.Errorf("flag -mission: no mission file %q in the current folder or in %s", ref, dataDir)
	}

	if ref := strings.TrimSpace(*flags.save); ref != "" {
		slot, _ := strconv.Atoi(ref) // validateSaveSlot already checked the number
		if _, ok := saveload.FindSlot(baseDir, slot); !ok {
			return fmt.Errorf("flag -save: no save in slot %d", slot)
		}
	}

	if code := strings.TrimSpace(*flags.language); code != "" && !settings.IsSupportedLanguage(code) && !assets.HasLanguage(dataDir, code) {
		return badCommandLine(fmt.Errorf("flag -language: no strings for %q; the bundled language is %s, others need a %s next to the data or the binary",
			code, strings.Join(settings.SupportedLanguages, ", "), assets.LanguageFile(code)))
	}

	return nil
}

// checkMediaFile checks that an -mve / -flic value names a file, looked up the way the player does.
func checkMediaFile(cfg settings.Settings, flagName, ref, defaultExt string) error {
	if ref == "" {
		return nil
	}
	path, err := resolveMediaPath(cfg, ref, defaultExt)
	if err != nil {
		return err
	}
	if !isFile(path) {
		return fmt.Errorf("flag -%s: no such file %q", flagName, ref)
	}
	return nil
}

// isFile reports whether path names an existing file (not a directory).
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
