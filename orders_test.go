// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mdhender/tpty/internal/orders"
	"github.com/mdhender/tpty/internal/prng"
)

// newOrdersFixture builds a game, two players, and their turn-1 factions and
// entities. Player 1 owns entity 1 (faction 1); player 2 owns entity 2
// (faction 2).
func newOrdersFixture(t *testing.T) (*Game, *PlayerStore, *FactionStore, *EntityStore) {
	t.Helper()
	g, err := NewGame("smoke-test-1", prng.Seeds{Seed1: 1, Seed2: 2})
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	players := NewPlayerStore()
	if _, err := players.Create(g.Seeds, "alice@x.com", "alice", "(0,0)"); err != nil {
		t.Fatalf("Create(alice): %v", err)
	}
	if _, err := players.Create(g.Seeds, "bob@x.com", "bob", "(1,0)"); err != nil {
		t.Fatalf("Create(bob): %v", err)
	}
	factions := NewFactionStore()
	entities := NewEntityStore()
	if _, err := SeedTurnOne(players, factions, entities); err != nil {
		t.Fatalf("SeedTurnOne: %v", err)
	}
	return g, players, factions, entities
}

// openingFor builds a parsed file with just an opening record for the given
// player.
func openingFor(gameID string, playerID int, password string) *orders.File {
	return &orders.File{Opening: &orders.OpeningRecord{
		GameID:   gameID,
		PlayerID: playerID,
		Password: password,
		Line:     1,
	}}
}

func TestAuthenticateOrdersSuccess(t *testing.T) {
	g, players, _, _ := newOrdersFixture(t)
	alice, _ := players.ByID(1)
	f := openingFor(g.ID, alice.ID, alice.Password)
	p, err := g.AuthenticateOrders(f, players)
	if err != nil {
		t.Fatalf("AuthenticateOrders: %v", err)
	}
	if p.ID != alice.ID {
		t.Errorf("authenticated player = %d, want %d", p.ID, alice.ID)
	}
}

func TestAuthenticateOrdersFailures(t *testing.T) {
	g, players, _, _ := newOrdersFixture(t)
	alice, _ := players.ByID(1)

	tests := []struct {
		name string
		file *orders.File
		want error
	}{
		{name: "no opening record", file: &orders.File{}, want: ErrOrdersNoOpeningRecord},
		{name: "nil file", file: nil, want: ErrOrdersNoOpeningRecord},
		{name: "game mismatch", file: openingFor("other-game", alice.ID, alice.Password), want: ErrOrdersGameMismatch},
		{name: "unknown player", file: openingFor(g.ID, 999, alice.Password), want: ErrOrdersUnknownPlayer},
		{name: "bad password", file: openingFor(g.ID, alice.ID, "wrong"), want: ErrOrdersBadPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := g.AuthenticateOrders(tt.file, players)
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestAuthenticateOrdersInactivePlayer(t *testing.T) {
	g, players, _, _ := newOrdersFixture(t)
	alice, _ := players.ByID(1)
	if _, err := players.Deactivate(alice.ID); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	f := openingFor(g.ID, alice.ID, alice.Password)
	_, err := g.AuthenticateOrders(f, players)
	if !errors.Is(err, ErrOrdersInactivePlayer) {
		t.Errorf("err = %v, want %v", err, ErrOrdersInactivePlayer)
	}
}

func TestCheckOrderOwnership(t *testing.T) {
	_, _, factions, entities := newOrdersFixture(t)

	// Entity 1 belongs to faction 1, controlled by player 1.
	f := &orders.File{Entities: []orders.EntityBlock{
		{EntityID: 1, Line: 3},
	}}
	if errs := CheckOrderOwnership(f, 1, factions, entities); len(errs) != 0 {
		t.Errorf("owned entity produced errors: %v", errs)
	}

	// Player 2 does not own entity 1.
	f = &orders.File{Entities: []orders.EntityBlock{
		{EntityID: 1, Line: 7},
	}}
	errs := CheckOrderOwnership(f, 2, factions, entities)
	if len(errs) != 1 || errs[0].Line != 7 || errs[0].Col != 1 {
		t.Fatalf("unowned entity errors = %v", errs)
	}
	if want := "entity 1 is not owned by you"; errs[0].Msg != want {
		t.Errorf("msg = %q, want %q", errs[0].Msg, want)
	}

	// A nonexistent entity is rejected.
	f = &orders.File{Entities: []orders.EntityBlock{
		{EntityID: 999, Line: 5},
	}}
	errs = CheckOrderOwnership(f, 1, factions, entities)
	if len(errs) != 1 || errs[0].Msg != "entity 999 does not exist" {
		t.Fatalf("nonexistent entity errors = %v", errs)
	}
}

func TestCheckOrderOwnershipReportsAll(t *testing.T) {
	_, _, factions, entities := newOrdersFixture(t)
	// Player 1 owns entity 1 but not entity 2, and entity 999 does not exist.
	f := &orders.File{Entities: []orders.EntityBlock{
		{EntityID: 1, Line: 3},
		{EntityID: 2, Line: 5},
		{EntityID: 999, Line: 7},
	}}
	errs := CheckOrderOwnership(f, 1, factions, entities)
	if len(errs) != 2 {
		t.Fatalf("errors = %v, want 2", errs)
	}
	// Confirm the friendly rendering carries line and column.
	if got := fmt.Sprint(errs[0].Error()); got != "5:1: entity 2 is not owned by you" {
		t.Errorf("errs[0] = %q", got)
	}
}
