---
title: Why the Hex Is Measured by Apothem
weight: 3
---

This page is *about* how the game measures a hex, and why it settles on one
authoritative number. For the measurement itself — the definition, the formulas,
and the derived values — see the
[hex geometry reference]({{< relref "/docs/reference/hex-geometry.md" >}}). Here
we step back and discuss the choice.

Every hexagonal map begins with a single measurement. Many games describe a hex
by its width, others by its side length, and some simply state its area. This
project instead measures every hex by its **apothem** — the distance from the
center to the midpoint of a side — and derives everything else from it: the
width, the height, the area, the spacing between hex centers, and ultimately the
pixel dimensions of a rendered map. By choosing a single authoritative
measurement, the geometry of the world stays internally consistent and easy to
reproduce at every stage of the pipeline.

## Why not measure by side length

There is nothing wrong with defining a hexagon by its side length — many
mathematical texts do exactly that. But the side length is not the quantity most
map-generation algorithms naturally reach for. Most operations begin from the
center of a cell: neighborhood searches, terrain interpolation, Voronoi
generation, river routing, distance transforms, fog-of-war, visibility. The
apothem describes the "working radius" of a hex — how far its influence reaches
from the center — far more naturally than an edge length does.

It also produces cleaner numbers to reason about. "Five kilometers from the
center to an edge" is easier to hold in your head than "5.7735 kilometers to
each corner," and a round center-to-edge distance keeps the mental arithmetic of
distance and travel honest.

## One source of truth

A common source of bugs in map software is duplicated geometry. One module stores
a hex width, another a radius, another a pixel spacing. Over time the values
drift, and small inconsistencies accumulate until neighboring systems disagree
about where a hex actually is. The fix is to refuse to duplicate the number in
the first place: the apothem is the source of truth, and every other quantity is
computed from it rather than stored alongside it.

The payoff is that scale becomes a single knob. Changing the size of the world
means changing one value; every renderer, simulation, exporter, and editor stays
consistent automatically, because none of them holds an independent copy of the
geometry to fall out of step.

## Why this matters

Choosing the apothem as the foundational measurement is not merely an
implementation detail — it reflects how the project layers itself. Geometry
describes the world, rules describe how actors move through it, and rendering
describes how it is displayed. Each layer depends on the one beneath without
redefining it, so the map stays stable and predictable even as the rules and the
presentation evolve.

The result is a system in which every distance, from kilometers to pixels, is
derived rather than invented. The geometry becomes an invariant, which lets the
game itself change without ever changing the shape of its world.

## See also

- [Hex geometry reference]({{< relref "/docs/reference/hex-geometry.md" >}}) — the definition, formulas, and derived values
- [Why This Game Uses a 5 km Hex Apothem]({{< relref "/docs/explanation/why-this-game-uses-a-5-km-hex-apothem.md" >}}) — why that one value is 5 km
- [World generation explanation]({{< relref "/docs/explanation/world-generation.md" >}}) — the grid these hexes form
