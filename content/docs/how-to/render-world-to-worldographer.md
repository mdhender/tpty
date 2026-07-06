---
title: Render a world to Worldographer
weight: 2
---

This guide renders a generated world to a [Worldographer](https://worldographer.com)
map you can open and edit.

## Before you start

You need a generated world in your data directory. Generating a world writes two
files that `render` reads:

- `world.json` — the world itself
- `terrain-translation.json` — the map from each terrain to a Worldographer tile

If you have not generated one yet:

```sh
tpty world generate --rings 3 --data path/to/data --seed1 7 --seed2 13
```

## Render the world

```sh
tpty world render --data path/to/data
```

This writes `world.wxx` into the data directory. Open it in Worldographer:

```sh
open path/to/data/world.wxx
```

The origin province is at the center of the map, and the world fills a hex disc;
the rectangular corners around it are blank tiles.

{{< figure src="/images/rendered-world.jpg" alt="A three-ring world in Worldographer: a mountain at the center, surrounded by three rings of forest, plains, hills, desert, badlands, and water, with blank tiles filling the rectangular corners" caption="A three-ring world (seeds 7 and 13), generated and rendered with the commands above." >}}

## Change how a terrain looks

Each terrain is drawn with the Worldographer tile named in
`terrain-translation.json`. To use a different tile, edit that file and render
again. For example, to draw lakes as a lighter water tile:

```json
{
  "Lake": "Classic/Water Shallow"
}
```

Use any tile name from Worldographer's tile set.

## If render reports a missing translation

`render` stops with an error if a terrain has no entry in
`terrain-translation.json`:

```
tpty: no translation for terrain(s): [Swamp]
```

Add the missing terrain to `terrain-translation.json` with the Worldographer
tile to use for it, then render again.

## See also

- [World generation reference]({{< ref "/docs/reference/world-generation.md" >}}) — the terrains and coordinate system
