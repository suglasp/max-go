// portrait_test.go : tests portrait.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/flic"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/testutil"
)

func TestPortraitResTag(t *testing.T) {
	cases := map[assets.UnitType]string{
		assets.Tank:     "F_TANK",
		assets.Awac:     "F_AWAC",
		assets.PowGen:   "F_POWGEN",
		assets.Constrct: "F_CONSTR",
		assets.SpFlak:   "F_FLAK",   // exception
		assets.CommTwr:  "F_COMMT",  // exception
		assets.Cnct4W:   "F_CNCT4W", // exception
	}
	for typ, want := range cases {
		if got := portraitResTag(typ); got != want {
			t.Errorf("portraitResTag(%v) = %q, want %q", typ, got, want)
		}
	}
}

func TestPortraitFileName(t *testing.T) {
	if got := portraitFileName([]byte("tank03.flc\x00")); got != "tank03.flc" {
		t.Errorf("portraitFileName = %q", got)
	}
}

// TestPortraitPipeline walks the real data path: F_<tag> resource -> loose .FLC
// file -> decoded first frame, for the few units whose portraits should exist.
func TestPortraitPipeline(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	dst := gfx.New(640, 480)
	for _, typ := range []assets.UnitType{assets.Tank, assets.Awac, assets.Constrct} {
		raw, err := lib.Read(portraitResTag(typ))
		if err != nil {
			t.Errorf("%v: read tag: %v", typ, err)
			continue
		}
		f := openPortraitFile(dir, portraitFileName(raw))
		if f == nil {
			t.Errorf("%v: portrait file %q not found", typ, portraitFileName(raw))
			continue
		}
		p, err := flic.NewPlayer(f, CornerFlic, true)
		if err != nil {
			f.Close()
			t.Errorf("%v: new player: %v", typ, err)
			continue
		}
		if _, err := p.DrawNext(dst); err != nil {
			t.Errorf("%v: draw: %v", typ, err)
		}
		f.Close()
	}
}
