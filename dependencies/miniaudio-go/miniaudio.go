// miniaudio.go : defines a no-CGO Go binding for miniaudio, loaded at runtime through purego.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// miniaudio itself is in the public domain / MIT-0 (David Reid, https://miniaud.io).
package miniaudio

import (
	"fmt"

	"github.com/ebitengine/purego"
)

var libHandle uintptr

// Load loads the miniaudio wrapper library using the platform default name and binds all wrapper functions.
// The library is miniaudio_x64.dll on Windows and libminiaudio_x64.so or .dylib elsewhere. Call Load before anything else.
func Load() error {
	return LoadPath(DefaultLibName())
}

// LoadPath loads the wrapper library from an explicit path and binds all functions.
// Calling it again after a successful load does nothing.
func LoadPath(path string) (err error) {
	if libHandle != 0 {
		return nil
	}

	h, err := openLibrary(path)
	if err != nil {
		return fmt.Errorf("miniaudio: cannot load %q: %w", path, err)
	}
	defer func() {
		// purego panics if a symbol is missing. Report it as an error instead.
		if r := recover(); r != nil {
			libHandle = 0
			err = fmt.Errorf("miniaudio: failed to bind %q: %v", path, r)
		}
	}()

	libHandle = h

	registerGlobal()
	registerEngine()
	registerSound()
	registerGroup()
	registerStream()
	registerContext()

	return nil
}

// reg binds a single exported wrapper function to a Go function pointer.
func reg[T any](fptr *T, name string) {
	purego.RegisterLibFunc(fptr, libHandle, name)
}

// Global bindings.
var (
	mawVersion           func() string
	mawResultDescription func(int32) string
)

func registerGlobal() {
	reg(&mawVersion, "maw_version")
	reg(&mawResultDescription, "maw_result_description")
}

// Version returns the miniaudio version string, e.g. "0.11.25".
func Version() string {
	return mawVersion()
}

// Result is a miniaudio result code (0 == MA_SUCCESS).
// Nonzero codes implement error through their miniaudio description string.
type Result int32

func (r Result) Error() string {
	return fmt.Sprintf("miniaudio: %s (%d)", mawResultDescription(int32(r)), int32(r))
}

// toErr converts a wrapper result code to a Go error (nil on success).
func toErr(code int32) error {
	if code == 0 {
		return nil
	}

	return Result(code)
}

// errCreate is returned when a create_* wrapper call yields a NULL handle.
// The wrapper hides the underlying ma_result, so this reports the failure generically.
func errCreate(what string) error {
	return fmt.Errorf("miniaudio: failed to create %s", what)
}

// b2i converts a bool to the 0/1 int32 the wrapper expects.
func b2i(b bool) int32 {
	if b {
		return 1
	}

	return 0
}
