// missionfile_test.go : testing unit for missionfile.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package missionfile

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/testutil"
)

func TestReadInfoRetailSmartFiles(t *testing.T) {
	dir := testutil.DataDir(t)
	paths := missionPaths(t, dir)
	if len(paths) == 0 {
		t.Fatalf("no retail SmartFile V70 mission files found in %s", dir)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			info, err := ReadInfo(f)
			if err != nil {
				t.Fatal(err)
			}
			if info.Version != smartFileV70 {
				t.Fatalf("Version = %d, want %d", info.Version, smartFileV70)
			}
			if info.Category < CategoryCustom || info.Category >= CategoryCount {
				t.Fatalf("Category = %d, want valid range", info.Category)
			}
		})
	}
}

func TestReadInfoSAVE1Golden(t *testing.T) {
	dir := testutil.DataDir(t)
	f, err := os.Open(filepath.Join(dir, "SAVE1.SCE"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	info, err := ReadInfo(f)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name == "" {
		t.Fatal("Name is empty")
	}
	sum := sha256.Sum256(serializeInfo(info))
	const want = "6580274c6881e286fdfa66244e0d438da5dab816e1c7db9b94baf9b79168719a"
	if got := fmt.Sprintf("%x", sum); got != want {
		t.Fatalf("SAVE1.SCE hash = %s, want %s", got, want)
	}
}

func TestReadInfoErrors(t *testing.T) {
	if _, err := ReadInfo(bytes.NewReader([]byte{0x46, 0x00, 0x00, 'a', 'b'})); err == nil {
		t.Fatal("truncated input returned nil error")
	}
	if _, err := ReadInfo(bytes.NewReader([]byte{0x47, 0x00})); err == nil {
		t.Fatal("invalid version returned nil error")
	}
}

func TestReadInfoRejectsRetailDescriptionText(t *testing.T) {
	dir := testutil.DataDir(t)
	f, err := os.Open(filepath.Join(dir, "DESCR1.SCE"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := ReadInfo(f); err == nil {
		t.Fatal("description text file returned nil error")
	}
}

func serializeInfo(info *Info) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "version=%d\n", info.Version)
	fmt.Fprintf(&b, "save_type=%d\n", info.SaveType)
	fmt.Fprintf(&b, "category=%d\n", info.Category)
	fmt.Fprintf(&b, "name=%q\n", info.Name)
	fmt.Fprintf(&b, "world_index=%d\n", info.WorldIndex)
	fmt.Fprintf(&b, "mission_index=%d\n", info.MissionIndex)
	for i, name := range info.TeamNames {
		fmt.Fprintf(&b, "team_name_%d=%q\n", i, name)
	}
	for i, typ := range info.TeamType {
		fmt.Fprintf(&b, "team_type_%d=%d\n", i, typ)
	}
	for i, clan := range info.TeamClan {
		fmt.Fprintf(&b, "team_clan_%d=%d\n", i, clan)
	}
	fmt.Fprintf(&b, "rng_seed=%d\n", info.RNGSeed)
	return []byte(b.String())
}

/* // note: unused code in Go port
// a one line wrapper of testutil.DataDir; the tests call it directly
func dataDir(t *testing.T) string {
	t.Helper()
	return testutil.DataDir(t)
}
*/

func missionPaths(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	exts := map[string]bool{
		".CAM": true,
		".SCE": true,
		".TRA": true,
		".MPS": true,
		".DMO": true,
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(dir, name)
		if exts[strings.ToUpper(filepath.Ext(name))] && hasV70Header(t, path) {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func hasV70Header(t *testing.T, path string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return len(data) >= 2 && data[0] == 0x46 && data[1] == 0x00
}
