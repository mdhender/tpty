---
title: SQL Schema
weight: 12
---

The engine can persist a game to a **SQLite database** instead of the
[`game.json` manifest]({{< relref "/docs/reference/games.md" >}}) and its JSON
data files. This page describes the relational model. The authoritative DDL is
`internal/stores/sqlite/schema.sql`; the semantics of each entity live in the
other reference pages, which this page links to rather than restates.

One database holds **every** game. Each game-scoped row carries a `game_id`, so
the rows of one game never mingle with another's.

The store targets the pure-Go [ZombieZen](https://pkg.go.dev/zombiezen.com/go/sqlite)
driver. The DDL is standard SQLite.

## The server / engine boundary

The schema is split across a boundary that separates two concerns:

- **Server (application) side** — authentication and authorization. The
  `accounts` table lives here. It is not scoped to any one game.
- **Engine (game) side** — the game itself: `games`, `players`, `factions`,
  `entities`, the world, orders, and turn results.
- **The bridge** — the `memberships` table is the *only* table that spans both
  sides. It records that an account holds a seat at a game's table, and in which
  role.

The game engine does not need to know about accounts, and the game server does
not need to know about players. Everything that must cross the line crosses it
through `memberships`.

Note the deliberate name overlap: an account with a seat is loosely called a
"player," but the engine-side [`players`]({{< relref "/docs/reference/players.md" >}})
table holds a different thing (`display_name`, seeds, provinces) — though it is
keyed by the same id as the seat. A GM holds a membership but never gets a
`players` row — a GM controls nothing in the game.

## Conventions

These conventions hold across the schema.

### Seeds

Master and derived [seeds]({{< relref "/docs/reference/determinism.md" >}}) are
`uint64` in the engine, but SQLite `INTEGER` is a signed 64-bit value. Each seed
is stored as its **bit-cast** `int64` (Go: `int64(u)` on write, `uint64(i)` on
read), so all 64 bits round-trip. A seed near the top of the `uint64` range is
therefore stored as a negative number. Seeds live on `game_engine_state` (the
game's master seeds), `worlds` (the world's derived seeds), and `players` (a
player's private seeds).

### Booleans

A boolean is an `INTEGER` that is `NOT NULL` and constrained to `0` (false) or
`1` (true) with `CHECK (col IN (0, 1))`. Examples: `accounts.is_admin`,
`memberships.is_gm`, `turn_carryover.active`.

### Timestamps

A timestamp is an `INTEGER` holding **Unix seconds (UTC)**. Columns that record a
creation time default to `unixepoch()` (e.g. `accounts.created_at`). SQLite has
no `ON UPDATE`, so the store sets `updated_at` on every write. `sessions` carries
`issued_at`/`expires_at`/`revoked_at` in the same units.

### Display names

The `display_name` columns (`accounts`, `players`, `factions`, `entities`) are
plain `TEXT` with **no** database-level format check. Their validation — a leading
letter, then letters, digits, spaces, dashes, and quotes; UTF-8; and no characters
that could confuse JSON or enable an XSS attack — is richer than a SQLite `CHECK`
or `GLOB` can express, so it is enforced by the **API service**, not the database.
(By contrast, `games.code` has a simple uppercase-alphanumeric rule that *is* a
`CHECK`.)

### Ids

The schema **standardizes on globally-unique ids**: every surrogate id is an
`INTEGER PRIMARY KEY AUTOINCREMENT` — never scoped to a game, and **never reused**
(`AUTOINCREMENT` guarantees it, even across hard deletes). There are no per-game
id counters; an id means the same thing everywhere in the database.

- `accounts.id`, `games.id`, `memberships.id`, `factions.id`, and `entities.id`
  are each such an id. `games.id` is the target of every `game_id` foreign key;
  the game's human-facing slug is `games.code`.
- **The player_id is `memberships.id`.** `players.id` *is* the membership's id (a
  `REFERENCES memberships(id)` primary key); only a member with `is_gm = 0` gets
  a `players` row.
- Tables with a global id keep a plain `game_id` column for querying
  (`players`, `factions`, `entities`); it is not part of their key.

### Coordinates

A hex is stored as two `INTEGER` columns, `q` and `r` — never as the canonical
`"(q,r)"` string. Locations that may lie off the generated map (an entity's
location, a starting province) are still plain `q`/`r` columns and are not
foreign keys into `provinces`.

### Soft delete

Removal is a soft delete: an `inactive` flag is set to `1` and the row is kept.
Uniqueness (e.g. `players` email/display_name, a `memberships` seat) is enforced
across active **and** inactive rows, so removing and re-adding reactivates the
existing row rather than inserting a duplicate.

### Foreign keys

Foreign keys are declared throughout but SQLite only enforces them when the
connection runs `PRAGMA foreign_keys = ON;`. The store sets that on every
connection. Game-scoped tables use `ON DELETE CASCADE` from `games(id)`, so
deleting a game removes all of its rows.

### Same-game foreign keys

Because ids are global, a foreign key on a single id column does **not** by itself
keep the two rows in the same game — a child could point at a parent belonging to
a different game. Where that must not happen, the parent carries a **redundant**
`UNIQUE (game_id, id)` (redundant because `id` is already unique on its own) so
the child can foreign-key the *pair*:

```
FOREIGN KEY (game_id, <parent>_id) REFERENCES parent(game_id, id)
```

The composite FK forces the child's `game_id` to equal the parent's, catching a
cross-game reference at write time.

This guards three relationships:

- `players → memberships` — `players` foreign-keys `(game_id, id)`, so a player
  and its membership (seat) are in the same game.
- `entities → factions` — an entity and its faction are in the same game.
- `order_submissions → players` — a submission and its player are in the same
  game.

In each case the parent (`memberships`, `factions`, `players`) carries the
redundant `UNIQUE (game_id, id)`.

## Global / static tables

The migration version is not stored in a table: it is SQLite's `user_version`
pragma, managed by the ZombieZen `sqlitemigration` package and equal to the
number of migrations applied.

### `terrains`

The frozen [terrain]({{< relref "/docs/reference/world-generation.md" >}}) enum
and its Worldographer tile mapping. It is global (not game-scoped) and seeded
once.

- `code` — `PRIMARY KEY`, `0`–`6`, matching the engine's terrain enum
  (`Mountain = 0` … `Badlands = 6`). **Never renumbered.**
- `name` — the terrain name, unique.
- `worldographer_tile` — the tile name used by the terrain-translation export.

## Server-side tables

### `accounts`

An account authenticates a person with the game server. It is server-level, not
scoped to a game.

- `id` — `AUTOINCREMENT` primary key (globally unique, never reused).
- `email` — unique. The application lowercases it before saving.
- `display_name` — how the person wants to be addressed; default `''`. A
  convenience for administrators.
- `password_hash` — `NOT NULL DEFAULT '*'`. A **bcrypt** hash. `'*'` is not a
  valid bcrypt output, so an account created without an explicit hash **fails
  every login** until one is set (a fail-closed default).
- `inactive` — boolean, default `0`.
- `is_admin` — boolean, default `0`.
- `created_at`, `updated_at` — Unix-seconds timestamps, default `unixepoch()`
  (see [Timestamps](#timestamps)).

### `sessions`

A session authenticates API requests. It is minted at login and the client
presents its `token` as an opaque bearer credential.

- `id` — a public, opaque session identifier used to address a session in the
  API (e.g. to revoke one). Primary key.
- `account_id` — `→ accounts(id)`, the effective identity.
- `token` — the bearer credential, `UNIQUE`. A hex-encoded random N-bit value —
  high enough entropy that it is stored **as-is, not hashed** (unlike
  `accounts.password_hash`, which is bcrypt). The auth middleware resolves it by
  equality.
- `issued_at`, `expires_at` — Unix seconds.
- `revoked_at` — Unix seconds, or `NULL` for an active session. Sessions record
  revocation as a timestamp rather than an `inactive` flag because the API
  reasons about *when* a session ended. An active session is
  `revoked_at IS NULL AND expires_at > now`.
- Index `sessions_by_account (account_id)` for listing/revoking an account's
  sessions.

## The boundary table

### `memberships`

A seat at a game's table: the authorization that an account may participate in a
game, and in which role. This is the [boundary](#the-server--engine-boundary)
between the server and engine sides.

- `id` — `AUTOINCREMENT` primary key. **This is the engine's `player_id`**:
  globally unique, never reused, not a per-game sequence.
- `account_id` — `→ accounts(id)`, `ON DELETE CASCADE`. Required.
- `game_id` — `→ games(id)`, `ON DELETE CASCADE`. Required.
- `is_gm` — boolean, default `0`. `1` marks the game master; `0` an ordinary
  player. Exactly two mutually exclusive roles.
- `inactive` — boolean, default `0`.
- `UNIQUE (account_id, game_id)` — one seat per account per game, spanning
  active and inactive rows.
- `UNIQUE (game_id, id)` — redundant, present as the FK target that ties a
  `players` row to a membership in the same game (see
  [Same-game foreign keys](#same-game-foreign-keys)).

A membership with `is_gm = 0` is what later gains an engine-side
[`players`]({{< relref "/docs/reference/players.md" >}}) record — keyed by this
same `id` — when the account enters play; a membership with `is_gm = 1` never
does.

## Engine-side tables

### `games`

The top-level [game]({{< relref "/docs/reference/games.md" >}}) — the shared
identity every `game_id` foreign key targets. It carries only identity; the
engine's per-game state lives in [`game_engine_state`](#game_engine_state).

- `id` — `INTEGER PRIMARY KEY AUTOINCREMENT` (never reused), the surrogate key
  that every `game_id` foreign key targets.
- `code` — the game's human-facing slug (the id the GM chooses and that orders
  files carry). `UNIQUE`, `1`–`6` characters, uppercase letters and digits only
  (`CHECK (length(code) BETWEEN 1 AND 6)` and
  `CHECK (code NOT GLOB '*[^A-Z0-9]*')`).

The `game.json` manifest's file-path map has no counterpart here — in the
relational model the data *is* the tables, so there are no file locations to
record.

### `game_engine_state`

The engine's root state for one game: its master seeds and current turn. Kept
**separate from the application `games` row** so engine state stays out of the
application and server tables. One row per game.

- `game_id` — `PRIMARY KEY`, `→ games(id)`, `ON DELETE CASCADE`. One row per game.
- `seed1`, `seed2` — the game's master seeds (bit-cast; see [Seeds](#seeds)).
- `current_turn` — the current turn, default `0` (setup; play begins at `1`).

### `worlds`

The one generated [world]({{< relref "/docs/reference/world-generation.md" >}})
per game. Its seeds are derived from the game's master seeds.

- `game_id` — `PRIMARY KEY`, `→ games(id)`. One row per game.
- `seed1`, `seed2` — the world's derived seeds.
- `rings` — the ring count, `CHECK (rings > 0 AND rings < 100)`.

### `provinces`

Every hex of a world and its terrain.

- `game_id` — `→ games(id)`.
- `q`, `r` — the hex coordinates.
- `terrain` — `→ terrains(code)`.
- `PRIMARY KEY (game_id, q, r)`.

### `starting_provinces`

A game's allowed starting-province set: the provinces a player may be placed on.
Entries are unique (the primary key) and the set is unordered — list it by
`(q, r)`. A starting province must name a province of the generated world; the
composite foreign key into `provinces` enforces that.

- `game_id` — `→ games(id)`.
- `q`, `r` — the hex coordinates.
- `PRIMARY KEY (game_id, q, r)`.
- `FOREIGN KEY (game_id, q, r) → provinces(game_id, q, r)`, `ON DELETE CASCADE` —
  the entry must be a real province, and is dropped if that province is removed.

### `players`

The engine-side [player]({{< relref "/docs/reference/players.md" >}}): a
participant with an in-game handle, private seeds, and a starting province.
Distinct from a server-side account, but keyed by the same id as its
[`memberships`](#memberships) seat.

- `id` — `PRIMARY KEY`. **The global player_id** — there is one `players` row per
  player membership (`is_gm = 0`).
- `game_id` — `→ games(id)`. A plain column kept for querying.
- `FOREIGN KEY (game_id, id) → memberships(game_id, id)` — ties the player to its
  membership *and* enforces the same game (see
  [Same-game foreign keys](#same-game-foreign-keys)).
- `UNIQUE (game_id, id)` — redundant, the FK target for `order_submissions`.
- `display_name` — the player's in-game handle; the private seeds derive from it,
  so it is stable for the life of the game.
- `start_q`, `start_r` — the starting province.
- `password` — the plaintext order-authentication secret.
- `seed1`, `seed2` — the player's private seeds.
- `inactive` — boolean, default `0`.
- `UNIQUE (game_id, display_name)` — spans active and inactive rows.

There is **no** `email` column: a player's email is an `accounts` attribute,
reached through the membership (`players.id → memberships.account_id →
accounts.email`).

### `factions`

A group of entities under one controller — see
[Factions]({{< relref "/docs/reference/factions.md" >}}).

- `id` — `INTEGER PRIMARY KEY AUTOINCREMENT`. Globally unique, never reused,
  aligning with the player_id.
- `game_id` — `→ games(id)`. A plain column kept for querying.
- `display_name` — `UNIQUE (game_id, display_name)`.
- `controller_kind` — `CHECK (controller_kind IN ('player', 'npc'))`.
- `controller_id` — `CHECK (controller_id >= 1)`. It names a player or an NPC;
  because the target depends on `controller_kind` it is **not** a single foreign
  key. For a player controller it is the global player_id (`memberships.id` /
  `players.id`).
- `UNIQUE (game_id, id)` — redundant (`id` is already unique) but present so
  `entities` can foreign-key `(game_id, faction_id)` and share a game with its
  faction.

### `entities`

The actors orders act on — see
[Entities]({{< relref "/docs/reference/entities.md" >}}). Each belongs to one
faction and occupies one province (which may lie off the generated map).

- `id` — `INTEGER PRIMARY KEY AUTOINCREMENT`. Globally unique, never reused.
- `game_id` — `→ games(id)`. A plain column kept for querying.
- `display_name` — a display label; need not be unique.
- `faction_id` — the owning faction;
  `FOREIGN KEY (game_id, faction_id) → factions(game_id, id)`, so an entity and
  its faction must be in the same game.
- `loc_q`, `loc_r` — the occupied province.

## Orders

Submitted [orders]({{< relref "/docs/reference/orders" >}}) are kept both as the
verbatim text (the source of truth) and as a normalized parse.

### `order_submissions`

One player's verbatim orders text for one turn. The raw text is re-parsed by the
engine, so it is authoritative.

- `game_id` — `→ games(id)`.
- `turn`, `player_id` — the submitting player and turn;
  `FOREIGN KEY (game_id, player_id) → players(game_id, id)`, so the submission is
  in the same game as the player (see
  [Same-game foreign keys](#same-game-foreign-keys)).
- `raw` — the submission text, exactly as received.
- `PRIMARY KEY (game_id, turn, player_id)` — one submission per player per turn.

### `parsed_orders`

The normalized parse of a submission: one row per order line, grouped by the
entity block it appeared in and kept in order.

- `game_id`, `turn`, `player_id` — `FOREIGN KEY → order_submissions`,
  `ON DELETE CASCADE`.
- `entity_id` — the entity the block names.
- `seq` — the order's position within the submission.
- `command_id` — the frozen order command id (`0`–`29`).
- `word` — the command word as written.
- `line`, `col` — the 1-based source position.
- `PRIMARY KEY (game_id, turn, player_id, entity_id, seq)`.

### `parsed_order_args`

The ordered raw argument fields of a parsed order.

- `game_id`, `turn`, `player_id`, `entity_id`, `seq` — `FOREIGN KEY →
  parsed_orders`, `ON DELETE CASCADE`.
- `arg_index` — 0-based position.
- `value` — the raw field, with quotes removed.
- `PRIMARY KEY (game_id, turn, player_id, entity_id, seq, arg_index)`.

## Turn results

The result of processing a turn, per
[Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}}). These
tables are **denormalized**: outcome and carryover rows carry a flattened copy of
the order data (arguments joined into `args_text`) so the report writer reads
them without joining back to `parsed_orders`.

### `turn_results`

Marks a turn as processed — the existence of the row **is** the "already
processed" marker.

- `game_id` — `→ games(id)`.
- `turn` — the processed turn.
- `PRIMARY KEY (game_id, turn)`.

### `turn_outcomes`

The per-order outcome, in completion order.

- `game_id`, `turn` — `FOREIGN KEY → turn_results`, `ON DELETE CASCADE`.
- `seq` — completion order.
- `entity_id` — the acting entity.
- `command_id`, `word`, `args_text` — the flattened order.
- `stub` — boolean; `1` if handled by the no-op stub rather than a real handler.
- `message` — the human-readable outcome for the turn report.
- `PRIMARY KEY (game_id, turn, seq)`.

### `turn_carryover`

A per-entity order queue carried into the next turn. `active` and `ticks_left`
describe the front (active) order.

- `game_id`, `turn` — `FOREIGN KEY → turn_results`, `ON DELETE CASCADE`.
- `entity_id` — the entity whose queue this is.
- `active` — boolean; whether the front order has been activated.
- `ticks_left` — remaining ticks on the front order.
- `PRIMARY KEY (game_id, turn, entity_id)`.

### `turn_carryover_orders`

The queued orders of a carryover queue, flattened and kept in queue position.

- `game_id`, `turn`, `entity_id` — `FOREIGN KEY → turn_carryover`,
  `ON DELETE CASCADE`.
- `seq` — position in the queue.
- `command_id`, `word`, `args_text` — the flattened order.
- `line`, `col` — the order's original source position.
- `PRIMARY KEY (game_id, turn, entity_id, seq)`.

### `turn_log`

The ordered processing log of a turn, for the turn writer.

- `game_id`, `turn` — `FOREIGN KEY → turn_results`, `ON DELETE CASCADE`.
- `seq` — line order.
- `message` — the log line.
- `PRIMARY KEY (game_id, turn, seq)`.
