---
title: Remove and reactivate a player
weight: 7
---

When a player leaves a game — drops out, is replaced, or is set aside for a
while — **remove** them so they no longer appear in the active roster. Removing a
player does not delete anything: the record is retained, and you can
**reactivate** the player later to bring them back exactly as they were.

## Before you start

You need a game with the player already added. See
[Recruit players and add them to a game]({{< relref "/docs/how-to/recruit-players.md" >}}).

Identify the player by **id** or **handle**, whichever you have on hand. Use
[player list]({{< relref "/docs/how-to/list-and-inspect-players.md" >}}) to look
either up.

## Remove a player

```sh
tpty player remove --data path/to/data --handle "Bo Peep"
```

or by id:

```sh
tpty player remove --data path/to/data --id 1
```

This marks the player inactive, writes `players.json`, and reports the change:

```
removed player 1 in game "my-game"
  handle:   Bo Peep
  email:    player@example.com
  status:   inactive
wrote path/to/data/players.json
```

The player is now hidden from `player list` unless you pass `--all`. Their id,
email, and handle stay reserved — no new player can take them (see
[Notes](#notes)).

## Reactivate a player

Bring a removed player back with the same id and handle:

```sh
tpty player reactivate --data path/to/data --handle "Bo Peep"
```

or by id:

```sh
tpty player reactivate --data path/to/data --id 1
```

```
reactivated player 1 in game "my-game"
  handle:   Bo Peep
  email:    player@example.com
  status:   active
wrote path/to/data/players.json
```

The player is active again, unchanged from before they were removed — same id,
handle, email, starting province, seeds, and password.

## Options

Both commands take the same flags:

- `--id` **or** `--handle` (exactly one required) — identifies the player.
  Providing both, or neither, is an error.
- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.

## Notes

- Removing is not deleting. The record is kept in full, so no data is lost and
  the player can always be reactivated.
- Ids are never reused. A removed player keeps its id, and new players continue
  to get the next id in sequence.
- A removed player still holds its email and handle: neither can be reused by
  another player while the record exists, active or not. See the
  [Players reference]({{< relref "/docs/reference/players.md#active-state" >}}).
- Removing an already-removed player, or reactivating an already-active one, is a
  no-op error — the command tells you the player is already in that state and
  changes nothing.

## See also

- [List and inspect players]({{< relref "/docs/how-to/list-and-inspect-players.md" >}})
- [Players reference]({{< relref "/docs/reference/players.md" >}})
