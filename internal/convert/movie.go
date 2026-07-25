// movie.go : splits an Interplay MVE movie into numbered frames and a WAV track.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/suglasp/max-go/internal/mve"
)

// ConvertMovie decodes an MVE stream into outDir: one image per presentation
// frame, named base_0000 upward, plus base.wav holding the whole sound track.
// The decoder already hands back signed 16-bit samples whether the stream was
// compressed or not, so the WAV writer never sees an encoding. It returns the
// names of the files written.
func ConvertMovie(r io.ReadSeeker, outDir, base string, format Format) ([]string, error) {
	movie, err := mve.Open(r)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	var written []string
	var samples []int16
	for index := 0; ; index++ {
		frame, err := movie.NextFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return written, err
		}
		samples = append(samples, frame.Audio...)
		if frame.W <= 0 || frame.H <= 0 {
			continue
		}
		img := paletteImage(frame.W, frame.H, frame.Pix, frame.Palette[:])
		name := frameName(base, index, format.Ext())
		if err := os.WriteFile(filepath.Join(outDir, name), encode(img, format), 0o644); err != nil {
			return written, err
		}
		written = append(written, name)
	}

	if len(samples) > 0 {
		af := movie.AudioFormat()
		name := base + ".wav"
		wav := encodeWAV(af.Rate, af.Channels, samples)
		if err := os.WriteFile(filepath.Join(outDir, name), wav, 0o644); err != nil {
			return written, err
		}
		written = append(written, name)
	}
	if len(written) == 0 {
		return nil, errors.New("convert: movie holds no frames")
	}
	return written, nil
}
