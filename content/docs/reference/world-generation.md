---
title: World Generation
weight: 3
---

The world is a hexagonal grid. Each hex is a **province** and is assigned
exactly one terrain. The generator produces the grid deterministically from the
game's master seeds.

## Grid

- The grid uses **flat-top** hexes, with north toward the top of the map.
- The **origin** is the center hex, at axial coordinates `(0, 0)`.
- The grid grows outward from the origin in concentric **rings**. Ring `0` is
  the origin. Ring `k` (for `k > 0`) contains `6k` provinces.
- The grid may expand outward beyond the generated rings as the world is
  explored.

## Coordinates

Coordinates are reported using **axial** coordinates `(q, r)`. The origin is
`(0, 0)`.

Wherever coordinates are written as data — in files, as program input and
output, and in messages exchanged about a game — a province's coordinates use a
**compact** form with no spaces, for example `(-1,0)` and `(0,0)`. The spaced
form `(q, r)` used elsewhere in this documentation is for readability only.

The six neighbor directions, in clockwise order from north:

| Direction      | Axial `(q, r)` |
|----------------|----------------|
| North (N)      | `(0, -1)`      |
| Northeast (NE) | `(+1, -1)`     |
| Southeast (SE) | `(+1, 0)`      |
| South (S)      | `(0, +1)`      |
| Southwest (SW) | `(-1, +1)`     |
| Northwest (NW) | `(-1, 0)`      |

Properties of the coordinate system:

- Opposite directions negate: `N = -S`, `NE = -SW`, `SE = -NW`.
- Increasing `r` moves **south**. Increasing `q` moves **southeast**.
- A flat-top grid has no due-east or due-west neighbor.

The hex directly north of the origin is `(0, -1)`; the hex `k` steps north of the
origin is `(0, -k)`.

## Provinces and tiles

- A **province** is a single hex. Every province has exactly one terrain.
- A **tile** refers to the permanent aspect of a province (for example, "an
  ocean tile").

## Terrain types

| Terrain  | Notes                                    |
|----------|------------------------------------------|
| Mountain | Assigned to the origin `(0, 0)` only.    |
| Plains   |                                          |
| Forests  |                                          |
| Desert   |                                          |
| Hills    |                                          |
| Lake     |                                          |
| Badlands |                                          |

## Determinism

The world is deterministic: the same seeds always produce the same world. When
the world is generated it derives its own master seeds from the game's (see
[Games]({{< relref "/docs/reference/games.md" >}})), and every terrain draw comes
from the world's seeds. For how seeds, streams, and terrain addresses work, see
[Determinism]({{< relref "/docs/reference/determinism.md" >}}).

## Generation

Input: the GM specifies the number of rings, `numberOfRings`, which must satisfy
`0 < numberOfRings < 100`.

The generator assigns terrain as follows:

1. The origin `(0, 0)` is assigned **Mountain**.
2. For each ring from `1` to `numberOfRings`:
   - Start at the province directly north of the origin, `(0, -ring)`.
   - Advance clockwise around the ring, stopping when the starting province is
     reached again.
   - For each province, roll `1d6` and assign terrain:

     | Roll | Terrain  |
     |------|----------|
     | 1    | Plains   |
     | 2    | Forests  |
     | 3    | Desert   |
     | 4    | Hills    |
     | 5    | Lake     |
     | 6    | Badlands |

A world of `n` rings contains `1 + 3n(n + 1)` provinces (the origin plus `6k`
provinces for each ring `k` from `1` to `n`). For example, `n = 1` yields 7
provinces, and `n = 2` yields 19.

## Starting provinces

A game has a set of **allowed starting provinces**: the provinces on which
players may be placed. A player's starting province must be one of them (see
[Players]({{< relref "/docs/reference/players.md" >}})). This is a hard
invariant that player creation depends on.

The set is a function of world geometry, so it is meaningful only once the world
exists. The default set is six provinces, chosen purely by geometry — terrain is
never consulted, and any province may be a starting province:

- Pick a **ring distance** `d` from the origin. The default is
  `d = ceil(numberOfRings / 2)` — halfway out, toward the outermost ring:

  | `numberOfRings` | default `d` |
  |-----------------|-------------|
  | 1               | 1           |
  | 2               | 1           |
  | 3               | 2           |
  | 4               | 2           |
  | 5               | 3           |
  | 6               | 3           |

  `d` may be chosen freely, but must satisfy `0 < d <= numberOfRings`.

- Place one province in each of the six flat-top directions, each at distance
  `d`: the direction vector scaled by `d`. The six are always listed in the
  deterministic order **N, NE, SE, S, SW, NW**.

Because `0 < d <= numberOfRings`, all six provinces are always distinct and lie
within the generated world. There is always exactly **six**.

Worked example, a world with `numberOfRings = 3` and the default `d = 2`:

| Direction | Province  |
|-----------|-----------|
| N         | `(0,-2)`  |
| NE        | `(2,-2)`  |
| SE        | `(2,0)`   |
| S         | `(0,2)`   |
| SW        | `(-2,2)`  |
| NW        | `(-2,0)`  |

For `d = 1` the six are the origin's immediate neighbors:
`(0,-1)`, `(1,-1)`, `(1,0)`, `(0,1)`, `(-1,1)`, `(-1,0)`.

{{< callout type="warning" >}}
On a large world the six defaults sit far from the center (distance
`ceil(numberOfRings / 2)`), which a new GM may not expect. Choose a nearer ring
with `--ring`, or edit `starting-provinces.json` afterward. Large worlds are
allowed; this is only a caution.
{{< /callout >}}

The default set is a starting point that the GM may edit afterward. The current
selection is purely geometric; it is expected to improve later (for example, a
short walk from the computed hex toward better nearby terrain).

## Command

World generation is provided by the `cmd/tpty` command. It writes two files into
the data directory:

- `world.json` — the generated world: the master seeds, the ring count, and
  every province with its coordinates and terrain.
- `terrain-translation.json` — a map from each terrain name to a
  [Worldographer](https://worldographer.com) tile name, for importing the world
  into Worldographer.

## See also

- [Determinism]({{< relref "/docs/reference/determinism.md" >}})
- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
