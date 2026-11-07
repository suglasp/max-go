// assets.go : loads the game's unit, clan, and language tables.
// Suglasp
// github.com/suglasp/max-go
// doc: docs/formats/tables.md.
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package assets reads ATTRIBS, CLANATRB, the language strings and the unit description keys (LANG.RES, or a
// loose lang-<code>.ini override).
package assets

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/suglasp/max-go/internal/resfile"
)

// UnitAttributes are the base stats of a unit type, one ATTRIBS row.
//
// C++: struct UnitAttributes (attributes.hpp)
type UnitAttributes struct {
	TurnsToBuild int
	HitPoints    int
	Armor        int
	Attack       int
	MoveAndFire  bool
	Speed        int
	Fuel         int
	Range        int
	Rounds       int // shots per turn
	Scan         int
	Storage      int
	Ammo         int
	Area         int // blast radius
}

// ParseAttribs parses the ATTRIBS resource: ';' comment lines and one
// row of 13 numbers (separated by whitespace) per unit type, in canonical
// unit order.
func ParseAttribs(data []byte) ([]UnitAttributes, error) {
	var rows []UnitAttributes

	sc := bufio.NewScanner(bytes.NewReader(data))
	line := 0

	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, ";") {
			continue
		}

		fields := strings.Fields(text)
		if len(fields) != 13 {
			return nil, fmt.Errorf("assets: ATTRIBS line %d: %d columns, want 13", line, len(fields))
		}

		var v [13]int
		for i, f := range fields {
			n, err := strconv.Atoi(f)
			if err != nil {
				return nil, fmt.Errorf("assets: ATTRIBS line %d: %q: %w", line, f, err)
			}
			v[i] = n
		}

		rows = append(rows, UnitAttributes{
			TurnsToBuild: v[0], HitPoints: v[1], Armor: v[2], Attack: v[3],
			MoveAndFire: v[4] != 0, Speed: v[5], Fuel: v[6], Range: v[7],
			Rounds: v[8], Scan: v[9], Storage: v[10], Ammo: v[11], Area: v[12],
		})
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}

	if len(rows) < UnitCount {
		return nil, fmt.Errorf("assets: ATTRIBS has %d rows, want >= %d", len(rows), UnitCount)
	}

	return rows, nil
}

// ClanUpgrade is one bonus line from a clan definition. Mods retains the raw
// display string, such as "+2 scan, +1 speed", for later parsing.
type ClanUpgrade struct {
	Unit string
	Mods string
}

// ClanMod is one resolved stat change a clan grants to a named unit. Attr is the
// canonical CLANATRB stat word ("attack", "range", "armor", "hits", "speed",
// "scan", "storage", "ammo", "shots" or "turns"); the caller maps it to a
// UnitValues attribute. Delta is the signed change ("+2 scan" -> +2); for an
// absolute "N turns" token (no leading '+') Abs is true and the build time is set
// to Delta rather than incremented.
//
// C++: the tradeoff offsets per attribute in ResourceManager_SetClanUpgradesFromClans.
type ClanMod struct {
	Attr  string
	Delta int
	Abs   bool
}

// Clan is one CLANATRB section.
//
// C++: class Clans (clans.cpp)
type Clan struct {
	/* // note: unused code in Go port
	// Key was only read by a test; the game picks a clan by its index.
	Key      string // "Clan A" .. "Clan H"
	*/
	Name     string
	Text     string
	Upgrades []ClanUpgrade // raw display name lines, for the clan select menu text
	// Mods is Upgrades resolved to unit types and parsed into stat changes, the
	// form the unit value layer applies. Built by ParseClans.
	Mods map[UnitType][]ClanMod
}

// ParseClans parses the CLANATRB resource (INI sections).
func ParseClans(data []byte) ([]Clan, error) {
	var clans []Clan
	var cur *Clan

	sc := bufio.NewScanner(bytes.NewReader(data))

	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())

		switch {
		case text == "" || strings.HasPrefix(text, ";"):
		case strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]"):
			if text == "[EOD]" { // end of data terminator section
				cur = nil
				continue
			}
			clans = append(clans, Clan{ /* Key: text[1 : len(text)-1], note: unused code in Go port, see Clan */ })
			cur = &clans[len(clans)-1]

		case cur == nil:
			return nil, fmt.Errorf("assets: CLANATRB: %q before any section", text)

		default:
			key, value, ok := strings.Cut(text, "=")
			if !ok {
				return nil, fmt.Errorf("assets: CLANATRB: bad line %q", text)
			}

			switch key {
			case "Name":
				cur.Name = value
			case "Text":
				cur.Text = value
			default:
				cur.Upgrades = append(cur.Upgrades, ClanUpgrade{Unit: key, Mods: value})
			}
		}
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}

	if len(clans) != 8 {
		return nil, fmt.Errorf("assets: CLANATRB has %d clans, want 8", len(clans))
	}

	for i := range clans {
		resolveClanMods(&clans[i])
	}

	return clans, nil
}

