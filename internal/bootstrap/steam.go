// steam.go : finds Steam app 615250 by parsing VDF and ACF files.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package bootstrap

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// vdfPair matches one quoted key/value pair on a VDF line: "key"  "value".
var vdfPair = regexp.MustCompile(`^\s*"([^"]+)"\s+"([^"]*)"\s*$`)

// steamDataDir resolves a Steam root to the M.A.X. data directory: it reads
// steamapps/libraryfolders.vdf for every library, finds the library holding
// appmanifest_615250.acf, resolves the manifest's installdir under
// steamapps/common, and walks that folder for MAX.RES (the Steam M.A.X. is a
// DOSBox bundle, so the data sits in a subfolder).
func steamDataDir(root string) (string, bool) {
	for _, lib := range steamLibraries(root) {
		manifest := filepath.Join(lib, "steamapps", "appmanifest_"+SteamAppID+".acf")
		data, err := os.ReadFile(manifest)

		if err != nil {
			continue
		}

		installDir := vdfValue(string(data), "installdir")

		if installDir == "" {
			continue
		}

		gameRoot := filepath.Join(lib, "steamapps", "common", installDir)

		if dir, ok := FindDataDir(gameRoot); ok {
			return dir, true
		}
	}

	return "", false
}

// steamLibraries returns root and each library in steamapps/libraryfolders.vdf. It supports current "path" entries and legacy numeric key/path pairs.
func steamLibraries(root string) []string {
	libs := []string{root}
	data, err := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))

	if err != nil {
		return libs
	}

	seen := map[string]bool{cleanPath(root): true}

	for _, line := range strings.Split(string(data), "\n") {
		m := vdfPair.FindStringSubmatch(line)

		if m == nil {
			continue
		}

		key, val := m[1], m[2]

		// Current layout: a "path" key. Old layout: a plain number key whose
		// value is the library path. Everything else ("apps", ids, ...) is noise.
		if key != "path" && !isAllDigits(key) {
			continue
		}

		p := cleanPath(val)

		if p == "" || seen[p] {
			continue
		}

		// The old layout's numbered keys also cover non path values (label
		// counts etc. do not exist there, but stay defensive): require the
		// directory to exist before treating it as a library.
		if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
			continue
		}

		seen[p] = true
		libs = append(libs, p)
	}

	return libs
}

// vdfValue returns the value of the first key match (case insensitive) in a
// VDF/ACF document, or "".
func vdfValue(doc, key string) string {
	for _, line := range strings.Split(doc, "\n") {
		m := vdfPair.FindStringSubmatch(line)

		if m != nil && strings.EqualFold(m[1], key) {
			return m[2]
		}
	}

	return ""
}

// cleanPath unescapes the doubled backslashes VDF files use and normalizes
// the result for the running OS.
func cleanPath(p string) string {
	p = strings.ReplaceAll(p, `\\`, `\`)
	return filepath.Clean(p)
}

// isAllDigits reports whether s is a plain number (an old layout library key).
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}
