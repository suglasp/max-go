// audio.go : validates M.A.X. sound assets as RIFF/WAVE data.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import "errors"

// ErrNotAudio is returned when a file that should hold sound does not open
// with a RIFF/WAVE header.
var ErrNotAudio = errors.New("convert: not a RIFF/WAVE stream")

// ConvertAudio returns a M.A.X. sound asset ready to save as .wav. Both WAV and
// MSC files contain RIFF/WAVE data, so conversion only validates the header.
func ConvertAudio(data []byte) ([]byte, error) {
	if !isRIFFWave(data) {
		return nil, ErrNotAudio
	}
	return data, nil
}
