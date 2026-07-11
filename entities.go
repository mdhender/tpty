// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
)

// EntityStore holds the entities of a single game. Ids are assigned sequentially
// in increasing order and are never reused within a game, so NextID is persisted
// and never decreases.
//
// See content/docs/reference/entities.md for the rules.
type EntityStore struct {
	NextID   int      `json:"next_id"`
	Entities []Entity `json:"entities"`
}

// NewEntityStore returns an empty store whose first assigned id will be 1.
func NewEntityStore() *EntityStore {
	return &EntityStore{NextID: 1}
}

// Create validates the name, faction id, and location, assigns the next
// sequential id, and adds a new entity to the store. On success it returns the
// created entity.
//
// Name is stored as given (its case preserved) and need not be unique. The
// faction id must be positive. The location must be a province coordinate in the
// canonical compact form "(q,r)"; it is stored in that canonical form.
func (s *EntityStore) Create(name string, factionID int, location string) (Entity, error) {
	if err := ValidateEntityName(name); err != nil {
		return Entity{}, err
	}
	if factionID < 1 {
		return Entity{}, fmt.Errorf("faction id %d: %w", factionID, ErrInvalidFactionID)
	}
	loc, err := ParseProvince(location)
	if err != nil {
		return Entity{}, err
	}

	if s.NextID < 1 {
		s.NextID = 1
	}
	e := Entity{
		ID:        s.NextID,
		Name:      name,
		FactionID: factionID,
		Location:  loc,
	}
	s.Entities = append(s.Entities, e)
	s.NextID++
	return e, nil
}

// SetLocation moves the entity with the given id to loc, canonicalizing loc via
// ParseProvince and storing it in canonical compact form "(q,r)". It returns
// ErrUnknownEntity if the store has no entity with that id, and the province
// validation error (ErrInvalidProvince) if loc is not a canonical coordinate.
// Grounded by content/docs/reference/entities.md (§Location: "An entity's
// location changes as it moves").
func (s *EntityStore) SetLocation(id int, loc string) error {
	canonical, err := ParseProvince(loc)
	if err != nil {
		return err
	}
	for i := range s.Entities {
		if s.Entities[i].ID == id {
			s.Entities[i].Location = canonical
			return nil
		}
	}
	return fmt.Errorf("entity %d: %w", id, ErrUnknownEntity)
}

// ByID returns the entity with the given id, if any.
func (s *EntityStore) ByID(id int) (Entity, bool) {
	for _, e := range s.Entities {
		if e.ID == id {
			return e, true
		}
	}
	return Entity{}, false
}

// ByFaction returns all entities currently belonging to the given faction.
func (s *EntityStore) ByFaction(factionID int) []Entity {
	var out []Entity
	for _, e := range s.Entities {
		if e.FactionID == factionID {
			out = append(out, e)
		}
	}
	return out
}
