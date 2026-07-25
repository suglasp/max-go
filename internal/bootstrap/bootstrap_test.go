// bootstrap_test.go : tests discovery and copying against synthetic install trees.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeProber feeds Locate fixed locations instead of the host registry.
type fakeProber struct {
	gog   string
	steam []string
}

func (f fakeProber) GOGDir() string       { return f.gog }
func (f fakeProber) SteamRoots() []string { return f.steam }

// write creates a file (and its parents) with small throwaway content.
func write(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// vdfEscape doubles backslashes the way Steam writes paths into VDF files.
func vdfEscape(p string) string {
	return strings.ReplaceAll(p, `\`, `\\`)
}

// TestHasMaxRes checks the boot gate: a max.res directly in the dir counts
// (any case), one in a subfolder does not.
func TestHasMaxRes(t *testing.T) {
	root := t.TempDir()

	if HasMaxRes(root) {
		t.Error("empty dir: HasMaxRes = true, want false")
	}

	write(t, filepath.Join(root, "sub", "MAX.RES"), "x")
	if HasMaxRes(root) {
		t.Error("nested max.res: HasMaxRes = true, want false (direct holder only)")
	}

	write(t, filepath.Join(root, "Max.Res"), "x")
	if !HasMaxRes(root) {
		t.Error("mixed case max.res present: HasMaxRes = false, want true")
	}
}

// TestFindDataDir checks the max.res walk: any case matches, the directory
// holding the file is returned, and the shallowest hit beats a nested one.
func TestFindDataDir(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "MAX", "max.res"), "x")
	write(t, filepath.Join(root, "MAX", "backup", "deep", "MAX.RES"), "x")

	dir, ok := FindDataDir(root)

	if !ok {
		t.Fatal("FindDataDir found nothing")
	}

	if want := filepath.Join(root, "MAX"); dir != want {
		t.Fatalf("FindDataDir = %q, want the shallow %q", dir, want)
	}

	if _, ok := FindDataDir(t.TempDir()); ok {
		t.Fatal("FindDataDir reported a hit in an empty tree")
	}
}

// TestLocateGOG checks the GOG probe: the DOSBOXFOLDER tree wins even when a
// Steam root exists too, and the source is labelled "gog".
func TestLocateGOG(t *testing.T) {
	gog := t.TempDir()
	write(t, filepath.Join(gog, "MAX", "MAX.RES"), "x")

	inst, err := Locate(fakeProber{gog: gog, steam: []string{t.TempDir()}})

	if err != nil {
		t.Fatal(err)
	}

	if inst.Source != "gog" || inst.DataDir != filepath.Join(gog, "MAX") {
		t.Fatalf("Locate = %+v, want the gog MAX folder", inst)
	}
}

// TestLocateSteam checks the Steam path end to end: the root's
// libraryfolders.vdf lists a second library, the app manifest lives there,
// and the data folder is found under steamapps/common/<installdir>.
func TestLocateSteam(t *testing.T) {
	root := t.TempDir()
	lib := t.TempDir()

	write(t, filepath.Join(root, "steamapps", "libraryfolders.vdf"),
		"\"libraryfolders\"\n{\n\t\"0\"\n\t{\n\t\t\"path\"\t\t\""+vdfEscape(root)+"\"\n\t}\n\t\"1\"\n\t{\n\t\t\"path\"\t\t\""+vdfEscape(lib)+"\"\n\t}\n}\n")
	write(t, filepath.Join(lib, "steamapps", "appmanifest_"+SteamAppID+".acf"),
		"\"AppState\"\n{\n\t\"appid\"\t\t\""+SteamAppID+"\"\n\t\"installdir\"\t\t\"MAX\"\n}\n")
	write(t, filepath.Join(lib, "steamapps", "common", "MAX", "max", "MAX.RES"), "x")

	inst, err := Locate(fakeProber{steam: []string{root}})

	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(lib, "steamapps", "common", "MAX", "max")

	if inst.Source != "steam" || inst.DataDir != want {
		t.Fatalf("Locate = %+v, want steam data dir %q", inst, want)
	}
}

// TestLocateSteamOldLayout checks the pre 2021 libraryfolders.vdf layout
// (bare numbered key -> path pairs) still resolves a second library.
func TestLocateSteamOldLayout(t *testing.T) {
	root := t.TempDir()
	lib := t.TempDir()

	write(t, filepath.Join(root, "steamapps", "libraryfolders.vdf"),
		"\"LibraryFolders\"\n{\n\t\"TimeNextStatsReport\"\t\t\"0\"\n\t\"1\"\t\t\""+vdfEscape(lib)+"\"\n}\n")
	write(t, filepath.Join(lib, "steamapps", "appmanifest_"+SteamAppID+".acf"),
		"\"AppState\"\n{\n\t\"installdir\"\t\t\"MAX\"\n}\n")
	write(t, filepath.Join(lib, "steamapps", "common", "MAX", "MAX.RES"), "x")

	inst, err := Locate(fakeProber{steam: []string{root}})

	if err != nil {
		t.Fatal(err)
	}

	if want := filepath.Join(lib, "steamapps", "common", "MAX"); inst.DataDir != want {
		t.Fatalf("Locate = %+v, want %q", inst, want)
	}
}

// TestLocateGOGDosboxParent checks the real GOG layout: DOSBOXFOLDER points
// at the DOSBox emulator subfolder while MAX.RES sits flat in its parent, so
// Locate must probe the parent too.
func TestLocateGOGDosboxParent(t *testing.T) {
	install := t.TempDir()
	write(t, filepath.Join(install, "DOSBox", "dosbox.conf"), "x")
	write(t, filepath.Join(install, "MAX.RES"), "x")

	inst, err := Locate(fakeProber{gog: filepath.Join(install, "DOSBox")})

	if err != nil {
		t.Fatal(err)
	}

	if inst.Source != "gog" || inst.DataDir != install {
		t.Fatalf("Locate = %+v, want the DOSBox parent %q", inst, install)
	}
}

// TestLocateNotFound checks the empty machine case: no GOG dir, a Steam root
// with no manifest -> ErrNotFound.
func TestLocateNotFound(t *testing.T) {
	if _, err := Locate(fakeProber{steam: []string{t.TempDir()}}); err != ErrNotFound {
		t.Fatalf("Locate err = %v, want ErrNotFound", err)
	}
}

// TestCopy checks extension filtering, preservation of existing files, progress
// reporting, and final MAX.RES verification.
func TestCopy(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	dataFiles := []string{
		"MAX.RES", "PATCHES.RES", "GREEN_1.WRL", "COLOR.PAL", "FONT10.FON",
		"MAIN_MSC.MSC", "F001.WAV", "TANK.FLC", "MAXINT.MVE",
		"SAVE1.CAM", "SAVE2.SCE", "SAVE3.TRA", "SAVE4.MPS", "DEMO1.DMO",
	}

	for _, name := range dataFiles {
		write(t, filepath.Join(src, name), "new "+name)
	}

	// Files outside the data set must not travel.
	write(t, filepath.Join(src, "README.TXT"), "keep out")
	write(t, filepath.Join(src, "MAX.EXE"), "keep out")

	// A file the user already has keeps its content.
	write(t, filepath.Join(dst, "PATCHES.RES"), "user data")

	var calls int
	var lastDone, lastTotal int

	copied, err := Copy(src, dst, func(done, total int, name string) {
		calls++
		lastDone, lastTotal = done, total

		if name == "" {
			t.Error("progress with an empty name")
		}
	})

	if err != nil {
		t.Fatal(err)
	}

	if want := len(dataFiles) - 1; copied != want {
		t.Fatalf("copied = %d, want %d (PATCHES.RES already present)", copied, want)
	}

	if calls != len(dataFiles) || lastDone != lastTotal || lastTotal != len(dataFiles) {
		t.Fatalf("progress calls=%d last=%d/%d, want %d calls ending %d/%d",
			calls, lastDone, lastTotal, len(dataFiles), len(dataFiles), len(dataFiles))
	}

	for _, name := range dataFiles {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Errorf("%s missing in destination", name)
		}
	}

	if b, _ := os.ReadFile(filepath.Join(dst, "PATCHES.RES")); string(b) != "user data" {
		t.Error("an existing destination file was overwritten")
	}

	if _, err := os.Stat(filepath.Join(dst, "README.TXT")); err == nil {
		t.Error("a file outside the data set was copied")
	}
}

// TestCopyMissingMaxRes checks the verification: a source without MAX.RES
// (and a destination without one) is an error even when other files copied.
func TestCopyMissingMaxRes(t *testing.T) {
	src := t.TempDir()
	write(t, filepath.Join(src, "GREEN_1.WRL"), "x")

	if _, err := Copy(src, t.TempDir(), nil); err == nil {
		t.Fatal("Copy passed without a MAX.RES in the destination")
	}
}