// clanUnitTypes maps each CLANATRB upgrade display name to its unit type. The
// CLANATRB keys are the original game's singular unit names, which differ from a
// few of the port's labels; checked against the resource tags and the MaxR unit
// folders. Notably "Ground Attack Plane" is the Bomber, "Infiltrator" the
// Commando, "Gunboat" the ship tagged Battleship, "Missile Crawler" the mobile
// MISSLLCH, "Missile Launcher" the ANTIMSSL defence turret, "Assault Gun" the
// mobile ARTILLRY and "Artillery" the ARTYTRRT defence turret.
var clanUnitTypes = map[string]UnitType{
	"AWAC":                 Awac,
	"Fighter":              Fighter,
	"Air Transport":        AirTrans,
	"Ground Attack Plane":  Bomber,
	"Air Units Plant":      AirPlt,
	"Missile Cruiser":      MsslBoat,
	"Escort":               FastBoat,
	"Corvette":             Corvette,
	"Gunboat":              Battlshp,
	"Submarine":            Submarne,
	"Sea Transport":        SeaTrans,
	"Cargo Ship":           CargoShp,
	"Shipyard":             Shipyard,
	"Scanner":              Scanner,
	"Scout":                Scout,
	"Infiltrator":          Commando,
	"Radar":                Radar,
	"Personnel Carrier":    ClnTrans,
	"Rocket Launcher":      RocktLch,
	"Missile Crawler":      MisslLch,
	"Missile Launcher":     AntiMssl,
	"Gun Turret":           GunTurrt,
	"Tank":                 Tank,
	"Mobile Anti Aircraft": SpFlak,
	"Assault Gun":          Artillry,
	"Sea Mine":             SeaMine,
	"Land Mine":            LandMine,
	"Artillery":            ArtyTrrt,
	"Anti Aircraft":        AntiAir,
	"Heavy Vehicle Plant":  LandPlt,
	"Light Vehicle Plant":  LightPlt,
}

// clanStatWords are the CLANATRB stat words that map onto a UnitValues attribute.
// Words outside this set (e.g. the Infiltrator's "disable", which has no value
// field) are skipped by ParseClanMods.
var clanStatWords = map[string]bool{
	"attack": true, "shots": true, "range": true, "armor": true,
	"hits": true, "speed": true, "scan": true, "turns": true,
	"ammo": true, "storage": true,
}

// ParseClanMods splits a CLANATRB upgrade value ("+2 scan, 7 turns") into its
// component stat changes. A "+N word" token is a delta; a bare "N turns" token is
// an absolute build time set (Abs). Tokens whose stat word has no UnitValues field
// are skipped.
func ParseClanMods(value string) []ClanMod {
	var mods []ClanMod
	for _, token := range strings.Split(value, ",") {
		fields := strings.Fields(token)
		if len(fields) != 2 {
			continue
		}
		number, word := fields[0], strings.ToLower(fields[1])
		if !clanStatWords[word] {
			continue
		}
		delta, err := strconv.Atoi(strings.TrimPrefix(number, "+"))
		if err != nil {
			continue
		}
		// Only an unprefixed "N turns" is an absolute set; everything else adds.
		abs := word == "turns" && !strings.HasPrefix(number, "+")
		mods = append(mods, ClanMod{Attr: word, Delta: delta, Abs: abs})
	}
	return mods
}

// resolveClanMods maps each upgrade display name to a unit type and parses its
// stat changes into c.Mods. Unknown unit names are skipped, while the raw
// Upgrades remain available for menu text.
func resolveClanMods(c *Clan) {
	c.Mods = make(map[UnitType][]ClanMod)
	for _, up := range c.Upgrades {
		ut, ok := clanUnitTypes[up.Unit]
		if !ok {
			continue
		}
		if mods := ParseClanMods(up.Mods); len(mods) > 0 {
			c.Mods[ut] = append(c.Mods[ut], mods...)
		}
	}
}

// Strings holds the localized text table from lang-en-us.ini,
// keyed by the 4 hex digit ids the code references.
type Strings map[string]string

