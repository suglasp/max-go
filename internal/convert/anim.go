// anim.go : writes FLI and FLC animation frames as numbered images.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/suglasp/max-go/internal/flic"
)

// frameName numbers a frame so a directory listing keeps playback order.
func frameName(base string, index int, ext string) string {
	return fmt.Sprintf("%s_%04d%s", base, index, ext)
}

// ConvertAnimation decodes a FLI/FLC stream and writes one image per frame into
// outDir, named base_0000, base_0001 and so on. Every frame carries the palette
// in force at that point, so each image colours itself. It returns the names of
// the files written.
func ConvertAnimation(r io.ReadSeeker, outDir, base string, format Format) ([]string, error) {
	anim, err := flic.Open(r)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	var written []string
	for index := 0; ; index++ {
		frame, err := anim.NextFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return written, err
		}
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
	if len(written) == 0 {
		return nil, errors.New("convert: animation holds no frames")
	}
	return written, nil
}
