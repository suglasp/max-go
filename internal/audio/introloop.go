// introloop.go : the intro loop player: plays a clip to its SMPL loop end boundary then loops from the loop start.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

import (
	"os"
	"sync"
	"time"

	"github.com/suglasp/max-go/internal/dbg"
)

type sfxTimer interface {
	Stop() bool
}

var afterSfxIntro = func(d time.Duration, f func()) sfxTimer {
	return time.AfterFunc(d, f)
}

// PlaySfxIntroThenLoop plays the clip up to its smpl loop end boundary, then
// loops only the smpl running region. It is an audio helper for sounds that
// have a startup transient followed by a steady loop.
func (m *Manager) PlaySfxIntroThenLoop(primary, fallback string, pan, volScale float64) int {
	if !m.cfg.SfxEnabled || m.cfg.SfxVolume == 0 {
		return 0
	}
	held, canHold := m.be.(HeldSfxBackend)
	mem, canMem := m.be.(MemHeldSfxBackend)
	if !canHold || !canMem {
		return m.PlaySfxHeld(primary, fallback, pan, volScale, true)
	}

	v := vol(m.cfg.SfxVolume) * volScale
	tried := ""
	for _, tag := range [2]string{primary, fallback} {
		if tag == "" || tag == tried {
			continue
		}
		tried = tag
		path, err := m.resolvePath(tag)
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			dbg.Printf("sfx (%s) : intro-loop read: %v", tag, err)
			continue
		}
		// Prefer a single continuous voice (no swap between an intro voice and a
		// loop voice) when the backend can set its own loop point: it plays
		// [0,loopEnd) once then loops [loopStart,loopEnd) with NO gap. This is the
		// fix for Oto/SoLoud, whose startup latency for a new voice makes the two
		// voice swap audibly "stop and repeat"; miniaudio (instant new voices) has
		// no such gap and falls through.
		if il, isIL := m.be.(IntroLoopSfxBackend); isIL {
			if introWAV, loopStart, hasLoop := introLoopWAV(raw); hasLoop {
				h, err := il.PlaySfxHeldMemIntroLoop(introWAV, loopStart, v, pan)
				if err != nil {
					m.disableAudio("sfx", tag, err)
					return 0
				}
				if m.sfxHeld == nil {
					m.sfxHeld = map[int]SfxHandle{}
				}
				m.nextHeldID++
				m.sfxHeld[m.nextHeldID] = h
				return m.nextHeldID
			}
		}
		loopWAV := sliceFromLoop(raw)
		delay, ok := wavLoopEndDuration(raw)
		if loopWAV == nil || !ok {
			return m.PlaySfxHeld(tag, "", pan, volScale, true)
		}

		intro, err := held.PlaySfxHeld(path, v, pan, false)
		if err != nil {
			m.disableAudio("sfx", tag, err)
			return 0
		}
		h := &introLoopHandle{current: intro}
		h.timer = afterSfxIntro(delay, func() {
			loop, err := mem.PlaySfxHeldMem(loopWAV, v, pan, true)
			if err != nil {
				dbg.Printf("sfx (%s) : intro-loop start loop: %v", tag, err)
				return
			}
			h.swap(loop)
		})
		if m.sfxHeld == nil {
			m.sfxHeld = map[int]SfxHandle{}
		}
		m.nextHeldID++
		m.sfxHeld[m.nextHeldID] = h
		return m.nextHeldID
	}
	dbg.Printf("sfx (%s/%s) : intro-loop, no resolvable clip", primary, fallback)
	return 0
}

type introLoopHandle struct {
	mu      sync.Mutex
	current SfxHandle
	timer   sfxTimer
	stopped bool
	pan     float64 // last requested pan, applied again to the loop voice on swap
	volume  float64 // last requested volume, applied again to the loop voice on swap
	faded   bool    // volume was set at least once (the loop voice otherwise keeps its start volume)
}

func (h *introLoopHandle) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return
	}
	h.stopped = true
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	if h.current != nil {
		h.current.Stop()
		h.current = nil
	}
}

// SetPan pans the live voice again and remembers the value so the swap from
// the intro voice to the loop voice applies it again to the new loop voice.
func (h *introLoopHandle) SetPan(pan float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pan = pan
	if h.current != nil {
		h.current.SetPan(pan)
	}
}

// SetVolume fades the live voice and remembers the value so the swap from the
// intro voice to the loop voice applies it again.
func (h *introLoopHandle) SetVolume(volume float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.volume, h.faded = volume, true
	if h.current != nil {
		h.current.SetVolume(volume)
	}
}

func (h *introLoopHandle) swap(next SfxHandle) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		next.Stop()
		return
	}
	if h.current != nil {
		h.current.Stop()
	}
	h.current = next
	h.current.SetPan(h.pan) // carry the latest pan over to the new loop voice
	if h.faded {
		h.current.SetVolume(h.volume) // and the latest positional fade
	}
	h.timer = nil
}
