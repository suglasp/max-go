//go:build windows

// load_windows.go : loads the miniaudio DLL on Windows.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package miniaudio

import "syscall"

func openLibrary(path string) (uintptr, error) {
	h, err := syscall.LoadLibrary(path)
	return uintptr(h), err
}

func defaultLibName() string {
	return "miniaudio_x64.dll"
}
