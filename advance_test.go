// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"testing"

	"github.com/mdhender/tpty/internal/prng"
)

// TestAdvanceTurnZeroToOneSeeds verifies that advancing from turn 0 to turn 1
// increments the turn, seeds each active player one faction and one starting
// entity in their starting province, and returns the seeded count.
func TestAdvanceTurnZeroToOneSeeds(t *testing.T) {
	seeds := prng.Seeds{Seed1: 1, Seed2: 2}
	game, err := NewGame("test-game", seeds)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	// game.Turn defaults to 0 (setup).

	players := NewPlayerStore()
	alice, err := players.Create(seeds, "a@x.com", "alice", "(1,-1)")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := players.Create(seeds, "b@x.com", "bob", "(0,1)")
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	factions := NewFactionStore()
	entities := NewEntityStore()

	seeded, err := AdvanceTurn(game, players, factions, entities)
	if err != nil {
		t.Fatalf("AdvanceTurn = %v, want nil", err)
	}
	if game.Turn != 1 {
		t.Errorf("game.Turn = %d, want 1", game.Turn)
	}
	if seeded != 2 {
		t.Errorf("seeded = %d, want 2", seeded)
	}
	if len(factions.Factions) != 2 {
		t.Errorf("factions = %d, want 2", len(factions.Factions))
	}
	if len(entities.Entities) != 2 {
		t.Errorf("entities = %d, want 2", len(entities.Entities))
	}

	// Each seeded entity sits in its player's starting province.
	for _, p := range []Player{alice, bob} {
		var located bool
		for _, e := range entities.Entities {
			f, ok := factions.ByID(e.FactionID)
			if !ok {
				t.Fatalf("entity %d references unknown faction %d", e.ID, e.FactionID)
			}
			if f.Controller.Kind == ControllerPlayer && f.Controller.ID == p.ID {
				located = true
				if e.Location != p.StartingProvince {
					t.Errorf("player %d entity location = %q, want %q", p.ID, e.Location, p.StartingProvince)
				}
			}
		}
		if !located {
			t.Errorf("player %d got no seeded entity", p.ID)
		}
	}
}

// TestAdvanceTurnOneToTwoSeedsNothing verifies that advancing a turn at or beyond
// turn 1 only increments the turn and seeds nothing.
func TestAdvanceTurnOneToTwoSeedsNothing(t *testing.T) {
	seeds := prng.Seeds{Seed1: 1, Seed2: 2}
	game, err := NewGame("test-game", seeds)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	game.Turn = 1

	players := NewPlayerStore()
	if _, err := players.Create(seeds, "a@x.com", "alice", "(1,-1)"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	factions := NewFactionStore()
	entities := NewEntityStore()

	seeded, err := AdvanceTurn(game, players, factions, entities)
	if err != nil {
		t.Fatalf("AdvanceTurn = %v, want nil", err)
	}
	if game.Turn != 2 {
		t.Errorf("game.Turn = %d, want 2", game.Turn)
	}
	if seeded != 0 {
		t.Errorf("seeded = %d, want 0", seeded)
	}
	if len(factions.Factions) != 0 || len(entities.Entities) != 0 {
		t.Errorf("stores changed: %d factions, %d entities, want 0/0", len(factions.Factions), len(entities.Entities))
	}
}

// TestAdvanceTurnSkipsInactivePlayers verifies that an inactive (removed) player
// is not seeded on the 0→1 transition.
func TestAdvanceTurnSkipsInactivePlayers(t *testing.T) {
	seeds := prng.Seeds{Seed1: 1, Seed2: 2}
	game, err := NewGame("test-game", seeds)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}

	players := NewPlayerStore()
	if _, err := players.Create(seeds, "a@x.com", "alice", "(1,-1)"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := players.Create(seeds, "b@x.com", "bob", "(0,1)")
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}
	if _, err := players.Deactivate(bob.ID); err != nil {
		t.Fatalf("deactivate bob: %v", err)
	}

	factions := NewFactionStore()
	entities := NewEntityStore()

	seeded, err := AdvanceTurn(game, players, factions, entities)
	if err != nil {
		t.Fatalf("AdvanceTurn = %v, want nil", err)
	}
	if seeded != 1 {
		t.Errorf("seeded = %d, want 1 (only the active player)", seeded)
	}
	// No faction may be controlled by the inactive player.
	for _, f := range factions.Factions {
		if f.Controller.Kind == ControllerPlayer && f.Controller.ID == bob.ID {
			t.Errorf("inactive player %d was seeded a faction", bob.ID)
		}
	}
}
