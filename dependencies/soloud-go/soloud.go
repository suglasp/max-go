// soloud.go : defines a no-CGO Go binding for the SoLoud C API, loaded at runtime through purego.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// SoLoud (c) 2013-2020 Jari Komppa, http://soloud-audio.com
package soloud

import (
	"fmt"

	"github.com/ebitengine/purego"
)

var libHandle uintptr

// Load loads the SoLoud shared library using the platform default name and binds all C API functions.
// The library is soloud_x64.dll or soloud_x86.dll on Windows and libsoloud_x64.so or libsoloud_x64.dylib elsewhere. Call Load before any other package function.
func Load() error {
	return LoadPath(DefaultLibName())
}

// LoadPath loads the SoLoud shared library from an explicit path and binds all C API functions.
// Calling it again after a successful load does nothing.
func LoadPath(path string) (err error) {
	if libHandle != 0 {
		return nil
	}

	h, err := openLibrary(path)
	if err != nil {
		return fmt.Errorf("soloud: cannot load %q: %w", path, err)
	}

	defer func() {
		// purego panics if a symbol is missing. Report it as an error instead.
		if r := recover(); r != nil {
			libHandle = 0
			err = fmt.Errorf("soloud: failed to bind %q: %v", path, r)
		}
	}()

	libHandle = h

	registerEngine()
	registerBus()
	registerFilters()
	registerSources()

	return nil
}

// reg binds a single exported C function to a Go function pointer.
func reg[T any](fptr *T, name string) {
	purego.RegisterLibFunc(fptr, libHandle, name)
}

// Error codes, from SoLoud::SOLOUD_ERRORS (soloud.h).
const (
	SO_NO_ERROR       = 0 // No error
	INVALID_PARAMETER = 1 // Some parameter is invalid
	FILE_NOT_FOUND    = 2 // File not found
	FILE_LOAD_FAILED  = 3 // File found, but could not be loaded
	DLL_NOT_FOUND     = 4 // DLL not found, or wrong DLL
	OUT_OF_MEMORY     = 5 // Out of memory
	NOT_IMPLEMENTED   = 6 // Feature not implemented
	UNKNOWN_ERROR     = 7 // Other error
)

// SoloudError is a nonzero SoLoud result code returned by the C API.
type SoloudError int32

func (e SoloudError) Error() string {
	switch int32(e) {
	case INVALID_PARAMETER:
		return "soloud: invalid parameter"

	case FILE_NOT_FOUND:
		return "soloud: file not found"

	case FILE_LOAD_FAILED:
		return "soloud: file found, but could not be loaded"

	case DLL_NOT_FOUND:
		return "soloud: DLL not found, or wrong DLL"

	case OUT_OF_MEMORY:
		return "soloud: out of memory"

	case NOT_IMPLEMENTED:
		return "soloud: feature not implemented"

	default:
		return fmt.Sprintf("soloud: unknown error (%d)", int32(e))
	}
}

// toErr converts a C API result code to a Go error (nil on SO_NO_ERROR).
func toErr(code int32) error {
	if code == SO_NO_ERROR {
		return nil
	}

	return SoloudError(code)
}

// AudioSource is implemented by every playable object in this package, such as Wav, WavStream, Speech, Sfxr, and Bus.
// It mirrors the C API AudioSource handle.
type AudioSource interface {
	handle() uintptr
}

// Filter is implemented by every package filter, such as EchoFilter and BiquadResonantFilter.
// It mirrors the C API Filter handle.
type Filter interface {
	filterHandle() uintptr
}

func sourcePtr(s AudioSource) uintptr {
	if s == nil {
		return 0
	}

	return s.handle()
}

func filterPtr(f Filter) uintptr {
	if f == nil {
		return 0
	}

	return f.filterHandle()
}

// slicePtr returns a pointer to the first element of s, or nil for an empty slice (C takes a null pointer then).
func slicePtr[T any](s []T) *T {
	if len(s) == 0 {
		return nil
	}

	return &s[0]
}
