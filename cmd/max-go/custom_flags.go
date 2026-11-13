// custom_flags.go : parses and validates Custom Game command line options.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/suglasp/max-go/internal/settings"
)

// CustomGameFlags holds command line overrides for the Custom Game setup
// screen. This file parses and validates them before the game setup applies
// them to StartParams.
type CustomGameFlags struct {
	TurnLimit   *string
	EndTurn     *string
	Score       *string
	Duration    *string
	TurnBased   *bool
	Simultane   *bool
	Raw         *string
	Fuel        *string
	Gold        *string
	Derelicts   *string
	StartCredit *string
}

// registerCustomFlags registers command-line counterparts for Custom Game setup fields.
//
// C++ ported : gameconfigmenu.cpp - GameConfigMenu event handlers.
func registerCustomFlags(fs *flag.FlagSet) *CustomGameFlags {
	flags := &CustomGameFlags{}
	flags.TurnLimit = fs.String("turnlimit", "", "custom game turn timer seconds (the setup screen Turn Time): 60..250")
	fs.StringVar(flags.TurnLimit, "turns", "", "alias of -turnlimit")
	flags.EndTurn = fs.String("endturn", "", "custom game end of turn grace seconds: 15..90")
	flags.Score = fs.String("score", "", "custom game score victory target: Low, Medium, High")
	flags.Duration = fs.String("duration", "", "custom game duration victory turn count: 1..9999")
	flags.TurnBased = fs.Bool("turnbased", false, "custom game play mode: turn-based")
	flags.Simultane = fs.Bool("simultane", false, "custom game play mode: simultaneous")
	flags.Raw = fs.String("raw", "Medium", "custom game raw density: Poor, Medium, Rich")
	flags.Fuel = fs.String("fuel", "Medium", "custom game fuel density: Poor, Medium, Rich")
	flags.Gold = fs.String("gold", "Medium", "custom game gold density: Poor, Medium, Rich")
	flags.Derelicts = fs.String("derelicts", "", "custom game alien derelict density: none, Rare, Common")
	flags.StartCredit = fs.String("startcredit", "", "custom game starting credits: Low, Limited, Normal, High, TooMuch")
	fs.StringVar(flags.StartCredit, "credit", "", "alias of -startcredit")
	return flags
}

// ValidateCustomFlags checks Custom Game CLI values and normalizes named values
// to their canonical spelling.
func ValidateCustomFlags(flags *CustomGameFlags) error {
	if flags == nil {
		return nil
	}
	if err := validateOptionalInt(flags.TurnLimit, "turnlimit", 60, 250); err != nil {
		return err
	}
	if err := validateOptionalInt(flags.EndTurn, "endturn", 15, 90); err != nil {
		return err
	}
	if err := validateChoice(flags.Score, "score", "", "Low", "Medium", "High"); err != nil {
		return err
	}
	if err := validateOptionalInt(flags.Duration, "duration", 1, 9999); err != nil {
		return err
	}
	if strings.TrimSpace(stringValue(flags.Score)) != "" && strings.TrimSpace(stringValue(flags.Duration)) != "" {
		return fmt.Errorf("flags -score and -duration cannot be combined (one victory condition)")
	}
	if boolValue(flags.TurnBased) && boolValue(flags.Simultane) {
		return fmt.Errorf("flags -turnbased and -simultane cannot be combined")
	}
	if err := validateChoice(flags.Raw, "raw", "Poor", "Medium", "Rich"); err != nil {
		return err
	}
	if err := validateChoice(flags.Fuel, "fuel", "Poor", "Medium", "Rich"); err != nil {
		return err
	}
	if err := validateChoice(flags.Gold, "gold", "Poor", "Medium", "Rich"); err != nil {
		return err
	}
	if err := validateChoice(flags.Derelicts, "derelicts", "", "None", "Rare", "Common"); err != nil {
		return err
	}
	if err := validateChoice(flags.StartCredit, "startcredit", "", "Low", "Limited", "Normal", "High", "TooMuch"); err != nil {
		return err
	}
	return nil
}

