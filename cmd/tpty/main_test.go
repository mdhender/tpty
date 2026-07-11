// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mdhender/tpty"
	"github.com/mdhender/tpty/internal/prng"
)

// TestLoadStartingProvincesMissingFile verifies that an absent file yields an
// empty, non-nil set and no error, so createPlayer can distinguish "no
// provinces defined" from a genuine read failure.
func TestLoadStartingProvincesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	set, err := loadStartingProvinces(path)
	if err != nil {
		t.Fatalf("loadStartingProvinces(missing) error = %v, want nil", err)
	}
	if set == nil {
		t.Fatal("loadStartingProvinces(missing) set = nil, want empty non-nil set")
	}
	if set.Len() != 0 {
		t.Errorf("loadStartingProvinces(missing) len = %d, want 0", set.Len())
	}
}

// TestLoadStartingProvincesEmptyArray verifies that an explicit empty array is
// treated the same as a missing file: an empty set with no error.
func TestLoadStartingProvincesEmptyArray(t *testing.T) {
	path := writeTempFile(t, "[]")
	set, err := loadStartingProvinces(path)
	if err != nil {
		t.Fatalf("loadStartingProvinces([]) error = %v, want nil", err)
	}
	if set.Len() != 0 {
		t.Errorf("loadStartingProvinces([]) len = %d, want 0", set.Len())
	}
}

// TestLoadStartingProvincesValid verifies that valid entries are parsed into the
// set keyed by their canonical compact form.
func TestLoadStartingProvincesValid(t *testing.T) {
	path := writeTempFile(t, `["(0,0)","(1,-1)"]`)
	set, err := loadStartingProvinces(path)
	if err != nil {
		t.Fatalf("loadStartingProvinces(valid) error = %v, want nil", err)
	}
	for _, want := range []string{"(0,0)", "(1,-1)"} {
		if !set.Contains(want) {
			t.Errorf("loadStartingProvinces(valid) missing %q; set = %v", want, set.List())
		}
	}
	if set.Len() != 2 {
		t.Errorf("loadStartingProvinces(valid) len = %d, want 2", set.Len())
	}
}

// TestLoadStartingProvincesErrors verifies that malformed JSON and invalid
// province strings still surface as errors, so the missing-file special case
// does not mask genuine problems.
func TestLoadStartingProvincesErrors(t *testing.T) {
	tests := map[string]string{
		"malformed json":     `{`,
		"not a string":       `[123]`,
		"invalid province":   `["not-a-province"]`,
		"duplicate province": `["(0,0)","(0,0)"]`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := writeTempFile(t, content)
			if _, err := loadStartingProvinces(path); err == nil {
				t.Errorf("loadStartingProvinces(%q) error = nil, want an error", content)
			}
		})
	}
}

// setupSubmitGame builds a data directory holding a game at the given turn, one
// player, and the turn-1 seeded faction and entity for that player, then writes
// the players, factions, and entities files. It returns the data directory and
// the created player (whose Password is needed to build a valid opening record).
func setupSubmitGame(t *testing.T, turn int) (dir string, player tpty.Player) {
	t.Helper()
	dir = t.TempDir()

	seeds := prng.Seeds{Seed1: 1, Seed2: 2}
	game, err := tpty.NewGame("test-game", seeds)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	game.Turn = turn
	if err := writeJSON(filepath.Join(dir, "game.json"), game); err != nil {
		t.Fatalf("write game.json: %v", err)
	}

	players := tpty.NewPlayerStore()
	player, err = players.Create(seeds, "a@x.com", "alice", "(1,-1)")
	if err != nil {
		t.Fatalf("create player: %v", err)
	}

	factions := tpty.NewFactionStore()
	entities := tpty.NewEntityStore()
	if _, err := tpty.SeedTurnOne(players, factions, entities); err != nil {
		t.Fatalf("SeedTurnOne: %v", err)
	}

	if err := writeJSON(filepath.Join(dir, "players.json"), players); err != nil {
		t.Fatalf("write players.json: %v", err)
	}
	if err := writeJSON(filepath.Join(dir, "factions.json"), factions); err != nil {
		t.Fatalf("write factions.json: %v", err)
	}
	if err := writeJSON(filepath.Join(dir, "entities.json"), entities); err != nil {
		t.Fatalf("write entities.json: %v", err)
	}
	return dir, player
}

