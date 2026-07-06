---
title: World Generation
weight: 1
---

This page is *about* world generation: why it works the way it does. For the
rules themselves — the coordinate system, the terrain table, the algorithm —
see the [world generation reference]({{< relref "/docs/reference/world-generation.md" >}}).
Here we step back and discuss the choices behind them.

## Why a hex grid

A square grid forces an awkward choice: are diagonal cells neighbors or not? If
they are, diagonal moves are "cheaper" than they look; if they aren't, movement
feels boxy. Hexes sidestep the problem. Every hex has exactly six neighbors, all
the same distance away, so adjacency and movement are uniform in every
direction. For a game about exploring and moving across terrain, that uniformity
is worth a great deal.

We use **flat-top** hexes with north at the top of the map. The alternative,
pointy-top, is equally valid; the choice is mostly aesthetic and a matter of how
the map reads on screen. Flat-top gives us a clean north/south axis with slanted
edges to the east and west, which suits a map that grows vertically as easily as
horizontally.

## Why the origin is the center

The origin sits at the center of the world, `(0, 0)`, and the world grows
outward from it in rings. A tempting alternative is to put `(0, 0)` in a corner
and let coordinates run only in the positive direction, the way array indices
do. That works right up until the world needs to grow past an edge the players
have reached — and then every coordinate has to shift.

By centering the origin, the world can expand in *any* direction without
renumbering anything. A province's coordinates are stable for the life of the
game. This matters because T'Pty is meant to reveal the world through
exploration: the map you generate at the start is a seed crystal, not a fixed
board, and it should be able to grow north, south, or any direction with equal
ease.

## Why axial coordinates

There are three common ways to name hexes: offset (row/column, like a
spreadsheet), cube (three coordinates that always sum to zero), and axial (two
coordinates, `q` and `r`). We report positions in axial throughout.

Each has a use. Offset coordinates are friendly to display but clumsy for hex
arithmetic. Cube coordinates make the geometry beautifully symmetric — rotations
and distances are trivial — but carry a redundant third number. Axial is cube
with the redundant coordinate dropped: compact to store and pass around, while
still recoverable to cube (the implied `s = -q - r`) whenever a calculation
wants the symmetry. Reporting one system consistently, rather than switching
between them, keeps the whole game legible; axial is the pragmatic middle.

## Why the world is deterministic

A game is defined by two master seeds. From them, the entire world — and
everything else the engine rolls for — follows. This is a deliberate and
load-bearing decision, not a convenience.

Determinism means a world is *reproducible*. The same seeds always produce the
same world, so a world can be shared or archived as two numbers instead of a
file, a bug can be reproduced exactly, and a save can be trusted to reload into
precisely the state it left. It also draws a bright line under testing: a test
can assert an exact outcome because there is exactly one outcome. The cost is a
discipline the engine must keep — nothing that affects a roll may depend on
anything outside the seeds — but that discipline buys confidence everywhere
else.

## Why terrain is keyed by position, not drawn in sequence

This is the least obvious choice and the most important. The reference describes
walking each ring clockwise and rolling `1d6` per province. The natural way to
implement that is a single stream of random numbers consumed in walk order: roll,
roll, roll, around the ring. We deliberately do *not* do that.

Instead, each province's terrain is drawn from its own stream, derived by hashing
the master seeds together with a **key** (what the stream is for) and a **leaf**
(the province's own coordinates). A province at `(q, r)` gets the same terrain
for a given pair of seeds no matter what else happens.

The consequence is that terrain depends only on the seeds and the province's
position — never on the order in which provinces are visited. That independence
is what makes the rest of the engine safe to write. Iteration order can change
during a refactor, generation could run in parallel, and the world can grow
lazily — computing the terrain of a newly explored province on demand, years of
game-time later — and every one of those still yields the identical world. A
single sequential stream would couple the result to the exact walk, making all
of that fragile.

The trade-off is real but small: we hash once per province instead of pulling
from one cheap sequential generator. For a world of a few thousand provinces that
cost is invisible, and we happily pay it for order-independence. This is also why
the engine is careful never to let Go's map iteration order influence a draw:
the whole design goal is that *order never matters*.

The clockwise ring walk still earns its place. Even though it no longer decides
*which* random numbers a province gets, it defines the one canonical order in
which provinces are enumerated and written out — useful for stable output and
for any future process that genuinely is sequential.

## Why the origin is always a mountain

For this round of development, the center province is always a mountain, and
mountains appear nowhere else — the terrain roll can't produce one. Treating the
origin as a fixed landmark gives every world a known, stable reference point at
`(0, 0)` while the rest of the systems are still taking shape.

This is a simplification, not a law of the world. It is easy to imagine the
origin becoming an ordinary province later, or the mountain becoming something
more meaningful — a capital, a starting location, a point of interest. For now it
is a deliberate anchor, and reading it as anything more permanent would be
reading too much in.

## Why terrain is a flat `1d6` for now

Every non-origin province rolls a plain, uniform `1d6` over six terrains, with
each equally likely and no regard for its neighbors. The result is a confetti of
terrain rather than anything a mapmaker would recognize: lakes do not gather into
bodies of water, forests do not form belts, deserts do not sit where deserts
belong.

That is a known and accepted limitation of this stage, not the intended end
state. A uniform roll is the simplest thing that exercises the whole pipeline —
generation, determinism, output — end to end, so we can build and test the
machinery before investing in the *character* of the world. Making terrain feel
like a place is a separate, later concern: weighting the table so some terrains
are rarer, and making a province's terrain depend on its surroundings so like
gathers with like. Both are deliberately out of scope until the foundation is
solid.

## See also

- [World generation reference]({{< relref "/docs/reference/world-generation.md" >}}) — the rules
- [Glossary]({{< relref "/docs/reference/glossary.md" >}}) — terms used above
