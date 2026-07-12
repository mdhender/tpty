---
title: Glossary
weight: 90
---

Definitions of terms used across the reference.

**Axial coordinates**
: The coordinate system used to report hex positions, written `(q, r)`. The
  origin is `(0, 0)`.

**Entity**
: An actor in a game's world. It occupies a province and carries out its
  faction's orders. Belongs to exactly one faction. See
  [Entities]({{< relref "/docs/reference/entities.md" >}}).

**Faction**
: A group of entities under a single controller — a player or an NPC. Belongs to
  a game and owns the entities that carry out its orders. See
  [Factions]({{< relref "/docs/reference/factions.md" >}}).

**Flat-top**
: The hex orientation used by the grid, with a flat edge at the top and bottom
  of each hex and north toward the top of the map.

**Game**
: The top-level unit of play; the world and the players belong to it. Described
  by a `game.json` manifest. See [Games]({{< relref "/docs/reference/games.md" >}}).

**GM**
: The game master; the operator who generates and runs a game.

**Handle**
: A player's short display name, unique within a game. Starts with a letter and
  is at least two characters; see [Players]({{< relref "/docs/reference/players.md" >}}).

**Master seed**
: One of the two `uint64` values (`seed1`, `seed2`) saved for a game in its
  `game.json`. Together they are the root of the game's randomness; each
  subsystem derives its own seeds from them. See
  [Games]({{< relref "/docs/reference/games.md" >}}) and
  [Determinism]({{< relref "/docs/reference/determinism.md" >}}).

**NPC**
: A computer agent that generates a faction's orders automatically — never a
  player or the GM. An NPC can control a faction in place of a player. See
  [Factions]({{< relref "/docs/reference/factions.md#controller" >}}).

**Operator**
: The person who administers a T'Pty server and its database with the `tdb`
  tool — creating, migrating, backing up, and compacting it, and creating
  accounts. Distinct from the game master (GM), who runs individual games. See
  [Administer a database with tdb]({{< relref "/docs/how-to/administer-a-database.md" >}}).

**Origin**
: The center hex of the world, at axial coordinates `(0, 0)`.

**Player**
: A person participating in a game, identified within that game by a sequential
  `id`, a unique `email`, and a unique `handle`. See
  [Players]({{< relref "/docs/reference/players.md" >}}).

**Province**
: A single hex. Every province is assigned exactly one terrain.

**Ring**
: The set of provinces at a fixed distance from the origin. Ring `0` is the
  origin; ring `k` contains `6k` provinces.

**Seniority**
: An entity's position in the FIFO list of the location it occupies; the earlier
  in the list, the more senior. Unique within a location, so it is the
  scheduler's tie-free tie-breaker. See
  [Turn Processing]({{< relref "/docs/reference/turn-processing.md#seniority" >}}).

**Starting province**
: The province a player occupies when entering a game. Must be one of the game's
  allowed starting provinces. See [Players]({{< relref "/docs/reference/players.md" >}}).

**Terrain**
: The kind of land assigned to a province (for example, Plains or Mountain).

**Tick**
: The engine's unit of time within a turn; one tick is one day. A turn is
  processed as 32 ticks. See
  [Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}}).

**Tile**
: The permanent aspect of a province (for example, "an ocean tile").

**Turn**
: The unit of play a game advances by. Turn `0` is setup (no turn); play begins
  at turn `1` and counts up. See [Turns]({{< relref "/docs/reference/turns.md" >}}).

**Turn report**
: What a player receives at the start of a turn, describing their factions and
  entities at the start of that turn; the basis for the player's orders. See
  [Reports]({{< relref "/docs/reference/reports.md" >}}).
