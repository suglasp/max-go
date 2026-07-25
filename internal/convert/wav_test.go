// wav_test.go : tests RIFF/WAVE encoding and header detection.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import (
	"encoding/binary"
	"testing"
)

func TestEncodeWAVHeader(t *testing.T) {
	samples := []int16{0, 1, -1, 32767}
	out := encodeWAV(11025, 2, samples)

	dataSize := len(samples) * 2
	if len(out) != wavHeaderSize+dataSize {
		t.Fatalf("len = %d, want %d", len(out), wavHeaderSize+dataSize)
	}
	if string(out[0:4]) != "RIFF" || string(out[8:12]) != "WAVE" {
		t.Fatalf("missing RIFF/WAVE tags")
	}
	if string(out[12:16]) != "fmt " || string(out[36:40]) != "data" {
		t.Fatalf("missing fmt/data chunks")
	}
	if got := binary.LittleEndian.Uint32(out[4:8]); int(got) != 36+dataSize {
		t.Errorf("riff size = %d, want %d", got, 36+dataSize)
	}
	if got := binary.LittleEndian.Uint16(out[20:22]); got != 1 {
		t.Errorf("format tag = %d, want 1 (PCM)", got)
	}
	if got := binary.LittleEndian.Uint16(out[22:24]); got != 2 {
		t.Errorf("channels = %d, want 2", got)
	}
	if got := binary.LittleEndian.Uint32(out[24:28]); got != 11025 {
		t.Errorf("sample rate = %d, want 11025", got)
	}
	if got := binary.LittleEndian.Uint32(out[28:32]); got != 11025*4 {
		t.Errorf("byte rate = %d, want %d", got, 11025*4)
	}
	if got := binary.LittleEndian.Uint16(out[32:34]); got != 4 {
		t.Errorf("block align = %d, want 4", got)
	}
	if got := binary.LittleEndian.Uint32(out[40:44]); int(got) != dataSize {
		t.Errorf("data size = %d, want %d", got, dataSize)
	}
	for i, want := range samples {
		got := int16(binary.LittleEndian.Uint16(out[wavHeaderSize+i*2:]))
		if got != want {
			t.Errorf("sample %d = %d, want %d", i, got, want)
		}
	}
}

// A movie that never announced its audio format still yields a playable file.
func TestEncodeWAVDefaultsFormat(t *testing.T) {
	out := encodeWAV(0, 0, nil)
	if got := binary.LittleEndian.Uint16(out[22:24]); got != 1 {
		t.Errorf("channels = %d, want 1", got)
	}
	if got := binary.LittleEndian.Uint32(out[24:28]); got != 22050 {
		t.Errorf("sample rate = %d, want 22050", got)
	}
}

func TestIsRIFFWave(t *testing.T) {
	wav := encodeWAV(22050, 1, []int16{0})
	if !isRIFFWave(wav) {
		t.Fatalf("a written WAV should be recognised")
	}
	if isRIFFWave([]byte("RIFFxxxxAVI ")) {
		t.Fatalf("RIFF alone is not a WAVE")
	}
	if isRIFFWave([]byte("short")) {
		t.Fatalf("a truncated header should be rejected")
	}
}

func TestConvertAudioRejectsNonWave(t *testing.T) {
	if _, err := ConvertAudio([]byte("not a riff at all")); err == nil {
		t.Fatalf("expected an error for a non RIFF stream")
	}
	wav := encodeWAV(22050, 1, []int16{1, 2})
	out, err := ConvertAudio(wav)
	if err != nil {
		t.Fatalf("ConvertAudio: %v", err)
	}
	if len(out) != len(wav) {
		t.Fatalf("ConvertAudio should hand the bytes back untouched")
	}
}
