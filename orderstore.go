// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// StoredOrders is one player's submitted orders for one turn, as persisted under
// the game's orders directory. Raw is the orders file text exactly as the player
// submitted it, kept verbatim so it can be re-parsed by internal/orders (the
// parser may evolve; the submission is the source of truth). See
// content/docs/reference/orders/_index.md and content/docs/reference/games.md.
type StoredOrders struct {
	Turn     int    `json:"turn"`
	PlayerID int    `json:"player_id"`
	Raw      string `json:"raw"`
}

// OrdersTurnDir returns the directory, under the game's orders directory, that
// holds every player's submitted orders for the given turn. The turn number is
// zero-padded to four digits ("turn-0001") as a cosmetic sort aid; turns beyond
// four digits expand naturally.
func OrdersTurnDir(ordersDir string, turn int) string {
	return filepath.Join(ordersDir, fmt.Sprintf("turn-%04d", turn))
}

// PlayerOrdersPath returns the path, under the game's orders directory, of the
// file holding the given player's submitted orders for the given turn. It joins
// OrdersTurnDir with PlayerOrdersFilename, so orders are keyed by turn and player
// id: one file per turn per player.
func PlayerOrdersPath(ordersDir string, turn, playerID int) string {
	return filepath.Join(OrdersTurnDir(ordersDir, turn), PlayerOrdersFilename(playerID))
}

// PlayerOrdersFilename returns the base filename holding a single player's
// submitted orders for a turn. The player id is zero-padded to four digits
// ("player-0003.json") as a cosmetic sort aid; ids beyond four digits expand
// naturally. ParsePlayerOrdersFilename is the inverse.
func PlayerOrdersFilename(playerID int) string {
	return fmt.Sprintf("player-%04d.json", playerID)
}

// ParsePlayerOrdersFilename parses a player-orders base filename back to its
// player id. It is the inverse of PlayerOrdersFilename: for any id >= 1,
// ParsePlayerOrdersFilename(PlayerOrdersFilename(id)) returns id and true.
//
// The name must match the exact "player-<n>.json" shape, where <n> is one or
// more decimal digits denoting a positive id (leading zeros are allowed, e.g.
// "player-0003.json" yields 3). Any other name — a missing prefix or suffix, an
// empty, non-digit, or non-positive middle — is rejected with (0, false).
func ParsePlayerOrdersFilename(name string) (playerID int, ok bool) {
	const prefix, suffix = "player-", ".json"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return 0, false
	}
	digits := name[len(prefix) : len(name)-len(suffix)]
	if digits == "" {
		return 0, false
	}
	// strconv.Atoi accepts a leading sign; reject anything that is not purely
	// digits so "player--1.json" and "player-+3.json" do not slip through.
	for _, r := range digits {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	id, err := strconv.Atoi(digits)
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}
