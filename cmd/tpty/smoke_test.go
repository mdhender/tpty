// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mdhender/tpty"
	"github.com/mdhender/tpty/internal/prng"
)

// TestSmokeEndToEnd drives the whole MVP loop through the real command impl
// functions, in a throwaway data directory, and proves the two properties the
// end-to-end milestone (burndown items 22 & 23, epic #24) asks for:
//
//   - the turn counter moves 0 → 1 → 2 across advance/process/advance; and
//   - reports reflect start-of-turn state — the turn-1 report shows the seeded
//     entity at its STARTING province (before it acts), while the turn-2 report,
//     generated after processing the move, shows it at the MOVED province.
//
// The loop is: create → world → starting provinces → player → advance 0→1
// (seeds a faction + entity) → report (turn 1) → submit orders (real + stubbed
// commands) → process turn 1 → advance 1→2 → report (turn 2). The submitted
// orders exercise both a real handler and the stub no-op: "move 2" (NE) and
// "hold" execute for real; "work" is recorded as a stub. 7+7+7=21 ticks, all
// completing within the 30 working ticks.
//
// See content/docs/reference/turn-processing.md and reports.md.
func TestSmokeEndToEnd(t *testing.T) {
	dir := t.TempDir()
	const gameID = "smoke-game"

	// create → the game manifest exists at turn 0.
	if _, _, err := captureErr(t, func() error {
		return createGame(dir, gameID, prng.New(1, 2))
	}); err != nil {
		t.Fatalf("createGame = %v, want nil", err)
	}
	if got := reloadGame(t, dir); got.Turn != 0 {
		t.Fatalf("after create, game.Turn = %d, want 0", got.Turn)
	}

	// world → a world.json of 3 rings is written.
	if _, _, err := captureErr(t, func() error {
		return generateWorld(3, dir)
	}); err != nil {
		t.Fatalf("generateWorld = %v, want nil", err)
	}

	// starting provinces → the ring-1 default set, which includes (1,-1).
	if _, _, err := captureErr(t, func() error {
		return generateStartingProvinces(dir, 1, false)
	}); err != nil {
		t.Fatalf("generateStartingProvinces = %v, want nil", err)
	}
	set, err := loadStartingProvinces(filepath.Join(dir, "starting-provinces.json"))
	if err != nil {
		t.Fatalf("loadStartingProvinces = %v, want nil", err)
	}
	const startProvince = "(1,-1)"
	if !set.Contains(startProvince) {
		t.Fatalf("generated starting provinces %v do not contain %q", set.List(), startProvince)
	}

	// player → placed on the starting province.
	if _, _, err := captureErr(t, func() error {
		return createPlayer(dir, "smoke@example.com", "smoker", startProvince)
	}); err != nil {
		t.Fatalf("createPlayer = %v, want nil", err)
	}
	players, err := loadPlayers(filepath.Join(dir, "players.json"))
	if err != nil {
		t.Fatalf("loadPlayers = %v, want nil", err)
	}
	player, ok := players.ByHandle("smoker")
	if !ok {
		t.Fatal("player 'smoker' missing from players.json")
	}

	// advance 0→1 → begins play and seeds each active player a faction and a
	// starting entity at their starting province.
	if _, _, err := captureErr(t, func() error { return advanceTurn(dir) }); err != nil {
		t.Fatalf("advanceTurn 0→1 = %v, want nil", err)
	}
	if got := reloadGame(t, dir); got.Turn != 1 {
		t.Fatalf("after first advance, game.Turn = %d, want 1", got.Turn)
	}
	factions := reloadFactions(t, dir)
	if len(factions.Factions) != 1 {
		t.Fatalf("seeded factions = %d, want 1", len(factions.Factions))
	}
	if c := factions.Factions[0].Controller; c.Kind != tpty.ControllerPlayer || c.ID != player.ID {
		t.Fatalf("seeded faction controller = %+v, want player %d", c, player.ID)
	}
	entities := reloadEntities(t, dir)
	if len(entities.Entities) != 1 {
		t.Fatalf("seeded entities = %d, want 1", len(entities.Entities))
	}
	if loc := entities.Entities[0].Location; loc != startProvince {
		t.Fatalf("seeded entity location = %q, want the starting province %q", loc, startProvince)
	}
	entityID := entities.Entities[0].ID

	// report (turn 1) → generated before processing, so it shows the entity at
	// its STARTING province. Keep this location for the later comparison.
	if _, _, err := captureErr(t, func() error { return writeReports(dir) }); err != nil {
		t.Fatalf("writeReports turn 1 = %v, want nil", err)
	}
	turn1Report := readReport(t, dir, 1, player.ID)
	if turn1Report.PlayerID != player.ID || turn1Report.PlayerHandle != player.Handle {
		t.Fatalf("turn-1 report player = (%d,%q), want (%d,%q)",
			turn1Report.PlayerID, turn1Report.PlayerHandle, player.ID, player.Handle)
	}
	if turn1Report.Turn != 1 {
		t.Fatalf("turn-1 report Turn = %d, want 1", turn1Report.Turn)
	}
	if len(turn1Report.Factions) != 1 || turn1Report.Factions[0].Controller.ID != player.ID {
		t.Fatalf("turn-1 report factions = %+v, want one controlled by player %d", turn1Report.Factions, player.ID)
	}
	if len(turn1Report.Entities) != 1 {
		t.Fatalf("turn-1 report entities = %d, want 1", len(turn1Report.Entities))
	}
	turn1Loc := turn1Report.Entities[0].Location
	if turn1Loc != startProvince {
		t.Fatalf("turn-1 report entity location = %q, want the starting province %q", turn1Loc, startProvince)
	}

	// The expected destination: one NE step from the starting province. NE is the
	// axial vector (+1,-1) (tpty.DirNE), matching direction 2 of the move command.
	movedHex := mustParseHex(t, startProvince).Add(tpty.DirNE)
	wantMoved := movedHex.String()

	// submit orders → a real + stubbed mix for the seeded entity. "move 2" (NE)
	// and "hold" run for real; "work" is a recorded stub no-op.
	raw := fmt.Sprintf("%q %d %q\n\nentity %d, \"Entity %d\"\n    move 2\n    hold\n    work\n",
		gameID, player.ID, player.Password, entityID, entityID)
	orderFile := filepath.Join(dir, "orders.txt")
	if err := os.WriteFile(orderFile, []byte(raw), 0o644); err != nil {
		t.Fatalf("write orders file: %v", err)
	}
	if _, _, err := captureErr(t, func() error { return submitOrders(dir, orderFile) }); err != nil {
		t.Fatalf("submitOrders = %v, want nil", err)
	}

	// process turn 1 → the TurnResult must carry BOTH an executed (non-stub) and a
	// stubbed outcome, and the entity's stored location must have changed (the
	// move applied).
	if _, _, err := captureErr(t, func() error { return processTurn(dir) }); err != nil {
		t.Fatalf("processTurn = %v, want nil", err)
	}
	result, err := tpty.LoadTurnResult(filepath.Join(dir, "turns"), 1)
	if err != nil {
		t.Fatalf("LoadTurnResult(turn 1) = %v, want nil", err)
	}
	var executed, stubbed int
	for _, o := range result.Outcomes {
		if o.Stub {
			stubbed++
		} else {
			executed++
		}
	}
	if executed == 0 {
		t.Fatalf("turn result has no executed (non-stub) outcome; outcomes = %+v", result.Outcomes)
	}
	if stubbed == 0 {
		t.Fatalf("turn result has no stub outcome; outcomes = %+v", result.Outcomes)
	}
	movedEntities := reloadEntities(t, dir)
	moved, ok := movedEntities.ByID(entityID)
	if !ok {
		t.Fatalf("entity %d missing from entities.json after processing", entityID)
	}
	if moved.Location == startProvince {
		t.Fatalf("entity location = %q, want it changed from the starting province after the move", moved.Location)
	}
	if moved.Location != wantMoved {
		t.Fatalf("entity location = %q, want the NE destination %q", moved.Location, wantMoved)
	}

	// advance 1→2 → the processed turn commits; no new seeding.
	if _, _, err := captureErr(t, func() error { return advanceTurn(dir) }); err != nil {
		t.Fatalf("advanceTurn 1→2 = %v, want nil", err)
	}
	if got := reloadGame(t, dir); got.Turn != 2 {
		t.Fatalf("after second advance, game.Turn = %d, want 2", got.Turn)
	}
	if f := reloadFactions(t, dir); len(f.Factions) != 1 {
		t.Fatalf("after 1→2 advance, factions = %d, want 1 (no re-seeding)", len(f.Factions))
	}
	if e := reloadEntities(t, dir); len(e.Entities) != 1 {
		t.Fatalf("after 1→2 advance, entities = %d, want 1 (no re-seeding)", len(e.Entities))
	}

	// report (turn 2) → now reflects the MOVED province, differing from turn 1.
	// This is the "reports reflect start-of-turn state" proof.
	if _, _, err := captureErr(t, func() error { return writeReports(dir) }); err != nil {
		t.Fatalf("writeReports turn 2 = %v, want nil", err)
	}
	turn2Report := readReport(t, dir, 2, player.ID)
	if turn2Report.Turn != 2 {
		t.Fatalf("turn-2 report Turn = %d, want 2", turn2Report.Turn)
	}
	if len(turn2Report.Entities) != 1 {
		t.Fatalf("turn-2 report entities = %d, want 1", len(turn2Report.Entities))
	}
	turn2Loc := turn2Report.Entities[0].Location
	if turn2Loc != wantMoved {
		t.Fatalf("turn-2 report entity location = %q, want the moved province %q", turn2Loc, wantMoved)
	}
	if turn2Loc == turn1Loc {
		t.Fatalf("turn-1 and turn-2 report locations both %q; reports must reflect start-of-turn state", turn1Loc)
	}
}

// readReport reads and decodes one player's report for a turn from the reports
// directory under dir.
func readReport(t *testing.T, dir string, turn, playerID int) tpty.Report {
	t.Helper()
	path := tpty.PlayerReportPath(filepath.Join(dir, "reports"), turn, playerID)
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report %s: %v", path, err)
	}
	var report tpty.Report
	if err := json.Unmarshal(buf, &report); err != nil {
		t.Fatalf("decode report %s: %v", path, err)
	}
	return report
}

// mustParseHex parses a canonical compact province string "(q,r)" into a Hex,
// failing the test on a malformed value.
func mustParseHex(t *testing.T, province string) tpty.Hex {
	t.Helper()
	var q, r int
	if _, err := fmt.Sscanf(province, "(%d,%d)", &q, &r); err != nil {
		t.Fatalf("parse province %q: %v", province, err)
	}
	return tpty.Hex{Q: q, R: r}
}