// TestSubmitOrdersAccepted verifies that a valid submission is stored verbatim
// under the orders directory, keyed by turn and player id.
func TestSubmitOrdersAccepted(t *testing.T) {
	dir, player := setupSubmitGame(t, 1)
	raw := "\"test-game\" " + strconv.Itoa(player.ID) + " \"" + player.Password + "\"\n\nentity 1, \"Entity 1\"\n    hold\n"
	orderFile := filepath.Join(dir, "alice-orders.txt")
	if err := os.WriteFile(orderFile, []byte(raw), 0o644); err != nil {
		t.Fatalf("write order file: %v", err)
	}

	if _, _, err := captureErr(t, func() error { return submitOrders(dir, orderFile) }); err != nil {
		t.Fatalf("submitOrders = %v, want nil", err)
	}

	path := tpty.PlayerOrdersPath(filepath.Join(dir, "orders"), 1, player.ID)
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stored orders: %v", err)
	}
	var stored tpty.StoredOrders
	if err := json.Unmarshal(buf, &stored); err != nil {
		t.Fatalf("decode stored orders: %v", err)
	}
	if stored.Raw != raw {
		t.Errorf("stored Raw = %q, want %q", stored.Raw, raw)
	}
	if stored.Turn != 1 {
		t.Errorf("stored Turn = %d, want 1", stored.Turn)
	}
	if stored.PlayerID != player.ID {
		t.Errorf("stored PlayerID = %d, want %d", stored.PlayerID, player.ID)
	}
}

// TestSubmitOrdersAuthFailureRejects verifies that a wrong password rejects the
// file in full: a non-nil error wrapping ErrOrdersBadPassword and nothing stored.
func TestSubmitOrdersAuthFailureRejects(t *testing.T) {
	dir, player := setupSubmitGame(t, 1)
	raw := "\"test-game\" " + strconv.Itoa(player.ID) + " \"wrong-password\"\n\nentity 1, \"Entity 1\"\n    hold\n"
	orderFile := filepath.Join(dir, "alice-orders.txt")
	if err := os.WriteFile(orderFile, []byte(raw), 0o644); err != nil {
		t.Fatalf("write order file: %v", err)
	}

	_, _, err := captureErr(t, func() error { return submitOrders(dir, orderFile) })
	if err == nil {
		t.Fatal("submitOrders with wrong password = nil error, want an error")
	}
	if !errors.Is(err, tpty.ErrOrdersBadPassword) {
		t.Errorf("submitOrders error = %v, want it to wrap ErrOrdersBadPassword", err)
	}

	path := tpty.PlayerOrdersPath(filepath.Join(dir, "orders"), 1, player.ID)
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("orders were stored despite the authentication failure")
	}
}

// TestSubmitOrdersTurnZeroGuard verifies that a game at turn 0 refuses to accept
// orders and stores nothing.
func TestSubmitOrdersTurnZeroGuard(t *testing.T) {
	dir, player := setupSubmitGame(t, 0)
	raw := "\"test-game\" " + strconv.Itoa(player.ID) + " \"" + player.Password + "\"\n\nentity 1, \"Entity 1\"\n    hold\n"
	orderFile := filepath.Join(dir, "alice-orders.txt")
	if err := os.WriteFile(orderFile, []byte(raw), 0o644); err != nil {
		t.Fatalf("write order file: %v", err)
	}

	_, _, err := captureErr(t, func() error { return submitOrders(dir, orderFile) })
	if err == nil {
		t.Fatal("submitOrders at turn 0 = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "turn") || !strings.Contains(err.Error(), "play") {
		t.Errorf("turn-0 error = %q, want it to mention the turn and play", err.Error())
	}
	path := tpty.PlayerOrdersPath(filepath.Join(dir, "orders"), 0, player.ID)
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("orders were stored despite the turn-0 guard")
	}
}

// TestSubmitOrdersWarningsButAccepted verifies that a submission with an unknown
// command and a block for an entity the player does not own is still accepted and
// stored, with the problems reported as warnings.
func TestSubmitOrdersWarningsButAccepted(t *testing.T) {
	dir, player := setupSubmitGame(t, 1)
	// entity 1 is owned (a bogus command warns); entity 99 does not exist (an
	// ownership warning).
	raw := "\"test-game\" " + strconv.Itoa(player.ID) + " \"" + player.Password + "\"\n\n" +
		"entity 1, \"Entity 1\"\n    bogus 1 2\n\n" +
		"entity 99, \"Ghost\"\n    hold\n"
	orderFile := filepath.Join(dir, "alice-orders.txt")
	if err := os.WriteFile(orderFile, []byte(raw), 0o644); err != nil {
		t.Fatalf("write order file: %v", err)
	}

	stdout, _, err := captureErr(t, func() error { return submitOrders(dir, orderFile) })
	if err != nil {
		t.Fatalf("submitOrders with warnings = %v, want nil", err)
	}
	if !strings.Contains(stdout, "warnings") {
		t.Errorf("stdout = %q, want it to report warnings", stdout)
	}

	path := tpty.PlayerOrdersPath(filepath.Join(dir, "orders"), 1, player.ID)
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("orders were not stored despite acceptance: %v", statErr)
	}
}

