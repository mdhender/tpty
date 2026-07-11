---
title: Entities
weight: 6
---

An **entity** is an actor in a game's world. It occupies a province, and it
carries out the orders of the faction that controls it. Entities are the things
that orders act on: an orders file addresses each entity by its id (see
[Orders]({{< relref "/docs/reference/orders" >}})). Entities are scoped to a
game: each game has its own entities, and an entity belongs to exactly one game.

## Identity

Every entity has an id and a name.

### ID

- A positive integer assigned by the engine when the entity is created.
- Unique within the game.
- Assigned sequentially in increasing order. IDs are never reused within a game.
- An orders file names an entity by this id, on the entity block's header line
  (`entity <id>, <name>`); see [Orders]({{< relref "/docs/reference/orders" >}}).

### Name

- A display name for the entity, shown on the entity block header and in reports.
- Required and non-empty.
- Stored as entered; its case is preserved.
- Need not be unique: an entity is identified by its id, so the name is only a
  label. It is a text field, quoted in an orders file per the quoting convention
  (see [Orders]({{< relref "/docs/reference/orders#text-fields" >}})).

## Faction

- Every entity belongs to exactly one **faction** at a time and records that
  faction's id (see [Factions]({{< relref "/docs/reference/factions.md" >}})).
- The faction's controller is the only source of the entity's orders: an order
  for an entity is accepted only when it comes from the controller of the
  entity's faction (see the ownership rule under
  [Orders]({{< relref "/docs/reference/orders#ownership" >}})).
- An entity's faction can change during play — for example, when control passes
  from one faction to another — so faction membership is not fixed for the life
  of the entity. The orders that transfer control define how; some are not yet
  implemented.

## Location

- Every entity occupies one **province** — a single hex — named by its
  coordinates in compact form, for example `(-1,0)` (see
  [World Generation]({{< relref "/docs/reference/world-generation.md" >}})).
- An entity's location changes as it moves. Movement is defined by the orders
  that move an entity; those orders define how far and where it may go.

## Creation

The engine creates entities.

- When a player enters play, the engine creates the player a single faction and
  one **starting entity**, located in the player's
  [starting province]({{< relref "/docs/reference/players.md#starting-province" >}}).
  This happens as the game advances into turn 1 (see
  [Turns]({{< relref "/docs/reference/turns.md" >}})).
- During play, orders can create further entities. The orders that do so define
  the result; some are not yet implemented.

## Attributes

Beyond its id, name, faction, and location, an entity carries whatever
attributes the orders acting on it require. This reference lists only the
attributes in use today; more are added here as the orders that need them are
implemented (matching how the rules are built up one at a time).

## Example

```json
{
  "id": 101,
  "name": "Conan the Copyright",
  "factionId": 1,
  "location": "(-1,0)"
}
```

## See also

- [Factions]({{< relref "/docs/reference/factions.md" >}})
- [Players]({{< relref "/docs/reference/players.md" >}})
- [Orders]({{< relref "/docs/reference/orders" >}})
- [Turns]({{< relref "/docs/reference/turns.md" >}})
- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
