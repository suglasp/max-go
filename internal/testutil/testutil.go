// testutil.go : the M.A.X. test utility helpers - locates and opens real game data for integration tests via envvar 'MAX_DATA_DIR'.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package testutil provides helpers for tests that use original game data supplied outside the repository.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// LookupDataDir returns the directory named by MAX_DATA_DIR and whether it holds MAX.RES. It never skips, so a fuzz target can add retail seeds when the data is there and still run its own seeds when it is not.
func LookupDataDir() (string, bool) {
	dir := os.Getenv("MAX_DATA_DIR")

	if dir == "" {
		return "", false
	}

	// A folder without MAX.RES is not a game data folder.
	if _, err := os.Stat(filepath.Join(dir, "MAX.RES")); err != nil {
		return dir, false
	}

	return dir, true
}

// DataDir returns the directory named by MAX_DATA_DIR. It skips the calling test or fuzz target when the variable is unset or the directory does not contain MAX.RES.
func DataDir(tb testing.TB) string {
	tb.Helper()

	dir, ok := LookupDataDir()

	// Skip with the reason that tells the user what to set.
	if dir == "" {
		tb.Skip("game data not found (set MAX_DATA_DIR)")
	}

	if !ok {
		tb.Skipf("game data not found in %q (set MAX_DATA_DIR): no MAX.RES", dir)
	}

	return dir
}
