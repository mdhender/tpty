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

Players choose where they start, from a list you control. Maintain that list in
the `starting-provinces.json` file named in the game's manifest — a JSON array of
provinces in compact `(q,r)` form:

```json
["(0,0)", "(-1,0)", "(2,0)"]
```

Choose provinces from the world you generated. `player create` rejects any
starting province that is not in this list.

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

- [Players reference]({{< relref "/docs/reference/players.md" >}})
- [Create a game]({{< relref "/docs/how-to/create-a-game.md" >}})
