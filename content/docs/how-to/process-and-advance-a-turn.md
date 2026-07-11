---
title: Process and advance a turn
weight: 9
---

This guide covers running the engine over a turn's collected orders and then
moving the game forward to the next turn. These are two separate steps:
processing writes the results for the current turn; advancing commits the turn
and increments the game. See [Turns]({{< relref "/docs/reference/turns.md" >}})
for where they fall in the per-turn lifecycle.

## Process the current turn

Once every player is in (check with
[`orders list`]({{< relref "/docs/how-to/accept-orders.md" >}})), run the engine
over the current turn's collected orders:

```sh
tpty turn process --data path/to/data
```

This applies the submitted orders and writes the turn's results; it does **not**
advance the turn. Three guards apply:

- **Turn 0 has no play.** Processing refuses at turn 0 (setup).
- **Orders must be collected.** Processing refuses if no orders have been
  submitted for the turn.
- **A turn is processed at most once.** The result file is the "processed"
  marker; re-processing the same turn is an error.

How a turn is processed — the tick timeline and the order scheduler — is
described in
[Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}}).

## Advance to the next turn

When you are satisfied with the result, commit the turn and move the game to the
next one:

```sh
tpty turn advance --data path/to/data
```

This increments the current turn (`N` → `N+1`). It **refuses to advance an
unprocessed turn**: once play has begun (turn 1 or later) the current turn must
be processed first. Turn 0 is exempt from that guard — setup has nothing to
process — and advancing **0 → 1 begins play**, seeding each active player a
faction and one starting entity.

## Options

- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.

## Next steps

- [Generate turn reports]({{< relref "/docs/how-to/generate-turn-reports.md" >}})
  for the new turn and send each player their file.

## See also

- [Turns reference]({{< relref "/docs/reference/turns.md" >}})
- [Turn Processing reference]({{< relref "/docs/reference/turn-processing.md" >}})
