// voicelines.go : holds the announcer's line table: every situation that speaks, and
// the F###.WAV tags, priority and cooldown it speaks with.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : sound_manager.cpp - the V_* voice groups and their resource tags
package gamescreen

import (
	"github.com/suglasp/max-go/internal/voice"
)

// voiceState names an in game situation that speaks an announcer voice line. Each
// state maps to a set of interchangeable F###.WAV lines (one picked at random), a
// priority, and a cooldown, played via the V_F### redirect.
// C++ ported : the V_* voice groups dispatched through SoundManager::PlayVoice.
type voiceState int

const (
	vsSelectUnit voiceState = iota
	vsEnemyDetected
	vsTurnBegin
	vsBuildComplete
	vsUnitComplete
	vsNewUnits
	vsResearch
	vsStatusYellow
	vsStatusRed
	vsUnderAttack
	vsUnitDestroyed
	vsNoPath
	vsStorageFuelFull
	vsStorageRawFull
	vsStorageGoldFull
	vsGameSaved
	vsSurveying
	vsAttacking
	vsLayingMines
	vsRemovingMines
	vsClearing
	vsOnSentry
	vsBuilding
	vsMovementExhausted
	vsAmmoLow
	vsAmmoDepleted
	vsTransferComplete
	vsRepaired
	vsReloaded
	vsTimeWarn
	vsMissionSuccess
	vsMissionFailed
	vsSelectSite
	vsUnitCaptured
	vsUnitDisabled
	vsInfiltrateFailed
	vsUnitSeized
	vsUnitNotFunctioning
	vsInfiltrateDetected
	vsSelectSentry
	vsBuildPlacement
	vsGoldRefined
	vsReactionFire
	vsSubmarineDetected
	vsSelectDamaged
	vsSelectCritical
	vsGraceWarn
	vsWaitingForOpponents
	vsStateCount
)

// voiceLine is one state's playable lines plus its dispatch gating.
type voiceLine struct {
	tags     []string // interchangeable V_F### tags; one is chosen at random
	name     string   // short label for debug logs
	priority int      // higher wins the "still speaking" gate
	cooldown uint64   // minimum ms between repeats of this state (0 = none)
}