// TestListOrdersNoSubmissions verifies that at the current turn, before anyone
// has submitted (the turn directory is absent), every active player is shown as
// "not submitted" and the summary reports 0 submitted.
func TestListOrdersNoSubmissions(t *testing.T) {
	dir, player := setupSubmitGame(t, 1)

	stdout, _, err := captureErr(t, func() error { return listOrders(dir) })
	if err != nil {
		t.Fatalf("listOrders = %v, want nil", err)
	}
	if !strings.Contains(stdout, player.Handle) {
		t.Errorf("stdout = %q, want it to list player %q", stdout, player.Handle)
	}
	if !strings.Contains(stdout, "not submitted") {
		t.Errorf("stdout = %q, want the player shown as not submitted", stdout)
	}
	if !strings.Contains(stdout, "0 of 1 active player(s) have submitted") {
		t.Errorf("stdout = %q, want a '0 of 1' summary", stdout)
	}
}

// TestListOrdersAfterSubmission verifies that once a submission exists for a
// player they are shown as "submitted" and the summary count reflects it.
func TestListOrdersAfterSubmission(t *testing.T) {
	dir, player := setupSubmitGame(t, 1)

	// Store a submission for the player directly, the same way "orders submit"
	// would.
	stored := tpty.StoredOrders{Turn: 1, PlayerID: player.ID, Raw: "\"test-game\" 1 \"pw\"\n"}
	if err := writeJSON(tpty.PlayerOrdersPath(filepath.Join(dir, "orders"), 1, player.ID), stored); err != nil {
		t.Fatalf("write stored orders: %v", err)
	}

	stdout, _, err := captureErr(t, func() error { return listOrders(dir) })
	if err != nil {
		t.Fatalf("listOrders = %v, want nil", err)
	}
	if !strings.Contains(stdout, "submitted") {
		t.Errorf("stdout = %q, want the player shown as submitted", stdout)
	}
	if strings.Contains(stdout, "not submitted") {
		t.Errorf("stdout = %q, want no 'not submitted' rows (the only player has submitted)", stdout)
	}
	if !strings.Contains(stdout, "1 of 1 active player(s) have submitted") {
		t.Errorf("stdout = %q, want a '1 of 1' summary", stdout)
	}
}

// TestListOrdersTurnZeroGuard verifies that a game at turn 0 refuses the status
// view, consistent with "orders submit".
func TestListOrdersTurnZeroGuard(t *testing.T) {
	dir, _ := setupSubmitGame(t, 0)

	_, _, err := captureErr(t, func() error { return listOrders(dir) })
	if err == nil {
		t.Fatal("listOrders at turn 0 = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "turn") || !strings.Contains(err.Error(), "play") {
		t.Errorf("turn-0 error = %q, want it to mention the turn and play", err.Error())
	}
}

// reloadEntities reads and decodes the entities.json written under dir.
func reloadEntities(t *testing.T, dir string) *tpty.EntityStore {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join(dir, "entities.json"))
	if err != nil {
		t.Fatalf("read entities.json: %v", err)
	}
	var store tpty.EntityStore
	if err := json.Unmarshal(buf, &store); err != nil {
		t.Fatalf("decode entities.json: %v", err)
	}
	return &store
}

