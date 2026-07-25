// briefing_test.go : tests mission briefing text layout.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/missionfile"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/testutil"
)

// TestBriefingSkipKey: a normal typed key, Return, or ESC finishes the typewriter
// reveal, as the original does on any input. ESC only finishes the TEXT; it takes a
// second press on the OK plate to leave the screen, and the reveal loop flushes the
// key that skipped it so one press cannot do both. The F keys stay out of it (the
// port's one deliberate departure, so they keep their in game functions).
func TestBriefingSkipKey(t *testing.T) {
	for name, key := range map[string]int{
		"a": 'a', "Z": 'Z', "space": ' ', "digit": '5', "return": platform.KeyReturn,
		"esc": platform.KeyEscape,
	} {
		if !briefingSkipKey(key) {
			t.Errorf("briefingSkipKey(%s=%d) = false, want true", name, key)
		}
	}

	for name, key := range map[string]int{
		"f1": platform.KeyF1, "f10": platform.KeyF10, "none": -1,
	} {
		if briefingSkipKey(key) {
			t.Errorf("briefingSkipKey(%s=%d) = true, want false", name, key)
		}
	}
}

// TestBriefingLines: paragraphs wrap to the box width with one blank row
// between them and no blank edges, and the campaign intro text fits the box.
func TestBriefingLines(t *testing.T) {
	set, err := fonts.LoadSet(testutil.DataDir(t))

	if err != nil {
		t.Fatal(err)
	}

	f := set.Get(fonts.Font1)
	lines := briefingLines(f, "alpha beta gamma\n\ndelta")

	if len(lines) < 3 {
		t.Fatalf("briefingLines = %q, want two paragraphs and a blank row", lines)
	}

	blanks := 0

	for i, line := range lines {
		if line == "" {
			blanks++

			if i == 0 || i == len(lines)-1 {
				t.Errorf("blank row at the edge (index %d) in %q", i, lines)
			}

			continue
		}

		if f.Width(line) > briefingTextW {
			t.Errorf("line %q is wider than the %d px box", line, briefingTextW)
		}
	}

	if blanks != 1 {
		t.Errorf("%d blank rows, want exactly 1 between the two paragraphs", blanks)
	}

	// The campaign opening text must fit the box rows, or the reveal would clip.
	intro := briefingLines(f, campaignIntroFallback)
	maxRows := briefingTextH / (f.Height + 1)

	if len(intro) > maxRows {
		t.Errorf("campaign intro wraps to %d rows, box holds %d", len(intro), maxRows)
	}
}

// TestCampaignVictoryText: the shipped WIN9.CAM ending loads and fits the story
// box, and no other WIN<N>.CAM exists so only the final mission shows one.
func TestCampaignVictoryText(t *testing.T) {
	dir := testutil.DataDir(t)

	text, err := missionfile.ReadDescription(filepath.Join(dir, "WIN9.CAM"))

	if err != nil {
		t.Fatalf("read WIN9.CAM: %v", err)
	}

	if !strings.HasPrefix(text, "The last threat to the Great Ships") {
		t.Errorf("WIN9.CAM start = %.40q, want the Concord ending", text)
	}

	set, err := fonts.LoadSet(dir)

	if err != nil {
		t.Fatal(err)
	}

	f := set.Get(fonts.Font1)
	lines := briefingLines(f, text)
	maxRows := briefingTextH / (f.Height + 1)

	if len(lines) == 0 {
		t.Fatal("victory text wrapped to no rows")
	}

	if len(lines) > maxRows {
		t.Errorf("victory text wraps to %d rows, box holds %d", len(lines), maxRows)
	}

	// Only the final campaign mission ships a WIN file; the victory screen fires
	// only when one exists, so missions before the first entry have none.
	for _, n := range []int{1, 3, 8} {
		if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("WIN%d.CAM", n))); err == nil {
			t.Errorf("unexpected WIN%d.CAM present; victory story would fire on mission %d", n, n)
		}
	}
}
