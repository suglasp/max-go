// testutil.go : the M.A.X. test utility helpers - locates and opens real game data for integration tests via envvar 'MAX_DATA_DIR', and builds a silent WAVE for the audio tests.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package testutil provides helpers for tests that use original game data supplied outside the repository, and small
// fixtures shared by several packages.
package testutil

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// LookupDataDir returns the directory named by MAX_DATA_DIR and whether it holds MAX.RES. It never skips, so a fuzz target can add retail seeds when the data is there and still run its own seeds when it is not.
func LookupDataDir() (string, bool) {
	dir := os.Getenv("MAX_DATA_DIR")

	if dir == "" {
		return "", false
	}

	// A folder without MAX.RES is not a game data folder.
	if _, err := os.Stat(filepath.Join(dir, "MAX.RES")); err != nil {
		return dir, false
	}

	return dir, true
}

// DataDir returns the directory named by MAX_DATA_DIR. It skips the calling test or fuzz target when the variable is unset or the directory does not contain MAX.RES.
func DataDir(tb testing.TB) string {
	tb.Helper()

	dir, ok := LookupDataDir()

	// Skip with the reason that tells the user what to set.
	if dir == "" {
		tb.Skip("game data not found (set MAX_DATA_DIR)")
	}

	if !ok {
		tb.Skipf("game data not found in %q (set MAX_DATA_DIR): no MAX.RES", dir)
	}

	return dir
}

// SilentWAV returns one second of silence as a 16 bit mono 22050 Hz PCM WAVE file, for the audio backend tests.
func SilentWAV() []byte {
	const (
		sampleRate    = 22050
		channels      = 1
		bitsPerSample = 16
		samples       = 22050
	)

	// A 44 byte RIFF / WAVE header, then zeroed samples.
	dataBytes := samples * channels * bitsPerSample / 8
	out := make([]byte, 44+dataBytes)
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	copy(out[8:12], "WAVE")
	copy(out[12:16], "fmt ")
	binary.LittleEndian.PutUint32(out[16:20], 16)
	binary.LittleEndian.PutUint16(out[20:22], 1)
	binary.LittleEndian.PutUint16(out[22:24], channels)
	binary.LittleEndian.PutUint32(out[24:28], sampleRate)
	binary.LittleEndian.PutUint32(out[28:32], sampleRate*channels*bitsPerSample/8)
	binary.LittleEndian.PutUint16(out[32:34], channels*bitsPerSample/8)
	binary.LittleEndian.PutUint16(out[34:36], bitsPerSample)
	copy(out[36:40], "data")
	binary.LittleEndian.PutUint32(out[40:44], uint32(dataBytes))

	return out
}