// TestProcessTurnHappyPath verifies that processing a turn with a submitted move
// writes the result file, reports at least one executed (non-stub) order, and
// updates the entity's location in the reloaded entities.json.
func TestProcessTurnHappyPath(t *testing.T) {
	dir, player := setupSubmitGame(t, 1)

	// The seeded entity 1 starts at the player's starting province (1,-1). A
	// one-step move north (direction 1 = (0,-1)) takes it to (1,-2).
	raw := "\"test-game\" " + strconv.Itoa(player.ID) + " \"" + player.Password + "\"\n\nentity 1, \"Entity 1\"\n    move 1\n"
	orderFile := filepath.Join(dir, "alice-orders.txt")
	if err := os.WriteFile(orderFile, []byte(raw), 0o644); err != nil {
		t.Fatalf("write order file: %v", err)
	}
	if _, _, err := captureErr(t, func() error { return submitOrders(dir, orderFile) }); err != nil {
		t.Fatalf("submitOrders = %v, want nil", err)
	}

	stdout, _, err := captureErr(t, func() error { return processTurn(dir) })
	if err != nil {
		t.Fatalf("processTurn = %v, want nil", err)
	}

	// The result file marks the turn processed.
	resultPath := tpty.TurnResultPath(filepath.Join(dir, "turns"), 1)
	if _, statErr := os.Stat(resultPath); statErr != nil {
		t.Errorf("result file %s not written: %v", resultPath, statErr)
	}
	// The summary reports at least one executed (non-stub) order.
	if !strings.Contains(stdout, "1 executed") {
		t.Errorf("stdout = %q, want it to report 1 executed order", stdout)
	}

	// The move actually updated the entity's location in the store.
	store := reloadEntities(t, dir)
	e, ok := store.ByID(1)
	if !ok {
		t.Fatal("entity 1 missing from reloaded entities.json")
	}
	if e.Location != "(1,-2)" {
		t.Errorf("entity 1 Location = %q, want %q", e.Location, "(1,-2)")
	}
}

// TestProcessTurnTurnZeroGuard verifies that a game at turn 0 refuses to process
// and writes no result.
func TestProcessTurnTurnZeroGuard(t *testing.T) {
	dir, _ := setupSubmitGame(t, 0)

	_, _, err := captureErr(t, func() error { return processTurn(dir) })
	if err == nil {
		t.Fatal("processTurn at turn 0 = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "turn") || !strings.Contains(err.Error(), "play") {
		t.Errorf("turn-0 error = %q, want it to mention the turn and play", err.Error())
	}
	if _, statErr := os.Stat(tpty.TurnResultPath(filepath.Join(dir, "turns"), 0)); statErr == nil {
		t.Error("a result file was written despite the turn-0 guard")
	}
}

// TestProcessTurnNoOrdersGuard verifies that processing a turn with no collected
// orders is refused with a clear message.
func TestProcessTurnNoOrdersGuard(t *testing.T) {
	dir, _ := setupSubmitGame(t, 1)

	_, _, err := captureErr(t, func() error { return processTurn(dir) })
	if err == nil {
		t.Fatal("processTurn with no orders = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "no orders collected for turn 1") {
		t.Errorf("no-orders error = %q, want it to say 'no orders collected for turn 1'", err.Error())
	}
}

// TestProcessTurnDoubleProcessGuard verifies that processing an already-processed
// turn is refused.
func TestProcessTurnDoubleProcessGuard(t *testing.T) {
	dir, player := setupSubmitGame(t, 1)

	raw := "\"test-game\" " + strconv.Itoa(player.ID) + " \"" + player.Password + "\"\n\nentity 1, \"Entity 1\"\n    hold\n"
	orderFile := filepath.Join(dir, "alice-orders.txt")
	if err := os.WriteFile(orderFile, []byte(raw), 0o644); err != nil {
		t.Fatalf("write order file: %v", err)
	}
	if _, _, err := captureErr(t, func() error { return submitOrders(dir, orderFile) }); err != nil {
		t.Fatalf("submitOrders = %v, want nil", err)
	}

	if _, _, err := captureErr(t, func() error { return processTurn(dir) }); err != nil {
		t.Fatalf("first processTurn = %v, want nil", err)
	}

	_, _, err := captureErr(t, func() error { return processTurn(dir) })
	if err == nil {
		t.Fatal("second processTurn = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "already processed") {
		t.Errorf("double-process error = %q, want it to say 'already processed'", err.Error())
	}
}

// reloadGame reads and decodes the game.json written under dir.
func reloadGame(t *testing.T, dir string) *tpty.Game {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join(dir, "game.json"))
	if err != nil {
		t.Fatalf("read game.json: %v", err)
	}
	var game tpty.Game
	if err := json.Unmarshal(buf, &game); err != nil {
		t.Fatalf("decode game.json: %v", err)
	}
	return &game
}

// reloadFactions reads and decodes the factions.json written under dir.
func reloadFactions(t *testing.T, dir string) *tpty.FactionStore {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join(dir, "factions.json"))
	if err != nil {
		t.Fatalf("read factions.json: %v", err)
	}
	var store tpty.FactionStore
	if err := json.Unmarshal(buf, &store); err != nil {
		t.Fatalf("decode factions.json: %v", err)
	}
	return &store
}

// setupAdvanceGameTurnZero builds a data directory holding a fresh game at turn
// 0 with one player and empty faction/entity stores — the state just before the
// setup→play transition — so the 0→1 seeding path can be exercised without the
// double-seeding that setupSubmitGame would cause. It returns the data directory
// and the created player.
func setupAdvanceGameTurnZero(t *testing.T) (dir string, player tpty.Player) {
	t.Helper()
	dir = t.TempDir()

	seeds := prng.Seeds{Seed1: 1, Seed2: 2}
	game, err := tpty.NewGame("test-game", seeds)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	// game.Turn defaults to 0 (setup).
	if err := writeJSON(filepath.Join(dir, "game.json"), game); err != nil {
		t.Fatalf("write game.json: %v", err)
	}

	players := tpty.NewPlayerStore()
	player, err = players.Create(seeds, "a@x.com", "alice", "(1,-1)")
	if err != nil {
		t.Fatalf("create player: %v", err)
	}
	if err := writeJSON(filepath.Join(dir, "players.json"), players); err != nil {
		t.Fatalf("write players.json: %v", err)
	}
	if err := writeJSON(filepath.Join(dir, "factions.json"), tpty.NewFactionStore()); err != nil {
		t.Fatalf("write factions.json: %v", err)
	}
	if err := writeJSON(filepath.Join(dir, "entities.json"), tpty.NewEntityStore()); err != nil {
		t.Fatalf("write entities.json: %v", err)
	}
	return dir, player
}

// TestAdvanceTurnZeroSeedsAndPersists verifies that advancing from turn 0 makes
// the game turn 1, seeds the player a faction and starting entity, persists all
// three files, and reports the seeding.
func TestAdvanceTurnZeroSeedsAndPersists(t *testing.T) {
	dir, player := setupAdvanceGameTurnZero(t)

	stdout, _, err := captureErr(t, func() error { return advanceTurn(dir) })
	if err != nil {
		t.Fatalf("advanceTurn = %v, want nil", err)
	}

	if got := reloadGame(t, dir); got.Turn != 1 {
		t.Errorf("game.Turn = %d, want 1", got.Turn)
	}

	factions := reloadFactions(t, dir)
	if len(factions.Factions) != 1 {
		t.Fatalf("factions = %d, want 1", len(factions.Factions))
	}
	if c := factions.Factions[0].Controller; c.Kind != tpty.ControllerPlayer || c.ID != player.ID {
		t.Errorf("faction controller = %+v, want player %d", c, player.ID)
	}

	entities := reloadEntities(t, dir)
	if len(entities.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(entities.Entities))
	}
	if loc := entities.Entities[0].Location; loc != player.StartingProvince {
		t.Errorf("entity location = %q, want %q", loc, player.StartingProvince)
	}

	if !strings.Contains(stdout, "turn 1") {
		t.Errorf("stdout = %q, want it to report turn 1", stdout)
	}
	if !strings.Contains(stdout, "seeded 1") {
		t.Errorf("stdout = %q, want it to report 1 player seeded", stdout)
	}
}

