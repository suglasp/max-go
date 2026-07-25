// paths.go : the M.A.X. saveload path helpers - resolves the save directory and per saveslot file paths next to the binary.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"fmt"
	"os"
	"path/filepath"
)

// Ext is the save game file extension.
const Ext = ".sav"

// FindSlot locates a one-based slot, preferring save<N> and accepting the legacy slot%02d name. It returns false when neither file exists.
func FindSlot(baseDir string, slot int) (path string, ok bool) {
	for _, stem := range []string{fmt.Sprintf("save%d", slot), fmt.Sprintf("slot%02d", slot)} {
		p, err := Path(baseDir, stem)
		if err != nil {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// Dir returns the saves folder next to the binary (baseDir/saves), creating it
// if missing. C++: the game keeps saves alongside the executable.
func Dir(baseDir string) (string, error) {
	dir := filepath.Join(baseDir, "saves")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Path is the full path of a named save slot in the saves folder.
func Path(baseDir, name string) (string, error) {
	dir, err := Dir(baseDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+Ext), nil
}

/* // note: unused code in Go port
// List returns the save slot names (without extension) in the saves folder.
func List(baseDir string) ([]string, error) {
	dir, err := Dir(baseDir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == Ext {
			names = append(names, e.Name()[:len(e.Name())-len(Ext)])
		}
	}
	return names, nil
}
*/
