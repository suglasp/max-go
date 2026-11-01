// voice.go : implements the announcer engine: the queue, the cooldown and priority
// gate, and the playback entry points every voice line goes through.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : sound_manager.cpp - voice line selection and playback
package gamescreen

import (
	"math/rand"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
)

// queuedVoice is one serialised announcer line waiting in the voice queue, drained
// one at a time by tickVoiceQueue so back to back lines do not cut each other off.
type queuedVoice struct {
	tag      string // the V_F### resource tag to speak
	priority int    // the "still speaking" priority it asserts once played
	desc     string // short label for debug logs
}

// announcerState holds the voice queue, cooldowns, and events already announced to the player.
type announcerState struct {
	voiceFn           func(tag string)     // plays an announcer voice line by resource tag
	colonyAnnounced   [game.TeamCount]bool // "colony started!" line already played, per team
	goldRefinedVoiced [game.TeamCount]bool // "first gold refined" voice already played, per team
	voiceLastMS       [vsStateCount]uint64 // per state voice cooldown gate
	lastVoiceMS       uint64               // last time any voice line played
	lastVoicePriority int                  // priority of the currently speaking line
	voiceQueue        []queuedVoice        // serialised announcer lines (played one at a time)
	pendingVoice      queuedVoice          // the one line waiting for the speaking voice to finish (single voice job)
	hasPendingVoice   bool                 // pendingVoice holds a line
	voicePlayingFn    func() bool          // whether a voice is still audible; nil = fixed spacing (tests)
	clock             func() uint64        // voice timing source; platform.Ticks when nil (tests override)
	lastBeginKey      int                  // turn+team the "Begin!" line last announced (-1 = none yet)
	enemySeen         map[uint16]bool      // enemy unit ids already announced as spotted
	lastSpottedCellX  int                  // cell of the most recently newly spotted enemy (F1 recenter key), X
	lastSpottedCellY  int                  // cell of the most recently newly spotted enemy (F1 recenter key), Y
	hasSpottedEnemy   bool                 // false until the first enemy is seen
	subDetected       map[uint16]bool      // enemy submarine ids the player currently detects
	subSurfacedUntil  map[uint16]uint64    // submarine id -> tick it stays surfaced until
	fuelFull          bool                 // player raw fuel storage is full
	rawFull           bool                 // player raw material storage is full
	goldFull          bool                 // player gold storage is full
	openingTurnDone   bool                 // storage full lines may play after the opening turn ends
	pointsLeader      int                  // team that led on points at the last round close (announcerNoLeader = not seeded)
}

// announcerNoLeader marks the points leader as not yet seeded: the first round close
// records who leads without announcing it.
const announcerNoLeader = -2

// resetAnnouncerForLoad clears queued lines and rebuilds one-shot latches from the restored board. Existing
// colonies, visible enemies, and full stores stay quiet after loading.
func (s *Screen) resetAnnouncerForLoad() {
	a := &s.announcer
	a.voiceQueue = nil // stale "team ended turn" lines must not play into the new game
	a.hasPendingVoice = false
	a.lastBeginKey = -1
	a.pointsLeader = announcerNoLeader // reseeded (silently) at the restored game's first round close
	a.hasSpottedEnemy = false          // F1 recenters only on a spot made IN this game
	a.enemySeen = map[uint16]bool{}
	a.subDetected = map[uint16]bool{}
	a.subSurfacedUntil = nil
	a.colonyAnnounced = [game.TeamCount]bool{}
	a.goldRefinedVoiced = [game.TeamCount]bool{}

	player := s.game.State.PlayerTeam

	// Units the player already sees at load are OLD news: seed them as seen so the
	// first detect poll does not read the whole visible board as "Enemy detected!".
	if s.game.Vis != nil {
		for _, u := range s.game.Units.Units() {
			if u.Team == player || u.Team < 0 || !s.game.Vis.VisibleToTeam(u, player) {
				continue
			}
			if u.Type == assets.Submarne {
				a.subDetected[u.ID] = true
			} else if u.Flags.Has(game.FlagSelectable) {
				a.enemySeen[u.ID] = true
			}
		}
	}

	// A colony already running at save time was announced back then.
	for t := 0; t < game.TeamCount; t++ {
		if t != player && s.workingEcospheres(t) > 0 {
			a.colonyAnnounced[t] = true
		}
		// The "first gold refined" moment is not derivable from the board; a team
		// holding any credits has plausibly had it, so treat it as voiced rather
		// than re-announce a mid game refinery on the first post load income.
		if s.teamCredits(t) > 0 {
			a.goldRefinedVoiced[t] = true
		}
	}

	// Storage that is already full at load stays quiet until it drops and refills.
	var stored, capacity economy.Cargo
	for _, c := range s.game.Complexes {
		if c.Team != player {
			continue
		}
		m, cp := c.CargoInfo()
		stored.Add(m)
		capacity.Add(cp)
	}
	a.fuelFull = capacity.Fuel > 0 && stored.Fuel >= capacity.Fuel
	a.rawFull = capacity.Raw > 0 && stored.Raw >= capacity.Raw
	a.goldFull = capacity.Gold > 0 && stored.Gold >= capacity.Gold
}