// TestAdvanceUnprocessedTurnRefused verifies that advancing a turn at or beyond
// turn 1 that has not been processed is refused and game.json is left unchanged.
func TestAdvanceUnprocessedTurnRefused(t *testing.T) {
	dir, _ := setupSubmitGame(t, 1)

	_, _, err := captureErr(t, func() error { return advanceTurn(dir) })
	if err == nil {
		t.Fatal("advanceTurn on an unprocessed turn = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "process turn 1 before advancing") {
		t.Errorf("guard error = %q, want it to say 'process turn 1 before advancing'", err.Error())
	}
	if got := reloadGame(t, dir); got.Turn != 1 {
		t.Errorf("game.Turn = %d, want it unchanged at 1", got.Turn)
	}
}

// TestAdvanceProcessedTurnIncrements verifies that advancing a processed turn
// N≥1 increments the turn (and seeds nothing), persisting game.json. The
// processed precondition is written directly with tpty.SaveTurnResult.
func TestAdvanceProcessedTurnIncrements(t *testing.T) {
	dir, _ := setupSubmitGame(t, 1)

	// Mark turn 1 processed by writing its result file.
	if err := tpty.SaveTurnResult(filepath.Join(dir, "turns"), tpty.TurnResult{Turn: 1}); err != nil {
		t.Fatalf("SaveTurnResult: %v", err)
	}

	stdout, _, err := captureErr(t, func() error { return advanceTurn(dir) })
	if err != nil {
		t.Fatalf("advanceTurn = %v, want nil", err)
	}
	if got := reloadGame(t, dir); got.Turn != 2 {
		t.Errorf("game.Turn = %d, want 2", got.Turn)
	}
	if !strings.Contains(stdout, "turn 2") {
		t.Errorf("stdout = %q, want it to report turn 2", stdout)
	}
	if strings.Contains(stdout, "seeded") {
		t.Errorf("stdout = %q, want no seeding on a 1→2 advance", stdout)
	}
}

// TestLoadFactionsMissingFile verifies that an absent factions file yields an
// empty, non-nil store and no error.
func TestLoadFactionsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	store, err := loadFactions(path)
	if err != nil {
		t.Fatalf("loadFactions(missing) error = %v, want nil", err)
	}
	if store == nil {
		t.Fatal("loadFactions(missing) store = nil, want empty non-nil store")
	}
	if len(store.Factions) != 0 {
		t.Errorf("loadFactions(missing) has %d factions, want 0", len(store.Factions))
	}
}

