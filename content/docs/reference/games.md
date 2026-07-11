---
title: Games
weight: 1
---

A **game** is the top-level unit of play. Everything else — the world, the
players — belongs to a game. A game is described by a manifest file,
`game.json`.

## Identity

Every game has an id and a pair of master seeds. It also tracks its current
turn.

### ID

- A short slug the GM chooses to name the game.
- Quoted text with the same character restrictions as a
  [password]({{< relref "/docs/reference/players.md#password" >}}): it contains
  no characters that require escaping in JSON and none that could be confused
  with an ASCII space.
- It is the first field of an orders file's opening record.

### Master seeds

- Two `uint64` values, `seed1` and `seed2`, that make the game deterministic.
- They are the root of every random outcome in the game (see
  [Seeds and subsystems](#seeds-and-subsystems)).

### Current turn

- The turn the game is on now, stored in the manifest as `turn`.
- A new game starts at turn `0` (setup — no turn); play begins at turn `1`. See
  [Turns]({{< relref "/docs/reference/turns.md" >}}).

## Manifest

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
    "starting-provinces": "./starting-provinces.json",
    "terrain-translation": "./terrain-translation.json"
  }
}
```

- Each file path is resolved relative to the directory that contains
  `game.json`.
- A path may point outside that directory, so two games can share a file (for
  example, a common `world.json`) while keeping the rest separate. This is a
  convenience for development and testing.

A player's submitted orders are stored under the `orders` directory, one file
per turn per player, keyed by turn and player id.

## Seeds and subsystems

The game's master seeds are the root of all randomness. Each subsystem derives
its own master seeds from the game's, deterministically:

- The **world** derives its master seeds from the game's when it is generated;
  every terrain draw comes from the world's seeds. See
  [World Generation]({{< relref "/docs/reference/world-generation.md" >}}).
- Each **player** derives private seeds from the game's, keyed by handle. See
  [Players]({{< relref "/docs/reference/players.md" >}}).

A subsystem's derived seeds are stored with the subsystem's own data — the
world's in `world.json`, a player's in the player record — so the subsystem
carries everything it needs to reproduce its randomness on its own. This lets a
scenario be exercised without standing up a whole game, which is convenient when
writing and testing code.

Because those seeds were derived from one game's master seeds, a data file
belongs to the game that created it. Sharing a data file between games is a
convenience for development and testing only; production games do not share data
files.

## See also

- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
