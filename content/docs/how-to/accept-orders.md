---
title: Accept a player's orders
weight: 8
---

This guide covers ingesting a player's saved orders file into the current turn
and checking which players have submitted.

Orders are collected for the **current turn**, so the game must be at turn 1 or
later. Turn 0 is setup and has no play; `orders submit` and `orders list` both
refuse there. See [Turns]({{< relref "/docs/reference/turns.md" >}}) for the
per-turn lifecycle.

## Submit an orders file

When a player emails you their orders file, ingest it with:

```sh
tpty orders submit --file path/to/orders.txt --data path/to/data
```

The file authenticates itself through its **opening record** — the game id, the
player id, and the player's password on the first non-blank line. The engine
validates that record against the game and its players; a file whose opening
record fails authentication is rejected in full and nothing is stored.

Once the file authenticates, the submission is **accepted and stored** verbatim
under the game's orders directory, keyed to the submitting player and the current
turn. Parse errors and ownership problems (an order given to an entity the player
does not own) are reported as **warnings** — they do not block acceptance; those
individual orders simply will not be executed, and the rest of the submission
stands. Submitting again for the same player replaces their previous submission.

## Check who has submitted

To see which active players are in for the current turn:

```sh
tpty orders list --data path/to/data
```

This prints each active player and whether they have submitted, with a summary
count, so you can tell when everyone is in and it is time to
[process the turn]({{< relref "/docs/how-to/process-and-advance-a-turn.md" >}}).

## Options

- `--file` (required, `orders submit` only) — path to the player's saved orders
  file to ingest.
- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.

## Next steps

- [Process and advance a turn]({{< relref "/docs/how-to/process-and-advance-a-turn.md" >}})
  once every player is in.

## See also

- [Orders reference]({{< relref "/docs/reference/orders/_index.md" >}})
- [Turns reference]({{< relref "/docs/reference/turns.md" >}})
