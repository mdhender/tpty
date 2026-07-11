// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mdhender/tpty/internal/orders"
)

// writeStoredOrders writes a StoredOrders file for the given player into the
// turn's order directory under ordersDir, the same way "orders submit" would.
func writeStoredOrders(t *testing.T, ordersDir string, turn, playerID int, raw string) {
	t.Helper()
	path := PlayerOrdersPath(ordersDir, turn, playerID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	buf, err := json.Marshal(StoredOrders{Turn: turn, PlayerID: playerID, Raw: raw})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestLoadTurnOrdersMissingDir verifies that a missing turn directory yields an
// empty slice and no error, not a failure.
func TestLoadTurnOrdersMissingDir(t *testing.T) {
	ordersDir := filepath.Join(t.TempDir(), "orders")
	got, err := LoadTurnOrders(ordersDir, 1)
	if err != nil {
		t.Fatalf("LoadTurnOrders(missing) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("LoadTurnOrders(missing) len = %d, want 0", len(got))
	}
}

// TestLoadTurnOrdersDeterministicOrder verifies that submissions are returned
// sorted by player id, independent of the order the files are written or read.
func TestLoadTurnOrdersDeterministicOrder(t *testing.T) {
	ordersDir := filepath.Join(t.TempDir(), "orders")
	// Write out of order to prove the load sorts by player id.
	writeStoredOrders(t, ordersDir, 1, 3, "three")
	writeStoredOrders(t, ordersDir, 1, 1, "one")
	writeStoredOrders(t, ordersDir, 1, 10, "ten")
	writeStoredOrders(t, ordersDir, 1, 2, "two")
	// A file that is not a player-orders name is skipped.
	if err := os.WriteFile(filepath.Join(OrdersTurnDir(ordersDir, 1), "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write stray file: %v", err)
	}

	got, err := LoadTurnOrders(ordersDir, 1)
	if err != nil {
		t.Fatalf("LoadTurnOrders error = %v, want nil", err)
	}
	wantIDs := []int{1, 2, 3, 10}
	if len(got) != len(wantIDs) {
		t.Fatalf("LoadTurnOrders len = %d, want %d", len(got), len(wantIDs))
	}
	for i, id := range wantIDs {
		if got[i].PlayerID != id {
			t.Errorf("got[%d].PlayerID = %d, want %d", i, got[i].PlayerID, id)
		}
	}
}

// TestTurnResultPath verifies the per-turn result path layout.
func TestTurnResultPath(t *testing.T) {
	got := filepath.ToSlash(TurnResultPath(filepath.FromSlash("/g/turns"), 7))
	if want := "/g/turns/turn-0007/result.json"; got != want {
		t.Errorf("TurnResultPath = %q, want %q", got, want)
	}
}

// TestTurnResultSaveLoadRoundTrip verifies that a TurnResult survives a
// save/load round-trip, carryover included.
func TestTurnResultSaveLoadRoundTrip(t *testing.T) {
	turnsDir := filepath.Join(t.TempDir(), "turns")
	want := TurnResult{
		Turn: 2,
		Outcomes: []OrderOutcome{
			{EntityID: 1, Order: orders.Order{ID: orders.CmdHold, Word: "hold"}, Stub: false, Message: "held"},
		},
		Carryover: []EntityQueue{
			{EntityID: 1, Orders: []orders.Order{{ID: orders.CmdMove, Word: "move", Args: []string{"1"}}}, Active: true, TicksLeft: 3},
		},
		Log: []string{"tick 0: setup"},
	}

	if err := SaveTurnResult(turnsDir, want); err != nil {
		t.Fatalf("SaveTurnResult: %v", err)
	}
	got, err := LoadTurnResult(turnsDir, 2)
	if err != nil {
		t.Fatalf("LoadTurnResult: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip changed TurnResult:\n got %+v\nwant %+v", got, want)
	}
}

// TestLoadTurnResultMissing verifies that a missing result file returns the
// ErrTurnNotProcessed sentinel, which callers treat as "not processed yet".
func TestLoadTurnResultMissing(t *testing.T) {
	turnsDir := filepath.Join(t.TempDir(), "turns")
	_, err := LoadTurnResult(turnsDir, 1)
	if !errors.Is(err, ErrTurnNotProcessed) {
		t.Errorf("LoadTurnResult(missing) error = %v, want ErrTurnNotProcessed", err)
	}
}
