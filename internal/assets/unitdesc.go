// unitdesc.go : parses the unit description key table (LANG.RES UNITDESC, langres/unit-descriptions.txt).
// Suglasp
// github.com/suglasp/max-go
// doc: docs/formats/lang-res.md.
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package assets

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// ParseUnitDescriptions reads unit-descriptions.txt: one TAG=key line per unit (for example CONSTRCT=d28e),
// where the key names the unit's description in the language file. Blank lines and # comments are skipped;
// an unknown unit tag or a line without "=" is an error naming the line.
//
// C++ ported : units.cpp - CreateUnitFromJson "description" (SC_UNITS)
func ParseUnitDescriptions(data []byte) ([UnitCount]string, error) {
	var keys [UnitCount]string

	index := make(map[string]UnitType, UnitCount)
	for t, tag := range UnitTags {
		index[tag] = UnitType(t)
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}

		tag, key, ok := strings.Cut(text, "=")
		if !ok {
			return keys, fmt.Errorf("assets: unit descriptions line %d: want TAG=key, got %q", line, text)
		}

		t, known := index[strings.ToUpper(strings.TrimSpace(tag))]
		if !known {
			return keys, fmt.Errorf("assets: unit descriptions line %d: unknown unit tag %q", line, tag)
		}

		keys[t] = strings.ToLower(strings.TrimSpace(key))
	}

	return keys, sc.Err()
}

// DescriptionKey returns the language key of a unit type's description text, or "" when it has none.
// The build, factory, cargo and upgrade menus draw that text over the unit portrait.
//
// C++ ported : unit.cpp - Unit::GetDescription
func (a *Assets) DescriptionKey(t UnitType) string {
	if a == nil || t < 0 || int(t) >= UnitCount {
		return ""
	}

	return a.UnitDescKeys[t]
}
