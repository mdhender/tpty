---
title: Reports
weight: 8
---

A **turn report** is what a player receives at the start of a turn. It describes
the player's part of the game at the **start** of turn `N` — before any of that
turn's orders are applied — and is what the player reads to decide their orders
for turn `N` (see [Turns]({{< relref "/docs/reference/turns.md#reports-show-the-start-of-a-turn" >}})).
Reports and [orders]({{< relref "/docs/reference/orders" >}}) are the two halves
of the per-turn loop: report → the player decides → orders → the turn is
processed → the game advances → the next report.

This reference describes what a report **contains** — its model. It does not
specify how a report is laid out; a concrete report format is not yet part of
the rules.

## One report per player

A report is scoped to a single player. It covers only that player's own
factions and entities; it does not describe other players' factions or entities.
The engine produces one report for each active player.

What a player can observe of the wider game beyond their own entities — other
entities, or provinces they do not occupy — is not part of the current rules and
is left to a later rule.

## Contents

A report is identified by, and contains, the following, all as of the **start**
of the turn it describes:

- **Player** — the [player]({{< relref "/docs/reference/players.md" >}}) the
  report is for, within their game.
- **Turn** — the turn number `N` the report describes. The report reflects the
  state at the start of turn `N`, before that turn's orders (see
  [Turns]({{< relref "/docs/reference/turns.md" >}})).
- **Factions** — the player's
  [factions]({{< relref "/docs/reference/factions.md" >}}).
- **Entities** — the [entities]({{< relref "/docs/reference/entities.md" >}})
  those factions own, each as it stands at the start of the turn (its id, name,
  faction, and location).

## Delivery

Delivery is manual. The engine generates a report for each active player; the GM
sends each player their own report. There is no automated mail delivery.

## See also

- [Turns]({{< relref "/docs/reference/turns.md" >}})
- [Orders]({{< relref "/docs/reference/orders" >}})
- [Players]({{< relref "/docs/reference/players.md" >}})
- [Factions]({{< relref "/docs/reference/factions.md" >}})
- [Entities]({{< relref "/docs/reference/entities.md" >}})
- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
