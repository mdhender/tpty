---
title: Move
weight: 4
---

`move` walks an entity across the hex grid along a **path** of one or more
direction steps. On completion the entity's location becomes the province
reached by taking each step in order from where it started.

```
move <direction>…
```

`move` takes a path of one or more **direction numbers**.

## Direction numbers

A direction is written as a number `1`–`6`, in clockwise order from north —
exactly the order and the axial vectors listed for the six neighbor directions in
[World Generation]({{< relref "/docs/reference/world-generation.md" >}}):

| Number | Direction      | Axial `(q, r)` step |
|--------|----------------|---------------------|
| 1      | North (N)      | `(0, -1)`           |
| 2      | Northeast (NE) | `(+1, -1)`          |
| 3      | Southeast (SE) | `(+1, 0)`           |
| 4      | South (S)      | `(0, +1)`           |
| 5      | Southwest (SW) | `(-1, +1)`          |
| 6      | Northwest (NW) | `(-1, 0)`           |

Each number in the path is one step to the adjacent province in that direction.
The path is followed left to right: `move 2 3` steps northeast, then southeast,
from the entity's starting province.

## Time cost

One step costs **7 days** (one week). The total cost of a `move` is
`7 × (number of steps)` — so `move 1` costs 7 days and `move 1 2 3` costs 21.
This matches the "7-day move" worked example in
[Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}}), where a
single-step move is 7 days. The engine measures this as ticks (one day per
tick).

## Effect

On completion the entity's location becomes the province reached by applying
each step's axial vector in order to its starting province, as grounded in
[Entities]({{< relref "/docs/reference/entities.md#location" >}}) ("An entity's
location changes as it moves"). For example, an entity at `(0,0)` given
`move 2` (northeast) ends at `(1,-1)`; given `move 2 4` it ends back at `(1,0)`.

## Invalid input

A direction argument that is not an integer `1`–`6` makes the whole order
**fail**: the entity does not move, its location is unchanged, and the failure is
recorded as a completed (non-stub) outcome with a message naming the bad
argument. A failed `move` costs **0 days**. The parser guarantees each argument
is a syntactic field; interpreting it as a direction number is this order's job.

## Scope

This is the MVP movement rule. In this pass **every step is one adjacent hex at
7 days**: terrain has no effect on cost and there is no map-boundary check, so a
step always reaches the adjacent province regardless of its terrain or whether it
has been generated yet. Per-terrain movement cost and map-bounds handling are
left as future work; this section will be revised when they land.

## See also

- [Orders]({{< relref "/docs/reference/orders" >}})
- [Hold]({{< relref "/docs/reference/orders/hold.md" >}})
- [World Generation]({{< relref "/docs/reference/world-generation.md" >}})
- [Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}})
- [Entities]({{< relref "/docs/reference/entities.md" >}})
