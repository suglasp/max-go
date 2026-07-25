// bootstrap.go : locates an installed GOG or Steam copy of M.A.X. and copies its data.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package bootstrap copies data from a locally installed M.A.X. into the port's
// work directory. The caller provides the UI, and Prober isolates registry and
// filesystem discovery so tests can use synthetic trees.
package bootstrap

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNotFound reports that no installed M.A.X. was found on this machine.
var ErrNotFound = errors.New("bootstrap: no installed M.A.X. found")

// SteamAppID is the Steam store id of M.A.X. (1996). M.A.X. 2 is 756810 and
// is not what the port needs.
const SteamAppID = "615250"

// GOGProductID is the GOG product id of the M.A.X. + M.A.X. 2 bundle.
const GOGProductID = "1207663773"

// DataExts lists the lower-case game data extensions copied by bootstrap. It includes resources, worlds, palettes, media, unit animations, and
// mission files; the port provides its own language INI file.
var DataExts = []string{
	".res", ".wrl", ".pal", ".fon", ".msc", ".wav", ".flc", ".mve",
	".cam", ".sce", ".tra", ".mps", ".dmo",
}

// Progress reports one processed file during Copy: done runs 1..total and
// name is the bare file name. A skipped (already present) file reports too.
type Progress func(done, total int, name string)

// Install is a located M.A.X. install: DataDir holds MAX.RES, Source names
// where it came from ("gog" or "steam").
type Install struct {
	DataDir string
	Source  string
}

// Prober lists the host's candidate install locations. The real probers are
// per OS (registry on Windows, home directory paths on Linux); tests supply
// a fake.
type Prober interface {
	// GOGDir returns the GOG install folder (the DOSBOXFOLDER registry
	// value) or "" when GOG's M.A.X. is not installed.
	GOGDir() string
	// SteamRoots returns candidate Steam installation roots, best first.
	SteamRoots() []string
}

// NewProber returns the prober for the running OS.
func NewProber() Prober {
	return hostProber{}
}

// Locate finds an installed M.A.X. through the prober. It prefers GOG, then searches Steam libraries for app 615250, returning ErrNotFound when
// neither has MAX.RES. A GOG DOSBOXFOLDER can point at the emulator subfolder, so its parent is also checked.
func Locate(p Prober) (Install, error) {
	if root := p.GOGDir(); root != "" {
		for _, candidate := range []string{root, filepath.Dir(root)} {
			if dir, ok := FindDataDir(candidate); ok {
				return Install{DataDir: dir, Source: "gog"}, nil
			}
		}
	}

	for _, root := range p.SteamRoots() {
		if dir, ok := steamDataDir(root); ok {
			return Install{DataDir: dir, Source: "steam"}, nil
		}
	}

	return Install{}, ErrNotFound
}

// FindDataDir walks root for a file named max.res (any case) and returns the
// directory holding it. Both store versions ship M.A.X. as a DOSBox bundle,
// so MAX.RES usually sits in a subfolder of the install root. The shallowest
// match wins so a nested duplicate cannot shadow the real data folder.
func FindDataDir(root string) (string, bool) {
	var hits []string

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}

		if !d.IsDir() && strings.EqualFold(d.Name(), "max.res") {
			hits = append(hits, filepath.Dir(path))
		}

		return nil
	})

	if len(hits) == 0 {
		return "", false
	}

	// Shallowest path first; ties break lexically for a deterministic pick.
	sort.Slice(hits, func(i, j int) bool {
		di := strings.Count(hits[i], string(filepath.Separator))
		dj := strings.Count(hits[j], string(filepath.Separator))

		if di != dj {
			return di < dj
		}

		return hits[i] < hits[j]
	})

	return hits[0], true
}

// Copy transfers files whose extensions appear in DataExts without replacing
// existing destination files. fn may report each processed file. The operation
// returns the number copied and fails if dstDir still has no MAX.RES.
func Copy(srcDir, dstDir string, fn Progress) (int, error) {
	entries, err := os.ReadDir(srcDir)

	if err != nil {
		return 0, fmt.Errorf("bootstrap: read source: %w", err)
	}

	var names []string

	for _, e := range entries {
		if !e.IsDir() && isDataFile(e.Name()) {
			names = append(names, e.Name())
		}
	}

	sort.Strings(names)

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return 0, fmt.Errorf("bootstrap: create destination: %w", err)
	}

	copied := 0

	for i, name := range names {
		if fn != nil {
			fn(i+1, len(names), name)
		}

		dst := filepath.Join(dstDir, name)

		// Never overwrite: a file the user already has stays as it is.
		if _, err := os.Stat(dst); err == nil {
			continue
		}

		if err := copyFile(filepath.Join(srcDir, name), dst); err != nil {
			return copied, fmt.Errorf("bootstrap: copy %s: %w", name, err)
		}

		copied++
	}

	if !HasMaxRes(dstDir) {
		return copied, fmt.Errorf("bootstrap: MAX.RES missing in %s after copy", dstDir)
	}

	return copied, nil
}

// isDataFile reports whether name carries one of the game data extensions.
func isDataFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))

	for _, e := range DataExts {
		if ext == e {
			return true
		}
	}

	return false
}

// HasMaxRes reports whether dir directly holds a max.res (any case).
// The boot uses it as the "is the game data here" gate.
func HasMaxRes(dir string) bool {
	entries, err := os.ReadDir(dir)

	if err != nil {
		return false
	}

	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), "max.res") {
			return true
		}
	}

	return false
}

// copyFile copies one file src -> dst; a partial write is removed so a
// failed bootstrap never leaves a truncated data file behind.
func copyFile(src, dst string) error {
	in, err := os.Open(src)

	if err != nil {
		return err
	}

	defer in.Close()
	out, err := os.Create(dst)

	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}

	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}

	return nil
}
