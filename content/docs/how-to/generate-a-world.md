---
title: Generate a world
weight: 2
---

Generating a world fills in the map for a game you have already created.

## Before you start

You need a game. If you have not created one yet:

```sh
tpty game create --id my-game --data path/to/data
```

See [Create a game]({{< relref "/docs/how-to/create-a-game.md" >}}).

## Generate the world

```sh
tpty world generate --rings 5 --data path/to/data
```

`world generate` reads the game's master seeds from `game.json` and writes two
files to the locations named in the manifest:

- `world.json` — the generated world
- `terrain-translation.json` — the terrain-to-Worldographer tile map used by
  [render]({{< relref "/docs/how-to/render-world-to-worldographer.md" >}})

## Options

- `--rings` (required) — the number of rings of provinces around the center.
  Must be greater than 0 and less than 100.
- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.

## Reproduce a world

The seeds come from the game, not from this command. A world is fully determined
by the game's master seeds and the ring count, so to recreate the same world,
[create the game]({{< relref "/docs/how-to/create-a-game.md" >}}) with the same
`--seed1` and `--seed2` and generate with the same `--rings`.

## See also

- [Create a game]({{< relref "/docs/how-to/create-a-game.md" >}})
- [Render a world to Worldographer]({{< relref "/docs/how-to/render-world-to-worldographer.md" >}})
- [World generation reference]({{< relref "/docs/reference/world-generation.md" >}})
