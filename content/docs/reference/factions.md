---
title: Factions
weight: 5
---

A **faction** is a group of entities under a single controller. It sits between
a game's controllers — its players and NPCs — and the entities that act in the
world. Everything a faction does comes from **orders**, and a faction accepts
orders only from its controller; the entities it owns carry those orders out.
Factions are scoped to a game: each game has its own factions, and a faction
belongs to exactly one game.

## Identity

Every faction has an id and a name.

### ID

- A positive integer assigned by the engine when the faction is created.
- Unique within the game.
- Assigned sequentially in increasing order. The first faction created in a game
  is `1`, the next `2`, and so on. IDs are never reused within a game.

### Name

- A display name for the faction, shown in reports.
- Required and non-empty.
- Stored as entered; its case is preserved.
- Unique within the game, compared exactly (case-sensitively).

## Controller

Every faction is controlled by exactly one **controller**: the sole source of
the faction's orders. Orders are a faction's only input, and a faction accepts
them only from its controller. A controller is either a **player** or an **NPC**.
A faction records its controller as a kind (`player` or `npc`) and the
controller's id within that kind, so player ids and NPC ids cannot be confused.

### Player-controlled factions

- The controller is a [player]({{< relref "/docs/reference/players.md" >}}) in the
  same game, named by the player's id.
- The player supplies the faction's orders in an orders file, proving they are
  the controller by authenticating it with the game id, player id, and password
  in the opening record (see [Orders]({{< relref "/docs/reference/orders" >}})).
- A player controls one or more factions. When a player enters play, the engine
  creates the player a single faction (see
  [Turns]({{< relref "/docs/reference/turns.md" >}})). Until a name generator
  exists, a faction the engine seeds at turn 1 is given the placeholder name
  `Faction <id>`, using its own id.

### NPC-controlled factions

- The controller is an **NPC**: a computer agent that generates the faction's
  orders automatically. An NPC is never a person — neither a player nor the GM
  controls it or its factions.
- An NPC's orders reach its faction the same way a player's do; only the source
  differs. An NPC faction has no password, because no person submits its orders.

Which NPC agents a game has, and how an NPC decides its orders, are defined
outside this reference and are not part of the current rules.

## Entities

A faction owns zero or more **entities**. Every entity belongs to exactly one
faction, and the entity records the id of the faction that controls it. An order
for an entity is accepted only when it comes from the controller of that entity's
faction (see the ownership rule under
[Orders]({{< relref "/docs/reference/orders" >}})).

## Relationships

- A game has many factions.
- A faction is controlled by exactly one controller — one player, or one NPC.
- A player controls one or more of the game's factions; an NPC controls zero or
  more.
- A faction owns zero or more entities; an entity belongs to exactly one faction.

## Example

A player-controlled faction:

```json
{
  "id": 1,
  "name": "The Slaves of Darkness",
  "controller": { "kind": "player", "id": 3 }
}
```

An NPC-controlled faction:

```json
{
  "id": 2,
  "name": "The Wild Tribes",
  "controller": { "kind": "npc", "id": 1 }
}
```

## See also

- [Games]({{< relref "/docs/reference/games.md" >}})
- [Players]({{< relref "/docs/reference/players.md" >}})
- [Orders]({{< relref "/docs/reference/orders" >}})
- [Turns]({{< relref "/docs/reference/turns.md" >}})
- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