// voiceSpeakWindowMS is how long a played line is treated as still speaking for the
// priority gate: a lower priority line within this window is dropped.
const voiceSpeakWindowMS = 2500

// now returns the voice timing clock in milliseconds (platform.Ticks in normal play;
// tests inject s.announcer.clock for deterministic cooldown/priority checks).
func (s *Screen) now() uint64 {
	if s.announcer.clock != nil {
		return s.announcer.clock()
	}

	return platform.Ticks()
}

// playVoice speaks the line for state, honoring its cooldown and priority gate and
// picking a random tag. No op when no voice hook is wired or the state has no tags
// (the audio Manager already silences voices at Voice volume 0 or when disabled).
// C++ ported : sound_manager.cpp - SoundManager::PlayVoice
func (s *Screen) playVoice(state voiceState) {
	if s.announcer.voiceFn == nil || state < 0 || state >= vsStateCount {
		return
	}

	line := voiceLines[state]

	if len(line.tags) == 0 {
		return
	}

	now := s.now()

	if line.cooldown > 0 && s.announcer.voiceLastMS[state] != 0 && now-s.announcer.voiceLastMS[state] < line.cooldown {
		return
	}

	tag := line.tags[0]

	if len(line.tags) > 1 {
		tag = line.tags[rand.Intn(len(line.tags))]
	}

	// With a playing query wired, a line never cuts the one still speaking: it waits in the pending slot.
	if s.announcer.voicePlayingFn != nil {
		s.announcer.voiceLastMS[state] = now
		s.speakOrHold(queuedVoice{tag: tag, priority: line.priority, desc: line.name}, now)
		return
	}

	// No playing query (tests): do not let a lower priority line cut a higher priority one still playing.
	if s.announcer.lastVoiceMS != 0 && now-s.announcer.lastVoiceMS < voiceSpeakWindowMS && line.priority < s.announcer.lastVoicePriority {
		return
	}

	s.announcer.voiceLastMS[state] = now
	s.speak(queuedVoice{tag: tag, priority: line.priority, desc: line.name}, now)
}

// voiceBusy reports whether a voice line is still audible. A clip that started under voiceQueueMinGapMS ago
// counts as audible, since the channel may not report it yet.
func (s *Screen) voiceBusy(now uint64) bool {
	if s.announcer.voicePlayingFn == nil {
		return false
	}
	if s.announcer.lastVoiceMS != 0 && now-s.announcer.lastVoiceMS < voiceQueueMinGapMS {
		return true
	}
	return s.announcer.voicePlayingFn()
}

// speakOrHold plays v now when the voice channel is free. Otherwise v waits in the single pending slot,
// replacing a waiting line of the same or lower priority; a lower priority v is dropped.
//
// C++ ported : sound_manager.cpp - SoundManager::PlayVoice (one voice job, replaced when priority >= its own)
// and SoundManager::ProcessJob (a voice job waits while the voice still plays).
func (s *Screen) speakOrHold(v queuedVoice, now uint64) {
	if !s.voiceBusy(now) {
		s.speak(v, now)
		return
	}
	if !s.announcer.hasPendingVoice || v.priority >= s.announcer.pendingVoice.priority {
		s.announcer.pendingVoice, s.announcer.hasPendingVoice = v, true
	}
}

// speak starts v on the voice channel and records it as the speaking line.
func (s *Screen) speak(v queuedVoice, now uint64) {
	s.announcer.lastVoiceMS = now
	s.announcer.lastVoicePriority = v.priority
	dbg.Printf("voice (%s) : %s", v.tag, v.desc)
	s.announcer.voiceFn(v.tag)
}

// playVoiceForced bypasses the cross-voice speaking window for a direct player-action acknowledgement.
// Per-state cooldowns still apply.
func (s *Screen) playVoiceForced(state voiceState) {
	if s.announcer.voiceFn == nil || state < 0 || state >= vsStateCount {
		return
	}

	line := voiceLines[state]

	if len(line.tags) == 0 {
		return
	}

	now := s.now()

	if line.cooldown > 0 && s.announcer.voiceLastMS[state] != 0 && now-s.announcer.voiceLastMS[state] < line.cooldown {
		return
	}

	tag := line.tags[0]

	if len(line.tags) > 1 {
		tag = line.tags[rand.Intn(len(line.tags))]
	}

	s.announcer.voiceLastMS[state] = now
	s.announcer.lastVoiceMS = now
	s.announcer.lastVoicePriority = line.priority
	dbg.Printf("voice (%s) : %s [forced]", tag, line.name)
	s.announcer.voiceFn(tag)
}

