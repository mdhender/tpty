---
title: Map Wraparound
weight: 12
---

The world map is **finite** and **wraps around**: a step that would leave the
outer ring does not fall off the edge — it re-enters on the far side of the map.
The map has the topology of a torus built from a hexagon of hexes.

This page defines the wrap precisely, in terms of the axial `(q, r)` coordinates
and the ring model from
[World Generation]({{< relref "/docs/reference/world-generation.md" >}}). The
construction follows the hexagonal-map wraparound described by
[Red Blob Games](https://www.redblobgames.com/grids/hexagons/#wraparound).

## The map is finite

Let `N = numberOfRings`, the ring count fixed at world generation
(`0 < N < 100`). The map is the hexagon of provinces within ring `N` of the
origin: the origin plus rings `1` through `N`, for `1 + 3N(N + 1)` provinces in
all. There are no provinces beyond ring `N`; the map does not expand outward.

Every province on the map is at ring distance `d ≤ N` from the origin, where the
ring distance is the hex (cube) distance defined below.

## Cube coordinates

The wrap is stated most cleanly in **cube** coordinates, the standard companion
to axial. A hex axial `(q, r)` has cube coordinates `(x, y, z)` with

```
x = q      z = r      y = −x − z      (so x + y + z = 0)
```

The **ring distance** of a hex from the origin is its cube distance:

```
distance(x, y, z) = max(|x|, |y|, |z|)
```

A hex is on the map exactly when this distance is `≤ N`, and on the outer ring
when it equals `N`.

## The wrap rule

A single `move` step (see
[Move]({{< relref "/docs/reference/orders/move.md" >}})) applies one of the six
direction vectors. From a province on the outer ring, a step in an outward
direction reaches a hex at ring distance `N + 1` — one hex beyond the map. That
hex does not exist; the wrap **maps it back** to the province at the mirror
position on the far side of the map.

Steps that stay within ring `N` (distance `≤ N`) are unaffected: they reach the
adjacent province directly, with no wrap.

## Mirror centers

The map tiles the plane. Six identical copies of the hexagon surround the base
map, one across each edge; the center of each copy is a **mirror center**. To
wrap a hex that has stepped off the edge, subtract the mirror center of the copy
it stepped into, which slides it back onto the base map.

The six mirror centers, as a function of `N`:

| Mirror | Cube `(x, y, z)`      | Axial `(q, r)`   |
|--------|-----------------------|------------------|
| `M0`   | `(2N+1, −N, −N−1)`    | `(2N+1, −N−1)`   |
| `M1`   | `(N+1, −2N−1, N)`     | `(N+1, N)`       |
| `M2`   | `(−N, −N−1, 2N+1)`    | `(−N, 2N+1)`     |
| `M3`   | `(−2N−1, N, N+1)`     | `(−2N−1, N+1)`   |
| `M4`   | `(−N−1, 2N+1, −N)`    | `(−N−1, −N)`     |
| `M5`   | `(N, N+1, −2N−1)`     | `(N, −2N−1)`     |

Each is the previous one rotated 60° about the origin, so `M3 = −M0`,
`M4 = −M1`, and `M5 = −M2`. The first two generate the wrap lattice, and their
determinant `(2N+1)·N − (N+1)·(−N−1) = 3N² + 3N + 1` equals the province count —
confirming that the six copies tile the plane exactly, with the hexagon as a
perfect fundamental domain and no partial hexes.

## Canonicalization

To resolve a hex `p` that may lie off the map to its province on the map,
**canonicalize** it:

- If `distance(p) ≤ N`, `p` is already on the map; it is its own canonical form.
- Otherwise, subtract the one mirror center `Mi` for which
  `distance(p − Mi) ≤ N`. The result is the canonical province.

Because the six copies tile without overlap, the mirror center is **unique**: a
hex one step off the outer ring lands inside exactly one neighboring copy, so
exactly one `Mi` brings it back onto the map. There are no ties and no ambiguous
hexes. (Verified for every outer-ring exit at every `N` from 1 to 7.)

Canonicalization is a pure function of `(q, r)` and `N` — no randomness, no state
— so it is deterministic and reproducible, consistent with
[Determinism]({{< relref "/docs/reference/determinism.md" >}}).

## Stored coordinates are canonical

An entity's location (`entities.loc_q` / `loc_r`; see
[Entities]({{< relref "/docs/reference/entities.md#location" >}})) is **always
stored in canonical form** — the province on the base map, within ring `N`. A
move that crosses the edge stores the wrapped destination, not a coordinate
beyond the outer ring.

This gives every province one and only one coordinate. Locations never range
outside the map, `loc_q`/`loc_r` stay bounded by `N`, and a location always
names a province that exists in the generated world.

## Distance and adjacency

Because the map wraps, two provinces on opposite edges can be neighbors. The
distance between provinces `a` and `b` is the **toroidal** distance: the smallest
ring distance from `a` to `b` or to any of `b`'s six mirror images.

```
toroidal_distance(a, b) = min over M in {0, M0, …, M5} of distance(a − (b + M))
```

This is symmetric and never exceeds `N`. Two provinces are **adjacent** when
their toroidal distance is `1`. A province on the outer ring is therefore
adjacent to provinces on the opposite outer ring, reached by an edge-crossing
step.

## Frozen surface

The wrap mapping is part of the frozen coordinate system. Like the key-path
encoding in [Determinism]({{< relref "/docs/reference/determinism.md#frozen-surfaces" >}}),
the mirror-center formulas and the canonicalization rule are a compatibility
surface: they do not change while any game exists, because a change would move
existing entities. The map is finite and `N` is fixed for the life of a game, so
the wrap is fixed with it.

## Worked examples

A world with `N = 2` (19 provinces). An entity on the north corner of the outer
ring, at `(0,-2)`, issues a one-step `move` in each direction:

| `move` | Direction | Steps to  | Result                       |
|--------|-----------|-----------|------------------------------|
| `1`    | N         | `(0,-3)`  | wraps to `(-2,2)`            |
| `2`    | NE        | `(1,-3)`  | wraps to `(-1,2)`            |
| `3`    | SE        | `(1,-2)`  | on the map (ring 2)          |
| `4`    | S         | `(0,-1)`  | on the map (ring 1)          |
| `5`    | SW        | `(-1,-1)` | on the map (ring 2)          |
| `6`    | NW        | `(-1,-2)` | wraps to `(2,0)`             |

The three outward steps (N, NE, NW) leave ring 2 and wrap to the far side; the
three inward steps stay on the map. For the northward step, `(0,-3)` has cube
`(0,3,-3)` at distance 3; subtracting `M5 = (2,-5)` axial gives `(-2,2)`, the
southwest corner of the outer ring.

For `N = 1` (7 provinces), an entity at the north hex `(0,-1)` given `move 1`
(north) steps to `(0,-2)` and wraps to `(-1,1)`, the southwest province.

## See also

- [World Generation]({{< relref "/docs/reference/world-generation.md" >}}) — the finite ringed grid the wrap closes
- [Move]({{< relref "/docs/reference/orders/move.md" >}}) — the order whose steps cross the edge
- [Entities]({{< relref "/docs/reference/entities.md" >}}) — where a wrapped location is stored
- [Determinism]({{< relref "/docs/reference/determinism.md" >}}) — the frozen coordinate system the wrap belongs to
- [Glossary]({{< relref "/docs/reference/glossary.md" >}}) — terms used above