// voiceLines maps each state to its lines. Tags come from the female F###.WAV set in compare/_voice_lines.txt. Priorities keep status and destruction above acknowledgements and ambient orders. Enemy detection uses a five-second debounce.
var voiceLines = [vsStateCount]voiceLine{
	vsSelectUnit:        {voice.Select(), "select", 10, 250},
	vsEnemyDetected:     {[]string{"V_F070", "V_F071"}, "enemy detected", 40, 5000},
	vsTurnBegin:         {[]string{"V_F053"}, "turn begin", 25, 500},
	vsBuildComplete:     {[]string{"V_F162", "V_F163", "V_F164", "V_F165"}, "build complete", 20, 500},
	vsUnitComplete:      {[]string{"V_F166", "V_F169", "V_F215", "V_F216", "V_F217"}, "unit complete", 20, 500},
	vsNewUnits:          {[]string{"V_F206", "V_F207"}, "new units", 20, 500},
	vsResearch:          {[]string{"V_F093"}, "research complete", 20, 500},
	vsStatusYellow:      {voice.StatusYellow(), "status yellow", 50, 2000},
	vsStatusRed:         {voice.StatusRed(), "status red", 55, 2000},
	vsUnderAttack:       {voice.UnderAttack(), "under attack", 50, 2000},
	vsUnitDestroyed:     {voice.UnitDestroyed(), "unit destroyed", 60, 1500},
	vsNoPath:            {[]string{"V_F094", "V_F095"}, "no path", 15, 500},
	vsStorageFuelFull:   {[]string{"V_F057"}, "fuel storage full", 15, 1000},
	vsStorageRawFull:    {[]string{"V_F061"}, "raw storage full", 15, 1000},
	vsStorageGoldFull:   {[]string{"V_F066"}, "gold storage full", 15, 1000},
	vsGameSaved:         {[]string{"V_F013"}, "game saved", 15, 500},
	vsSurveying:         {[]string{"V_F191", "V_F192"}, "surveying", 10, 250},
	vsAttacking:         {voice.Combat(), "attacking", 10, 250},
	vsLayingMines:       {[]string{"V_F181", "V_F182"}, "laying mines", 10, 250},
	vsRemovingMines:     {[]string{"V_F186", "V_F187"}, "removing mines", 10, 250},
	vsClearing:          {[]string{"V_F171"}, "clearing", 10, 250},
	vsOnSentry:          {[]string{"V_F158"}, "on sentry", 10, 250},
	vsBuilding:          {[]string{"V_F284"}, "building", 10, 250},
	vsMovementExhausted: {[]string{"V_F145"}, "movement exhausted", 10, 250},
	vsAmmoLow:           {[]string{"V_F138", "V_F270", "V_F271"}, "ammo low", 30, 1000},
	vsAmmoDepleted:      {[]string{"V_F142"}, "ammo depleted", 35, 1000},
	vsTransferComplete:  {[]string{"V_F224"}, "transfer complete", 15, 500},
	vsRepaired:          {[]string{"V_F210", "V_F211", "V_F219", "V_F220"}, "repaired", 15, 500},
	vsReloaded:          {[]string{"V_F085", "V_F089"}, "reloaded", 15, 500},
	vsTimeWarn:          {[]string{"V_F272", "V_F273"}, "time warning", 45, 1000},
	// Game over lines from enterGameOver; top priority so nothing talks over the result.
	vsMissionSuccess: {[]string{"V_F283"}, "mission success", 100, 500},
	vsMissionFailed:  {[]string{"V_F010"}, "mission failed", 100, 500},
	// "Select site" / "Choose site" / "Select landing site!" - spoken when the
	// custom game landing site select opens. C++: GameManager_SelectSite PlayVoice.
	vsSelectSite: {[]string{"V_F176", "V_F177", "V_F278"}, "select landing site", 30, 0},
	// Commando infiltration outcomes for the attacking side: capture, disable, or failure. The failure pool uses F007 and F012; F010 is reserved for the game-over defeat line.
	vsUnitCaptured:     {[]string{"V_F239", "V_F242"}, "unit captured", 35, 500},
	vsUnitDisabled:     {[]string{"V_F244"}, "unit disabled", 35, 500},
	vsInfiltrateFailed: {[]string{"V_F007", "V_F012"}, "infiltration failed", 30, 500},
	// The victim hears capture, disable, or exposed-infiltrator outcomes. These lines take priority over selection acknowledgements. The exposed-infiltrator pool uses F012.
	vsUnitSeized:         {[]string{"V_F243"}, "own unit seized", 55, 500},
	vsUnitNotFunctioning: {[]string{"V_F249"}, "own unit not functioning", 55, 500},
	vsInfiltrateDetected: {[]string{"V_F012"}, "enemy infiltrator caught", 45, 500},
	// Selecting a unit (or defense building) ON GUARD: the "Ready!" select pool with
	// F158 "On sentry" mixed in, so a pick is sometimes "Ready", sometimes "On sentry".
	// The original plays a fixed V_F158 for a mobile sentry unit and nothing for a
	// building; we blend the pool and extend it to defense buildings on guard.
	// C++ ported : game_manager.cpp - GameManager_PlayUnitStatusVoice (sentry), blended.
	vsSelectSentry: {voice.SelectSentry(), "select (on sentry)", 10, 250},
	// "Position tape and click inside it to begin building." - spoken when a builder
	// enters placement mode (SELECT_SITE for a 2x2, an engineer's path pick). Tag from
	// the internal/voice package. C++: buildmenu.cpp - PlayVoice(V_M049, V_F050) on
	// entering ORDER_STATE_SELECT_SITE.
	vsBuildPlacement: {[]string{voice.BuildPlacementPrompt, voice.BuildPlacementPromptB}, "build placement prompt", 15, 250},
	// "Gold is now being refined" - spoken once, the first time a Gold Refinery on the
	// player's base produces spendable credit (the team's gold balance first rises from
	// refining). C++: unitinfo.cpp UpdateProduction COMMTWR (!gold_reserves && GetGold()>0).
	vsGoldRefined: {[]string{"V_F276"}, "first gold refined", 20, 500},
	// "Unit firing!" / "Unit engaging target!" - spoken when one of the player's own
	// units takes an opportunity (reaction) shot it was not ordered to. The 1s cooldown
	// collapses a multi reactor sweep to one cue. C++: units_manager.cpp - the player team
	// PlayVoice(V_M250, V_F251) inside UnitsManager_CheckReaction.
	vsReactionFire: {voice.ReactionFire(), "reaction fire", 30, 1000},
	// "Submarine detected!" - spoken when the player's sonar (Corvette / AWAC / Scanner)
	// first reveals a hostile submarine, DISTINCT from the general "Enemy detected!"
	// (F070) which covers surface units. Same 5s debounce as enemy detected.
	// C++: unitinfo.cpp - the V_M201/V_F201 sub detection cue.
	vsSubmarineDetected: {[]string{"V_F201"}, "submarine detected", 40, 5000},
	// Selecting a damaged unit uses status or repair calls. The damaged selection pools include F247 and F265, while notifyDamage keeps the pure status pools.
	vsSelectDamaged:  {[]string{"V_F150", "V_F151", "V_F247", "V_F265"}, "select (damaged)", 10, 250},
	vsSelectCritical: {[]string{"V_F154", "V_F155", "V_F247", "V_F265"}, "select (critical)", 10, 250},
	// "You are about to run out of time!" replaces the normal 20-second warning while the End Turn grace countdown runs. C++: game_manager.cpp:2165 GameManager_DrawTurnTimer.
	vsGraceWarn: {[]string{"V_F275"}, "grace time warning", 45, 1000},
	// "Waiting for A.I. to finish turn." plays in simultaneous mode when a computer team's units still move after End Turn. F080 and F075 belong to networking and are not used here.
	vsWaitingForOpponents: {[]string{"V_F081"}, "waiting for the computer", 25, 5000},
}
