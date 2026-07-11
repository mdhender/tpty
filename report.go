// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Report is one player's turn report: what a single player receives at the
// start of a turn. It is scoped to that player and carries only their own
// factions and the entities those factions own, all as of the start of turn
// Turn (before that turn's orders). It deliberately says nothing about the
// wider world — other players' factions or entities, or the provinces the
// player's entities occupy — which is left to a later rule; an entity's
// location is the bare coordinate only.
//
// PlayerEmail is included so the GM can address the manual delivery (there is
// no automated mail; see the Delivery section of the reference).
//
// See content/docs/reference/reports.md for the model.
type Report struct {
	PlayerID     int       `json:"player_id"`
	PlayerHandle string    `json:"player_handle"`
	PlayerEmail  string    `json:"player_email"`
	Turn         int       `json:"turn"`
	Factions     []Faction `json:"factions"`
	Entities     []Entity  `json:"entities"`
}

// GenerateReports builds one turn report per active player, describing the
// state at the start of the given turn. It returns the reports in ascending
// player-id order.
//
// A player's factions are those whose controller is
// {Kind: ControllerPlayer, ID: <player id>}; a player's entities are the
// entities those factions own (EntityStore.ByFaction). Both are ordered by id
// so the output is deterministic and independent of any map or slice iteration
// order (CLAUDE.md, determinism). Inactive (removed) players get no report. No
// world or terrain data is read — a report carries only the player, the turn,
// their factions, and their entities.
//
// The reports reflect the stores passed in; when the caller supplies the live
// stores right after advancing (before processing), that is the start-of-turn
// state the reference describes.
//
// See content/docs/reference/reports.md.
func GenerateReports(game *Game, players *PlayerStore, factions *FactionStore, entities *EntityStore, turn int) []Report {
	_ = game // the report model is scoped to the player, not the game manifest

	// Collect active players and sort by id so report order is deterministic and
	// not tied to slice/append order (CLAUDE.md, determinism).
	active := make([]Player, 0, len(players.Players))
	for _, p := range players.Players {
		if p.Active() {
			active = append(active, p)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		return active[i].ID < active[j].ID
	})

	reports := make([]Report, 0, len(active))
	for _, p := range active {
		// This player's factions: those they control, in ascending faction-id
		// order.
		var playerFactions []Faction
		for _, f := range factions.Factions {
			if f.Controller.Kind == ControllerPlayer && f.Controller.ID == p.ID {
				playerFactions = append(playerFactions, f)
			}
		}
		sort.SliceStable(playerFactions, func(i, j int) bool {
			return playerFactions[i].ID < playerFactions[j].ID
		})

		// This player's entities: everything owned by those factions, in
		// ascending entity-id order.
		var playerEntities []Entity
		for _, f := range playerFactions {
			playerEntities = append(playerEntities, entities.ByFaction(f.ID)...)
		}
		sort.SliceStable(playerEntities, func(i, j int) bool {
			return playerEntities[i].ID < playerEntities[j].ID
		})

		reports = append(reports, Report{
			PlayerID:     p.ID,
			PlayerHandle: p.Handle,
			PlayerEmail:  p.Email,
			Turn:         turn,
			Factions:     playerFactions,
			Entities:     playerEntities,
		})
	}
	return reports
}

// ReportsTurnDir returns the directory, under the game's reports directory, that
// holds every player's report for the given turn. The turn number is zero-padded
// to four digits ("turn-0001"), matching OrdersTurnDir and TurnDir; turns beyond
// four digits expand naturally.
//
// See content/docs/reference/games.md ("Manifest", storage layout).
func ReportsTurnDir(reportsDir string, turn int) string {
	return filepath.Join(reportsDir, fmt.Sprintf("turn-%04d", turn))
}

// PlayerReportFilename returns the base filename holding a single player's report
// for a turn. The player id is zero-padded to four digits ("player-0003.json"),
// matching PlayerOrdersFilename; ids beyond four digits expand naturally.
func PlayerReportFilename(playerID int) string {
	return fmt.Sprintf("player-%04d.json", playerID)
}

// PlayerReportPath returns the path, under the game's reports directory, of the
// file holding the given player's report for the given turn:
// reports/turn-NNNN/player-NNNN.json. One file per turn per player.
//
// See content/docs/reference/games.md ("Manifest", storage layout).
func PlayerReportPath(reportsDir string, turn, playerID int) string {
	return filepath.Join(ReportsTurnDir(reportsDir, turn), PlayerReportFilename(playerID))
}

// SaveReport writes a report as indented JSON to its path under reportsDir,
// keyed by report.Turn and report.PlayerID, creating parent directories as
// needed. The report is written in the structured model of content/docs/
// reference/reports.md; a human-readable presentation format is future work.
//
// See content/docs/reference/games.md ("Manifest", storage layout) and
// content/docs/reference/reports.md.
func SaveReport(reportsDir string, report Report) error {
	path := PlayerReportPath(reportsDir, report.Turn, report.PlayerID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	buf, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
