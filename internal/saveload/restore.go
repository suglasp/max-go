// restore.go : the M.A.X. save restore logic - rebuilds derived structures (passability, visibility, complexes) after loading.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/winloss"
	"github.com/suglasp/max-go/internal/world"
)

// Restore rebuilds a live game from a snapshot, using the already loaded world
// map (the caller loads it from s.MapName). It recreates the units, rejoins
// complexes and refreshes visibility through sim.Game.Restore, then restores the
// turn state, scores and RNG.
func (s *Snapshot) Restore(m *world.Map) *sim.Game {
	p := &game.StartParams{}
	for i := range p.Teams {
		p.Teams[i].Type = game.TeamType(s.Teams[i].Type)
	}
	g := sim.New(p, m)

	byID := s.restoreUnits(g)
	s.restoreOrders(g, byID)
	s.restoreFieldState(g, byID)
	s.restoreTurnState(g, byID)
	s.restoreMapState(g, m)
	s.restoreEconomy(g)

	// Re establish the AI's recent loss memory (v22); empty on an older save, which is
	// what the AI woke up with before.
	if len(s.Losses) > 0 {
		byTeam := make(map[int][]sim.LossSite, game.TeamCount)
		for _, ls := range s.Losses {
			byTeam[ls.Team] = append(byTeam[ls.Team], sim.LossSite{X: ls.X, Y: ls.Y, Turn: ls.Turn})
		}
		for team, sites := range byTeam {
			g.RestoreLossSites(team, sites)
		}
	}
	game.RandomizerSetState(s.RNG)
	return g
}

// restoreUnits rebuilds every unit and re links the relationships between them,
// returning the by ID index the later restore steps look their units up in.
func (s *Snapshot) restoreUnits(g *sim.Game) map[uint16]*game.UnitInfo {
	// Rebuild every unit, indexed by ID. Aboard passengers (ParentID >= 0) are NOT
	// placed on the grid - they are re linked into their carrier's Carried list
	// below, mirroring LoadUnit (which removes a boarded unit from g.Units).
	all := make([]*game.UnitInfo, len(s.Units))
	byID := make(map[uint16]*game.UnitInfo, len(s.Units))
	onMap := make([]*game.UnitInfo, 0, len(s.Units))
	for i := range s.Units {
		u := restoreUnit(&s.Units[i])
		all[i] = u
		byID[u.ID] = u
		if s.Units[i].ParentID < 0 {
			onMap = append(onMap, u)
		}
	}
	g.Restore(onMap)

	// Re link the relationship pointers now that every unit exists.
	for i := range s.Units {
		us := &s.Units[i]
		u := all[i]
		if us.ParentID >= 0 {
			if parent := byID[uint16(us.ParentID)]; parent != nil {
				u.Parent = parent
				parent.Carried = append(parent.Carried, u)
			}
		}
		if us.AttackTargetID >= 0 {
			u.AttackTarget = byID[uint16(us.AttackTargetID)] // nil if the target was not saved
		}
	}

	return byID
}

// restoreOrders re queues the work that was in flight when the game was saved: the
// build orders, the moves that ran out of budget, the routes an enemy halt is holding
// and the engineer line builds.
func (s *Snapshot) restoreOrders(g *sim.Game, byID map[uint16]*game.UnitInfo) {
	// Re create the in flight build orders (v3): rebuild each unplaced product and
	// re issue it through g.StartBuild (which re establishes a constructor's build
	// pose + footprint), then restore the order's accumulated progress. The product's
	// ID and live state are (re)assigned by finishBuild when the build completes.
	for i := range s.Builds {
		bo := &s.Builds[i]
		builder := byID[bo.BuilderID]
		if builder == nil {
			continue // builder is gone - drop the orphaned order
		}
		product := game.NewUnitInfo(assets.UnitType(bo.ProductType), bo.ProductTeam, 0, 0)
		product.Flags = game.Flags(bo.ProductFlags)
		v := &game.UnitValues{}
		for a := game.Attrib(0); a < game.AttribCount; a++ {
			v.Set(a, bo.ProductValues[a])
		}
		v.SetVersion(bo.ProductValVersion)
		product.Values = v
		o := g.StartBuild(builder, product, bo.GridX, bo.GridY, bo.BuildRate)
		o.Progress = bo.Progress
		o.Started = bo.Started
	}

	// Re queue the resumable (over budget) moves (v4): only the destination is
	// stored; resumePending re paths to it at the next BeginTeamTurn.
	for i := range s.PendingMoves {
		pm := &s.PendingMoves[i]
		if u := byID[pm.UnitID]; u != nil {
			g.SetPendingDestination(u, pm.DestX, pm.DestY)
		}
	}

	// Put back the routes a halt on enemy stop was holding (v24). Deliberately NOT
	// re queued as pending moves: a held route waits for the player, and resumePending
	// would drive the unit straight back into the enemy it stopped for.
	for i := range s.HeldMoves {
		hm := &s.HeldMoves[i]
		if u := byID[hm.UnitID]; u != nil {
			g.SetHeldDestination(u, hm.DestX, hm.DestY)
		}
	}

	// Re establish the engineer line (Path) builds (v5): the current cell's order
	// was restored above; this carries the remaining run.
	for i := range s.BuildPaths {
		bp := &s.BuildPaths[i]
		builder := byID[bp.BuilderID]
		if builder == nil {
			continue
		}
		v := &game.UnitValues{}
		for a := game.Attrib(0); a < game.AttribCount; a++ {
			v.Set(a, bp.ProductValues[a])
		}
		v.SetVersion(bp.ProductValVersion)
		g.RestoreBuildPath(builder, assets.UnitType(bp.ProductType), game.Flags(bp.ProductFlags), v, bp.Rate, bp.Cells, bp.Idx)
	}
}