// Get returns the string for key, or fallback when missing.
func (s Strings) Get(key, fallback string) string {
	if v, ok := s[key]; ok {
		return v
	}

	return fallback
}

// ParseLanguage parses a lang-*.ini file: a [language] section of
// key=value lines with \n escapes.
//
// C++: language.cpp
func ParseLanguage(data []byte) (Strings, error) {
	out := Strings{}
	inSection := false

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 64*1024)

	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())

		switch {
		case text == "" || strings.HasPrefix(text, ";"):
		case strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]"):
			inSection = text == "[language]"
		case inSection:
			key, value, ok := strings.Cut(text, "=")
			if !ok {
				continue
			}
			out[key] = strings.ReplaceAll(value, `\n`, "\n")
		}
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("assets: no [language] strings found")
	}

	return out, nil
}

// Assets bundles the loaded data tables.
type Assets struct {
	Attribs      []UnitAttributes // indexed by UnitType
	Clans        []Clan
	Strings      Strings
	UnitDescKeys [UnitCount]string // language key of each unit's description, "" when it has none
}

// Load reads all tables: ATTRIBS and CLANATRB from the resource
// library; the language strings from a loose lang-<language>.ini in the data
// directory or next to the binary, else from LANG.RES there (an empty
// language is treated as en-us); and the unit description keys from LANG.RES.
//
// C++: ResourceManager_InitUnitAttributes/InitClans/
// InitLanguageManager (resource_manager.cpp)
func Load(lib *resfile.Library, dataDir, language string) (*Assets, error) {
	a := &Assets{}

	data, err := lib.Read("ATTRIBS")
	if err != nil {
		return nil, err
	}

	if a.Attribs, err = ParseAttribs(data); err != nil {
		return nil, err
	}

	data, err = lib.Read("CLANATRB")
	if err != nil {
		return nil, err
	}

	if a.Clans, err = ParseClans(data); err != nil {
		return nil, err
	}

	langRes := openLangRes(dataDir)
	if langRes != nil {
		defer langRes.Close()
	}

	if a.Strings, err = loadStrings(dataDir, language, langRes); err != nil {
		return nil, err
	}

	if a.UnitDescKeys, err = loadUnitDescriptions(langRes); err != nil {
		return nil, err
	}

	return a, nil
}

/* // note: unused code in Go port
// languageStringsOrFallback was only read by a test; its body is one call to loadStrings, which the test now calls.
// languageStringsOrFallback loads the string table from a loose ini only,
// tolerating a missing file: the strings are a port asset, not vanilla game
// data, so without one the game boots on the built in English fallback
// strings and only logs it. Load also consults LANG.RES (loadStrings).
func languageStringsOrFallback(dataDir, language string) (Strings, error) {
	return loadStrings(dataDir, language, nil)
}
*/

// LanguageFile returns the strings filename for a language code, e.g.
// "en-us" -> "lang-en-us.ini".
func LanguageFile(language string) string {
	if language == "" {
		language = "en-us"
	}

	return "lang-" + language + ".ini"
}

// loadLanguageStrings reads lang-<language>.ini from dataDir, then from
// the binary directory, falling back to lang-en-us.ini in the same places.
func loadLanguageStrings(dataDir, language string) (Strings, error) {
	files := []string{LanguageFile(language)}

	if eng := LanguageFile("en-us"); files[0] != eng {
		files = append(files, eng)
	}

	dirs := languageSearchDirs(dataDir)
	var lastErr error

	for _, name := range files {
		for _, dir := range dirs {
			if dir == "" {
				continue
			}

			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				lastErr = err
				continue
			}

			return ParseLanguage(data)
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("assets: no language search directory")
}

var languageSearchDirs = defaultLanguageSearchDirs

func defaultLanguageSearchDirs(dataDir string) []string {
	var dirs []string
	add := func(dir string) {
		if dir == "" {
			return
		}

		for _, existing := range dirs {
			if existing == dir {
				return
			}
		}

		dirs = append(dirs, dir)
	}

	add(dataDir)

	if binDir, err := executableDir(); err == nil {
		add(binDir)
	}

	for _, dir := range extraLanguageDirs() {
		add(dir)
	}

	return dirs
}

var executableDir = func() (string, error) {
	exe, err := os.Executable()

	if err != nil {
		return "", err
	}

	return filepath.Dir(exe), nil
}

var extraLanguageDirs = func() []string {
	cwd, err := os.Getwd()

	if err != nil {
		return nil
	}

	dirs := []string{cwd, filepath.Join(cwd, "bin")}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			dirs = append(dirs, filepath.Join(dir, "bin"))
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	return dirs
}
