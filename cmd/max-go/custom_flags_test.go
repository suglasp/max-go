// custom_flags_test.go : tests Custom Game command line options.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/settings"
)

func TestCustomFlagsValidParse(t *testing.T) {
	fs := flag.NewFlagSet("custom", flag.ContinueOnError)
	cf := registerCustomFlags(fs)
	if err := fs.Parse([]string{
		"-turnlimit", "120",
		"-endturn", "45",
		"-score", "high",
		"-simultane",
		"-raw", "rich",
		"-fuel", "Poor",
		"-gold", "medium",
		"-derelicts", "common",
		"-startcredit", "TooMuch",
	}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := ValidateCustomFlags(cf); err != nil {
		t.Fatalf("ValidateCustomFlags: %v", err)
	}
	if got := *cf.TurnLimit; got != "120" {
		t.Errorf("TurnLimit = %q, want 120", got)
	}
	if got := *cf.EndTurn; got != "45" {
		t.Errorf("EndTurn = %q, want 45", got)
	}
	if got := *cf.Score; got != "High" {
		t.Errorf("Score = %q, want High", got)
	}
	if !*cf.Simultane {
		t.Error("Simultane = false, want true")
	}
	if got := *cf.Raw; got != "Rich" {
		t.Errorf("Raw = %q, want Rich", got)
	}
	if got := *cf.Fuel; got != "Poor" {
		t.Errorf("Fuel = %q, want Poor", got)
	}
	if got := *cf.Gold; got != "Medium" {
		t.Errorf("Gold = %q, want Medium", got)
	}
	if got := *cf.Derelicts; got != "Common" {
		t.Errorf("Derelicts = %q, want Common", got)
	}
	if got := *cf.StartCredit; got != "TooMuch" {
		t.Errorf("StartCredit = %q, want TooMuch", got)
	}
}

func TestCustomFlagsDefaultsValidate(t *testing.T) {
	fs := flag.NewFlagSet("custom", flag.ContinueOnError)
	cf := registerCustomFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := ValidateCustomFlags(cf); err != nil {
		t.Fatalf("ValidateCustomFlags defaults: %v", err)
	}
	if got := *cf.Raw; got != "Medium" {
		t.Errorf("Raw default = %q, want Medium", got)
	}
	if got := *cf.Derelicts; got != "" {
		t.Errorf("Derelicts default = %q, want empty", got)
	}
}

// TestCustomFlagsDerelicts checks -derelicts parses and normalises, and that the misspelled -derilicts is gone.
func TestCustomFlagsDerelicts(t *testing.T) {
	fs := flag.NewFlagSet("custom", flag.ContinueOnError)
	cf := registerCustomFlags(fs)
	if fs.Lookup("derilicts") != nil {
		t.Error("the misspelled -derilicts flag is still registered")
	}
	if err := fs.Parse([]string{"-derelicts", "rare"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := ValidateCustomFlags(cf); err != nil {
		t.Fatalf("ValidateCustomFlags: %v", err)
	}
	if got := *cf.Derelicts; got != "Rare" {
		t.Errorf("Derelicts = %q, want Rare", got)
	}
}

func TestCustomFlagsShortAliases(t *testing.T) {
	fs := flag.NewFlagSet("custom", flag.ContinueOnError)
	cf := registerCustomFlags(fs)
	if err := fs.Parse([]string{"-turns", "180", "-credit", "limited"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := ValidateCustomFlags(cf); err != nil {
		t.Fatalf("ValidateCustomFlags: %v", err)
	}
	if got := *cf.TurnLimit; got != "180" {
		t.Errorf("TurnLimit = %q, want 180", got)
	}
	if got := *cf.StartCredit; got != "Limited" {
		t.Errorf("StartCredit = %q, want Limited", got)
	}
}

// TestApplyCustomGameFlags checks that a full set of passed flags folds into the
// game settings with the faithful setup screen values.
func TestApplyCustomGameFlags(t *testing.T) {
	fs := flag.NewFlagSet("custom", flag.ContinueOnError)
	cf := registerCustomFlags(fs)
	args := []string{
		"-turnlimit", "240",
		"-endturn", "60",
		"-score", "high",
		"-startcredit", "TooMuch",
		"-raw", "rich",
		"-fuel", "poor",
		"-gold", "medium",
		"-derelicts", "common",
		"-simultane",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := ValidateCustomFlags(cf); err != nil {
		t.Fatalf("ValidateCustomFlags: %v", err)
	}

	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = true
	})

	g := settings.Default().Game
	applyCustomGameFlags(&g, cf, provided)

	// Turn clock and end of turn grace.
	if g.TurnTimer != 240 {
		t.Errorf("TurnTimer = %d, want 240", g.TurnTimer)
	}
	if g.EndTurn != 60 {
		t.Errorf("EndTurn = %d, want 60", g.EndTurn)
	}
	// -score High is a points victory of 800.
	if g.VictoryType != 1 || g.VictoryLimit != 800 {
		t.Errorf("victory = type %d limit %d, want type 1 limit 800", g.VictoryType, g.VictoryLimit)
	}
	// -startcredit TooMuch is the top 250 bucket.
	if g.StartGold != 250 {
		t.Errorf("StartGold = %d, want 250", g.StartGold)
	}
	// Resource densities: rich=2, poor=0, medium=1, derelicts common=2.
	if g.RawResource != 2 {
		t.Errorf("RawResource = %d, want 2", g.RawResource)
	}
	if g.FuelResource != 0 {
		t.Errorf("FuelResource = %d, want 0", g.FuelResource)
	}
	if g.GoldResource != 1 {
		t.Errorf("GoldResource = %d, want 1", g.GoldResource)
	}
	if g.AlienDerelicts != 2 {
		t.Errorf("AlienDerelicts = %d, want 2", g.AlienDerelicts)
	}
	// -simultane selects play mode 1.
	if g.PlayMode != 1 {
		t.Errorf("PlayMode = %d, want 1", g.PlayMode)
	}
}

// TestApplyCustomGameFlagsDuration checks that -duration folds into a turn count
// victory (type 0) with the passed limit.
func TestApplyCustomGameFlagsDuration(t *testing.T) {
	fs := flag.NewFlagSet("custom", flag.ContinueOnError)
	cf := registerCustomFlags(fs)
	if err := fs.Parse([]string{"-duration", "200"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := ValidateCustomFlags(cf); err != nil {
		t.Fatalf("ValidateCustomFlags: %v", err)
	}

	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = true
	})

	g := settings.Default().Game
	g.VictoryType = 1 // a saved score victory the flag must replace
	g.VictoryLimit = 800
	applyCustomGameFlags(&g, cf, provided)

	if g.VictoryType != 0 || g.VictoryLimit != 200 {
		t.Errorf("victory = type %d limit %d, want type 0 limit 200", g.VictoryType, g.VictoryLimit)
	}
}

// TestApplyCustomGameFlagsUnsetKeepsSaved checks that flags the player did NOT
// pass leave the saved settings untouched, even for raw/fuel/gold which default
// to a non empty Medium.
func TestApplyCustomGameFlagsUnsetKeepsSaved(t *testing.T) {
	fs := flag.NewFlagSet("custom", flag.ContinueOnError)
	cf := registerCustomFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := ValidateCustomFlags(cf); err != nil {
		t.Fatalf("ValidateCustomFlags: %v", err)
	}

	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = true
	})

	g := settings.Default().Game
	g.RawResource = 2 // a saved Rich level the default Medium must not clobber
	g.TurnTimer = 120
	applyCustomGameFlags(&g, cf, provided)

	if g.RawResource != 2 {
		t.Errorf("RawResource = %d, want 2 (no -raw passed)", g.RawResource)
	}
	if g.TurnTimer != 120 {
		t.Errorf("TurnTimer = %d, want 120 (no -turnlimit passed)", g.TurnTimer)
	}
}

func TestCustomFlagsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"turnlimit low", []string{"-turnlimit", "59"}, "60 to 250"},
		{"turnlimit text", []string{"-turnlimit", "many"}, "60 to 250"},
		{"endturn high", []string{"-endturn", "91"}, "15 to 90"},
		{"score", []string{"-score", "Huge"}, `"", Low, Medium, High`},
		{"duration low", []string{"-duration", "0"}, "1 to 9999"},
		{"duration text", []string{"-duration", "forever"}, "1 to 9999"},
		{"victory both", []string{"-score", "High", "-duration", "200"}, "flags -score and -duration cannot be combined"},
		{"mode", []string{"-turnbased", "-simultane"}, "flags -turnbased and -simultane cannot be combined"},
		{"raw", []string{"-raw", "Dense"}, "Poor, Medium, Rich"},
		{"fuel", []string{"-fuel", "Dense"}, "Poor, Medium, Rich"},
		{"gold", []string{"-gold", "Dense"}, "Poor, Medium, Rich"},
		{"derelicts", []string{"-derelicts", "Many"}, `"", None, Rare, Common`},
		{"startcredit", []string{"-startcredit", "Unlimited"}, `"", Low, Limited, Normal, High, TooMuch`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("custom", flag.ContinueOnError)
			cf := registerCustomFlags(fs)
			if err := fs.Parse(tt.args); err != nil {
				t.Fatalf("Parse: %v", err)
			}
			err := ValidateCustomFlags(cf)
			if err == nil {
				t.Fatal("ValidateCustomFlags returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}
