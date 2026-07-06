---
title: Generate a world
weight: 1
---

Generating a world creates a new game — it is the only way to start one.

```sh
tpty world generate --rings 5 --data path/to/data --seed1 7 --seed2 13
```

This writes two files into the data directory:

- `world.json` — the game
- `terrain-translation.json` — the terrain-to-Worldographer tile map used by
  [render]({{< ref "/docs/how-to/render-world-to-worldographer.md" >}})

## Options

- `--rings` (required) — the number of rings of provinces around the center.
  Must be greater than 0 and less than 100.
- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.
- `--seed1`, `--seed2` — the two master seeds. If omitted (or 0) they are chosen
  at random and printed. The same seeds always produce the same world.

## Reproduce a world

Generation prints the seeds it used:

```
seeds: seed1=7 seed2=13
```

To recreate the same world, pass those seeds back with `--seed1` and `--seed2`.
A world is fully determined by its seeds and ring count, so nothing else needs to
be saved.

## See also

- [Render a world to Worldographer]({{< ref "/docs/how-to/render-world-to-worldographer.md" >}})
- [World generation reference]({{< ref "/docs/reference/world-generation.md" >}})
