// moviepcm.go : the M.A.X. MVE movie PCM bridge: feeds decoded ADPCM audio into the audio backend's ring buffer.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

import (
	"sync"
	"time"

	eaudio "github.com/hajimehoshi/ebiten/v2/audio"

	"github.com/suglasp/max-go/internal/dbg"
)

// moviePCMPlayer streams decoded movie PCM through the Ebitengine/Oto context.
//
// NOTE: movie audio ALWAYS plays through this Oto path, regardless of the
// configured Backend (native/soloud). SoLoud is deliberately not used for movie
// audio because it has no continuous ring buffer source: its only streaming
// primitive is Queue, a chain of Wav sources played one chunk at a time that
// returns silence the instant it momentarily empties, which crackles on the
// steady stream of small MVE chunks arriving frame by frame. Oto's pcmStream is
// exactly the missing ring buffer (silence on underrun) behind a player with a
// 250ms cushion, so it plays the chunks gaplessly. When the configured backend
// is SoLoud this means SoLoud (music/sfx/voice) and Oto (movie audio) briefly
// hold devices at once, which is fine.
//
// The MVE data is 22050 Hz; PCMBytesPhase resamples it to the context rate
// continuously (phase carried across chunks).
type moviePCMPlayer struct {
	mu     sync.Mutex
	stream *pcmStream
	player *eaudio.Player
	format PCMFormat
	phase  int
}

// newMoviePCMPlayer opens an Oto streaming player for movie PCM.
func newMoviePCMPlayer(format PCMFormat, volume float64) (*moviePCMPlayer, error) {
	stream := newPCMStream()
	player, err := context().NewPlayer(stream)
	if err != nil {
		stream.Close()
		return nil, err
	}
	player.SetBufferSize(250 * time.Millisecond)
	player.SetVolume(volume)
	player.Play()
	dbg.Printf("movie pcm (oto): source rate=%d channels=%d -> device rate=%d channels=2, buffer=250ms",
		format.Rate, format.Channels, sampleRate)
	return &moviePCMPlayer{stream: stream, player: player, format: format}, nil
}

// queue appends one decoded movie PCM chunk, resampled to the context rate.
func (p *moviePCMPlayer) queue(samples []int16) {
	if p == nil || len(samples) == 0 {
		return
	}
	p.mu.Lock()
	stream := p.stream
	if stream == nil {
		p.mu.Unlock()
		return
	}
	data, phase := PCMBytesPhase(samples, p.format, sampleRate, p.phase)
	p.phase = phase
	p.mu.Unlock()
	stream.Write(data)
}

// stop halts the player before closing its stream, then drops both so the device is released.
func (p *moviePCMPlayer) stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.player != nil {
		p.player.PauseAndStopReading()
		p.player = nil
	}
	if p.stream != nil {
		p.stream.Close()
		p.stream = nil
	}
	p.phase = 0
}
