// wavloop_test.go : testing unit for wavloop.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

// TestIntroLoopWAV checks the single voice intro plus loop slice: it keeps
// [0, loopEnd) (intro plus running region, tail dropped) and reports the loop
// start offset.
func TestIntroLoopWAV(t *testing.T) {
	// 1000 frames, loop [200, 900): 100 frames of tail after the loop end.
	wav, loopStart, ok := introLoopWAV(buildWAVWithLoop(1000, 200, 900, true))
	if !ok {
		t.Fatal("introLoopWAV returned ok=false for a looped WAV")
	}
	w, parsed := parseWAV(wav)
	if !parsed {
		t.Fatal("intro-loop output is not a parseable WAV")
	}
	if frames := (w.dataEnd - w.dataStart) / w.blockAlign; frames != 900 {
		t.Errorf("intro-loop WAV has %d frames, want 900 ([0,loopEnd), tail dropped)", frames)
	}
	if want := time.Duration(200) * time.Second / 22050; loopStart != want {
		t.Errorf("loopStart = %v, want %v", loopStart, want)
	}
	// No smpl loop -> ok=false (the caller then loops the whole clip held).
	if _, _, ok := introLoopWAV(buildWAV(1000, 0, false)); ok {
		t.Error("introLoopWAV should return ok=false for a WAV with no smpl loop")
	}
}

// buildWAV makes a minimal mono 16 bit PCM WAV of n sample frames, optionally
// with a smpl chunk whose first loop runs from loopStart to the final frame
// boundary.
func buildWAV(n, loopStart int, withSmpl bool) []byte {
	return buildWAVWithLoop(n, loopStart, n, withSmpl)
}

func buildWAVWithLoop(n, loopStart, loopEnd int, withSmpl bool) []byte {
	var b bytes.Buffer
	data := make([]byte, n*2) // mono, 16 bit
	for i := range data {
		data[i] = byte(i)
	}
	// fmt chunk body (PCM, mono, 22050, 16 bit).
	fmtBody := make([]byte, 16)
	binary.LittleEndian.PutUint16(fmtBody[0:], 1)     // PCM
	binary.LittleEndian.PutUint16(fmtBody[2:], 1)     // channels
	binary.LittleEndian.PutUint32(fmtBody[4:], 22050) // rate
	binary.LittleEndian.PutUint32(fmtBody[8:], 44100) // byte rate
	binary.LittleEndian.PutUint16(fmtBody[12:], 2)    // block align
	binary.LittleEndian.PutUint16(fmtBody[14:], 16)   // bits

	var body bytes.Buffer
	body.WriteString("WAVE")
	body.WriteString("fmt ")
	binary.Write(&body, binary.LittleEndian, uint32(len(fmtBody)))
	body.Write(fmtBody)
	body.WriteString("data")
	binary.Write(&body, binary.LittleEndian, uint32(len(data)))
	body.Write(data)
	if withSmpl {
		smpl := make([]byte, 36+24)                                   // SamplerChunk + one SampleLoop
		binary.LittleEndian.PutUint32(smpl[28:], 1)                   // num_sample_loops
		binary.LittleEndian.PutUint32(smpl[36+8:], uint32(loopStart)) // loop start
		binary.LittleEndian.PutUint32(smpl[36+12:], uint32(loopEnd))  // loop end boundary
		body.WriteString("smpl")
		binary.Write(&body, binary.LittleEndian, uint32(len(smpl)))
		body.Write(smpl)
	}

	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(body.Len()))
	b.Write(body.Bytes())
	return b.Bytes()
}

func TestSliceFromLoop(t *testing.T) {
	// A WAV with a loop region: slice should keep only samples from loopStart on.
	n, loopStart := 1000, 400
	w := buildWAV(n, loopStart, true)

	info, ok := parseWAV(w)
	if !ok || !info.hasLoop || info.loopStartFrame != loopStart || info.loopEndFrame != n || info.blockAlign != 2 {
		t.Fatalf("parseWAV = %+v ok=%v", info, ok)
	}

	slice := sliceFromLoop(w)
	if slice == nil {
		t.Fatal("sliceFromLoop returned nil for a looped WAV")
	}
	si, ok := parseWAV(slice)
	if !ok {
		t.Fatal("sliced WAV does not parse")
	}
	gotFrames := (si.dataEnd - si.dataStart) / si.blockAlign
	if want := n - loopStart; gotFrames != want {
		t.Errorf("sliced data = %d frames, want %d", gotFrames, want)
	}
	// The slice must start at the loop point sample (first byte = loopStart*2).
	if slice[si.dataStart] != byte(loopStart*2) {
		t.Errorf("slice starts at byte %d, want %d", slice[si.dataStart], byte(loopStart*2))
	}

	// A WAV without a smpl chunk has no loop region: slice returns nil (loop whole).
	if got := sliceFromLoop(buildWAV(n, 0, false)); got != nil {
		t.Error("sliceFromLoop should be nil without a smpl chunk")
	}
}

func TestSliceFromLoopHonorsLoopEnd(t *testing.T) {
	n, loopStart, loopEnd := 12, 3, 7
	w := buildWAVWithLoop(n, loopStart, loopEnd, true)
	info, ok := parseWAV(w)
	if !ok {
		t.Fatal("source WAV does not parse")
	}
	wantData := append([]byte(nil), w[info.dataStart+loopStart*info.blockAlign:info.dataStart+loopEnd*info.blockAlign]...)

	slice := sliceFromLoop(w)
	if slice == nil {
		t.Fatal("sliceFromLoop returned nil")
	}
	si, ok := parseWAV(slice)
	if !ok {
		t.Fatal("sliced WAV does not parse")
	}
	gotData := slice[si.dataStart:si.dataEnd]
	if !bytes.Equal(gotData, wantData) {
		t.Fatalf("sliced loop data = %v, want %v", gotData, wantData)
	}
	if gotFrames := len(gotData) / si.blockAlign; gotFrames != loopEnd-loopStart {
		t.Fatalf("sliced frames = %d, want %d", gotFrames, loopEnd-loopStart)
	}
	if bytes.Contains(gotData, w[info.dataStart+loopEnd*info.blockAlign:info.dataEnd]) {
		t.Fatal("sliced loop data contains trailing frames after smpl loop end")
	}
}
