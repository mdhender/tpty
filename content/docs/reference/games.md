---
title: Games
weight: 1
---

A **game** is the top-level unit of play. Everything else — the world, the
players — belongs to a game.

## Identity

Every game has an id and a code. It also has a pair of master seeds and tracks
its current turn.

### ID and code

- **`code`** — a short slug the GM chooses to name the game: uppercase letters
  and digits, 1–6 characters, unique. It is the first field of an orders file's
  opening record.
- **`id`** — a surrogate integer the engine assigns; every other record refers to
  the game by this id. See the
  [SQL Schema]({{< relref "/docs/reference/sql-schema.md#games" >}}).

### Master seeds

- Two `uint64` values, `seed1` and `seed2`, that make the game deterministic.
- They are the root of every random outcome in the game (see
  [Seeds and subsystems](#seeds-and-subsystems)), stored in `game_engine_state`.

### Current turn

- The turn the game is on now, `game_engine_state.current_turn`.
- A new game starts at turn `0` (setup — no turn); play begins at turn `1`. See
  [Turns]({{< relref "/docs/reference/turns.md" >}}).

## Storage

A game is stored across two tables (see the
[SQL Schema]({{< relref "/docs/reference/sql-schema.md" >}})): `games` holds its
identity, and `game_engine_state` holds the engine's per-game state — the master
seeds and current turn — kept separate from the identity row.

```json
{
  "games":             { "id": 3, "code": "SMOKE1" },
  "game_engine_state": { "game_id": 3, "seed1": 12345, "seed2": 67890, "current_turn": 0 }
}
```

The rest of a game's data — its world, players, factions, entities, submitted
orders, and processed-turn results — lives in its own tables, each scoped to the
game by `game_id`. Submitted orders are keyed by game, turn, and player; turn
results by game and turn. See
[Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}}).

## Seeds and subsystems

The game's master seeds are the root of all randomness. Each subsystem derives
its own master seeds from the game's, deterministically:

- The **world** derives its master seeds from the game's when it is generated;
  every terrain draw comes from the world's seeds. See
  [World Generation]({{< relref "/docs/reference/world-generation.md" >}}).
- Each **player** derives private seeds from the game's, keyed by handle. See
  [Players]({{< relref "/docs/reference/players.md" >}}).

A subsystem's derived seeds are stored with the subsystem's own data — the
world's in the `worlds` row, a player's in the `players` row — so the subsystem
carries everything it needs to reproduce its randomness on its own. This lets a
scenario be exercised without standing up a whole game, which is convenient when
writing and testing code.

## See also

- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
