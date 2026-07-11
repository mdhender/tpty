// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
)

// FactionStore holds the factions of a single game. Ids are assigned
// sequentially in increasing order and are never reused within a game, so NextID
// is persisted and never decreases.
//
// See content/docs/reference/factions.md for the rules.
type FactionStore struct {
	NextID   int       `json:"next_id"`
	Factions []Faction `json:"factions"`
}

// NewFactionStore returns an empty store whose first assigned id will be 1.
func NewFactionStore() *FactionStore {
	return &FactionStore{NextID: 1}
}

// Create validates the name and controller, enforces name uniqueness within the
// game, assigns the next sequential id, and adds a new faction to the store. On
// success it returns the created faction.
//
// Name is stored as given (its case preserved) and must be unique within the
// game, compared exactly (case-sensitively). The controller kind must be player
// or npc, with a positive id.
func (s *FactionStore) Create(name string, controller Controller) (Faction, error) {
	if err := ValidateFactionName(name); err != nil {
		return Faction{}, err
	}
	if err := ValidateController(controller); err != nil {
		return Faction{}, err
	}

	if _, ok := s.ByName(name); ok {
		return Faction{}, fmt.Errorf("%q: %w", name, ErrDuplicateFactionName)
	}

	if s.NextID < 1 {
		s.NextID = 1
	}
	f := Faction{
		ID:         s.NextID,
		Name:       name,
		Controller: controller,
	}
	s.Factions = append(s.Factions, f)
	s.NextID++
	return f, nil
}

// ByID returns the faction with the given id, if any.
func (s *FactionStore) ByID(id int) (Faction, bool) {
	for _, f := range s.Factions {
		if f.ID == id {
			return f, true
		}
	}
	return Faction{}, false
}

// ByName returns the faction with the given name, compared exactly
// (case-sensitively).
func (s *FactionStore) ByName(name string) (Faction, bool) {
	for _, f := range s.Factions {
		if f.Name == name {
			return f, true
		}
	}
	return Faction{}, false
}
