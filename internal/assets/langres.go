// langres.go : reads the port's LANG.RES addon archive (language strings and unit description keys).
// Suglasp
// github.com/suglasp/max-go
// doc: docs/formats/lang-res.md.
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package assets

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/suglasp/max-go/internal/resfile"
)

// LangResFile is the port's own addon archive, shipped next to the binary. It holds the language strings
// (LANG_<XX>) and the unit description keys (UNITDESC), the way klei ships its own PATCHES.RES.
const LangResFile = "LANG.RES"

// unitDescTag is the LANG.RES entry holding unit-descriptions.txt.
const unitDescTag = "UNITDESC"

// LanguageTag returns the LANG.RES entry of a language code: the part before the dash, upper case, after
// "LANG_" ("en-us" -> "LANG_EN"; empty means en-us). RES tags hold at most 8 characters.
func LanguageTag(language string) string {
	if language == "" {
		language = "en-us"
	}

	code, _, _ := strings.Cut(language, "-")
	tag := "LANG_" + strings.ToUpper(code)

	if len(tag) > resfile.TagLen {
		tag = tag[:resfile.TagLen]
	}

	return tag
}

// openLangRes opens LANG.RES from the first language search directory that has it (the data folder, then
// the binary folder). It returns nil when no directory has one.
func openLangRes(dataDir string) *resfile.Library {
	for _, dir := range languageSearchDirs(dataDir) {
		path := filepath.Join(dir, LangResFile)

		if _, err := os.Stat(path); err != nil {
			continue
		}

		lib, err := resfile.Open(path)
		if err != nil {
			log.Printf("assets: %s: %v", path, err)
			continue
		}

		return lib
	}

	return nil
}

// langResStrings reads the language strings from LANG.RES: the requested language, then English.
func langResStrings(lib *resfile.Library, language string) (Strings, string, bool) {
	tags := []string{LanguageTag(language)}

	if eng := LanguageTag("en-us"); tags[0] != eng {
		tags = append(tags, eng)
	}

	for _, tag := range tags {
		data, err := lib.Read(tag)
		if err != nil {
			continue
		}

		s, err := ParseLanguage(data)
		if err != nil {
			log.Printf("assets: %s %s: %v", LangResFile, tag, err)
			continue
		}

		return s, tag, true
	}

	return nil, "", false
}

// loadStrings picks the language strings: a loose lang-<code>.ini wins (the override for translators and
// testing), then LANG.RES, then the built in English fallback (an empty table).
func loadStrings(dataDir, language string, langRes *resfile.Library) (Strings, error) {
	s, err := loadLanguageStrings(dataDir, language)
	if err == nil {
		return s, nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	if langRes != nil {
		if s, tag, ok := langResStrings(langRes, language); ok {
			log.Printf("assets: language strings from %s %s", LangResFile, tag)
			return s, nil
		}
	}

	log.Printf("assets: neither %s nor %s found, using the built in English strings", LanguageFile(language), LangResFile)
	return Strings{}, nil
}

// loadUnitDescriptions reads the unit description keys from LANG.RES; without it the keys stay empty and
// the menus show no description text.
func loadUnitDescriptions(langRes *resfile.Library) ([UnitCount]string, error) {
	if langRes == nil {
		return [UnitCount]string{}, nil
	}

	data, err := langRes.Read(unitDescTag)
	if err != nil {
		log.Printf("assets: %s has no %s entry, unit descriptions stay empty", LangResFile, unitDescTag)
		return [UnitCount]string{}, nil
	}

	return ParseUnitDescriptions(data)
}
