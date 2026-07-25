// steam_test.go : tests VDF and ACF parsing against fixture documents.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

// TestVdfValue checks the quoted pair lookup on the fixture app manifest:
// the installdir resolves, the lookup ignores case, and a missing key is "".
func TestVdfValue(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "appmanifest_615250.acf"))

	if err != nil {
		t.Fatal(err)
	}

	doc := string(data)

	if got := vdfValue(doc, "installdir"); got != "MAX" {
		t.Fatalf("installdir = %q, want MAX", got)
	}

	if got := vdfValue(doc, "InstallDir"); got != "MAX" {
		t.Fatalf("case insensitive installdir = %q, want MAX", got)
	}

	if got := vdfValue(doc, "no_such_key"); got != "" {
		t.Fatalf("missing key = %q, want empty", got)
	}
}

// TestSteamLibrariesFiltersMissingDirs checks that a listed library path
// that does not exist on disk is dropped, while the root always stays.
func TestSteamLibrariesFiltersMissingDirs(t *testing.T) {
	root := t.TempDir()
	real := t.TempDir()

	write(t, filepath.Join(root, "steamapps", "libraryfolders.vdf"),
		"\"libraryfolders\"\n{\n"+
			"\t\"0\"\n\t{\n\t\t\"path\"\t\t\""+vdfEscape(real)+"\"\n\t}\n"+
			"\t\"1\"\n\t{\n\t\t\"path\"\t\t\""+vdfEscape(filepath.Join(root, "gone"))+"\"\n\t}\n"+
			"}\n")

	libs := steamLibraries(root)

	if len(libs) != 2 || libs[0] != root || libs[1] != filepath.Clean(real) {
		t.Fatalf("steamLibraries = %v, want [root, real]", libs)
	}
}

// TestSteamLibrariesNoVdf checks a root with no libraryfolders.vdf still
// yields itself as the only library.
func TestSteamLibrariesNoVdf(t *testing.T) {
	root := t.TempDir()

	if libs := steamLibraries(root); len(libs) != 1 || libs[0] != root {
		t.Fatalf("steamLibraries = %v, want just the root", libs)
	}
}

// TestIsAllDigits covers numeric keys used by the legacy Steam library layout.
func TestIsAllDigits(t *testing.T) {
	for s, want := range map[string]bool{"1": true, "42": true, "": false, "path": false, "1a": false} {
		if got := isAllDigits(s); got != want {
			t.Errorf("isAllDigits(%q) = %v, want %v", s, got, want)
		}
	}
}
