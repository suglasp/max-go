// randomizer.go : implements the deterministic PRNG used by game logic.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import "time"

const (
	splitMix64Add  = uint64(0x9e3779b97f4a7c15)
	splitMix64Mul1 = uint64(0xbf58476d1ce4e5b9)
	splitMix64Mul2 = uint64(0x94d049bb133111eb)
)

var randomizer = NewRandomizer(uint32(time.Now().Unix()))

// Randomizer is the game's xoshiro128** PRNG, seeded the same way as the C++ port.
type Randomizer struct {
	state [4]uint32
}

// NewRandomizer creates a randomizer with the same SplitMix64 seeding path (applied twice), used by the original.
//
// C++ ported : randomizer.cpp - Randomizer_SetSeed
func NewRandomizer(seed uint32) *Randomizer {
	r := &Randomizer{}
	r.SetSeed(seed)
	return r
}

// RandomizerInit seeds the global randomizer from the current Unix time.
//
// C++ ported : randomizer.cpp - Randomizer_Init
func RandomizerInit() {
	randomizer = newRandomizerFromSeed64(splitMix64Next(uint64(time.Now().Unix())))
}

// RandomizerSetSeed seeds the global randomizer deterministically.
//
// C++ ported : randomizer.cpp - Randomizer_SetSeed
func RandomizerSetSeed(seed uint32) {
	randomizer = NewRandomizer(seed)
}

// RandomizerGetNext returns the next raw 32-bit random value.
//
// C++ ported : randomizer.cpp - Randomizer_GetNext
func RandomizerGetNext() uint32 {
	return randomizer.Next()
}

// RandomizerGenerate returns a bounded value using the original multiply high mapping.
//
// C++ ported : randomizer.hpp - Randomizer_Generate
func RandomizerGenerate(value int) int {
	return randomizer.Generate(value)
}

// SetSeed resets r to the same state as C++ Xoshiro128StarStar(SplitMix64(seed)()).
//
// C++ ported : randomizer.cpp - Randomizer_SetSeed
func (r *Randomizer) SetSeed(seed uint32) {
	*r = *newRandomizerFromSeed64(splitMix64Next(uint64(seed)))
}

// State returns the PRNG's raw internal state, for save games.
func (r *Randomizer) State() [4]uint32 {
	return r.state
}

// SetState restores the PRNG's raw internal state from a save game.
func (r *Randomizer) SetState(s [4]uint32) {
	r.state = s
}

// RandomizerState returns the global randomizer's raw state (save games).
func RandomizerState() [4]uint32 {
	return randomizer.State()
}

// RandomizerSetState restores the global randomizer's raw state (save games).
func RandomizerSetState(s [4]uint32) {
	randomizer.SetState(s)
}

// Next returns the next xoshiro128** value.
//
// C++ ported : XoshiroCpp::Xoshiro128StarStar::operator()
func (r *Randomizer) Next() uint32 {
	result := rotateLeft32(r.state[1]*5, 7) * 9
	t := r.state[1] << 9
	r.state[2] ^= r.state[0]
	r.state[3] ^= r.state[1]
	r.state[1] ^= r.state[2]
	r.state[0] ^= r.state[3]
	r.state[2] ^= t
	r.state[3] = rotateLeft32(r.state[3], 11)
	return result
}

// Generate returns int((Next() * value) >> 32), matching the C++ template.
//
// C++ ported : randomizer.hpp - Randomizer_Generate
func (r *Randomizer) Generate(value int) int {
	return int((int64(r.Next()) * int64(value)) >> 32)
}

// newRandomizerFromSeed64 expands one 64-bit seed into xoroshiro state.
func newRandomizerFromSeed64(seed uint64) *Randomizer {
	r := &Randomizer{}
	split := seed
	for i := range r.state {
		r.state[i] = uint32(splitMix64Step(&split))
	}
	return r
}

// splitMix64Next returns the first SplitMix64 output for seed.
func splitMix64Next(seed uint64) uint64 {
	return splitMix64Step(&seed)
}

// splitMix64Step advances state and returns the next SplitMix64 value.
func splitMix64Step(state *uint64) uint64 {
	*state += splitMix64Add
	z := *state
	z = (z ^ (z >> 30)) * splitMix64Mul1
	z = (z ^ (z >> 27)) * splitMix64Mul2
	return z ^ (z >> 31)
}

// rotateLeft32 rotates a 32-bit value left by k bits.
func rotateLeft32(x uint32, k int) uint32 {
	return (x << k) | (x >> (32 - k))
}
