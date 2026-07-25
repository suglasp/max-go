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

// DataDir returns the directory named by MAX_DATA_DIR. It skips the calling test when the variable is unset or the directory does not contain MAX.RES.
func DataDir(t *testing.T) string {
	t.Helper()

	dir := os.Getenv("MAX_DATA_DIR")

	if dir == "" {
		t.Skip("game data not found (set MAX_DATA_DIR)")
	}

	if _, err := os.Stat(filepath.Join(dir, "MAX.RES")); err != nil {
		t.Skipf("game data not found in %q (set MAX_DATA_DIR): %v", dir, err)
	}

	return dir
}
