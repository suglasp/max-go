// flic_test.go : tests FLI and FLC frame decoding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package flic

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/testutil"
)

func TestRetailFLCDecodeToEOF(t *testing.T) {
	dir := testutil.DataDir(t)
	files, err := filepath.Glob(filepath.Join(dir, "*.FLC"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no retail FLC files found")
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			anim, err := Open(f)
			if err != nil {
				t.Fatal(err)
			}
			header := anim.Header()
			if header.W <= 0 || header.H <= 0 || header.Depth != 8 {
				t.Fatalf("invalid header = %+v", header)
			}
			count := 0
			for {
				frame, err := anim.NextFrame()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if frame.W != header.W || frame.H != header.H {
					t.Fatalf("frame geometry = %dx%d, want %dx%d", frame.W, frame.H, header.W, header.H)
				}
				count++
			}
			if count != header.Frames {
				t.Fatalf("decoded %d frames, want %d", count, header.Frames)
			}
		})
	}
}

func TestAGTFrameGoldens(t *testing.T) {
	dir := testutil.DataDir(t)
	f, err := os.Open(filepath.Join(dir, "AGT.FLC"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	anim, err := Open(f)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]string{
		1:  "7f71b9883161228bdfa3819b77606fd6d1039aa4e592f2d563b75a72df256d1a",
		10: "8ad8eb13e0bbc2d1e6a57f7607572b037676e8ba37c4a4a197f6cb370c0319bb",
		30: "f3ab0e3a0d9d240ed40b78f05656d300fd143b91db303cab7b4c5ba251a1ad19",
	}
	for i := 1; ; i++ {
		frame, err := anim.NextFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hash, ok := want[i]; ok {
			sum := sha256.Sum256(frame.Pix)
			got := hex.EncodeToString(sum[:])
			if got != hash {
				t.Errorf("frame %d hash = %s, want %s", i, got, hash)
			}
		}
	}
}

func TestTruncatedInputReturnsError(t *testing.T) {
	if _, err := Open(strings.NewReader("short")); err == nil {
		t.Fatal("Open succeeded on truncated input")
	}
	data := makeMinimalFLC()
	for _, n := range []int{128, len(data) - 1} {
		anim, err := Open(strings.NewReader(string(data[:n])))
		if err == nil {
			_, err = anim.NextFrame()
		}
		if err == nil {
			t.Fatalf("truncated size %d returned nil error", n)
		}
	}
}

func makeMinimalFLC() []byte {
	data := make([]byte, 128+16+6)
	binaryLE32(data[0:4], uint32(len(data)))
	binaryLE16(data[4:6], typeFLC)
	binaryLE16(data[6:8], 1)
	binaryLE16(data[8:10], 1)
	binaryLE16(data[10:12], 1)
	binaryLE16(data[12:14], 8)
	binaryLE32(data[16:20], 71)
	binaryLE32(data[80:84], 128)
	binaryLE32(data[128:132], 22)
	binaryLE16(data[132:134], typeFrame)
	binaryLE16(data[134:136], 1)
	binaryLE32(data[144:148], 6)
	binaryLE16(data[148:150], chunkBlack)
	return data
}

func binaryLE16(dst []byte, v uint16) { dst[0], dst[1] = byte(v), byte(v>>8) }

func binaryLE32(dst []byte, v uint32) {
	dst[0], dst[1], dst[2], dst[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
}
