// icon.go : embeds the application icon sizes used by ebiten.SetWindowIcon.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package icon provides the port's original window icon as decoded images for ebiten.SetWindowIcon.
// The master is ico/max-go.ico, which is also compiled into the Windows executable through cmd/max-go/max_icon_windows_amd64.syso.
// These PNG copies come from that ICO so image/png can decode them without another dependency. They sit here because go:embed cannot use `..` paths.
package icon

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
)

//go:embed max-go-32.png
var png32 []byte

//go:embed max-go-16.png
var png16 []byte

// Images returns the decoded 32- and 16-pixel icons, largest first. Invalid PNGs are skipped, so callers must handle an empty result.
func Images() []image.Image {
	var imgs []image.Image
	for _, data := range [][]byte{png32, png16} {
		if im, err := png.Decode(bytes.NewReader(data)); err == nil {
			imgs = append(imgs, im)
		}
	}
	return imgs
}