// TestLoadEntitiesMissingFile verifies that an absent entities file yields an
// empty, non-nil store and no error.
func TestLoadEntitiesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	store, err := loadEntities(path)
	if err != nil {
		t.Fatalf("loadEntities(missing) error = %v, want nil", err)
	}
	if store == nil {
		t.Fatal("loadEntities(missing) store = nil, want empty non-nil store")
	}
}

// writeTempFile writes content to a file in a fresh temp dir and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "starting-provinces.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// setupGameDir creates a data directory with a game.json and, when rings > 0, a
// world.json of that many rings. When rings <= 0 no world file is written, so
// the world-absent path can be exercised.
func setupGameDir(t *testing.T, rings int) string {
	t.Helper()
	dir := t.TempDir()
	game, err := tpty.NewGame("test-game", prng.Seeds{Seed1: 1, Seed2: 2})
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := writeJSON(filepath.Join(dir, "game.json"), game); err != nil {
		t.Fatalf("write game.json: %v", err)
	}
	if rings > 0 {
		world := map[string]any{"rings": rings, "provinces": []any{}}
		if err := writeJSON(filepath.Join(dir, "world.json"), world); err != nil {
			t.Fatalf("write world.json: %v", err)
		}
	}
	return dir
}

// readProvinces reads and decodes the starting-provinces.json in dir.
func readProvinces(t *testing.T, dir string) []string {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join(dir, "starting-provinces.json"))
	if err != nil {
		t.Fatalf("read starting-provinces.json: %v", err)
	}
	var got []string
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatalf("decode starting-provinces.json: %v", err)
	}
	return got
}

// captureOutput runs fn with os.Stdout and os.Stderr redirected to pipes and
// returns whatever each received.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	outC, errC := make(chan string, 1), make(chan string, 1)
	go func() { var b bytes.Buffer; _, _ = io.Copy(&b, outR); outC <- b.String() }()
	go func() { var b bytes.Buffer; _, _ = io.Copy(&b, errR); errC <- b.String() }()

	fn()

	_ = outW.Close()
	_ = errW.Close()
	return <-outC, <-errC
}

