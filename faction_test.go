// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"errors"
	"testing"
)

// newTestFactionStore returns a store with one faction already created, for
// tests that exercise uniqueness and id assignment.
func newTestFactionStore(t *testing.T) *FactionStore {
	t.Helper()
	s := NewFactionStore()
	if _, err := s.Create("The Slaves of Darkness", Controller{Kind: ControllerPlayer, ID: 3}); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	return s
}

func TestFactionStoreAssignsSequentialIDs(t *testing.T) {
	s := NewFactionStore()
	for i, name := range []string{"Alpha", "Bravo", "Charlie"} {
		f, err := s.Create(name, Controller{Kind: ControllerPlayer, ID: i + 1})
		if err != nil {
			t.Fatalf("Create(%q): %v", name, err)
		}
		if want := i + 1; f.ID != want {
			t.Errorf("Create(%q) id = %d, want %d", name, f.ID, want)
		}
	}
	if s.NextID != 4 {
		t.Errorf("NextID = %d, want 4", s.NextID)
	}
}

func TestFactionStoreRejectsDuplicateName(t *testing.T) {
	s := newTestFactionStore(t)
	_, err := s.Create("The Slaves of Darkness", Controller{Kind: ControllerNPC, ID: 1})
	if !errors.Is(err, ErrDuplicateFactionName) {
		t.Errorf("Create with duplicate name: err = %v, want ErrDuplicateFactionName", err)
	}
}

func TestFactionStoreNameUniquenessIsCaseSensitive(t *testing.T) {
	s := newTestFactionStore(t) // has "The Slaves of Darkness"
	// A differently-cased name is not a duplicate; comparison is case-sensitive.
	if _, err := s.Create("the slaves of darkness", Controller{Kind: ControllerPlayer, ID: 4}); err != nil {
		t.Errorf("Create with differently-cased name should be allowed: %v", err)
	}
}

func TestFactionStoreRejectsEmptyName(t *testing.T) {
	s := NewFactionStore()
	_, err := s.Create("", Controller{Kind: ControllerPlayer, ID: 1})
	if !errors.Is(err, ErrInvalidFactionName) {
		t.Errorf("Create with empty name: err = %v, want ErrInvalidFactionName", err)
	}
}

func TestFactionStoreRejectsInvalidController(t *testing.T) {
	s := NewFactionStore()
	cases := []Controller{
		{Kind: "", ID: 1},               // missing kind
		{Kind: "gm", ID: 1},             // unknown kind
		{Kind: ControllerPlayer, ID: 0}, // non-positive id
		{Kind: ControllerNPC, ID: -1},   // negative id
	}
	for _, c := range cases {
		if _, err := s.Create("Faction", c); !errors.Is(err, ErrInvalidController) {
			t.Errorf("Create with controller %+v: err = %v, want ErrInvalidController", c, err)
		}
	}
}

func TestFactionStoreLookups(t *testing.T) {
	s := NewFactionStore()
	created, err := s.Create("The Wild Tribes", Controller{Kind: ControllerNPC, ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := s.ByID(created.ID); !ok || got != created {
		t.Errorf("ByID(%d) = (%+v, %v), want (%+v, true)", created.ID, got, ok, created)
	}
	if got, ok := s.ByName("The Wild Tribes"); !ok || got != created {
		t.Errorf("ByName = (%+v, %v), want (%+v, true)", got, ok, created)
	}
	// Case-sensitive: a differently-cased name does not match.
	if _, ok := s.ByName("the wild tribes"); ok {
		t.Error("ByName matched a differently-cased name; comparison must be case-sensitive")
	}
	if _, ok := s.ByID(999); ok {
		t.Error("ByID matched an unknown id")
	}
}

// TestFactionStoreIDsSurviveReload confirms ids are not reused after a store is
// saved and reloaded: NextID persists, so the next id continues past the highest
// id ever assigned rather than restarting or reusing.
func TestFactionStoreIDsSurviveReload(t *testing.T) {
	s := NewFactionStore()
	for _, name := range []string{"Alpha", "Bravo", "Charlie"} {
		if _, err := s.Create(name, Controller{Kind: ControllerPlayer, ID: 1}); err != nil {
			t.Fatal(err)
		}
	}

	buf, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded FactionStore
	if err := json.Unmarshal(buf, &reloaded); err != nil {
		t.Fatal(err)
	}

	// Simulate removing a faction, then creating a new one; the id must continue.
	reloaded.Factions = reloaded.Factions[:2]
	f, err := reloaded.Create("Delta", Controller{Kind: ControllerNPC, ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if f.ID != 4 {
		t.Errorf("id after reload = %d, want 4 (ids are never reused)", f.ID)
	}
}

// TestFactionJSONShape confirms the on-disk shape matches the reference example:
// the controller serializes as {"kind":...,"id":...}.
func TestFactionJSONShape(t *testing.T) {
	f := Faction{
		ID:         1,
		Name:       "The Slaves of Darkness",
		Controller: Controller{Kind: ControllerPlayer, ID: 3},
	}
	buf, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]any
	if err := json.Unmarshal(buf, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["id"] != float64(1) {
		t.Errorf("id = %v, want 1", raw["id"])
	}
	if raw["name"] != "The Slaves of Darkness" {
		t.Errorf("name = %v, want %q", raw["name"], "The Slaves of Darkness")
	}
	controller, ok := raw["controller"].(map[string]any)
	if !ok {
		t.Fatalf("controller is not an object: %s", buf)
	}
	if controller["kind"] != "player" {
		t.Errorf("controller.kind = %v, want %q", controller["kind"], "player")
	}
	if controller["id"] != float64(3) {
		t.Errorf("controller.id = %v, want 3", controller["id"])
	}

	// Round-trip: unmarshaling back yields an identical faction.
	var got Faction
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatal(err)
	}
	if got != f {
		t.Errorf("round-trip changed the faction:\n got %+v\nwant %+v", got, f)
	}
}
