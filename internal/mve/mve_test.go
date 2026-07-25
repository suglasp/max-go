// mve_test.go : testing unit for mve.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package mve

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/testutil"
)

func TestMoviesDecodeToEOF(t *testing.T) {
	dir := testutil.DataDir(t)
	for _, name := range []string{"MAXINT.MVE", "MAXMVE1.MVE", "MAXMVE2.MVE"} {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			m, err := Open(f)
			if err != nil {
				t.Fatal(err)
			}
			if m.W() <= 0 || m.H() <= 0 {
				t.Fatalf("Open did not expose video dimensions: %dx%d", m.W(), m.H())
			}
			if m.AudioFormat().Rate != 22050 || m.AudioFormat().Channels != 2 || m.AudioFormat().Bits != 16 {
				t.Fatalf("Open did not expose audio format: %+v", m.AudioFormat())
			}
			var frames, w, h int
			for {
				fr, err := m.NextFrame()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatalf("frame %d: %v", frames+1, err)
				}
				if fr.W <= 0 || fr.H <= 0 || len(fr.Pix) != fr.W*fr.H {
					t.Fatalf("frame %d shape = %dx%d len=%d", frames+1, fr.W, fr.H, len(fr.Pix))
				}
				if frames == 0 {
					w, h = fr.W, fr.H
				} else if fr.W != w || fr.H != h {
					t.Fatalf("frame %d shape changed to %dx%d, first was %dx%d", frames+1, fr.W, fr.H, w, h)
				}
				frames++
			}
			if frames == 0 {
				t.Fatal("decoded zero frames")
			}
		})
	}
}

func TestMAXINTFrameGoldens(t *testing.T) {
	dir := testutil.DataDir(t)
	f, err := os.Open(filepath.Join(dir, "MAXINT.MVE"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := Open(f)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]string{
		1:  "82658302b5f3e1782bae197df87176f3a7b55d9f1fd243c38bad0f6f27a521b6",
		10: "59f5ec38022fb7976c842745d8724f152e95cec5f631ff0c68a4997b8e8de719",
		50: "13fb58b56ed2347fad08925b29e5b8666b5d9a43a13e87ae96e453f5c53c44be",
	}
	for i := 1; i <= 50; i++ {
		fr, err := m.NextFrame()
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if i == 1 || i == 10 || i == 50 {
			sum := sha256.Sum256(fr.Pix)
			got := hex.EncodeToString(sum[:])
			if w := want[i]; w != "" && got != w {
				t.Errorf("frame %d hash = %s, want %s", i, got, w)
			}
		}
	}
}

func TestTruncatedInputsReturnErrors(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("short"),
		append([]byte("Interplay MVE File\x1a\x00"), 0, 0),
	} {
		if _, err := Open(bytes.NewReader(data)); err == nil {
			t.Fatalf("Open(%d bytes) succeeded", len(data))
		}
	}
	data := validHeader(control{size: 4, opcode: 0, version: 0})
	data = append(data, 9, 0, opLoadPalette, 0) // opcode says 9 bytes, record has no payload
	data = append(data, 0, 0, opEndOfStream, 0)
	m, err := Open(bytes.NewReader(data))
	if err != nil {
		return
	}
	if _, err := m.NextFrame(); err == nil {
		t.Fatal("NextFrame on truncated opcode succeeded")
	}
}

func validHeader(next control) []byte {
	data := make([]byte, headerSize)
	copy(data, mveTag)
	data[20] = 0x1a
	data[22] = 0x00
	data[23] = 0x01
	version := uint32(0x0100)
	checksum := uint16((^version + 0x1234) & 0xffff)
	data[24] = byte(checksum)
	data[25] = byte(checksum >> 8)
	data[26] = byte(next.size)
	data[27] = byte(next.size >> 8)
	data[28] = next.opcode
	data[29] = next.version
	return data
}
