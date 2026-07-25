// randomizer_test.go : tests deterministic PRNG output and state restoration.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import "testing"

func TestRandomizerSequences(t *testing.T) {
	tests := []struct {
		seed uint32
		want []uint32
	}{
		{
			seed: 0,
			want: []uint32{3178874482, 1827297375, 2237709432, 2099518030, 779316060, 2002627202, 4166341118, 3752644448, 2651693504, 1763609627},
		},
		{
			seed: 1,
			want: []uint32{323185633, 1243117003, 1899959941, 2285029698, 1425567871, 501562882, 1720494177, 1817664814, 947661340, 446512671},
		},
		{
			seed: 0x12345678,
			want: []uint32{3751270866, 3432419414, 221261606, 2207902297, 3086724081, 1004980662, 1923520438, 2380616122, 860453628, 331473328},
		},
		{
			seed: 0xffffffff,
			want: []uint32{868588529, 3179226918, 925274858, 2351880086, 2067294267, 2860091318, 376648949, 3803016768, 3330736783, 2490186439},
		},
	}
	for _, tt := range tests {
		r := NewRandomizer(tt.seed)
		for i, want := range tt.want {
			if got := r.Next(); got != want {
				t.Fatalf("seed %#x value %d = %d, want %d", tt.seed, i, got, want)
			}
		}
	}
}

func TestRandomizerGenerate(t *testing.T) {
	r := NewRandomizer(0x12345678)
	want := []int{88, 80, 5, 51, 72, 23, 45, 55, 20, 7, 84, 76, 30, 73, 94, 36, 90, 100, 45, 90}
	for i, want := range want {
		got := r.Generate(101)
		if got != want {
			t.Fatalf("Generate(101) value %d = %d, want %d", i, got, want)
		}
		if got < 0 || got >= 101 {
			t.Fatalf("Generate(101) value %d out of range: %d", i, got)
		}
	}
	if got := NewRandomizer(7).Generate(0); got != 0 {
		t.Fatalf("Generate(0) = %d, want 0", got)
	}
}

func TestGlobalRandomizerWrappers(t *testing.T) {
	RandomizerSetSeed(1)
	if got, want := RandomizerGetNext(), uint32(323185633); got != want {
		t.Fatalf("RandomizerGetNext = %d, want %d", got, want)
	}
	RandomizerSetSeed(0x12345678)
	if got, want := RandomizerGenerate(101), 88; got != want {
		t.Fatalf("RandomizerGenerate(101) = %d, want %d", got, want)
	}
}

// TestNewGameStateReseedsTheRandomizer checks that each new game starts a fresh stream. Without reseeding, a game started after loading would
// continue the restored save stream.
func TestNewGameStateReseedsTheRandomizer(t *testing.T) {
	RandomizerSetSeed(1234) // stand in for the state a load would have restored
	before := RandomizerState()

	p := &StartParams{}
	p.Teams[0].Type = TeamPlayer
	NewGameState(p)

	if RandomizerState() == before {
		t.Error("a new game kept the old RNG state; it must reseed")
	}
}