// playVoiceTag speaks a single resource tag through the same priority gate, for
// lines (e.g. "<colour> team has ended turn!") that are not a fixed state. desc
// is the debug log description for the line.
func (s *Screen) playVoiceTag(tag string, priority int, desc string) {
	if s.announcer.voiceFn == nil || tag == "" {
		return
	}

	now := s.now()

	if s.announcer.voicePlayingFn != nil {
		s.speakOrHold(queuedVoice{tag: tag, priority: priority, desc: desc}, now)
		return
	}

	if s.announcer.lastVoiceMS != 0 && now-s.announcer.lastVoiceMS < voiceSpeakWindowMS && priority < s.announcer.lastVoicePriority {
		return
	}

	s.speak(queuedVoice{tag: tag, priority: priority, desc: desc}, now)
}

// voiceQueueMax caps the backlog so a long fast AI streak cannot pile up stale
// lines (4 teams = at most one "ended turn" line per active team per round).
const voiceQueueMax = game.TeamCount

// voiceQueueMinGapMS is the floor between two queued lines even when the channel
// reports free, so a just started clip registers before the next pop.
// voiceQueueFallbackMS is the spacing used when no voice playing query is wired
// (tests / a backend without it): long enough for a typical "ended turn" clip.
const (
	voiceQueueMinGapMS   = 250
	voiceQueueFallbackMS = 1800
)

// enqueueVoiceTag appends a line to the serial voice queue for events that arrive faster than they can be heard.
func (s *Screen) enqueueVoiceTag(tag string, priority int, desc string) {
	if s.announcer.voiceFn == nil || tag == "" || len(s.announcer.voiceQueue) >= voiceQueueMax {
		return
	}

	s.announcer.voiceQueue = append(s.announcer.voiceQueue, queuedVoice{tag: tag, priority: priority, desc: desc})
}

// tickVoiceQueue plays the next queued line once the previous finished, so the per
// team end turn lines are each heard in full when the fast AI resolves several turns
// quickly. Holds the next line until the channel is free (voicePlayingFn), or a fixed
// fallback spacing when no such query is wired. Called once per frame.
func (s *Screen) tickVoiceQueue() {
	now := s.now()

	// The pending line goes first, once the speaking one has finished; the queue waits behind it.
	if s.announcer.hasPendingVoice {
		if !s.voiceBusy(now) {
			s.announcer.hasPendingVoice = false
			s.speak(s.announcer.pendingVoice, now)
		}
		return
	}

	if len(s.announcer.voiceQueue) == 0 {
		return
	}

	// The spacing only applies AFTER an earlier line played (lastVoiceMS != 0); the
	// very first queued line plays straight away.
	if s.announcer.lastVoiceMS != 0 {
		if now-s.announcer.lastVoiceMS < voiceQueueMinGapMS {
			return // keep a minimum gap so a clip that just started is not clipped
		}

		if s.announcer.voicePlayingFn != nil {
			if s.announcer.voicePlayingFn() {
				return // a voice is still audible: wait for the channel to free
			}
		} else if now-s.announcer.lastVoiceMS < voiceQueueFallbackMS {
			return // no channel query: space lines by a fixed clip length fallback
		}
	}

	next := s.announcer.voiceQueue[0]
	s.announcer.voiceQueue = s.announcer.voiceQueue[1:]
	s.announcer.lastVoiceMS = now
	s.announcer.lastVoicePriority = next.priority
	dbg.Printf("voice (%s) : %s [queued]", next.tag, next.desc)
	s.announcer.voiceFn(next.tag)
}

// playSelectBlip sounds the unit select blip (KBUY0.WAV) once on the SFX channel, the
// default selection sound on every own unit/building select, independent of the
// ready/status voice (a separate channel). The original plays a V_START voice blip,
// but the port ships only the female voice set, so this path uses the KBUY0 UI blip.
// C++ ported : game_manager.cpp - the leading PlayVoice(V_START, V_START2) on select.
func (s *Screen) playSelectBlip() {
	if s.sound.sfxFn == nil {
		return
	}

	s.sound.sfxFn("KBUY0", "", 0) // center pan; the loose KBUY0.WAV resolves via the .WAV fallback
}

// playEnemySpottedPing sounds the enemy spotted alert blip (PING.WAV) once on the
// SFX channel, played alongside the "enemy detected" voice + red banner when a new
// hostile comes into view. PING is the loose alert SFX (resolves via the .WAV
// fallback); one blip per detection event (checkEnemyDetected already debounces).
func (s *Screen) playEnemySpottedPing() {
	if s.sound.sfxFn == nil {
		return
	}

	s.sound.sfxFn("PING", "", 0)
}
