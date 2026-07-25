//go:build windows

// probe_windows.go : reads GOG and Steam installation paths from the Windows registry.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package bootstrap

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// hostProber probes the Windows registry for GOG and Steam installs.
type hostProber struct{}

// GOGDir reads the GOG product's DOSBOXFOLDER value. The port is x86-64
// only, so the WOW6432Node view holds GOG's 32 bit keys; the bare path is
// tried too as a fallback.
func (hostProber) GOGDir() string {
	keys := []string{
		`SOFTWARE\WOW6432Node\GOG.com\Games\` + GOGProductID,
		`SOFTWARE\GOG.com\Games\` + GOGProductID,
	}

	for _, path := range keys {
		if v := regString(registry.LOCAL_MACHINE, path, "DOSBOXFOLDER"); v != "" {
			return v
		}

		// Some GOG installers record only the plain install path.
		if v := regString(registry.LOCAL_MACHINE, path, "PATH"); v != "" {
			return v
		}
	}

	return ""
}

// SteamRoots reads the Steam installation root from the machine wide
// InstallPath and the per user SteamPath, best first. The two values name
// the same folder on most machines (differing only in case and slashes), so
// duplicates are dropped.
func (hostProber) SteamRoots() []string {
	var roots []string

	add := func(v string) {
		if v == "" {
			return
		}

		v = filepath.Clean(v)

		for _, have := range roots {
			if strings.EqualFold(have, v) {
				return
			}
		}

		roots = append(roots, v)
	}

	add(regString(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath"))
	add(regString(registry.CURRENT_USER, `SOFTWARE\Valve\Steam`, "SteamPath"))
	return roots
}

// regString reads one registry string value, returning "" on any error.
func regString(root registry.Key, path, name string) string {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)

	if err != nil {
		return ""
	}

	defer k.Close()
	v, _, err := k.GetStringValue(name)

	if err != nil {
		return ""
	}

	return v
}
