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
	vsEnemyDetected:     {[]string{voice.EnemyDetectedA, voice.EnemyDetectedB}, "enemy detected", 40, 5000},
	vsTurnBegin:         {[]string{voice.TurnBegin}, "turn begin", 25, 500},
	vsBuildComplete:     {voice.ConstructionComplete(), "build complete", 20, 500},
	vsUnitComplete:      {voice.FactoryUnitComplete(), "unit complete", 20, 500},
	vsNewUnits:          {voice.FactoryMultipleNewUnits(), "new units", 20, 500},
	vsResearch:          {[]string{voice.ResearchComplete}, "research complete", 20, 500},
	vsStatusYellow:      {voice.StatusYellow(), "status yellow", 50, 2000},
	vsStatusRed:         {voice.StatusRed(), "status red", 55, 2000},
	vsUnderAttack:       {voice.UnderAttack(), "under attack", 50, 2000},
	vsUnitDestroyed:     {voice.UnitDestroyed(), "unit destroyed", 60, 1500},
	vsNoPath:            {[]string{voice.NoPathA, voice.NoPathB}, "no path", 15, 500},
	vsStorageFuelFull:   {[]string{voice.StorageFuel}, "fuel storage full", 15, 1000},
	vsStorageRawFull:    {[]string{voice.StorageRaw}, "raw storage full", 15, 1000},
	vsStorageGoldFull:   {[]string{voice.StorageGold}, "gold storage full", 15, 1000},
	vsGameSaved:         {[]string{voice.GameSaved}, "game saved", 15, 500},
	vsSurveying:         {[]string{voice.SurveyingA, voice.SurveyingB}, "surveying", 10, 250},
	vsAttacking:         {voice.Combat(), "attacking", 10, 250},
	vsLayingMines:       {[]string{voice.LayingMinesA, voice.LayingMinesB}, "laying mines", 10, 250},
	vsRemovingMines:     {[]string{voice.RemovingMinesA, voice.RemovingMinesB}, "removing mines", 10, 250},
	vsClearing:          {[]string{voice.ClearingArea}, "clearing", 10, 250},
	vsOnSentry:          {[]string{voice.OnSentry}, "on sentry", 10, 250},
	vsBuilding:          {[]string{voice.FactoryBuilding}, "building", 10, 250},
	vsMovementExhausted: {[]string{voice.MovementExhausted}, "movement exhausted", 10, 250},
	vsAmmoLow:           {[]string{voice.AmmoLowA, voice.AmmoLowB, voice.AmmoLowC}, "ammo low", 30, 1000},
	vsAmmoDepleted:      {[]string{voice.AmmoDepleted}, "ammo depleted", 35, 1000},
	vsTransferComplete:  {[]string{voice.TransferDone}, "transfer complete", 15, 500},
	vsRepaired:          {[]string{voice.RepairedAllA, voice.RepairedAllB, voice.RepairedOneA, voice.RepairedOneB}, "repaired", 15, 500},
	vsReloaded:          {[]string{voice.ReloadedA, voice.ReloadedB}, "reloaded", 15, 500},
	vsTimeWarn:          {[]string{voice.TimeWarningA, voice.TimeWarningB}, "time warning", 45, 1000},
	// Game over lines from enterGameOver; top priority so nothing talks over the result.
	vsMissionSuccess: {[]string{voice.MissionWon}, "mission success", 100, 500},
	vsMissionFailed:  {[]string{voice.MissionLost}, "mission failed", 100, 500},
	// "Select site" / "Choose site" / "Select landing site!" - spoken when the
	// custom game landing site select opens. C++: GameManager_SelectSite PlayVoice.
	vsSelectSite: {[]string{voice.SelectSiteA, voice.SelectSiteB, voice.SelectSiteC}, "select landing site", 30, 0},
	// Commando infiltration outcomes for the attacking side: capture, disable, or failure. The failure pool uses F007 and F012; F010 is reserved for the game-over defeat line.
	vsUnitCaptured:     {voice.CapturedUnits(), "unit captured", 35, 500},
	vsUnitDisabled:     {[]string{voice.DisabledUnit}, "unit disabled", 35, 500},
	vsInfiltrateFailed: {[]string{voice.AttemptFailed, voice.InfiltrateBad}, "infiltration failed", 30, 500},
	// The victim hears capture, disable, or exposed-infiltrator outcomes. These lines take priority over selection acknowledgements. The exposed-infiltrator pool uses F012.
	vsUnitSeized:         {[]string{voice.OwnUnitSeized}, "own unit seized", 55, 500},
	vsUnitNotFunctioning: {[]string{voice.OwnUnitStopped}, "own unit not functioning", 55, 500},
	vsInfiltrateDetected: {[]string{voice.InfiltrateBad}, "enemy infiltrator caught", 45, 500},
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
	vsGoldRefined: {[]string{voice.GoldRefined}, "first gold refined", 20, 500},
	// "Unit firing!" / "Unit engaging target!" - spoken when one of the player's own
	// units takes an opportunity (reaction) shot it was not ordered to. The 1s cooldown
	// collapses a multi reactor sweep to one cue. C++: units_manager.cpp - the player team
	// PlayVoice(V_M250, V_F251) inside UnitsManager_CheckReaction.
	vsReactionFire: {voice.ReactionFire(), "reaction fire", 30, 1000},
	// "Submarine detected!" - spoken when the player's sonar (Corvette / AWAC / Scanner)
	// first reveals a hostile submarine, DISTINCT from the general "Enemy detected!"
	// (F070) which covers surface units. Same 5s debounce as enemy detected.
	// C++: unitinfo.cpp - the V_M201/V_F201 sub detection cue.
	vsSubmarineDetected: {[]string{voice.SubmarineDetected}, "submarine detected", 40, 5000},
	// Selecting a damaged unit uses status or repair calls. The damaged selection pools include F247 and F265, while notifyDamage keeps the pure status pools.
	vsSelectDamaged:  {voice.SelectDamaged(), "select (damaged)", 10, 250},
	vsSelectCritical: {voice.SelectCritical(), "select (critical)", 10, 250},
	// "You are about to run out of time!" replaces the normal 20-second warning while the End Turn grace countdown runs. C++: game_manager.cpp:2165 GameManager_DrawTurnTimer.
	vsGraceWarn: {[]string{voice.GraceWarning}, "grace time warning", 45, 1000},
	// "Waiting for A.I. to finish turn." plays in simultaneous mode when a computer team's units still move after End Turn. F080 and F075 belong to networking and are not used here.
	vsWaitingForOpponents: {[]string{voice.WaitingForComputer}, "waiting for the computer", 25, 5000},
}
