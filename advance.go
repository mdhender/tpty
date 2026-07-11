// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

// AdvanceTurn advances the game from its current turn N to turn N+1 and, on the
// setup→play transition (0→1), seeds each active player their starting faction
// and entity. It returns the number of players seeded (0 for every transition
// other than 0→1).
//
// Advancing is the GM's "commit this turn, move on" action: step 4 of the
// per-turn lifecycle, where the GM advances the game to turn N+1 (see
// content/docs/reference/turns.md, "Per-turn lifecycle"). AdvanceTurn performs
// only the in-memory state change; it does no I/O. The "refuse to advance an
// unprocessed turn" guard reads the turn's result file and so lives in the
// command layer, not here.
//
// When the new turn is 1 — the one-time transition out of setup and into play —
// AdvanceTurn calls SeedTurnOne to give each active player one faction and one
// starting entity in their starting province (see
// content/docs/reference/entities.md, content/docs/reference/factions.md, and
// SeedTurnOne). Because 0→1 happens exactly once in a game's life, the seeding
// runs exactly once. Any other transition (N→N+1 for N≥1) seeds nothing.
//
// On a seeding error the game turn has already been incremented, but the stores
// are left as SeedTurnOne left them and the seeded count reflects how far it got.
func AdvanceTurn(game *Game, players *PlayerStore, factions *FactionStore, entities *EntityStore) (seeded int, err error) {
	game.Turn++
	if game.Turn == 1 {
		return SeedTurnOne(players, factions, entities)
	}
	return 0, nil
}
