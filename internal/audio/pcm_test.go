// pcm_test.go : testing unit for pcm.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

import (
	"io"
	"testing"
)

func TestPCMBytesUpsamplesStereo(t *testing.T) {
	got, _ := PCMBytesPhase([]int16{1, 2, 3, 4}, PCMFormat{Rate: 22050, Channels: 2, Bits: 16}, 44100, 0)
	want := []byte{
		1, 0, 2, 0,
		1, 0, 2, 0,
		3, 0, 4, 0,
		3, 0, 4, 0,
	}
	if string(got) != string(want) {
		t.Fatalf("pcm bytes = %v, want %v", got, want)
	}
}

func TestPCMBytesUpmixesMono(t *testing.T) {
	got, _ := PCMBytesPhase([]int16{-1, 0x1234}, PCMFormat{Rate: 22050, Channels: 1, Bits: 16}, 22050, 0)
	want := []byte{
		0xff, 0xff, 0xff, 0xff,
		0x34, 0x12, 0x34, 0x12,
	}
	if string(got) != string(want) {
		t.Fatalf("pcm bytes = %v, want %v", got, want)
	}
}

func TestPCMBytesPhaseMatchesOneShot(t *testing.T) {
	format := PCMFormat{Rate: 32000, Channels: 2, Bits: 16}
	samples := []int16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	want, _ := PCMBytesPhase(samples, format, 44100, 0)

	var got []byte
	phase := 0
	for _, chunk := range [][]int16{samples[:2], samples[2:6], samples[6:]} {
		var part []byte
		part, phase = PCMBytesPhase(chunk, format, 44100, phase)
		got = append(got, part...)
	}
	if string(got) != string(want) {
		t.Fatalf("chunked pcm bytes = %v, want one-shot %v", got, want)
	}
}

func TestPCMResamplePhaseMatchesOneShot(t *testing.T) {
	// 22050 to 44100 is the movie versus device case that produced the SoLoud
	// crackle; chunked resampling must equal the result from a single pass so
	// no boundary artifacts are introduced.
	format := PCMFormat{Rate: 22050, Channels: 2, Bits: 16}
	samples := []int16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	want, _ := PCMResamplePhase(samples, format, 44100, 0)

	var got []int16
	phase := 0
	for _, chunk := range [][]int16{samples[:2], samples[2:8], samples[8:]} {
		var part []int16
		part, phase = PCMResamplePhase(chunk, format, 44100, phase)
		got = append(got, part...)
	}
	if len(got) != len(want) {
		t.Fatalf("chunked resample len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chunked resample[%d] = %d, want %d", i, got[i], want[i])
		}
	}
	// 2x upsample of 6 stereo frames -> 12 stereo frames -> 24 samples.
	if len(got) != 24 {
		t.Errorf("got %d samples, want 24 (6 frames x2 x2ch)", len(got))
	}
}

func TestPCMStreamPadsUnderrunWithSilence(t *testing.T) {
	s := newPCMStream()
	s.Write([]byte{1, 2, 3})
	buf := make([]byte, 6)
	n, err := s.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(buf) {
		t.Fatalf("Read n = %d, want %d", n, len(buf))
	}
	if want := []byte{1, 2, 3, 0, 0, 0}; string(buf) != string(want) {
		t.Fatalf("Read buf = %v, want %v", buf, want)
	}
	s.Close()
	n, err = s.Read(buf)
	if n != 0 || err != io.EOF {
		t.Fatalf("Read after close = %d, %v; want 0, EOF", n, err)
	}
}
