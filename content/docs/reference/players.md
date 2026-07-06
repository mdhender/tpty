---
title: Players
weight: 3
---

A **player** is a person participating in a game. Players are scoped to a game:
each game has its own set of players, and a player belongs to exactly one game.

## Identity

Every player has three identifying fields: an id, an email, and a handle.

### ID

- A positive integer assigned by the engine when the player is created.
- Unique within the game.
- Assigned sequentially in increasing order. The first player created in a game
  is `1`, the next `2`, and so on. IDs are never reused within a game, even
  after a player is removed.

### Email

- The player's email address.
- Stored in lowercase. The email is lowercased when the player is saved, so
  `Alice@Example.com` is stored and matched as `alice@example.com`.
- Unique within the game, compared after lowercasing.

### Handle

- A short display name for the player.
- Stored as entered; its case is preserved.
- Unique within the game, compared exactly (case-sensitively).
- At least two characters. The first character is a letter (`a`–`z` or `A`–`Z`).
- Each remaining character is a letter, a digit, one of hyphen (`-`), underscore
  (`_`), or period (`.`), or a space.
- A space must be an ASCII space (`U+0020`); tabs and other whitespace are not
  allowed.
- Spaces appear only between two non-space characters. A handle may not begin or
  end with a space, and may not contain two or more spaces in a row.
- Matches the pattern `[a-zA-Z]( ?[a-zA-Z0-9_.-])+` against the whole handle.
- Fixed for the life of the game: a handle cannot be changed once the player is
  created (see [Randomness](#randomness)).

## Uniqueness and scope

Within a single game, each of `id`, `email`, and `handle` is unique. Email
uniqueness is checked after lowercasing; handle uniqueness is checked exactly.

Players in different games are independent. The same email or handle may appear
in more than one game, and ids restart at `1` for each game.

## Starting province

Each player has a **starting province**: where the player's first entity is
created when the game begins. It is retained afterward so the player can be
restarted from the same place.

- A player's starting province must be one of the game's **allowed starting
  provinces** — a set the GM maintains for the game.
- Provinces are named by their coordinates in compact form (see
  [World Generation]({{< relref "/docs/reference/world-generation.md" >}})), for
  example `(-1,0)`.

## Password

Each player has a **password**: a shared secret used to authenticate the
player's orders. The first record of an orders file is a tuple of the game id,
the player id, and the password as quoted text.

- Generated when the player is created.
- Stored in plain text.
- Contains no characters that require escaping in JSON and none that could be
  confused with an ASCII space.

## Randomness

Each player has a **private randomness stream**, used to draw per-player
outcomes independently of other players and of world generation. It is derived
deterministically from the game's master seeds and the player's handle.

Because the handle determines this stream, a player's handle is fixed for the
life of the game: changing it would change every random outcome derived for that
player.

## Examples

Valid handles: `ab`, `Al`, `player-1`, `j.doe`, `a_b.c-d`, `Bo Peep`.

Invalid handles: `a` (too short), `1player` (does not start with a letter),
`Bo  Peep` (two spaces in a row), `Bo ` (ends with a space), `joe!` (`!` is not
an allowed character).

## See also

- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
