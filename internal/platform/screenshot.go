// screenshot.go : the M.A.X. screenshot writer - saves the current 8 bit screen as a PNG to the 'screenshots' folder.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package platform

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

// SaveScreenshot writes the current frame to a uniquely named PNG in the screenshots folder, creating the folder when needed.
func (p *Platform) SaveScreenshot() (string, error) {
	dir := filepath.Join(p.cfg.BaseDir, "screenshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("platform: create %s: %w", dir, err)
	}

	base := time.Now().Format("max-go_20060102_150405")
	path := filepath.Join(dir, base+".png")
	for n := 2; ; n++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s_%d.png", base, n))
	}
	return path, p.WriteScreenshot(path)
}

// ScreenshotErr reports why the one-shot -screenshot file could not be written, or nil when it was written or
// was never asked for. The run ends after that frame either way; the caller turns the error into exit code 1.
func (p *Platform) ScreenshotErr() error {
	return p.shotErr
}

// WriteScreenshot encodes the current logical frame (the indexed
// screen through the current palette) as a PNG file.
func (p *Platform) WriteScreenshot(path string) error {
	p.mu.Lock()
	pal := make(color.Palette, len(p.lut))
	for i, c := range p.lut {
		pal[i] = color.RGBA{c[0], c[1], c[2], c[3]}
	}
	img := image.NewPaletted(image.Rect(0, 0, p.screen.W, p.screen.H), pal)
	for y := 0; y < p.screen.H; y++ {
		copy(img.Pix[y*img.Stride:], p.screen.Row(y))
	}
	p.mu.Unlock()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
