// voice.go : the M.A.X. voice line constants - all announcer V_F### resource tags for gameplay events (F*.wav files)
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package voice

// General announcer tags used in game. These helpers mirror the voice pools currently
// used by internal/gamescreen without wiring any new callers.
//
// C++ ported : game_manager.cpp - GameManager_PlayUnitStatusVoice and the
// related SoundManager::PlayVoice call sites.
const (
	SelectReadyA = "V_F001" // "Ready!"
	SelectReadyB = "V_F004" // "All system go!"
	SelectReadyC = "V_F005" // "Standing by!"
	SelectReadyD = "V_F006" // "Awaiting Orders!"

	EnemyDetectedA = "V_F070" // "Enemy detected!"
	EnemyDetectedB = "V_F071" // "Enemy spotted!"

	TurnBegin = "V_F053" // "Begin."

	NoPathA = "V_F094" // "No path to destination!"
	NoPathB = "V_F095" // "Path blocked!"

	SurveyingA = "V_F191" // "Surveying"
	SurveyingB = "V_F192" // "Searching"

	AttackingA = "V_F196" // "Attacking"
	AttackingB = "V_F198" // "Engaging enemy"

	LayingMinesA   = "V_F181" // "Laying mines"
	LayingMinesB   = "V_F182" // "Dropping mines"
	RemovingMinesA = "V_F186" // "Removing mines"
	RemovingMinesB = "V_F187" // "Picking up mines"
	ClearingArea   = "V_F171" // "Clearing area"

	OnSentry = "V_F158" // "On sentry"

	MovementExhausted = "V_F145" // "Movement exhausted"

	StatusYellowA = "V_F150" // "Status yellow"
	StatusYellowB = "V_F151" // "Status caution"
	StatusRedA    = "V_F154" // "Status red"
	StatusRedB    = "V_F155" // "Status critical"

	UnderAttackA = "V_F229" // "Unit under attack!"
	UnderAttackB = "V_F230" // "Enemy firing!"
	UnderAttackC = "V_F231" // "Incoming!"
	UnderAttackD = "V_F232" // "We're under attack!"
	UnderAttackE = "V_F255" // "Unit under attack!"
	UnderAttackF = "V_F256" // "Enemy firing on unit!"

	UnitDestroyedA = "V_F234" // "Unit destroyed!"
	UnitDestroyedB = "V_F236" // "Unit is gone!"

	ReactionFireA = "V_F250" // "Unit firing!"
	ReactionFireB = "V_F251" // "Unit engaging target!"

	AmmoLowA      = "V_F138" // "Ammunition is low"
	AmmoLowB      = "V_F270" // "Ammunitions low!"
	AmmoLowC      = "V_F271" // "Ammo is low!"
	AmmoDepleted  = "V_F142" // "Ammo depleted"
	TransferDone  = "V_F224" // "Transfer complete!"
	RepairedAllA  = "V_F210" // "Units repaired!"
	RepairedAllB  = "V_F211" // "Repairs complete!"
	RepairedOneA  = "V_F219" // "Unit repaired!"
	RepairedOneB  = "V_F220" // "Repaired!"
	ReloadedA     = "V_F085" // "Reloaded"
	ReloadedB     = "V_F089" // "Ammo reload successful"
	TimeWarningA  = "V_F272" // "20 seconds left"
	TimeWarningB  = "V_F273" // "20 seconds to end turn!"
	MissionWon    = "V_F283" // "Mission successful!"
	MissionLost   = "V_F010" // "Mission failed!"
	GameSaved     = "V_F013" // "Game Saved!"
	StorageFuel   = "V_F057" // "Fuel storage is full!"
	StorageRaw    = "V_F061" // "Raw material storage is full!"
	StorageGold   = "V_F066" // "Gold storage is full!"
	SelectSiteA   = "V_F176" // "Select site"
	SelectSiteB   = "V_F177" // "Choose site"
	SelectSiteC   = "V_F278" // "Select landing site!"
	CapturedUnit  = "V_F239" // "Unit captured!"
	DisabledUnit  = "V_F244" // "Unit disabled"
	InfiltrateBad = "V_F012" // "Infiltration unsuccessful!"
	AttemptFailed = "V_F007" // "Attempt failed!"

	// The victim's side of an enemy commando raid.
	OwnUnitSeized  = "V_F243" // "Units have been seized by the enemy!"
	OwnUnitStopped = "V_F249" // "Unit not functioning!"
)

// Select returns the normal pool of acknowledgements for selecting a unit.
func Select() []string {
	return []string{SelectReadyA, SelectReadyB, SelectReadyC, SelectReadyD}
}

// SelectSentry returns the blended ready and on sentry pool used by the Go port
// for sentry units and sentry defensive buildings.
func SelectSentry() []string {
	return []string{SelectReadyA, SelectReadyB, SelectReadyC, SelectReadyD, OnSentry}
}

// MoveAck returns the current movement acknowledgement pool. The port uses the
// normal ready acknowledgements for confirming a click to move order (a click to
// move reuses the vsSelectUnit ready pool, so gamescreen has no distinct caller;
// the pool is exercised by the voice package's own test).
func MoveAck() []string {
	return Select()
}

// Combat returns the direct attack acknowledgement pool.
func Combat() []string {
	return []string{AttackingA, AttackingB}
}

// ReactionFire returns the reactive/sentry/manual fire voice pool.
func ReactionFire() []string {
	return []string{ReactionFireA, ReactionFireB}
}

// StatusYellow returns the status pool for medium damage.
func StatusYellow() []string {
	return []string{StatusYellowA, StatusYellowB}
}

// StatusRed returns the status pool for heavy damage.
func StatusRed() []string {
	return []string{StatusRedA, StatusRedB}
}

// UnderAttack returns the warning pool for incoming fire.
func UnderAttack() []string {
	return []string{UnderAttackA, UnderAttackB, UnderAttackC, UnderAttackD, UnderAttackE, UnderAttackF}
}

// UnitDestroyed returns the pool for a destroyed unit.
func UnitDestroyed() []string {
	return []string{UnitDestroyedA, UnitDestroyedB}
}
