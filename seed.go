// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
)

// SeedTurnOne creates, for each active player, one player-controlled faction
// and one starting entity located in the player's starting province, appending
// them to the given stores. It returns the number of players seeded.
//
// This is the engine's turn-1 seeding step: as a game advances into turn 1,
// every player that enters play is given a single faction and one starting
// entity in their starting province (see
// content/docs/reference/entities.md, "Creation";
// content/docs/reference/factions.md, "Player-controlled factions"; and
// content/docs/reference/turns.md). The entity is located in the player's
// starting province (see content/docs/reference/players.md).
//
// Only active players are seeded: a player with Active() false (removed) gets
// neither a faction nor an entity. Players are visited in players.Players slice
// order, which is ascending id order, so the ids assigned to the seeded
// factions and entities are deterministic and independent of any map ordering
// (no PRNG is used here — see the determinism section of CLAUDE.md).
//
// Seeded factions and entities are given deterministic placeholder names until
// a naming generator exists: the faction is named "Faction <id>" and the entity
// "Entity <id>", each using its own newly-assigned id. The name is formed from
// the store's NextID immediately before Create, which assigns that id, so the
// name always matches the assigned id.
//
// The faction's controller is {Kind: ControllerPlayer, ID: <player id>}. The
// entity's FactionID is the seeded faction's id.
//
// SeedTurnOne is intended to be called exactly once, when the game advances into
// turn 1; enforcing that it runs only once is the advance command's job (see
// BURNDOWN items 17/18), not this function's.
//
// If a Create fails for any player, SeedTurnOne stops and returns the number of
// players seeded so far along with an error identifying the player.
func SeedTurnOne(players *PlayerStore, factions *FactionStore, entities *EntityStore) (int, error) {
	seeded := 0
	for _, p := range players.Players {
		if !p.Active() {
			continue
		}

		factionName := fmt.Sprintf("Faction %d", factions.NextID)
		f, err := factions.Create(factionName, Controller{Kind: ControllerPlayer, ID: p.ID})
		if err != nil {
			return seeded, fmt.Errorf("player %d (%q): create faction: %w", p.ID, p.Handle, err)
		}

		entityName := fmt.Sprintf("Entity %d", entities.NextID)
		if _, err := entities.Create(entityName, f.ID, p.StartingProvince); err != nil {
			return seeded, fmt.Errorf("player %d (%q): create entity: %w", p.ID, p.Handle, err)
		}

		seeded++
	}
	return seeded, nil
}
