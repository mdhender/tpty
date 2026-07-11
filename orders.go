// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"

	"github.com/mdhender/tpty/internal/cerrs"
	"github.com/mdhender/tpty/internal/orders"
)

// Errors returned when authenticating an orders file's opening record. Each is a
// distinct sentinel so callers can distinguish the failures.
//
// See content/docs/reference/orders/_index.md, "Authentication".
const (
	ErrOrdersNoOpeningRecord = cerrs.Error("orders file has no valid opening record")
	ErrOrdersGameMismatch    = cerrs.Error("orders file is for a different game")
	ErrOrdersUnknownPlayer   = cerrs.Error("orders file names an unknown player")
	ErrOrdersBadPassword     = cerrs.Error("orders file password does not match")
	ErrOrdersInactivePlayer  = cerrs.Error("orders file names an inactive player")
)

// AuthenticateOrders validates a parsed orders file's opening record against
// this game and its players, returning the authenticated player. A missing or
// failing opening record rejects the file in full.
//
// The checks run in order: the file must have a valid opening record; its game
// id must match this game; the player id must name a player in the game; the
// password must match exactly; and the player must be active. Each failure
// returns a distinct sentinel error wrapped with a descriptive message.
//
// See content/docs/reference/orders/_index.md, "Authentication".
func (g *Game) AuthenticateOrders(f *orders.File, players *PlayerStore) (Player, error) {
	if f == nil || f.Opening == nil {
		return Player{}, ErrOrdersNoOpeningRecord
	}
	op := f.Opening
	if op.GameID != g.ID {
		return Player{}, fmt.Errorf("orders game %q, this game %q: %w", op.GameID, g.ID, ErrOrdersGameMismatch)
	}
	p, ok := players.ByID(op.PlayerID)
	if !ok {
		return Player{}, fmt.Errorf("player %d: %w", op.PlayerID, ErrOrdersUnknownPlayer)
	}
	if p.Password != op.Password {
		return Player{}, fmt.Errorf("player %d: %w", op.PlayerID, ErrOrdersBadPassword)
	}
	if !p.Active() {
		return Player{}, fmt.Errorf("player %d: %w", op.PlayerID, ErrOrdersInactivePlayer)
	}
	return p, nil
}

// CheckOrderOwnership returns a friendly error for every entity block whose
// entity is not owned by the authenticated player through one of that player's
// factions. Blocks the player does own produce no error. Each error carries the
// entity header's line and column 1.
//
// An entity that does not exist, or one whose faction is not player-controlled
// by this player, is rejected. The returned errors use the same friendly form
// as the parser's own errors.
//
// See content/docs/reference/orders/_index.md, "Ownership".
func CheckOrderOwnership(f *orders.File, playerID int, factions *FactionStore, entities *EntityStore) []orders.Error {
	if f == nil {
		return nil
	}
	var errs []orders.Error
	for _, block := range f.Entities {
		e, ok := entities.ByID(block.EntityID)
		if !ok {
			errs = append(errs, orders.Error{Line: block.Line, Col: 1,
				Msg: fmt.Sprintf("entity %d does not exist", block.EntityID)})
			continue
		}
		fac, ok := factions.ByID(e.FactionID)
		if !ok || fac.Controller.Kind != ControllerPlayer || fac.Controller.ID != playerID {
			errs = append(errs, orders.Error{Line: block.Line, Col: 1,
				Msg: fmt.Sprintf("entity %d is not owned by you", block.EntityID)})
		}
	}
	return errs
}
