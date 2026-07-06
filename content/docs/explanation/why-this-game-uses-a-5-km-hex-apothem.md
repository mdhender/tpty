---
title: Why This Game Uses a 5 km Hex Apothem
weight: 4
---

Every strategy game built on a hexagonal map must answer one deceptively simple
question: *how large is a hex?* The answer shapes almost everything that
follows — travel times, map size, terrain density, settlements, logistics,
visibility, and even the kinds of stories the game naturally tells. Rather than
starting from travel speed or historical precedent, this project starts from
geometry, and fixes a single value:

> **Apothem = 5 km**

Everything else is derived from that one decision. The measurement itself and its
consequences live in the
[hex geometry reference]({{< relref "/docs/reference/hex-geometry.md" >}}); *why*
the game measures by the apothem at all is the subject of a
[separate discussion]({{< relref "/docs/explanation/why-the-hex-is-measured-by-apothem.md" >}}).
This page is about why that apothem is **5 km**.

## Beginning with the world

The original goal was not to imitate another game but to draw a strategic map of
Panama containing roughly one thousand land hexes. That number is large enough to
support exploration and regional strategy while staying computationally cheap and
visually understandable. Working backward from Panama's land area produced a
pleasant surprise: a hex with an apothem of about **5 km** yields almost exactly
that many land hexes. The geometry emerged from the desired scale of the world
rather than being imposed on it.

## A delightful coincidence

Only afterward did a historical coincidence surface. Classic fantasy
role-playing games often used the famous **6-mile wilderness hex**, which
measures about 3 miles — **4.83 km** — from center to flat. That is remarkably
close to this project's 5 km apothem. The two designs reached nearly the same
size by completely different routes: the older games began with practical
tabletop play, this one with the geography of Panama, and both converged on
nearly identical dimensions. When independent solutions arrive at the same
answer, it is often a sign that the design sits in a practical sweet spot.

## What fits inside a hex

At this scale a hex is not a single location but a small landscape. One hex might
reasonably hold a village, cultivated fields, patches of jungle, a stream, a
ruined temple, several farms, rocky hills, caves, and perhaps something dangerous
lurking in the interior. The player is not moving from one point to another; they
are entering a region. That distinction matters, because exploration happens
*inside* the hex, not merely between hexes.

## Exploration versus travel

One of the enduring insights of early wilderness games is that **travel** and
**exploration** are different activities. Travel asks how far someone can move;
exploration asks how much of the world they can truly come to understand. A
healthy traveler on a flat road can cover many tens of kilometers in a day. That
same traveler pushing into unfamiliar jungle spends the day choosing routes,
crossing rivers, climbing ridges, avoiding hazards, scouting, becoming lost,
searching for water, and investigating what they find. The kilometers covered may
be similar; the territory actually understood is far smaller.

This is why classic hexcrawls often assumed a party could travel several known
hexes in a day while thoroughly exploring only one unfamiliar hex. The geometry
of the map does not dictate movement — the terrain does.

## A day's march, a day's exploration

On good roads across flat country, a party might cross several 10 km hexes in a
day. Roads exist precisely to reduce the cost of movement: they make it
predictable, ease supply, and simplify navigation. Roads do not change the size
of the hexes; they change how quickly those hexes are crossed.

Dense jungle tells the opposite story. The same physical distance becomes
dramatically harder, slowed by vegetation, uncertain footing, weather, wildlife,
and the constant work of not getting lost. A day spent there may reveal only one
new hex — and yet the player has still done a full day's work, because the goal
was never to walk ten kilometers but to understand that landscape. Holding both
stories on the same map forces a meaningful choice between moving quickly and
learning thoroughly.

## Consequences for game design

Because the map scale is fixed by geometry rather than by time, the movement
systems stay free to vary. Roads, mounts, and boats can increase movement; heavy
rain can reduce it; a forced march can buy distance at the cost of fatigue. None
of these require touching the map — only the movement rules change, while the
world stays consistent. That separation of concerns is what makes the game
easier to extend over time.

Choosing the apothem first is what buys that freedom. With a stable physical
scale established up front, terrain generation, river placement, climate,
settlement spacing, visibility, supply ranges, movement costs, and even artwork
all inherit a consistent sense of distance instead of each inventing its own.

## Conclusion

The decision to use a 5 km apothem did not begin with tradition. It began with a
practical goal — representing Panama in about a thousand land hexes — and only
later proved to match the classic wilderness hex used across decades of tabletop
gaming. That coincidence is more than trivia: it suggests both designs
independently found a scale that feels natural to human exploration, large enough
that each hex holds interesting terrain and small enough that every hex can
matter.

At this scale the map becomes more than a collection of cells; it becomes a
landscape. Travel across that landscape is measured in kilometers, and
understanding it is measured in exploration. The distinction between those two
ideas is subtle, but it is one of the foundations on which compelling wilderness
games are built.

## See also

- [Why the Hex Is Measured by Apothem]({{< relref "/docs/explanation/why-the-hex-is-measured-by-apothem.md" >}}) — why the apothem is the authoritative measurement
- [Hex geometry reference]({{< relref "/docs/reference/hex-geometry.md" >}}) — the derived measurements at 5 km
