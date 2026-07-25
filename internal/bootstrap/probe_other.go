//go:build !windows

// probe_other.go : searches common Steam locations on non-Windows systems.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package bootstrap

import (
	"os"
	"path/filepath"
)

// hostProber probes the usual Steam locations under the user's home. There
// is no registry, so a GOG (or otherwise placed) install is handled by the
// boot UI's manual path entry instead.
type hostProber struct{}

// GOGDir returns "" on this OS: GOG ships M.A.X. for Windows only, so a
// Linux copy is always a manually pointed directory.
func (hostProber) GOGDir() string {
	return ""
}

// SteamRoots returns the common Steam roots that exist under $HOME.
func (hostProber) SteamRoots() []string {
	home, err := os.UserHomeDir()

	if err != nil {
		return nil
	}

	candidates := []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
	}

	var roots []string

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			roots = append(roots, c)
		}
	}

	return roots
}
