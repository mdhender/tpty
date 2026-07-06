---
title: Create a game
weight: 1
---

A game is the top-level unit of play; the world and the players belong to it.
Creating a game is the first step — it writes the `game.json` manifest that every
other command reads.

```sh
tpty game create --game-id my-game --data path/to/data
```

This writes `game.json` into the data directory. It records the game's id and a
pair of master seeds, and names the game's other data files (`world.json`,
`players.json`, `starting-provinces.json`, and `terrain-translation.json`).

## Options

- `--game-id` (required) — a slug naming the game. Any printable ASCII character
  is allowed except a space, a double quote, or a backslash. May also be supplied
  as the `TPTY_GAME_ID` environment variable.
- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.
- `--seed1`, `--seed2` — the two master seeds. If omitted (or 0) they are chosen
  at random and printed. The same seeds always produce the same game, so record
  them if you want to recreate it.

`game create` will not overwrite an existing game. To start over, remove
`game.json` or use a different data directory.

## Next steps

- [Generate a world]({{< relref "/docs/how-to/generate-a-world.md" >}}) for the game.
- [Recruit players and add them to a game]({{< relref "/docs/how-to/recruit-players.md" >}}).

## See also

- [Games reference]({{< relref "/docs/reference/games.md" >}})
