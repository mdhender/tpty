// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestPlayerOrdersPath(t *testing.T) {
	got := filepath.ToSlash(PlayerOrdersPath(filepath.FromSlash("/g/orders"), 1, 3))
	if want := "/g/orders/turn-0001/player-0003.json"; got != want {
		t.Errorf("PlayerOrdersPath = %q, want %q", got, want)
	}
}

func TestOrdersTurnDir(t *testing.T) {
	got := filepath.ToSlash(OrdersTurnDir(filepath.FromSlash("/g/orders"), 42))
	if want := "/g/orders/turn-0042"; got != want {
		t.Errorf("OrdersTurnDir = %q, want %q", got, want)
	}
	// Turns beyond four digits expand naturally.
	got = filepath.ToSlash(OrdersTurnDir(filepath.FromSlash("/g/orders"), 12345))
	if want := "/g/orders/turn-12345"; got != want {
		t.Errorf("OrdersTurnDir(12345) = %q, want %q", got, want)
	}
}

func TestPlayerOrdersFilenameRoundTrip(t *testing.T) {
	for _, id := range []int{1, 3, 42, 1000, 9999, 10000, 123456} {
		name := PlayerOrdersFilename(id)
		got, ok := ParsePlayerOrdersFilename(name)
		if !ok || got != id {
			t.Errorf("round-trip id %d: filename %q -> (%d, %v), want (%d, true)", id, name, got, ok, id)
		}
	}
}

func TestPlayerOrdersFilenameFormat(t *testing.T) {
	if got, want := PlayerOrdersFilename(3), "player-0003.json"; got != want {
		t.Errorf("PlayerOrdersFilename(3) = %q, want %q", got, want)
	}
}

func TestParsePlayerOrdersFilenameLeadingZeros(t *testing.T) {
	got, ok := ParsePlayerOrdersFilename("player-0003.json")
	if !ok || got != 3 {
		t.Errorf("ParsePlayerOrdersFilename(\"player-0003.json\") = (%d, %v), want (3, true)", got, ok)
	}
}

func TestParsePlayerOrdersFilenameRejectsBadNames(t *testing.T) {
	bad := []string{
		"player-.json",
		"player-x.json",
		"foo.json",
		"player-3.txt",
		"player--1.json",
		"player-+3.json",
		"player-0.json",
		"player-3.json.bak",
		"",
		"player-3",
	}
	for _, name := range bad {
		if got, ok := ParsePlayerOrdersFilename(name); ok {
			t.Errorf("ParsePlayerOrdersFilename(%q) = (%d, true), want (_, false)", name, got)
		}
	}
}

func TestStoredOrdersJSONRoundTrip(t *testing.T) {
	so := StoredOrders{Turn: 1, PlayerID: 3, Raw: "move (1,2) to (1,3)\n"}
	buf, err := json.Marshal(so)
	if err != nil {
		t.Fatal(err)
	}
	var got StoredOrders
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatal(err)
	}
	if got != so {
		t.Errorf("round-trip changed StoredOrders:\n got %+v\nwant %+v", got, so)
	}
	// The on-disk field names must be present.
	var raw map[string]any
	if err := json.Unmarshal(buf, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"turn", "player_id", "raw"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected a %q key in %s", key, buf)
		}
	}
}
