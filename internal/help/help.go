// help.go : reads the retail context help database (the HELP_ENG resource).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : help.cpp - Help / HelpMenu_Menu's data source
package help

import (
	"strconv"
	"strings"
)

// The help database is a plain CRLF text file stored in MAX.RES under the tag HELP_ENG,
// and it documents its own grammar in its first lines:
//
//	[SECTION_NAME]                      one screen
//	ENTRY_NAME   { ux, uy, lx, ly }     one clickable area of that screen, in 640x480 coords
//	the help text for it                one line (a literal \n inside it is a hard break)
//	                                    a blank line ends the entry
//	[END]                               ends the file
//
// Lookup walks a section's entries in file order and returns the first rectangle containing the click. Entry names are descriptive only.

// Tag is the resource the help database lives in (MAX.RES). The retail data ships the
// English file only; a localized build would add its own.
const Tag = "HELP_ENG"

// Entry is one clickable area of a screen: its rectangle in the original's 640x480
// coordinate space (inclusive on every edge) and the text shown for it. An entry with
// empty text exists in the data (each screen carries one for its own ? button) and reads
// as "no help here".
type Entry struct {
	ULX, ULY int
	LRX, LRY int
	Text     string
	/* // note: unused code in Go port
	// Name was only read by a test; Lookup matches by rectangle.
	Name     string
	*/
}

// Database is the parsed help file: for each section, its entries IN FILE ORDER.
type Database struct {
	sections map[string][]Entry
}

// Parse reads the help text file and skips malformed lines so a damaged entry cannot block startup. The retail ALLOCATE_LABEL rectangle omits one
// comma, so coordinates are split on commas and whitespace.
func Parse(data []byte) *Database {
	db := &Database{sections: map[string][]Entry{}}

	var (
		section string
		pending *Entry
	)

	// A pending entry takes the NEXT non blank line as its text, then closes.
	flush := func() {
		if pending != nil && section != "" {
			db.sections[section] = append(db.sections[section], *pending)
		}

		pending = nil
	}

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "":
			flush() // a blank line closes the entry, whether it got text or not

		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			flush()

			name := strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")
			if name == "END" {
				return db
			}

			section = name
			db.sections[section] = nil

		case pending != nil:
			// The line after a header is that entry's text. A literal \n in it is a hard
			// line break, which is how the file writes a multi line blurb.
			pending.Text = strings.ReplaceAll(trimmed, `\n`, "\n")
			flush()

		default:
			if e, ok := parseHeader(trimmed); ok && section != "" {
				pending = &e
			}
		}
	}

	flush()

	return db
}

// parseHeader reads an entry header line, "NAME { ux, uy, lx, ly }". Anything else (the
// file's own preamble and its human only ALL CAPS headings) is not a header.
func parseHeader(line string) (Entry, bool) {
	open := strings.Index(line, "{")
	closing := strings.Index(line, "}")

	if open < 0 || closing < open {
		return Entry{}, false
	}

	// The numbers are separated by commas, spaces, or (in one retail line) just a space.
	fields := strings.FieldsFunc(line[open+1:closing], func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})

	if len(fields) != 4 {
		return Entry{}, false
	}

	values := make([]int, 4)

	for i, f := range fields {
		n, err := strconv.Atoi(f)

		if err != nil {
			return Entry{}, false
		}

		values[i] = n
	}

	return Entry{
		/* Name: strings.TrimSpace(line[:open]): note: unused code in Go port, see Entry */
		ULX: values[0], ULY: values[1],
		LRX: values[2], LRY: values[3],
	}, true
}

// Lookup returns the help text for a screen click at (x, y), using the first matching rectangle with inclusive edges. It returns false for an
// unknown section, a miss, or an entry with no text.
//
// C++ ported : help.cpp - Help::GetEntry + Entry::IsInside (>= and <= on all four edges)
func (d *Database) Lookup(section string, x, y int) (string, bool) {
	if d == nil {
		return "", false
	}

	for _, e := range d.sections[section] {
		if x >= e.ULX && x <= e.LRX && y >= e.ULY && y <= e.LRY {
			if e.Text == "" {
				return "", false
			}

			return e.Text, true
		}
	}

	return "", false
}

/* // note: unused code in Go port
// Section was only read by tests; the game asks through Lookup, the tests read the parsed map.
// Section returns a screen's entries in file order (nil when it does not exist).
func (d *Database) Section(name string) []Entry {
	if d == nil {
		return nil
	}

	return d.sections[name]
}
*/

// Sections is how many screens the database holds (the help mode refuses to start on an empty one).
func (d *Database) Sections() int {
	if d == nil {
		return 0
	}

	return len(d.sections)
}
