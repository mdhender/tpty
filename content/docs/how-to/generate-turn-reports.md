---
title: Generate turn reports
weight: 10
---

This guide covers producing each active player's start-of-turn report so you can
deliver it. A report describes the state of the game at the **start** of the
current turn — before that turn's orders are applied — and is what the player
reads to decide their orders. Generate the reports right after
[advancing]({{< relref "/docs/how-to/process-and-advance-a-turn.md" >}}) into a
new turn.

## Generate the reports

```sh
tpty turn report --data path/to/data
```

This writes one JSON report per active player under the game's `reports`
directory. Each report contains only that player's own factions and entities as
of the start of the current turn. The command reads the live game state and does
not mutate or advance the turn, so it is safe to re-run.

## Deliver the reports

Delivery is **manual**: the engine writes each player's file to disk, and you
send each player their own report — for example by email. There is no automated
mail delivery. See [Reports]({{< relref "/docs/reference/reports.md" >}}) for
what a report contains.

## Options

- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.

## Next steps

- Wait for players to reply with their orders, then
  [accept each player's orders]({{< relref "/docs/how-to/accept-orders.md" >}})
  for the turn.

## See also

- [Reports reference]({{< relref "/docs/reference/reports.md" >}})
