---
title: Players
weight: 4
---

A **player** is a person participating in a game. Players are scoped to a game:
each game has its own set of players, and a player belongs to exactly one game.

## Identity

Every player has two identifying fields: an id and a display name (also called a
handle). The player's email address is **not** a player field — it belongs to the
player's [account]({{< relref "/docs/reference/sql-schema.md#accounts" >}}), which
the player joins the game through.

### ID

- A positive integer assigned by the engine when the player is created.
- **Globally unique** and never reused — the id is not scoped to a game and does
  not restart per game. The same value identifies the player everywhere.

### Display name

The field is named `display_name`; **handle** is the conventional label for it in
client-facing pages and reports.

- A short display name for the player.
- Stored as entered; its case is preserved.
- Unique within the game, compared exactly (case-sensitively).
- At least two characters. The first character is a letter (`a`–`z` or `A`–`Z`).
- Each remaining character is a letter, a digit, one of hyphen (`-`), underscore
  (`_`), or period (`.`), or a space.
- A space must be an ASCII space (`U+0020`); tabs and other whitespace are not
  allowed.
- Spaces appear only between two non-space characters. It may not begin or end
  with a space, and may not contain two or more spaces in a row.
- Matches the pattern `[a-zA-Z]( ?[a-zA-Z0-9_.-])+` against the whole value.
- Fixed for the life of the game: it cannot be changed once the player is created
  (see [Randomness](#randomness)).

## Active state

Every player is either **active** or **inactive**. A player is active when
created.

Players are never physically deleted. **Removing** a player marks them inactive;
the record — including its id and display name — is retained in full.
**Reactivating** an inactive player marks them active again. A player may move
between the two states any number of times.

Because the record is retained:

- The player's id is never freed or reused, matching the id rule above. A removed
  player's id stays assigned to that player.
- The player continues to occupy its display name. It cannot be taken by a new or
  existing player while the inactive record holds it — uniqueness is enforced
  across active and inactive players alike (see
  [Uniqueness and scope](#uniqueness-and-scope)).

## Uniqueness and scope

Within a single game, a player's `display_name` is unique, compared exactly.
Uniqueness spans every player in the game, active or inactive: an inactive player
still holds its display name, so it cannot be reused.

Player **ids** are globally unique (see [ID](#id)) — unique across all games, not
just within one. A display name is only game-scoped: the same display name may
appear in more than one game.

## Starting province

Each player has a **starting province**: where the player's first entity is
created when the game begins. It is retained afterward so the player can be
restarted from the same place.

- A player's starting province must be one of the game's **allowed starting
  provinces** — a set the GM maintains for the game. See
  [Starting provinces]({{< relref "/docs/reference/world-generation.md#starting-provinces" >}})
  for how the default set is chosen and its invariants.
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

### Resetting a password

A player's password can be **reset** — reissued — when the current one has been
exposed. The reset replaces the stored password with a new value:

- The new value is drawn from the player's own private randomness stream, keyed
  by the game's [current turn]({{< relref "/docs/reference/turns.md" >}}). It is
  therefore deterministic for the engine but not predictable by others.
- It is drawn from a different part of the player's stream than the creation
  password, so a reset always differs from the password the player was created
  with, at any turn.
- Two resets in the **same** turn reproduce the same value; resets in
  **different** turns differ. Successive resets are distinguished by the turn
  advancing.
- The new value is written to the player record, replacing the old password.
  Authentication always validates an order against the password **stored in the
  player record**, so a reset takes effect as soon as it is saved.
- Only the stored password changes. The player's id, display name, starting
  province, and private seeds are untouched.

A reset is looked up by the email on the player's **account** — the address on
record — and by nothing else. Keying on the distinctive registered email (rather
than the id or the display name) makes it harder to reset the wrong player's
password by mistake, and harder for one player to trick the GM into resetting
another player's password.

## Randomness

Each player has a **private randomness stream**, used to draw per-player
outcomes independently of other players and of world generation. It is derived
deterministically from the game's master seeds and the player's display name.

Because the display name determines this stream, it is fixed for the life of the
game: changing it would change every random outcome derived for that player.

## Examples

Valid display names: `ab`, `Al`, `player-1`, `j.doe`, `a_b.c-d`, `Bo Peep`.

Invalid display names: `a` (too short), `1player` (does not start with a letter),
`Bo  Peep` (two spaces in a row), `Bo ` (ends with a space), `joe!` (`!` is not
an allowed character).

## See also

- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
