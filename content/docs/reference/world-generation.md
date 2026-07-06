---
title: World Generation
weight: 1
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

The world is deterministic: the same master seeds always produce the same
world. A game records two master seeds, `seed1` and `seed2`, both `uint64`.

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

## Command

World generation is provided by the `cmd/tpty` command.

## See also

- [Glossary](glossary)