// applyCustomGameFlags applies explicit command line options before setup or
// quick start reads the settings. The provided map prevents default flag values
// from replacing saved resource levels.
//
// C++ ported : gameconfigmenu.cpp - the MAX.INI values a command line overrides.
func applyCustomGameFlags(g *settings.Game, cf *CustomGameFlags, provided map[string]bool) {
	if g == nil || cf == nil {
		return
	}

	// Play mode: -turnbased / -simultane (mutually exclusive, already validated).
	if boolValue(cf.TurnBased) {
		g.PlayMode = 0
	} else if boolValue(cf.Simultane) {
		g.PlayMode = 1
	}

	// -turnlimit / -turns: the per turn clock seconds (the setup screen Turn Timer).
	if n, ok := optionalInt(cf.TurnLimit); ok {
		g.TurnTimer = n
	}

	// -endturn: the end of turn grace seconds.
	if n, ok := optionalInt(cf.EndTurn); ok {
		g.EndTurn = n
	}

	// -score Low|Medium|High: a points victory target (victory type 1).
	if s := strings.TrimSpace(stringValue(cf.Score)); s != "" {
		g.VictoryType = 1
		g.VictoryLimit = scoreVictoryLimit(s)
	}

	// -duration: a turn count victory (victory type 0, the setup screen Turns box).
	if n, ok := optionalInt(cf.Duration); ok {
		g.VictoryType = 0
		g.VictoryLimit = n
	}

	// -startcredit: the landing purchase budget bucket.
	if s := strings.TrimSpace(stringValue(cf.StartCredit)); s != "" {
		/* // note: unused code in Go port
		// ValidateCustomFlags already refused a bad label, and a valid one always maps to >= 0.
		if v := startCreditValue(s); v >= 0 {
			g.StartGold = v
		}
		*/
		g.StartGold = startCreditValue(s)
	}

	// -derelicts: neutral alien derelict density.
	if s := strings.TrimSpace(stringValue(cf.Derelicts)); s != "" {
		g.AlienDerelicts = resourceDensity(s)
	}

	// -raw / -fuel / -gold: resource densities. These default to Medium, so only
	// an explicitly provided flag overrides the saved level.
	if provided["raw"] {
		g.RawResource = resourceDensity(stringValue(cf.Raw))
	}
	if provided["fuel"] {
		g.FuelResource = resourceDensity(stringValue(cf.Fuel))
	}
	if provided["gold"] {
		g.GoldResource = resourceDensity(stringValue(cf.Gold))
	}
}

// stringValue reads a flag pointer, treating nil as empty.
func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// optionalInt parses a numeric flag pointer, reporting ok=false when it is unset
// or empty (the value was already range validated).
func optionalInt(v *string) (int, bool) {
	if v == nil {
		return 0, false
	}
	text := strings.TrimSpace(*v)
	if text == "" {
		return 0, false
	}
	n, err := strconv.Atoi(text)
	if err != nil {
		return 0, false
	}
	return n, true
}

// resourceDensity maps a Poor|Medium|Rich (or None|Rare|Common for derelicts)
// label to the 0/1/2 level the setup screen and economy use. Anything else
// falls back to the middle level.
func resourceDensity(v string) int {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "poor", "none":
		return 0
	case "rich", "common":
		return 2
	default:
		return 1
	}
}

// startCreditValue maps a starting credit bucket label to its gold amount,
// matching gcCreditValues in the setup screen. Returns -1 for an unknown label.
func startCreditValue(v string) int {
	switch strings.ToLower(strings.TrimSpace(v)) {
	/* // note: unused code in Go port
	// ValidateCustomFlags refuses none for -startcredit, and the help no longer offers it.
	case "none":
		return 0
	*/
	case "low":
		return 50
	case "limited":
		return 100
	case "normal":
		return 150
	case "high":
		return 200
	case "toomuch":
		return 250
	}
	return -1
}

// scoreVictoryLimit maps a Low|Medium|High score target to its points value,
// matching the setup screen's score presets (200/400/800).
func scoreVictoryLimit(v string) int {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "medium":
		return 400
	case "high":
		return 800
	default:
		return 200
	}
}

func boolValue(v *bool) bool {
	return v != nil && *v
}

func validateOptionalInt(value *string, name string, min, max int) error {
	if value == nil || strings.TrimSpace(*value) == "" {
		if value != nil {
			*value = ""
		}
		return nil
	}
	text := strings.TrimSpace(*value)
	n, err := strconv.Atoi(text)
	if err != nil || n < min || n > max {
		return fmt.Errorf("flag -%s must be empty or an integer from %d to %d", name, min, max)
	}
	*value = strconv.Itoa(n)
	return nil
}

func validateChoice(value *string, name string, choices ...string) error {
	if value == nil {
		return nil
	}
	text := strings.TrimSpace(*value)
	for _, choice := range choices {
		if strings.EqualFold(text, choice) {
			*value = choice
			return nil
		}
	}
	return fmt.Errorf("flag -%s must be one of %s", name, describeChoices(choices))
}

func describeChoices(choices []string) string {
	labels := make([]string, len(choices))
	for i, choice := range choices {
		if choice == "" {
			labels[i] = `""`
		} else {
			labels[i] = choice
		}
	}
	return strings.Join(labels, ", ")
}
