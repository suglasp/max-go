// difficulty.go : the M.A.X. AI difficulty tiers
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

// Difficulty tiers for the computer opponent. A single global tier drives every
// computer team, matching vanilla M.A.X.'s one "opponent" setting (the value is
// the same 0..5 stored in settings.Game.Opponent). The tier selects a profile of
// AI knobs: how large a strike force the team musters before attacking, whether it
// retreats damaged units, the mining yield it plans against, and whether it ever
// marches out to hunt (low tiers stay defensive).
//
// A fixed profile table collects the OPPONENT_TYPE_* behaviour gates. AVERAGE preserves the default AI values.
//
// C++ ported : enums.hpp - OpponentType + aiplayer.cpp - the OPPONENT_TYPE_* gates

// Opponent difficulty tiers (0..5), matching the original's enums.hpp OpponentType
// and the persisted settings "opponent" value.
const (
	OpponentClueless = iota
	OpponentApprentice
	OpponentAverage // the default tier; reproduces the pre difficulty behaviour
	OpponentExpert
	OpponentMaster
	OpponentGod
	OpponentCount // number of tiers (Clueless..God)
)

// DifficultyCount is the number of selectable difficulty tiers, exposed for the
// F2 debug menu's difficulty cycle row.
const DifficultyCount = OpponentCount

var difficultyNames = [OpponentCount]string{
	OpponentClueless:   "Clueless",
	OpponentApprentice: "Apprentice",
	OpponentAverage:    "Average",
	OpponentExpert:     "Expert",
	OpponentMaster:     "Master",
	OpponentGod:        "God",
}

// DifficultyName returns the display name of tier d, clamping an out of range
// value to AVERAGE.
func DifficultyName(d int) string {
	if d < 0 || d >= OpponentCount {
		d = OpponentAverage
	}

	return difficultyNames[d]
}

// difficultyProfile is the set of AI knobs one tier selects. The values are tuned
// to the original's known anchors: AVERAGE reproduces the pre difficulty constants,
// retreat is off only at CLUELESS (the original retreats at >= APPRENTICE), and the
// mining factor rises to 5/6 at MASTER/GOD (the aiDifficultyFactor comment).
type difficultyProfile struct {
	muster        int  // armed mobile units to field before committing to an assault (minStrikeForce)
	retreat       bool // pull a damaged unit back to repair
	miningFactor  int  // mining yield multiplier the economy plans against (economy.NetProduction)
	defensiveOnly bool // never march out to hunt; only fire on enemies in range / entering territory
	harassStart   int  // turn the team begins LIGHT harassment probes (defensive tiers: neverAttack)
	graceTurns    int  // turn the team may begin its FULL assault (defensive tiers: neverAttack)
	keepReserve   bool // keep a home defence reserve during the assault; lower tiers overcommit (all in)
	escort        bool // shadow fragile high value movers (Scanner/MissileLcr/AWAC/Bomber) with a spare guard
	lossMemory    int  // turns the team remembers a spot where it lost a unit and avoids it (aiLossMemoryTurns)
	storageCap    int  // per cargo total storage capacity the team piles on before it stops adding dumps (aiStorageCap)
}

// neverAttack is a grace/harass turn no game reaches: the tier never sorties. Used
// for the defensive tiers (which aiDefensiveOnly short circuits anyway) to make the
// "never attacks first" intent explicit in the table.
const neverAttack = 1 << 30

