// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestEntityStoreAssignsSequentialIDs(t *testing.T) {
	s := NewEntityStore()
	for i, name := range []string{"Alpha", "Bravo", "Charlie"} {
		e, err := s.Create(name, 1, "(0,0)")
		if err != nil {
			t.Fatalf("Create(%q): %v", name, err)
		}
		if want := i + 1; e.ID != want {
			t.Errorf("Create(%q) id = %d, want %d", name, e.ID, want)
		}
	}
	if s.NextID != 4 {
		t.Errorf("NextID = %d, want 4", s.NextID)
	}
}

func TestEntityStoreRejectsEmptyName(t *testing.T) {
	s := NewEntityStore()
	_, err := s.Create("", 1, "(0,0)")
	if !errors.Is(err, ErrInvalidEntityName) {
		t.Errorf("Create with empty name: err = %v, want ErrInvalidEntityName", err)
	}
}

// TestEntityStoreAllowsDuplicateNames confirms two entities may share a name:
// the name is only a label and need not be unique.
func TestEntityStoreAllowsDuplicateNames(t *testing.T) {
	s := NewEntityStore()
	if _, err := s.Create("Conan", 1, "(0,0)"); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	e, err := s.Create("Conan", 1, "(1,0)")
	if err != nil {
		t.Errorf("Create with duplicate name should be allowed: %v", err)
	}
	if e.ID != 2 {
		t.Errorf("second entity id = %d, want 2", e.ID)
	}
}

func TestEntityStoreRejectsInvalidFactionID(t *testing.T) {
	s := NewEntityStore()
	for _, factionID := range []int{0, -1} {
		if _, err := s.Create("Entity", factionID, "(0,0)"); !errors.Is(err, ErrInvalidFactionID) {
			t.Errorf("Create with faction id %d: err = %v, want ErrInvalidFactionID", factionID, err)
		}
	}
}

func TestEntityStoreRejectsInvalidLocation(t *testing.T) {
	s := NewEntityStore()
	for _, loc := range []string{"(0, 0)", "garbage", ""} {
		if _, err := s.Create("Entity", 1, loc); !errors.Is(err, ErrInvalidProvince) {
			t.Errorf("Create with location %q: err = %v, want ErrInvalidProvince", loc, err)
		}
	}
}

func TestEntityStoreLookups(t *testing.T) {
	s := NewEntityStore()
	a, err := s.Create("Aragorn", 1, "(-1,0)")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Create("Boromir", 2, "(0,0)")
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Create("Celeborn", 1, "(1,0)")
	if err != nil {
		t.Fatal(err)
	}

	if got, ok := s.ByID(b.ID); !ok || got != b {
		t.Errorf("ByID(%d) = (%+v, %v), want (%+v, true)", b.ID, got, ok, b)
	}
	if _, ok := s.ByID(999); ok {
		t.Error("ByID matched an unknown id")
	}

	got := s.ByFaction(1)
	if len(got) != 2 || got[0] != a || got[1] != c {
		t.Errorf("ByFaction(1) = %+v, want [%+v %+v]", got, a, c)
	}
	if len(s.ByFaction(3)) != 0 {
		t.Errorf("ByFaction(3) = %+v, want empty", s.ByFaction(3))
	}
}

// TestEntityStoreIDsSurviveReload confirms ids are not reused after a store is
// saved and reloaded: NextID persists, so the next id continues past the highest
// id ever assigned rather than restarting or reusing.
func TestEntityStoreIDsSurviveReload(t *testing.T) {
	s := NewEntityStore()
	for _, name := range []string{"Alpha", "Bravo", "Charlie"} {
		if _, err := s.Create(name, 1, "(0,0)"); err != nil {
			t.Fatal(err)
		}
	}

	buf, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded EntityStore
	if err := json.Unmarshal(buf, &reloaded); err != nil {
		t.Fatal(err)
	}

	// Simulate removing an entity, then creating a new one; the id must continue.
	reloaded.Entities = reloaded.Entities[:2]
	e, err := reloaded.Create("Delta", 1, "(0,0)")
	if err != nil {
		t.Fatal(err)
	}
	if e.ID != 4 {
		t.Errorf("id after reload = %d, want 4 (ids are never reused)", e.ID)
	}
}

// TestEntityJSONShape confirms the on-disk shape matches the reference example:
// {"id":...,"name":...,"factionId":...,"location":...}.
func TestEntityJSONShape(t *testing.T) {
	e := Entity{
		ID:        101,
		Name:      "Conan the Copyright",
		FactionID: 1,
		Location:  "(-1,0)",
	}
	buf, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]any
	if err := json.Unmarshal(buf, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["id"] != float64(101) {
		t.Errorf("id = %v, want 101", raw["id"])
	}
	if raw["name"] != "Conan the Copyright" {
		t.Errorf("name = %v, want %q", raw["name"], "Conan the Copyright")
	}
	if raw["factionId"] != float64(1) {
		t.Errorf("factionId = %v, want 1", raw["factionId"])
	}
	if raw["location"] != "(-1,0)" {
		t.Errorf("location = %v, want %q", raw["location"], "(-1,0)")
	}

	// Round-trip: unmarshaling back yields an identical entity.
	var got Entity
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatal(err)
	}
	if got != e {
		t.Errorf("round-trip changed the entity:\n got %+v\nwant %+v", got, e)
	}
}
