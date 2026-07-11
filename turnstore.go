// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mdhender/tpty/internal/cerrs"
)

// ErrTurnNotProcessed is returned by LoadTurnResult when no result file exists
// for the requested turn — the turn has not been processed yet. The existence of
// the result file is the "already processed" marker (see
// content/docs/reference/turns.md, "Per-turn lifecycle").
const ErrTurnNotProcessed = cerrs.Error("turn not processed")

// TurnDir returns the directory, under the game's turns directory, that holds the
// per-turn processing output for the given turn. The turn number is zero-padded
// to four digits ("turn-0001"), matching OrdersTurnDir; turns beyond four digits
// expand naturally.
//
// See content/docs/reference/games.md ("Manifest", storage layout).
func TurnDir(turnsDir string, turn int) string {
	return filepath.Join(turnsDir, fmt.Sprintf("turn-%04d", turn))
}

// TurnResultPath returns the path of the result file for the given turn:
// turns/turn-NNNN/result.json. Its existence marks the turn as processed.
//
// See content/docs/reference/games.md ("Manifest", storage layout) and
// content/docs/reference/turn-processing.md.
func TurnResultPath(turnsDir string, turn int) string {
	return filepath.Join(TurnDir(turnsDir, turn), "result.json")
}

// LoadTurnOrders reads every player's submitted orders for the given turn from
// the game's orders directory and returns them sorted deterministically by player
// id. Each "player-NNNN.json" file under the turn's order directory is decoded
// into a StoredOrders; a filename that does not parse as a player-orders name
// (see ParsePlayerOrdersFilename) is skipped. A missing turn directory — nobody
// has submitted — yields an empty slice and a nil error, not an error.
//
// The result is sorted by player id so the engine is fed submissions in a
// deterministic order regardless of directory-read order (CLAUDE.md,
// determinism).
//
// See content/docs/reference/orders/_index.md and
// content/docs/reference/turn-processing.md.
func LoadTurnOrders(ordersDir string, turn int) ([]StoredOrders, error) {
	dir := OrdersTurnDir(ordersDir, turn)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read orders dir %s: %w", dir, err)
	}

	out := make([]StoredOrders, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := ParsePlayerOrdersFilename(entry.Name()); !ok {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		buf, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read orders %s: %w", path, err)
		}
		var so StoredOrders
		if err := json.Unmarshal(buf, &so); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		out = append(out, so)
	}

	// Sort by player id so draws are order-independent (CLAUDE.md, determinism).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].PlayerID < out[j].PlayerID
	})
	return out, nil
}

// SaveTurnResult writes the turn result to its result path under turnsDir,
// keyed by result.Turn, creating parent directories as needed. Writing the file
// marks the turn as processed.
//
// See content/docs/reference/turn-processing.md and
// content/docs/reference/games.md ("Manifest", storage layout).
func SaveTurnResult(turnsDir string, result TurnResult) error {
	path := TurnResultPath(turnsDir, result.Turn)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	buf, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode turn result: %w", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// LoadTurnResult reads the result file for the given turn from turnsDir. A
// missing result file returns ErrTurnNotProcessed, which callers treat as "not
// processed yet" (for example, when looking up the previous turn's carryover).
//
// See content/docs/reference/turn-processing.md.
func LoadTurnResult(turnsDir string, turn int) (TurnResult, error) {
	path := TurnResultPath(turnsDir, turn)
	buf, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return TurnResult{}, ErrTurnNotProcessed
	}
	if err != nil {
		return TurnResult{}, fmt.Errorf("read turn result %s: %w", path, err)
	}
	var result TurnResult
	if err := json.Unmarshal(buf, &result); err != nil {
		return TurnResult{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return result, nil
}
