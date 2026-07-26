// main_test.go : tests max-go launch behavior.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestPrintUsageMatchesLaunchFlags(t *testing.T) {
	fs := flag.NewFlagSet("max-go", flag.ContinueOnError)
	fs.SetOutput(new(bytes.Buffer))
	defineLaunchFlags(fs)

	var out bytes.Buffer
	printUsage(&out, "max-go", fs)
	text := out.String()

	fs.VisitAll(func(f *flag.Flag) {
		// -demo is an undocumented dev flag: deliberately kept out of -help.
		// -mapeditor stays undocumented too until the ME7 editor ships (user gated).
		if f.Name == "demo" || f.Name == "mapeditor" {
			return
		}
		if !strings.Contains(text, "-"+f.Name) {
			t.Errorf("usage output missing flag -%s", f.Name)
		}
	})

	want := []string{
		"max-go -random -npc 3 -difficulty 3 -derilicts common",
		"max-go -screenshot <file>",
		"max-go -mve <file> [-screenshot <file>]",
		"max-go -flic <file> [-screenshot <file>]",
		"-mve <file>",
		"-difficulty <0..5>",
		"-npc <1..3>",
		"-score <Low|Medium|High>",
		"-turnlimit <60..250>",
		"-turns <60..250>",
		"-startcredit <Low|Limited|Normal|High|TooMuch>",
		"-credit <Low|Limited|Normal|High|TooMuch>",
		"-datadir <path>",
		"-data <path>",
		"-playername <name>",
		"-audio <native|ebiten|soloud|miniaudio>",
		"-scale <1..8>",
		"-windowed",
		"-fullscreen",
		"-debug",
		"-reset",
		"-mve exits after movie playback",
		"-flic exits after animation playback",
	}

	// -emulate is registered and documented only in a DEBUG_GAME build, so its two usage
	// lines and the screen names it lists belong to that build alone. A release build must
	// not advertise the developer surface at all, which the negative check below pins.
	emulateOnly := []string{
		"max-go -emulate <screen> [-screenshot <file>]",
		"max-go -emulate sprite:<tag> [-screenshot <file>]",
		"planetselect", "newgame", "multiplayer", "gameconfig", "chooseplayer",
		"clanselect", "mapview", "drawmap", "shadow", "build", "sprite:<tag>",
	}
	if DEBUG_GAME {
		want = append(want, emulateOnly...)
	} else if strings.Contains(text, "-emulate") {
		t.Error("a release build (DEBUG_GAME false) must not document -emulate in its usage")
	}

	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("usage output missing %q", w)
		}
	}
	for _, old := range []string{
		"-mve string",
		"-difficulty int",
		"-npc int",
		"-startcredit string",
		"-credit string",
		"-turnlimit string",
		"-turns string",
		"-datadir string",
		"-playername string",
		"-audio string",
		"-scale int",
	} {
		if strings.Contains(text, old) {
			t.Errorf("usage output still has default Go flag rendering %q", old)
		}
	}
}

func TestDefineLaunchFlagsRegistersCustomFlags(t *testing.T) {
	fs := flag.NewFlagSet("max-go", flag.ContinueOnError)
	fs.SetOutput(new(bytes.Buffer))
	flags := defineLaunchFlags(fs)
	if flags.custom == nil {
		t.Fatal("custom flags were not registered")
	}

	for _, name := range []string{
		"turnlimit",
		"turns",
		"endturn",
		"score",
		"turnbased",
		"simultane",
		"raw",
		"fuel",
		"gold",
		"derilicts",
		"derelicts",
		"startcredit",
		"credit",
	} {
		if fs.Lookup(name) == nil {
			t.Errorf("flag -%s is not registered", name)
		}
	}
}

func TestDefineLaunchFlagsRegistersOverrideFlags(t *testing.T) {
	fs := flag.NewFlagSet("max-go", flag.ContinueOnError)
	fs.SetOutput(new(bytes.Buffer))
	flags := defineLaunchFlags(fs)
	if flags.override == nil {
		t.Fatal("override flags were not registered")
	}

	for _, name := range []string{
		"datadir",
		"data",
		"playername",
		"audio",
		"scale",
		"windowed",
		"fullscreen",
		"debug",
		"reset",
	} {
		if fs.Lookup(name) == nil {
			t.Errorf("flag -%s is not registered", name)
		}
	}
}
