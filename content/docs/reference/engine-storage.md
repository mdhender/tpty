---
title: Engine Storage
weight: 14
---

The engine can persist a game as a set of **JSON files** on disk — its local
store, on the engine side of the
[server/engine boundary]({{< relref "/docs/reference/sql-schema.md#the-server--engine-boundary" >}}).
This is distinct from the server's
[SQLite backend]({{< relref "/docs/reference/sql-schema.md" >}}); the two are the
engine-side (JSON) and server-side (SQLite) stores of the same game.

{{< callout type="info" >}}
The JSON store reflects the engine's current Go types (game-scoped ids, and the
field names `name`, `handle`, `email`, and a `location` string). These lag the
reconciled model used elsewhere in the reference (global ids, `display_name`).
This page describes the JSON files as they are written today; reconciling them to
the new model tracks with the broader engine reconciliation.
{{< /callout >}}

## The `game.json` manifest

A game is stored as a `game.json` manifest. It records the game's id, master
seeds, and current turn, and maps each of the game's data files to a location:

```json
{
  "id": "smoke-test-1",
  "seeds": { "seed1": 12345, "seed2": 67890 },
  "turn": 0,
  "files": {
    "world": "./world.json",
    "players": "./players.json",
    "factions": "./factions.json",
    "entities": "./entities.json",
    "orders": "./orders",
    "turns": "./turns",
    "reports": "./reports",
    "starting-provinces": "./starting-provinces.json",
    "terrain-translation": "./terrain-translation.json"
  }
}
```

- `id` is the game's slug; `seeds` are its two `uint64` master seeds; `turn` is
  the current turn (`0` at setup). See
  [Games]({{< relref "/docs/reference/games.md" >}}).
- Each file path is resolved relative to the directory that contains `game.json`.
- A path may point outside that directory, so two games can share a file (for
  example, a common `world.json`) while keeping the rest separate. This is a
  convenience for development and testing only; production games do not share
  data files.

## Data files

Each entry in the `files` map names where a part of the game's data lives.

- `world.json` — the generated [world]({{< relref "/docs/reference/world-generation.md" >}}):
  the world's derived master seeds, the ring count, and every province with its
  coordinates and terrain.
- `players.json` — the game's [players]({{< relref "/docs/reference/players.md" >}}),
  each with the private seeds derived for it.
- `factions.json` — the game's [factions]({{< relref "/docs/reference/factions.md" >}}).
- `entities.json` — the game's [entities]({{< relref "/docs/reference/entities.md" >}}).
- `starting-provinces.json` — the game's allowed
  [starting-province set]({{< relref "/docs/reference/world-generation.md#the-allowed-set" >}}),
  a JSON array of provinces in canonical compact `(q,r)` form:

  ```json
  [
    "(0,-2)",
    "(2,-2)",
    "(2,0)"
  ]
  ```

- `terrain-translation.json` — a map from each terrain name to its Worldographer
  tile name, used to import a generated world into Worldographer.

A subsystem's derived seeds are stored with the subsystem's own data — the
world's in `world.json`, a player's in the player record — so the subsystem
carries everything it needs to reproduce its randomness on its own. This lets a
scenario be exercised without standing up a whole game.

## Per-turn directories

Three entries in the `files` map are directories rather than single files, each
holding one entry per turn.

- `orders/` — a player's submitted orders, one file per turn per player, keyed by
  turn and player id. The raw submission text is kept verbatim (it is the source
  of truth and is re-parsed by the engine). See
  [Orders]({{< relref "/docs/reference/orders" >}}).
- `turns/` — the result of processing a turn, one subdirectory per turn (keyed by
  turn), holding that turn's processing output. See
  [Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}}).
- `reports/` — each player's turn [report]({{< relref "/docs/reference/reports.md" >}}),
  one subdirectory per turn (keyed by turn) holding one file per active player
  (keyed by player id). Reports are written in a structured JSON model; a
  human-readable presentation format is future work.

## See also

- [SQL Schema]({{< relref "/docs/reference/sql-schema.md" >}}) — the server-side
  SQLite store.
- [Games]({{< relref "/docs/reference/games.md" >}})