// restoreFieldState puts back what sits on the field rather than on a unit: the AI's
// spotted enemy memory, the built unit ID counter, the debris piles, the bulldozer
// clear jobs and the builders parked on a finished structure.
func (s *Snapshot) restoreFieldState(g *sim.Game, byID map[uint16]*game.UnitInfo) {
	// Re establish the AI spotted enemy memory (v6) so a fogged contact the AI was
	// tracking is not forgotten - Restore's RefreshVisibility only re derives the
	// currently visible enemies.
	for i := range s.Spotted {
		sp := &s.Spotted[i]
		if enemy := byID[sp.EnemyID]; enemy != nil {
			g.AddSpotted(sp.Team, enemy, sp.LastX, sp.LastY, sp.Visible)
		}
	}

	// Re establish the build unit ID counter (v7) so units/mines/markers built after
	// load continue from where the save left off, never colliding with a saved ID.
	g.SetNextBuilt(s.NextBuilt)

	// Re establish the debris piles (v7) - Restore otherwise starts on a clean field.
	if len(s.Rubble) > 0 {
		rubble := make([]sim.RubbleCell, len(s.Rubble))
		for i := range s.Rubble {
			rubble[i] = sim.RubbleCell{X: s.Rubble[i].X, Y: s.Rubble[i].Y, Large: s.Rubble[i].Large, Material: s.Rubble[i].Material}
		}
		g.RestoreRubble(rubble)
	}

	// Re establish the bulldozer clear jobs (v7): the dozer + its hazard markers are
	// on map units restored above; this re links them by ID.
	for i := range s.Clears {
		cj := &s.Clears[i]
		dozer := byID[cj.BulldozerID]
		if dozer == nil {
			continue
		}
		markers := make([]*game.UnitInfo, 0, len(cj.MarkerIDs))
		for _, id := range cj.MarkerIDs {
			if mk := byID[id]; mk != nil {
				markers = append(markers, mk)
			}
		}
		g.RestoreClearJob(dozer, cj.CellX, cj.CellY, cj.Footprint, cj.TurnsLeft, markers)
	}

	// Re establish the parked build ready builders (v7). A LIVE parked builder stays
	// in the unit registry (finishBuild re adds it to Cells+ByID), so it is in s.Units
	// too and was already rebuilt by g.Restore above - re use THAT instance so
	// g.buildReady is keyed by the same pointer the UI selects (clicking the builder
	// must read as build ready, or its BLDMRK exit never engages). Only a builder
	// saved off the registry is rebuilt from its own embedded snapshot. The finished
	// structure it guards is re linked by ID.
	for i := range s.BuildReady {
		brs := &s.BuildReady[i]
		builder := byID[brs.Builder.ID]
		if builder == nil {
			builder = restoreUnit(&brs.Builder)
		}
		building := byID[brs.BuildingID] // nil if the structure was not saved
		g.RestoreBuildReady(builder, building, assets.UnitType(brs.Product), brs.SiteX, brs.SiteY, brs.Footprint)
	}
}

