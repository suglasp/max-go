//go:build darwin || linux || freebsd

// load_unix.go : loads the SoLoud shared library on Unix platforms.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package soloud

import (
	"runtime"

	"github.com/ebitengine/purego"
)

func openLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

// DefaultLibName is the platform's default library file name, the name Load opens and the game looks for next to its exe.
func DefaultLibName() string {
	if runtime.GOOS == "darwin" {
		return "libsoloud_x64.dylib"
	}
	return "libsoloud_x64.so"
}
