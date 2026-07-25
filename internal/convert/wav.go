// wav.go : wraps PCM16 samples in a RIFF/WAVE file.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import "encoding/binary"

// wavHeaderSize is the canonical 44-byte RIFF/WAVE/fmt/data preamble.
const wavHeaderSize = 44

// encodeWAV wraps signed 16-bit samples in a RIFF/WAV container. Samples of a
// multi channel stream are interleaved, one frame after another.
func encodeWAV(rate, channels int, samples []int16) []byte {
	if rate <= 0 {
		rate = 22050
	}
	if channels <= 0 {
		channels = 1
	}
	const bits = 16
	blockAlign := channels * bits / 8
	dataSize := len(samples) * 2

	out := make([]byte, wavHeaderSize+dataSize)
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(36+dataSize))
	copy(out[8:12], "WAVE")

	copy(out[12:16], "fmt ")
	binary.LittleEndian.PutUint32(out[16:20], 16) // fmt chunk size
	binary.LittleEndian.PutUint16(out[20:22], 1)  // PCM, uncompressed
	binary.LittleEndian.PutUint16(out[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(out[24:28], uint32(rate))
	binary.LittleEndian.PutUint32(out[28:32], uint32(rate*blockAlign))
	binary.LittleEndian.PutUint16(out[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(out[34:36], bits)

	copy(out[36:40], "data")
	binary.LittleEndian.PutUint32(out[40:44], uint32(dataSize))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(out[wavHeaderSize+i*2:], uint16(s))
	}
	return out
}

// isRIFFWave reports whether data opens with a RIFF/WAVE header. M.A.X. ships
// its music as .MSC files that are plain RIFF/WAV under another extension, so
// this is the only check needed before renaming one.
func isRIFFWave(data []byte) bool {
	return len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE"
}
