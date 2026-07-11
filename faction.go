// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"

	"github.com/mdhender/tpty/internal/cerrs"
)

// Faction is a group of entities under a single controller. Factions are scoped
// to a game: a faction belongs to exactly one game and is identified within it
// by a sequential id and a unique name. A faction accepts orders only from its
// controller; the entities it owns carry those orders out.
//
// See content/docs/reference/factions.md for the rules.
type Faction struct {
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	Controller Controller `json:"controller"`
}

// ControllerKind identifies the kind of a faction's controller: a player or an
// NPC. Player ids and NPC ids cannot be confused because the kind is recorded
// alongside the id.
//
// See content/docs/reference/factions.md.
type ControllerKind string

// The controller kinds. A player controls a faction by supplying its orders; an
// NPC is a computer agent that generates its faction's orders automatically.
const (
	ControllerPlayer ControllerKind = "player"
	ControllerNPC    ControllerKind = "npc"
)

// Controller is the sole source of a faction's orders: a kind (player or npc)
// and the controller's id within that kind.
//
// See content/docs/reference/factions.md.
type Controller struct {
	Kind ControllerKind `json:"kind"`
	ID   int            `json:"id"`
}

// Errors returned when creating or validating a faction.
const (
	ErrInvalidFactionName   = cerrs.Error("invalid faction name")
	ErrDuplicateFactionName = cerrs.Error("duplicate faction name")
	ErrUnknownFaction       = cerrs.Error("unknown faction")
	ErrInvalidController    = cerrs.Error("invalid controller")
)

// ValidateFactionName reports whether name is well-formed, wrapping
// ErrInvalidFactionName if it is not. A name is required and non-empty. It is
// stored as entered, with its case preserved, so no trimming or normalization is
// applied here.
func ValidateFactionName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required: %w", ErrInvalidFactionName)
	}
	return nil
}

// ValidateController reports whether c is a well-formed controller, wrapping
// ErrInvalidController if it is not. The kind must be player or npc, and the id
// must be positive.
func ValidateController(c Controller) error {
	switch c.Kind {
	case ControllerPlayer, ControllerNPC:
	default:
		return fmt.Errorf("%q: %w", c.Kind, ErrInvalidController)
	}
	if c.ID < 1 {
		return fmt.Errorf("controller id %d: %w", c.ID, ErrInvalidController)
	}
	return nil
}
