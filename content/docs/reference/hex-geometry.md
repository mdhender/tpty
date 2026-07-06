---
title: Hex Geometry
weight: 5
---

The world is a grid of regular, **flat-top** hexes (see
[World Generation]({{< relref "/docs/reference/world-generation.md" >}})). Every
hex is the same size. That size is fixed by a single measurement, the
**apothem**; every other measurement is derived from it.

## Apothem

The **apothem** is the distance from the center of a hex to the midpoint of any
side — the shortest distance from the center to the boundary. For a flat-top
hex it is the distance from the center to the top or bottom edge.

It is not the side length, the radius to a corner, or half the corner-to-corner
width.

For this game:

> **Apothem = 5 km**

Every quantity below is computed from that value.

## Derived measurements

A regular hexagon is six equilateral triangles. Writing the apothem as `a` and
the side length as `s`, the two are related by `a = (√3 / 2) × s`, so
`s = 2a / √3`.

| Property                  | Formula        | At `a` = 5 km |
|---------------------------|----------------|--------------:|
| Apothem                   | `a`            |       5.00 km |
| Side length               | `2a / √3`      |     5.7735 km |
| Flat-to-flat width        | `2a`           |      10.00 km |
| Corner-to-corner width    | `2s`           |      11.55 km |
| Area                      | `2√3 × a²`     |    ≈86.6 km²  |

The side length is also the distance from the center to each corner.

## Lattice spacing

A hex grid is a lattice of centers, not a set of touching polygons. For flat-top
hexes the spacing between neighboring centers is:

| Property                  | Formula        | At `a` = 5 km |
|---------------------------|----------------|--------------:|
| Horizontal center spacing | `3/2 × s`      |      8.660 km |
| Vertical row spacing      | `2a`           |      10.00 km |

These two values generate every center on the map.

## Rendering scale

Pixel dimensions follow from the geometry and a single chosen scale, the number
of pixels per kilometer. At a scale of **4 px/km** — for example, a world about
1,280 km wide rendered 5,120 px wide — the measurements above become:

| Property                  | Distance | Pixels  |
|---------------------------|----------|--------:|
| Apothem                   | 5 km     |   20 px |
| Flat-to-flat width        | 10 km    |   40 px |
| Side length               | 5.773 km | 23.1 px |
| Corner-to-corner width    | 11.55 km | 46.2 px |
| Horizontal center spacing | 8.66 km  | 34.6 px |
| Vertical row spacing      | 10 km    |   40 px |

Changing the render scale changes only the pixels-per-kilometer factor; the
kilometer measurements are unchanged.

## See also

- [World Generation]({{< relref "/docs/reference/world-generation.md" >}}) — the grid the hexes form
- [Why the Hex Is Measured by Apothem]({{< relref "/docs/explanation/why-the-hex-is-measured-by-apothem.md" >}}) — why one measurement is authoritative
- [Why This Game Uses a 5 km Hex Apothem]({{< relref "/docs/explanation/why-this-game-uses-a-5-km-hex-apothem.md" >}}) — why the apothem is 5 km
