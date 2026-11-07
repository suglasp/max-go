// player_test.go : tests FLIC playback, looping, and scaling.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package flic

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
)

func TestDrawFrameScalesIntoRectGolden(t *testing.T) {
	dst := gfx.New(5, 5)
	dst.Fill(9)
	frame := &Frame{W: 2, H: 2, Pix: []byte{1, 2, 3, 4}}

	if err := DrawFrame(dst, gfx.NewRect(1, 1, 4, 4), frame); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(dst.Pix)
	const want = "262257a0e3edf010692bbba23363f89018fbca4befac21b15c02facd97a7418b"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("scaled portrait hash = %s, want %s", got, want)
	}
}

func TestPlayerDrawNextAndLoop(t *testing.T) {
	p, err := NewPlayer(bytes.NewReader(oneFrameLiteralFLC()), gfx.NewRect(1, 1, 2, 2), true)
	if err != nil {
		t.Fatal(err)
	}
	dst := gfx.New(4, 4)

	if err := p.DrawNext(dst); err != nil {
		t.Fatal(err)
	}
	if dst.At(1, 1) != 1 || dst.At(2, 1) != 2 || dst.At(1, 2) != 3 || dst.At(2, 2) != 4 {
		t.Error("first frame was not copied to the portrait rect")
	}

	dst.Fill(0)
	if err := p.DrawNext(dst); err != nil {
		t.Fatal(err)
	}
	if dst.At(1, 1) != 1 || dst.At(2, 2) != 4 {
		t.Error("looped frame was not redrawn after EOF")
	}
}

func TestPlayerDrawNextEOFWhenNotLooping(t *testing.T) {
	p, err := NewPlayer(bytes.NewReader(oneFrameLiteralFLC()), gfx.NewRect(0, 0, 1, 1), false)
	if err != nil {
		t.Fatal(err)
	}
	dst := gfx.New(2, 2)
	if err := p.DrawNext(dst); err != nil {
		t.Fatal(err)
	}
	if err := p.DrawNext(dst); !errors.Is(err, io.EOF) {
		t.Fatalf("second DrawNext err = %v, want io.EOF", err)
	}
}

func oneFrameLiteralFLC() []byte {
	data := make([]byte, 128+16+6+4)
	binaryLE32(data[0:4], uint32(len(data)))
	binaryLE16(data[4:6], typeFLC)
	binaryLE16(data[6:8], 1)
	binaryLE16(data[8:10], 2)
	binaryLE16(data[10:12], 2)
	binaryLE16(data[12:14], 8)
	binaryLE32(data[16:20], 71)
	binaryLE32(data[80:84], 128)

	binaryLE32(data[128:132], uint32(16+6+4))
	binaryLE16(data[132:134], typeFrame)
	binaryLE16(data[134:136], 1)
	binaryLE32(data[144:148], uint32(6+4))
	binaryLE16(data[148:150], chunkLiteral)
	copy(data[150:154], []byte{1, 2, 3, 4})
	return data
}