func TestGenerateStartingProvincesHardFailsWithoutWorld(t *testing.T) {
	dir := setupGameDir(t, 0) // no world.json
	err := generateStartingProvinces(dir, 0, false)
	if err == nil {
		t.Fatal("generateStartingProvinces without world.json = nil error, want error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "starting-provinces.json")); statErr == nil {
		t.Error("starting-provinces.json was written despite the world being absent")
	}
}

func TestGenerateStartingProvincesDefaultRing(t *testing.T) {
	dir := setupGameDir(t, 3) // default ring = ceil(3/2) = 2
	if _, _, err := runGenerate(t, dir, 0, false); err != nil {
		t.Fatalf("generateStartingProvinces: %v", err)
	}
	want := []string{"(0,-2)", "(2,-2)", "(2,0)", "(0,2)", "(-2,2)", "(-2,0)"}
	if got := readProvinces(t, dir); !equalStrings(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}
}

func TestGenerateStartingProvincesExplicitRing(t *testing.T) {
	dir := setupGameDir(t, 3)
	if _, _, err := runGenerate(t, dir, 1, false); err != nil {
		t.Fatalf("generateStartingProvinces: %v", err)
	}
	want := []string{"(0,-1)", "(1,-1)", "(1,0)", "(0,1)", "(-1,1)", "(-1,0)"}
	if got := readProvinces(t, dir); !equalStrings(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}
}

func TestGenerateStartingProvincesRejectsOutOfRangeRing(t *testing.T) {
	dir := setupGameDir(t, 3)
	for _, ring := range []int{-1, 4, 100} {
		if err := generateStartingProvinces(dir, ring, false); err == nil {
			t.Errorf("--ring %d with a 3-ring world = nil error, want error", ring)
		}
	}
}

func TestGenerateStartingProvincesFailsWhenFileExists(t *testing.T) {
	dir := setupGameDir(t, 3)
	spPath := filepath.Join(dir, "starting-provinces.json")
	if err := os.WriteFile(spPath, []byte(`["(0,0)"]`), 0o644); err != nil {
		t.Fatalf("seed starting-provinces.json: %v", err)
	}
	if err := generateStartingProvinces(dir, 0, false); err == nil {
		t.Fatal("existing file without --overwrite = nil error, want error")
	}
	// The pre-existing file must be untouched.
	if got := readProvinces(t, dir); !equalStrings(got, []string{"(0,0)"}) {
		t.Errorf("existing file was modified: %v", got)
	}
}

func TestGenerateStartingProvincesOverwrite(t *testing.T) {
	dir := setupGameDir(t, 3)
	spPath := filepath.Join(dir, "starting-provinces.json")
	if err := os.WriteFile(spPath, []byte(`["(0,0)"]`), 0o644); err != nil {
		t.Fatalf("seed starting-provinces.json: %v", err)
	}
	if _, _, err := runGenerate(t, dir, 0, true); err != nil {
		t.Fatalf("generateStartingProvinces --overwrite: %v", err)
	}
	want := []string{"(0,-2)", "(2,-2)", "(2,0)", "(0,2)", "(-2,2)", "(-2,0)"}
	if got := readProvinces(t, dir); !equalStrings(got, want) {
		t.Errorf("after overwrite wrote %v, want %v", got, want)
	}
}

func TestGenerateStartingProvincesWarnsWhenPlayersExist(t *testing.T) {
	dir := setupGameDir(t, 3)
	if err := os.WriteFile(filepath.Join(dir, "players.json"), []byte(`{"players":[]}`), 0o644); err != nil {
		t.Fatalf("seed players.json: %v", err)
	}
	_, stderr, err := runGenerate(t, dir, 0, false)
	if err != nil {
		t.Fatalf("generateStartingProvinces with players.json = %v, want success", err)
	}
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "players.json") {
		t.Errorf("stderr = %q, want a warning mentioning players.json", stderr)
	}
	// It still writes the set despite the warning.
	if got := readProvinces(t, dir); len(got) != 6 {
		t.Errorf("wrote %d provinces, want 6", len(got))
	}
}

// TestAddStartingProvinceCreatesAndAppends confirms "add" creates the file when
// absent and appends in order, keeping entries unique.
func TestAddStartingProvinceCreatesAndAppends(t *testing.T) {
	dir := setupGameDir(t, 3)

	for _, p := range []string{"(0,0)", "(1,-1)"} {
		if _, _, err := captureErr(t, func() error { return addStartingProvince(dir, p) }); err != nil {
			t.Fatalf("addStartingProvince(%q): %v", p, err)
		}
	}
	if got := readProvinces(t, dir); !equalStrings(got, []string{"(0,0)", "(1,-1)"}) {
		t.Errorf("after adds, file = %v, want [(0,0) (1,-1)]", got)
	}

	// A duplicate is rejected and the file is unchanged.
	if _, _, err := captureErr(t, func() error { return addStartingProvince(dir, "(0,0)") }); err == nil {
		t.Error("adding a duplicate = nil error, want an error")
	}
	if got := readProvinces(t, dir); len(got) != 2 {
		t.Errorf("duplicate add changed the file: %v", got)
	}

	// A non-canonical province is rejected.
	if _, _, err := captureErr(t, func() error { return addStartingProvince(dir, "(0, 0)") }); err == nil {
		t.Error("adding a non-canonical province = nil error, want an error")
	}
}

// TestRemoveStartingProvince confirms "remove" deletes an entry, rejects an
// absent one, and leaves the rest in order.
func TestRemoveStartingProvince(t *testing.T) {
	dir := setupGameDir(t, 3)
	seedStartingProvinces(t, dir, `["(0,0)","(1,-1)","(-2,0)"]`)

	if _, _, err := captureErr(t, func() error { return removeStartingProvince(dir, "(1,-1)") }); err != nil {
		t.Fatalf("removeStartingProvince: %v", err)
	}
	if got := readProvinces(t, dir); !equalStrings(got, []string{"(0,0)", "(-2,0)"}) {
		t.Errorf("after remove, file = %v, want [(0,0) (-2,0)]", got)
	}

	// Removing a province not in the set is an error, and the file is unchanged.
	if _, _, err := captureErr(t, func() error { return removeStartingProvince(dir, "(9,9)") }); err == nil {
		t.Error("removing an absent province = nil error, want an error")
	}
	if got := readProvinces(t, dir); len(got) != 2 {
		t.Errorf("failed remove changed the file: %v", got)
	}
}

