---
title: Turns
weight: 2
---

A game advances in **turns**. A turn is the unit of play: players issue orders
for a turn, the GM processes them, and the game moves on to the next turn. Every
game tracks its **current turn** as an attribute of the game (see
[Games]({{< relref "/docs/reference/games.md" >}})).

## Turn numbering

- **Turn 0** means **no turn**. It is the setup phase — creating the game,
  generating the world, and recruiting players all happen at turn 0, before play
  begins. A new game starts here. Turn 0 is the zero value.
- **Turn 1** is the first turn of play. Turns count up from there: 1, 2, 3, and
  so on.

## Per-turn lifecycle

Play proceeds one turn at a time. For a turn `N`:

1. The GM sends each player their **report** for turn `N`.
2. Players read their reports and issue **orders** for turn `N`.
3. The GM **processes** turn `N`, applying every player's orders. How a turn is
   processed — the tick timeline and the order scheduler — is described in
   [Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}}).
4. When satisfied with the result, the GM **advances** the game to turn `N+1`
   and sends the reports for turn `N+1`.

While collecting orders (step 2), the GM can check the **submission status** for
the current turn — which active players have submitted
[orders]({{< relref "/docs/reference/orders/_index.md" >}}) and which have not —
to tell when everyone is in and it is time to process.

The current turn does not change while orders are being collected or processed;
it changes only when the GM advances the game.

## Reports show the start of a turn

A turn's report describes the state of the game at the **start** of that turn —
before any of the turn's orders are applied — not the state at the end. A report
for turn `N` is what a player sees in order to decide their orders for turn `N`.

## See also

- [Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}})
- [Games]({{< relref "/docs/reference/games.md" >}})
- [Players]({{< relref "/docs/reference/players.md" >}})
- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
