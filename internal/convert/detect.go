// detect.go : classifies asset files and dispatches their conversion.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// mveTag opens every Interplay MVE movie.
const mveTag = "Interplay MVE File\x1a\x00"

// FLI and FLC animations carry their type in the second word of the header.
const (
	flicTypeFLI = 0xaf11
	flicTypeFLC = 0xaf12
)

// Kind names the sort of asset a file holds.
type Kind int

const (
	// KindResource is a loose resource unpacked from a RES archive: an image,
	// a font, a palette or text.
	KindResource Kind = iota
	// KindAudio is a RIFF/WAVE stream, whether named .WAV or .MSC.
	KindAudio
	// KindAnimation is a FLI/FLC animation.
	KindAnimation
	// KindMovie is an Interplay MVE movie.
	KindMovie
)

/* // note: unused code in Go port
// String names the kind for messages.
func (k Kind) String() string {
	switch k {
	case KindAudio:
		return "audio"
	case KindAnimation:
		return "animation"
	case KindMovie:
		return "movie"
	}
	return "resource"
}
*/

// Detect classifies a file. The extension decides first, since M.A.X. names its
// assets consistently, and the leading bytes settle anything left over: an
// unpacked RES resource has no extension at all.
func Detect(path string, data []byte) Kind {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav", ".msc":
		return KindAudio
	case ".flc", ".fli":
		return KindAnimation
	case ".mve":
		return KindMovie
	}

	switch {
	case len(data) >= 20 && string(data[:20]) == mveTag:
		return KindMovie
	case isRIFFWave(data):
		return KindAudio
	case isFLIC(data):
		return KindAnimation
	}
	return KindResource
}

// isFLIC reports whether data opens with a FLI or FLC file header.
func isFLIC(data []byte) bool {
	if len(data) < 6 {
		return false
	}
	switch binary.LittleEndian.Uint16(data[4:6]) {
	case flicTypeFLI, flicTypeFLC:
		return true
	}
	return false
}

// ConvertFile converts one asset into outDir and returns the files written.
// Sound comes out as a .wav, animations and movies as numbered frames (a movie
// also yields its sound track), and anything else is treated as a raw resource
// and rendered through ConvertResource.
func ConvertFile(path, outDir string, opt Options) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	switch Detect(path, data) {
	case KindAudio:
		wav, err := ConvertAudio(data)
		if err != nil {
			return nil, err
		}
		name := base + ".wav"
		if err := os.WriteFile(filepath.Join(outDir, name), wav, 0o644); err != nil {
			return nil, err
		}
		return []string{name}, nil

	case KindAnimation:
		return ConvertAnimation(bytes.NewReader(data), outDir, base, opt.Format)

	case KindMovie:
		return ConvertMovie(bytes.NewReader(data), outDir, base, opt.Format)
	}

	pal := opt.Palette
	if pal == nil {
		g := grayscalePalette()
		pal = &g
	}
	suffix, out, ok := ConvertResource(filepath.Base(path), data, pal, opt.Format)
	if !ok {
		return nil, fmt.Errorf("convert: %s: unrecognised resource", filepath.Base(path))
	}
	name := base + suffix
	if err := os.WriteFile(filepath.Join(outDir, name), out, 0o644); err != nil {
		return nil, err
	}
	return []string{name}, nil
}
