// frameinfo_test.go : tests sprite frame metadata decoding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import "testing"

// dTank is the real D_TANK data resource (MAX.RES): eight signed
// base/count bytes then eight signed turret offset pairs.
var dTank = []byte{
	0x00, 0x08, 0x08, 0x08, 0x10, 0x08, 0x00, 0x00,
	0x00, 0x07, 0xf9, 0x07, 0xf8, 0x00, 0xf8, 0xf9,
	0x00, 0xf8, 0x06, 0xf8, 0x07, 0x00, 0x07, 0x06,
}

func TestDecodeFrameInfoTank(t *testing.T) {
	fi := DecodeFrameInfo(dTank)
	if !fi.HasData {
		t.Fatal("HasData = false")
	}
	if fi.ImageBase != 0 || fi.ImageCount != 8 {
		t.Errorf("body base/count = %d/%d, want 0/8", fi.ImageBase, fi.ImageCount)
	}
	if fi.TurretImageBase != 8 || fi.TurretImageCount != 8 {
		t.Errorf("turret base/count = %d/%d, want 8/8", fi.TurretImageBase, fi.TurretImageCount)
	}
	if fi.FiringImageBase != 16 || fi.FiringImageCount != 8 {
		t.Errorf("firing base/count = %d/%d, want 16/8", fi.FiringImageBase, fi.FiringImageCount)
	}
	want := [8]Point{{0, 7}, {-7, 7}, {-8, 0}, {-8, -7}, {0, -8}, {6, -8}, {7, 0}, {7, 6}}
	if fi.AngleOffsets != want {
		t.Errorf("angle offsets = %v, want %v", fi.AngleOffsets, want)
	}
}

// dAwac is the real D_AWAC resource: a 30-frame spinning radar turret with
// zero mount offsets.
var dAwac = []byte{
	0x00, 0x08, 0x08, 0x1e, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

func TestDecodeFrameInfoAwac(t *testing.T) {
	fi := DecodeFrameInfo(dAwac)
	if fi.TurretImageBase != 8 || fi.TurretImageCount != 30 {
		t.Errorf("turret base/count = %d/%d, want 8/30", fi.TurretImageBase, fi.TurretImageCount)
	}
	if fi.AngleOffsets != ([8]Point{}) {
		t.Errorf("angle offsets = %v, want all zero", fi.AngleOffsets)
	}
}

func TestDecodeFrameInfoNegativeBases(t *testing.T) {
	// A signed image_base/turret_image_base must round trip negative.
	fi := DecodeFrameInfo([]byte{0xff, 0x08, 0xfe, 0x04, 0x00, 0x00, 0x00, 0x00})
	if fi.ImageBase != -1 || fi.TurretImageBase != -2 {
		t.Errorf("bases = %d/%d, want -1/-2", fi.ImageBase, fi.TurretImageBase)
	}
}

func TestDecodeFrameInfoNoData(t *testing.T) {
	for _, data := range [][]byte{nil, {}, {0x00, 0x08, 0x08}} {
		fi := DecodeFrameInfo(data)
		if fi.HasData || fi != (FrameInfo{}) {
			t.Errorf("DecodeFrameInfo(%v) = %+v, want zero value", data, fi)
		}
	}
}

func TestDecodeFrameInfoNoTurretOffsets(t *testing.T) {
	// Exactly 8 bytes: header decodes, angle offsets stay zero.
	fi := DecodeFrameInfo([]byte{0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if !fi.HasData {
		t.Fatal("HasData = false")
	}
	if fi.AngleOffsets != ([8]Point{}) {
		t.Errorf("angle offsets = %v, want all zero", fi.AngleOffsets)
	}
}