// TestRemoveStartingProvinceWarnsOnStrandedPlayer confirms removing a province a
// player is placed on warns (but proceeds).
func TestRemoveStartingProvinceWarnsOnStrandedPlayer(t *testing.T) {
	dir := setupGameDir(t, 3)
	seedStartingProvinces(t, dir, `["(0,0)","(1,-1)"]`)
	players := `{"next_id":2,"players":[{"id":1,"handle":"alice","email":"a@x.com","starting_province":"(1,-1)","password":"x","seeds":{"seed1":1,"seed2":2}}]}`
	if err := os.WriteFile(filepath.Join(dir, "players.json"), []byte(players), 0o644); err != nil {
		t.Fatalf("seed players.json: %v", err)
	}

	_, stderr, err := captureErr(t, func() error { return removeStartingProvince(dir, "(1,-1)") })
	if err != nil {
		t.Fatalf("removeStartingProvince: %v", err)
	}
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "alice") {
		t.Errorf("stderr = %q, want a warning naming the stranded player", stderr)
	}
	// The removal still happened.
	if got := readProvinces(t, dir); !equalStrings(got, []string{"(0,0)"}) {
		t.Errorf("after remove, file = %v, want [(0,0)]", got)
	}
}

// TestListStartingProvinces confirms "list" prints each province, and reports an
// empty set distinctly.
func TestListStartingProvinces(t *testing.T) {
	dir := setupGameDir(t, 3)

	// Empty set: a distinct message, no province lines.
	stdout, _, err := captureErr(t, func() error { return listStartingProvinces(dir) })
	if err != nil {
		t.Fatalf("listStartingProvinces(empty): %v", err)
	}
	if !strings.Contains(stdout, "no starting provinces") {
		t.Errorf("empty list stdout = %q, want a 'no starting provinces' message", stdout)
	}

	seedStartingProvinces(t, dir, `["(0,0)","(1,-1)"]`)
	stdout, _, err = captureErr(t, func() error { return listStartingProvinces(dir) })
	if err != nil {
		t.Fatalf("listStartingProvinces: %v", err)
	}
	for _, want := range []string{"(0,0)", "(1,-1)"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("list stdout = %q, want it to contain %q", stdout, want)
		}
	}
}

// TestAddedProvinceUnlocksPlayerCreate is an end-to-end check that a province is
// not accepted for a player until it has been added to the allowed set.
func TestAddedProvinceUnlocksPlayerCreate(t *testing.T) {
	dir := setupGameDir(t, 3)

	// With no allowed set, creating a player on (1,-1) fails.
	if _, _, err := captureErr(t, func() error {
		return createPlayer(dir, "a@x.com", "alice", "(1,-1)")
	}); err == nil {
		t.Fatal("createPlayer with no allowed set = nil error, want an error")
	}

	if _, _, err := captureErr(t, func() error { return addStartingProvince(dir, "(1,-1)") }); err != nil {
		t.Fatalf("addStartingProvince: %v", err)
	}
	if _, _, err := captureErr(t, func() error {
		return createPlayer(dir, "a@x.com", "alice", "(1,-1)")
	}); err != nil {
		t.Errorf("createPlayer after add = %v, want success", err)
	}
}

// seedStartingProvinces writes a starting-provinces.json into dir.
func seedStartingProvinces(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "starting-provinces.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("seed starting-provinces.json: %v", err)
	}
}

// captureErr runs fn with output captured, returning stdout, stderr, and fn's
// error. It is the error-returning companion to captureOutput.
func captureErr(t *testing.T, fn func() error) (stdout, stderr string, err error) {
	t.Helper()
	stdout, stderr = captureOutput(t, func() { err = fn() })
	return stdout, stderr, err
}

// runGenerate runs generateStartingProvinces with output captured, returning the
// captured stdout, stderr, and the command's error.
func runGenerate(t *testing.T, dir string, ring int, overwrite bool) (stdout, stderr string, err error) {
	t.Helper()
	stdout, stderr = captureOutput(t, func() { err = generateStartingProvinces(dir, ring, overwrite) })
	return stdout, stderr, err
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
