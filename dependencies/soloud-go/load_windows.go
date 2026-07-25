//go:build windows

// load_windows.go : loads the SoLoud DLL on Windows.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package soloud

import (
	"runtime"
	"syscall"
)

func openLibrary(path string) (uintptr, error) {
	h, err := syscall.LoadLibrary(path)
	return uintptr(h), err
}

func defaultLibName() string {
	if runtime.GOARCH == "386" {
		return "soloud_x86.dll"
	}
	return "soloud_x64.dll"
}
