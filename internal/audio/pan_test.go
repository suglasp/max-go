// pan_test.go : testing unit for pan.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestPanStereoPCMCenterLeavesPCMUnchanged(t *testing.T) {
	in := stereoPCM(1000, -2000, -3000, 4000)
	got := panStereoPCM(in, 0)
	if string(got) != string(in) {
		t.Fatalf("center pan changed PCM: %v, want %v", got, in)
	}
	got = panStereoPCM(in, math.NaN())
	if string(got) != string(in) {
		t.Fatalf("NaN pan changed PCM: %v, want %v", got, in)
	}
}

func TestPanStereoPCMHardLeftRightAndClamp(t *testing.T) {
	in := stereoPCM(1000, 2000, -3000, -4000)

	gotLeft := panStereoPCM(in, -2)
	wantLeft := stereoPCM(1000, 0, -3000, 0)
	if string(gotLeft) != string(wantLeft) {
		t.Fatalf("left pan = %v, want %v", gotLeft, wantLeft)
	}

	gotRight := panStereoPCM(in, 2)
	wantRight := stereoPCM(0, 2000, 0, -4000)
	if string(gotRight) != string(wantRight) {
		t.Fatalf("right pan = %v, want %v", gotRight, wantRight)
	}

	if string(in) != string(stereoPCM(1000, 2000, -3000, -4000)) {
		t.Fatalf("pan mutated source PCM: %v", in)
	}
}

func TestPanStereoPCMIntermediatePreservesTrailingBytes(t *testing.T) {
	in := append(stereoPCM(1000, 2000), 0xaa, 0xbb)
	got := panStereoPCM(in, 0.5)
	want := append(stereoPCM(500, 2000), 0xaa, 0xbb)
	if string(got) != string(want) {
		t.Fatalf("half-right pan = %v, want %v", got, want)
	}
}

// TestPanReaderAppliesLivePan checks the streaming reader that can be panned
// again live (the Oto held sound path): each frame is panned with the CURRENT
// pan, so a setPan between reads moves later frames, the positional pan of a
// driving unit applied frame by frame.
func TestPanReaderAppliesLivePan(t *testing.T) {
	// Two stereo frames, every sample 1000.
	src := bytes.NewReader(stereoPCM(1000, 1000, 1000, 1000))
	pr := newPanReader(src, 0) // start centred
	frame := make([]byte, frameSize)

	// First frame at centre pan: unchanged.
	if n, _ := pr.Read(frame); n != frameSize {
		t.Fatalf("first read n=%d, want %d", n, frameSize)
	}
	if string(frame) != string(stereoPCM(1000, 1000)) {
		t.Fatalf("centre pan changed frame: %v", frame)
	}

	// Pan hard right again (clamped) before the next frame: the left channel drops.
	pr.setPan(2)
	if n, _ := pr.Read(frame); n != frameSize {
		t.Fatalf("second read n=%d, want %d", n, frameSize)
	}
	if string(frame) != string(stereoPCM(0, 1000)) {
		t.Fatalf("hard-right pan = %v, want left muted", frame)
	}
}

func stereoPCM(samples ...int16) []byte {
	out := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(sample))
	}
	return out
}
