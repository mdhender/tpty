// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"

	"github.com/mdhender/tpty/internal/cerrs"
)

// Entity is an actor in a game's world. It occupies a province and carries out
// the orders of the faction that controls it. Entities are scoped to a game: an
// entity belongs to exactly one game and is identified within it by a sequential
// id. An entity records the faction it currently belongs to and the province it
// occupies.
//
// Name is a display label; it is required and non-empty, stored as entered with
// its case preserved, and need not be unique — an entity is identified by its
// id. FactionID is the id of the one faction the entity currently belongs to.
// Location is the province the entity occupies, in canonical compact form
// "(q,r)".
//
// See content/docs/reference/entities.md for the rules.
type Entity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	FactionID int    `json:"factionId"`
	Location  string `json:"location"`
}

// Errors returned when creating or validating an entity.
const (
	ErrInvalidEntityName = cerrs.Error("invalid entity name")
	ErrUnknownEntity     = cerrs.Error("unknown entity")
	ErrInvalidFactionID  = cerrs.Error("invalid faction id")
)

// ValidateEntityName reports whether name is well-formed, wrapping
// ErrInvalidEntityName if it is not. A name is required and non-empty. It is
// stored as entered, with its case preserved, so no trimming or normalization is
// applied here. Names need not be unique — the name is only a label.
func ValidateEntityName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required: %w", ErrInvalidEntityName)
	}
	return nil
}