// The harass/grace turns give every non defensive tier a build up before it can
// launch a full assault (so a new player is not wiped in the opening), then a window
// where it only PINGS with light probes, then the full assault. Tuned to a longer,
// gentler curve: Average pings ~turn 100 and assaults ~turn 200; God pings ~turn 20
// and assaults ~turn 40 (even the top tier gives the player an opening).
//
// lossMemory scales with the tier: a weaker AI forgets a spot where it lost a unit
// sooner (and probes it again recklessly), a stronger one remembers longer (and does
// not keep feeding the same kill zone). It spans 15..25 turns, capped so even the top
// tier eventually probes there again rather than writing an area off for good.
//
// storageCap scales the other way: a weaker AI is allowed MORE idle storage (it over
// invests in economy and reaches military late - a storage heavy opening visible
// at low difficulty), a stronger AI keeps LESS so mined material flows into
// military production sooner and it feels more aggressive. AVERAGE stays at
// storageCapLimit (500), so the default behaviour is unchanged.
var difficultyProfiles = [OpponentCount]difficultyProfile{
	OpponentClueless:   {muster: 6, retreat: false, miningFactor: 3, defensiveOnly: true, harassStart: neverAttack, graceTurns: neverAttack, keepReserve: false, escort: false, lossMemory: 15, storageCap: 700},
	OpponentApprentice: {muster: 5, retreat: true, miningFactor: 3, defensiveOnly: true, harassStart: neverAttack, graceTurns: neverAttack, keepReserve: false, escort: false, lossMemory: 15, storageCap: 600},
	OpponentAverage:    {muster: 3, retreat: true, miningFactor: 4, defensiveOnly: false, harassStart: 100, graceTurns: 200, keepReserve: false, escort: false, lossMemory: 18, storageCap: 500},
	OpponentExpert:     {muster: 3, retreat: true, miningFactor: 4, defensiveOnly: false, harassStart: 70, graceTurns: 140, keepReserve: false, escort: true, lossMemory: 20, storageCap: 400},
	OpponentMaster:     {muster: 2, retreat: true, miningFactor: 5, defensiveOnly: false, harassStart: 45, graceTurns: 90, keepReserve: true, escort: true, lossMemory: 23, storageCap: 350},
	OpponentGod:        {muster: 1, retreat: true, miningFactor: 6, defensiveOnly: false, harassStart: 20, graceTurns: 40, keepReserve: true, escort: true, lossMemory: 25, storageCap: 300},
}

// profileFor returns tier d's profile, clamping an out of range value to AVERAGE so
// a bad setting never makes the AI inert.
func profileFor(d int) difficultyProfile {
	if d < 0 || d >= OpponentCount {
		return difficultyProfiles[OpponentAverage]
	}

	return difficultyProfiles[d]
}

// aiTier is the active global difficulty tier, recorded by applyDifficulty so the
// win goal selection (wingoal.go) can weight its colonise roll by difficulty - a
// single global tier, like the other knobs. Defaults to AVERAGE.
var aiTier = OpponentAverage

// aiLossMemoryTurns is how many turns a team avoids a cell where it recently lost a
// unit (the loss memory the threat map stamps, buildThreatMap). Set per tier by
// applyDifficulty; defaults to the AVERAGE profile so a test that never applies a tier
// still gets a sane window.
var aiLossMemoryTurns = difficultyProfiles[OpponentAverage].lossMemory

// applyDifficulty sets the package level AI knobs from tier d. A single global tier
// drives every computer team (the original's single "opponent" setting). It is called from
// AI.SetDifficulty (launch path / F2 debug menu); tests that never call it keep the
// AVERAGE defaults the knobs are declared with, so they are unaffected.
func applyDifficulty(d int) {
	p := profileFor(d)

	minStrikeForce = p.muster
	retreatEnabled = p.retreat
	aiDifficultyFactor = p.miningFactor
	aiDefensiveOnly = p.defensiveOnly
	aiHarassStart = p.harassStart
	aiGraceTurns = p.graceTurns
	aiKeepReserve = p.keepReserve
	escortEnabled = p.escort
	aiLossMemoryTurns = p.lossMemory
	aiStorageCap = p.storageCap
	aiTier = d
}

// CombatPosture returns a short label for the AI's offensive stance, for the F2 debug
// readout: "defensive" when the active tier never marches out (aiDefensiveOnly), else
// "offensive". One global tier drives every computer team, like the difficulty row it
// sits beside.
func CombatPosture() string {
	if aiDefensiveOnly {
		return "defensive"
	}

	return "offensive"
}
