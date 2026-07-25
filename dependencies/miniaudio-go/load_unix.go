//go:build darwin || linux || freebsd

// load_unix.go : loads the miniaudio shared library on Unix platforms.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package miniaudio

import (
	"runtime"

	"github.com/ebitengine/purego"
)

func openLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

func defaultLibName() string {
	if runtime.GOOS == "darwin" {
		return "libminiaudio_x64.dylib"
	}
	return "libminiaudio_x64.so"
}
