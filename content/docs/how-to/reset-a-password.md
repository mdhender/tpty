---
title: Reset an exposed password
weight: 6
---

If a player's order password is exposed — posted in a public channel, sent to
the wrong person, or otherwise leaked — reissue it so the old value no longer
works. This guide covers resetting a password and sending the player the new one.

## Before you start

You need a game with the player already added. See
[Recruit players and add them to a game]({{< relref "/docs/how-to/recruit-players.md" >}}).

Reset by the player's **registered email** — the address on file for them. This
is the only way to identify the player for a reset; you cannot reset by id or
handle. Keying on the distinctive email guards against resetting the wrong
player by mistake, and makes it harder for one player to trick you into resetting
another player's password.

## Reset the password

```sh
tpty player reset-password --data path/to/data --email player@example.com
```

This generates a new password, stores it in `players.json`, and prints it:

```
reset password for player 1 in game "my-game" (turn 0)
  handle:   Bo Peep
  email:    player@example.com
  password: cargo.mixer.plank.dozen.wharf.gloom.tidal
wrote path/to/data/players.json
```

The old password stops working as soon as the file is saved; orders now
authenticate against the new value. Send the player their new password over a
private channel, the same way you sent the original.

## Options

- `--email` (required) — the registered email of the player whose password to
  reset. Matched after lowercasing. An address that matches no player is an
  error.
- `--data` (required) — the game's data directory. May also be supplied as the
  `TPTY_DATA` environment variable.

## Notes

- The new password is drawn from the player's private randomness stream keyed by
  the game's current turn, so it is reproducible for the engine but not
  predictable by others. It always differs from the password the player was
  created with.
- Resetting again in the **same** turn reproduces the same value; a reset in a
  later turn produces a different one. See
  [Turns]({{< relref "/docs/reference/turns.md" >}}).

## See also

- [List and inspect players]({{< relref "/docs/how-to/list-and-inspect-players.md" >}})
- [Players reference]({{< relref "/docs/reference/players.md" >}})
