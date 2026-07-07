---
title: Recruit players and add them to a game
weight: 4
---

This guide covers recruiting players for a game and adding each one so they can
submit orders.

## Before you start

You need a game with a generated world. See
[Create a game]({{< relref "/docs/how-to/create-a-game.md" >}}) and
[Generate a world]({{< relref "/docs/how-to/generate-a-world.md" >}}).

## Offer starting provinces

Players choose where they start, from a list you control: the game's **allowed
starting provinces**. `player create` rejects any starting province not in this
list.

The quickest way to seed the list is the default set:

```sh
tpty world starting-provinces generate --data path/to/data
```

Then tailor it with `add` and `remove`, and review it with `list`:

```sh
tpty world starting-provinces add    --data path/to/data --province "(-1,0)"
tpty world starting-provinces remove --data path/to/data --province "(2,0)"
tpty world starting-provinces list   --data path/to/data
```

Provinces are named in compact `(q,r)` form and must be unique. Choose provinces
from the world you generated. See the
[Starting provinces reference]({{< relref "/docs/reference/world-generation.md#starting-provinces" >}})
for the file format and the full rules.

## Recruit

Advertise the game — for example, on Discord — and include the starting provinces
you are offering. Ask each interested player to reply by email with:

- the **handle** they want (their in-game name), and
- their **preferred starting province**, in compact form (for example, `(-1,0)`).

## Add each player

For each player, run:

```sh
tpty player create \
  --data path/to/data \
  --email player@example.com \
  --handle "Bo Peep" \
  --starting-province "(-1,0)"
```

This assigns the next player id, adds the player to `players.json`, and prints
the id and a generated password:

```
created player 1 in game "my-game"
  handle:   Bo Peep
  email:    player@example.com
  province: (-1,0)
  password: audio.watch.chain.baker.twins.dizzy.blob
wrote path/to/data/players.json
```

Send each player their **id** and **password**; they use them to authenticate
their orders. The password is stored in plain text in `players.json`, so keep
that file private.

## Options

- `--email` (required) — the player's email address. Stored in lowercase and
  unique within the game.
- `--handle` (required) — the player's in-game name. Unique within the game and
  fixed for its life. Starts with a letter, is at least two characters, and
  allows letters, digits, hyphen, underscore, period, and single internal spaces.
- `--starting-province` (required) — one of the game's offered provinces, in
  compact `(q,r)` form.
- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.

## See also

- [List and inspect players]({{< relref "/docs/how-to/list-and-inspect-players.md" >}})
- [Players reference]({{< relref "/docs/reference/players.md" >}})
- [Create a game]({{< relref "/docs/how-to/create-a-game.md" >}})
