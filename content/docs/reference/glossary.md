---
title: Glossary
weight: 90
---

Definitions of terms used across the reference.

**Axial coordinates**
: The coordinate system used to report hex positions, written `(q, r)`. The
  origin is `(0, 0)`.

**Flat-top**
: The hex orientation used by the grid, with a flat edge at the top and bottom
  of each hex and north toward the top of the map.

**GM**
: The game master; the operator who generates and runs a game.

**Master seed**
: One of the two `uint64` values (`seed1`, `seed2`) saved for a game. Together
  they seed the engine's PRNG and determine every deterministic outcome.

**Origin**
: The center hex of the world, at axial coordinates `(0, 0)`.

**Province**
: A single hex. Every province is assigned exactly one terrain.

**Ring**
: The set of provinces at a fixed distance from the origin. Ring `0` is the
  origin; ring `k` contains `6k` provinces.

**Terrain**
: The kind of land assigned to a province (for example, Plains or Mountain).

**Tile**
: The permanent aspect of a province (for example, "an ocean tile").
