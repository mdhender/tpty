---
title: List and inspect players
weight: 5
---

Once you have [added players]({{< relref "/docs/how-to/recruit-players.md" >}}) to
a game, use these commands to review them — for example, to check who has joined
or to resend a player their password.

## List the players

```sh
tpty player list --data path/to/data
```

This prints a row per player with their id, handle, email, and starting
province:

```
ID  HANDLE     EMAIL              STARTING PROVINCE
1   Bo Peep    alice@example.com  (-1,0)
2   little.bo  bob@example.com    (2,0)
```

## Show one player

Look a player up by id or by handle to see their full details, including their
password:

```sh
tpty player show --data path/to/data --id 1
tpty player show --data path/to/data --handle "Bo Peep"
```

```
player 1 in game "my-game"
  handle:   Bo Peep
  email:    alice@example.com
  province: (-1,0)
  password: audio.watch.chain.baker.twins.dizzy.blob
```

This is how you look up a player's id and password to resend them.

## Options

`player list`:

- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.

`player show`:

- `--id` — the player's id.
- `--handle` — the player's handle. Provide exactly one of `--id` or `--handle`.
- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.

## See also

- [Recruit players and add them to a game]({{< relref "/docs/how-to/recruit-players.md" >}})
- [Players reference]({{< relref "/docs/reference/players.md" >}})