// restoreTurnState puts back the state of the match itself: the turn counter and phase,
// the per team roster, the scores, the victory rule and the mission it is running.
func (s *Snapshot) restoreTurnState(g *sim.Game, byID map[uint16]*game.UnitInfo) {
	g.State.TurnCounter = s.Turn
	g.State.ActiveTeam = s.ActiveTeam
	g.State.PlayerTeam = s.PlayerTeam
	g.State.Phase = game.GamePhase(s.Phase)
	// Honour the saved play mode, but clamp anything out of range to turn based so
	// a corrupt or future field can never wedge the turn loop on load.
	if s.PlayMode == int(game.PlaySimultaneous) {
		g.State.PlayMode = game.PlaySimultaneous
	} else {
		g.State.PlayMode = game.PlayTurnBased
	}
	// A loaded in game save resumes mid turn without a BeginTeamTurn for the human,
	// so arm reveal triggered reaction fire here (BeginTeamTurn does it for a fresh
	// game). Without this the first fog lift on an in range enemy stays silent until
	// the player cycles a full turn.
	if g.State.Phase == game.PhaseInGame {
		g.EnableRevealReactions()
	}
	// Only the playable teams are serialised (the snapshot is sized to TeamCount); the
	// fixed alien slot is re asserted below so a restored game keeps its alien faction.
	for i := 0; i < game.TeamCount; i++ {
		g.State.Teams[i].Type = game.TeamType(s.Teams[i].Type)
		g.State.Teams[i].Eliminated = s.Teams[i].Eliminated
		g.State.Teams[i].FinishedTurn = s.Teams[i].FinishedTurn
		// v13: per team research, bought upgrades and clan (zero for older saves).
		g.State.Teams[i].Clan = s.Teams[i].Clan
		g.State.Teams[i].Research = s.Teams[i].Research
		if len(s.Teams[i].Upgrades) > 0 {
			ups := make(map[assets.UnitType]*[game.AttribCount]int, len(s.Teams[i].Upgrades))
			for typ, offs := range s.Teams[i].Upgrades {
				arr := offs // copy the value array, then take its address (fresh per iteration)
				ups[assets.UnitType(typ)] = &arr
			}
			g.State.Teams[i].Upgrades = ups
		}
	}
	g.State.Teams[game.AlienTeam].Type = game.TeamAlien
	g.Gold = s.Gold
	g.TeamPoints = s.TeamPoints
	g.TeamStats = s.TeamStats // restored last (after unit reconstruction) so it cannot be inflated by a rebuild
	g.MiningDifficulty = s.Difficulty
	// v14: the AI opponent tier. Only a v14+ save actually carries it; a pre v14
	// save has no field, so gob left OpponentTier at 0 (CLUELESS) - keep the "unset"
	// default sim.New already set instead of trusting that zero.
	if s.Version >= 14 {
		g.State.Difficulty = s.OpponentTier
	}
	// v17: the committed AI win goal per computer team (goal+1). A pre v17 save has
	// all zeros = unset, so the AI rolls its goal fresh on the first turn (the prior
	// behaviour); a v17+ save keeps the AI's chosen kill vs colonise plan.
	g.State.AIWinGoal = s.AIWinGoal
	// v18: the committed AI strategy per computer team (strategy+1). A pre v18 save has
	// all zeros = unset, so each team's SelectStrategy roll stands (the prior behaviour);
	// a v18+ save keeps the AI's committed posture across the reload.
	g.State.AIStrategy = s.AIStrategy
	// v13: the match victory rule + the score graph history. WinLossConditions (the hook a
	// scripted mission would override the outcome through) is not serialised, because it is
	// a FUNCTION: the game layer re assigns it after the restore from the mission category
	// and index below (gamescreen applyMissionObjective), which is why both of those ride
	// the save. A skirmish has none and runs the default rule off VictoryType/VictoryLimit.
	g.VictoryType = winloss.VictoryType(s.VictoryType)
	g.VictoryLimit = s.VictoryLimit
	g.ScoreHistory = s.ScoreHistory
	// v19: the mission category. A pre v19 save decodes 0 = MissionNone, which is what
	// those saves effectively were (nothing keyed off the category before this).
	g.Mission = game.MissionCategory(s.Mission)
	// v23: WHICH mission of that category, so the game layer can put its objective back.
	// A pre v23 save decodes 0 = no scripted objective, which is what it ran on anyway.
	g.MissionIndex = s.MissionIndex
	// v13: re establish the commando disable recovery countdowns (the units carry
	// their OrderDisable from the registry; this restores the turns left timer).
	for i := range s.Disabled {
		if u := byID[s.Disabled[i].UnitID]; u != nil {
			g.RestoreDisabled(u, s.Disabled[i].Turns)
		}
	}
}

