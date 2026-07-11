// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
	"testing"
)

// newSeedTestPlayers returns a player store with three players created via the
// public Create path: two active and one deactivated. Provinces are distinct so
// each seeded entity's location can be checked against its player.
func newSeedTestPlayers(t *testing.T) *PlayerStore {
	t.Helper()
	s := NewPlayerStore()
	if _, err := s.Create(testSeeds, "alice@x.com", "alice", "(0,0)"); err != nil {
		t.Fatalf("Create(alice): %v", err)
	}
	if _, err := s.Create(testSeeds, "bob@x.com", "bob", "(1,0)"); err != nil {
		t.Fatalf("Create(bob): %v", err)
	}
	if _, err := s.Create(testSeeds, "carol@x.com", "carol", "(2,0)"); err != nil {
		t.Fatalf("Create(carol): %v", err)
	}
	// Deactivate carol (id 3); she must not be seeded.
	if _, err := s.Deactivate(3); err != nil {
		t.Fatalf("Deactivate(3): %v", err)
	}
	return s
}

func TestSeedTurnOneSeedsActivePlayers(t *testing.T) {
	players := newSeedTestPlayers(t) // alice(1), bob(2) active; carol(3) inactive
	factions := NewFactionStore()
	entities := NewEntityStore()

	n, err := SeedTurnOne(players, factions, entities)
	if err != nil {
		t.Fatalf("SeedTurnOne: %v", err)
	}

	// Two active players → two seeded (the inactive player is skipped).
	if n != 2 {
		t.Errorf("seeded count = %d, want 2", n)
	}
	if len(factions.Factions) != 2 {
		t.Fatalf("factions created = %d, want 2", len(factions.Factions))
	}
	if len(entities.Entities) != 2 {
		t.Fatalf("entities created = %d, want 2", len(entities.Entities))
	}

	// Expected seeding in ascending player-id order: alice then bob.
	type want struct {
		playerID    int
		factionName string
		entityName  string
		location    string
	}
	wants := []want{
		{playerID: 1, factionName: "Faction 1", entityName: "Entity 1", location: "(0,0)"},
		{playerID: 2, factionName: "Faction 2", entityName: "Entity 2", location: "(1,0)"},
	}
	for i, w := range wants {
		f := factions.Factions[i]
		if f.ID != i+1 {
			t.Errorf("faction[%d].ID = %d, want %d", i, f.ID, i+1)
		}
		if f.Name != w.factionName {
			t.Errorf("faction[%d].Name = %q, want %q", i, f.Name, w.factionName)
		}
		if f.Controller.Kind != ControllerPlayer {
			t.Errorf("faction[%d].Controller.Kind = %q, want %q", i, f.Controller.Kind, ControllerPlayer)
		}
		if f.Controller.ID != w.playerID {
			t.Errorf("faction[%d].Controller.ID = %d, want %d", i, f.Controller.ID, w.playerID)
		}

		e := entities.Entities[i]
		if e.ID != i+1 {
			t.Errorf("entity[%d].ID = %d, want %d", i, e.ID, i+1)
		}
		if e.Name != w.entityName {
			t.Errorf("entity[%d].Name = %q, want %q", i, e.Name, w.entityName)
		}
		if e.FactionID != f.ID {
			t.Errorf("entity[%d].FactionID = %d, want %d (its seeded faction)", i, e.FactionID, f.ID)
		}
		if e.Location != w.location {
			t.Errorf("entity[%d].Location = %q, want %q", i, e.Location, w.location)
		}
	}

	// The inactive player (carol, id 3) must own no faction or entity.
	for _, f := range factions.Factions {
		if f.Controller.ID == 3 {
			t.Errorf("inactive player 3 was seeded a faction: %+v", f)
		}
	}
}

func TestSeedTurnOneEmptyStore(t *testing.T) {
	players := NewPlayerStore()
	factions := NewFactionStore()
	entities := NewEntityStore()

	n, err := SeedTurnOne(players, factions, entities)
	if err != nil {
		t.Fatalf("SeedTurnOne: %v", err)
	}
	if n != 0 {
		t.Errorf("seeded count = %d, want 0", n)
	}
	if len(factions.Factions) != 0 {
		t.Errorf("factions created = %d, want 0", len(factions.Factions))
	}
	if len(entities.Entities) != 0 {
		t.Errorf("entities created = %d, want 0", len(entities.Entities))
	}
}

// TestSeedTurnOneOrderAndNames confirms placeholder names track ids across a run
// of all-active players, seeded in ascending id order.
func TestSeedTurnOneOrderAndNames(t *testing.T) {
	players := NewPlayerStore()
	handles := []struct{ email, handle, province string }{
		{"a@x.com", "aa", "(0,0)"},
		{"b@x.com", "bb", "(1,0)"},
		{"c@x.com", "cc", "(2,0)"},
	}
	for _, h := range handles {
		if _, err := players.Create(testSeeds, h.email, h.handle, h.province); err != nil {
			t.Fatalf("Create(%q): %v", h.handle, err)
		}
	}

	factions := NewFactionStore()
	entities := NewEntityStore()
	n, err := SeedTurnOne(players, factions, entities)
	if err != nil {
		t.Fatalf("SeedTurnOne: %v", err)
	}
	if n != 3 {
		t.Fatalf("seeded count = %d, want 3", n)
	}

	for i := range players.Players {
		f := factions.Factions[i]
		e := entities.Entities[i]
		if wantName := fmt.Sprintf("Faction %d", i+1); f.Name != wantName {
			t.Errorf("faction[%d].Name = %q, want %q", i, f.Name, wantName)
		}
		if wantName := fmt.Sprintf("Entity %d", i+1); e.Name != wantName {
			t.Errorf("entity[%d].Name = %q, want %q", i, e.Name, wantName)
		}
		if f.Controller.ID != players.Players[i].ID {
			t.Errorf("faction[%d].Controller.ID = %d, want %d", i, f.Controller.ID, players.Players[i].ID)
		}
	}
}