// restoreMapState puts back the two map wide layers a load cannot re derive from the
// units: the resource deposit map and the fog exploration memory.
func (s *Snapshot) restoreMapState(g *sim.Game, m *world.Map) {
	// Rebuild the resource deposit map (v9) so post load mining sites + the survey
	// overlay survive; older saves (nil Resources) leave it nil, as before. Surf is
	// the live map, not persisted.
	if s.Resources != nil {
		g.Resources = economy.ImportResourceMap(s.Resources.W, s.Resources.H, s.Resources.Deposits, s.Resources.Surveyed)
		g.Resources.Surf = m
	}
	// Re establish the fog memory (v22). g.Restore above rebuilt the scan coverage from
	// the units, which only lights the cells a unit is standing in range of; the map the
	// team has EXPLORED is the half that cannot be derived, so it comes back from here.
	// A pre v22 save carries no bitmap and keeps the old behaviour (a black map).
	if s.Explored != nil && g.Vis != nil && s.Explored.W == g.Vis.W && s.Explored.H == g.Vis.H {
		g.Vis.ImportExplored(s.Explored.Teams)
	}
}

// restoreEconomy hands each rebuilt complex its pooled stores back, then settles power
// and the colony latch so the first frame after a load reads correctly.
func (s *Snapshot) restoreEconomy(g *sim.Game) {
	// Apply v11 pooled stores to the rebuilt complex containing each saved anchor member.
	if len(s.Complexes) > 0 {
		byAnchor := make(map[uint16]ComplexSnapshot, len(s.Complexes))
		for _, cs := range s.Complexes {
			byAnchor[cs.AnchorID] = cs
		}
		for _, c := range g.Complexes {
			for _, m := range c.Members {
				if cs, ok := byAnchor[m.ID]; ok {
					c.Material, c.Fuel, c.Gold = cs.Material, cs.Fuel, cs.Gold
					break
				}
			}
		}
	}
	// Reconcile power before the first frame because loading resumes before the next economy turn.
	for team := 0; team < game.TeamCount; team++ {
		g.ReconcilePower(team)
	}
	// Seed the colony latch from the board now that power is settled, so the first score
	// pass after a load does not re announce a colony that never stopped running.
	g.RestoreColonyState()
}

// restoreUnit rebuilds one unit from its snapshot, including its attribute set.
func restoreUnit(us *UnitSnapshot) *game.UnitInfo {
	u := game.NewUnitInfo(assets.UnitType(us.Type), us.Team, int(us.ID), us.Angle)
	u.Flags = game.Flags(us.Flags)
	u.GridX, u.GridY = us.GridX, us.GridY
	u.Order = game.Order(us.Order)
	u.State = game.OrderState(us.State)
	// v22: the order the unit reverts to. A pre v22 save has 0 here, which is what
	// restoreUnit left behind before, so nothing changes for those.
	u.PriorOrder = game.Order(us.PriorOrder)
	u.PriorState = game.OrderState(us.PriorState)
	u.Angle = us.Angle
	u.TurretAngle = us.TurretAngle
	u.TurretTarget = us.TurretTarget
	u.Hits = us.Hits
	u.Speed = us.Speed
	u.Shots = us.Shots
	u.Ammo = us.Ammo
	u.Storage = us.Storage
	u.RawMining, u.FuelMining, u.GoldMining = us.RawMining, us.FuelMining, us.GoldMining
	u.RawMiningMax, u.FuelMiningMax, u.GoldMiningMax = us.RawMiningMax, us.FuelMiningMax, us.GoldMiningMax
	u.TotalMining = us.TotalMining
	u.HiddenUntilExpand = us.HiddenUntilExpand
	u.AutoSurvey = us.AutoSurvey
	u.RepeatBuild = us.RepeatBuild
	u.BuildRate = us.BuildRate
	// v13:
	u.Experience = us.Experience
	u.DisabledReactionFire = us.DisabledReactionFire
	u.LastBuildRate = us.LastBuildRate
	u.BridgeRaise = us.BridgeRaise
	u.SpottedBy = us.SpottedBy // v21: an uncovered mine stays uncovered
	for _, t := range us.BuildQueue {
		u.BuildQueue = append(u.BuildQueue, assets.UnitType(t))
	}

	v := &game.UnitValues{}
	for a := game.Attrib(0); a < game.AttribCount; a++ {
		v.Set(a, us.Values[a])
	}
	v.SetVersion(us.ValVersion)
	u.Values = v
	return u
}
