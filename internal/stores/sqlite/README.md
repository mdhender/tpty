# internal/stores/sqlite

Design notes for the SQLite storage backend.

> **Work in progress.** We are still iterating on the schema and the surrounding
> design. Treat this as a notebook, not a settled spec.

This store backs the **server** (`tapp`). The **engine** is expected to keep its
own local store — most likely the existing JSON file model — for a game master's
working state and step-by-step history while running a turn; `tpty` reconciles
that local state into this database on push. This split is not yet committed
(see Open questions).

## Files

- `schema.sql` — the authoritative DDL.
- Reference doc: `content/docs/reference/sql-schema.md` (the relational model and
  the server/engine boundary).

## Driver

[ZombieZen](https://pkg.go.dev/zombiezen.com/go/sqlite) (`zombiezen.com/go/sqlite`),
pure Go. Migrations use the ZombieZen migration package.

## Connection management

This package provides and manages the connection pools. It ensures that:

- every connection enables foreign keys (`PRAGMA foreign_keys = ON`) and WAL;
- in-memory instances are supported for testing.

## Opening an instance

- `OpenPersistent(ctx, path)` — `path` is the **directory** that holds the
  instance, **not** the file name. The package owns the file name under it.
- `OpenTemporary(ctx, name)` — an in-memory instance.
  - `name == ""` → a fresh, unique instance (no one else can reach it).
  - `name != ""` → a shareable instance; another caller that passes the same
    `name` reaches the same database.

## Migration policy

- Opening a persistent instance **almost always auto-migrates up**.
  - Exception: some `tdb` commands that must not alter the instance
    (`backup`, `compact`, `version`).
- **No migrate-down, ever.** To go back, the operator restores from a backup.
  (That is why backups are the operator's responsibility.)
- The migration version is SQLite's `PRAGMA user_version`, managed by the
  ZombieZen `sqlitemigration` package (it equals the number of migrations
  applied). There is no version table.
- Open **fails if the instance's migration version is newer than the caller's**
  compiled-in version — running against a future schema would break things.
  `sqlitemigration` does not do this itself (it only migrates up and no-ops when
  the DB is already at/above the known version), so the store adds a post-open
  `user_version != expected` guard.
  - Exception: the non-altering `tdb` commands (`backup`, `compact`, `version`).
- `tdb` assumes it is the **only** process accessing the database during a
  migration.

## Executables (consumers of this package)

Three separate CLIs sit on top of this store. They split along the
[server/engine boundary](../../../content/docs/reference/sql-schema.md).

### `tdb` — database administration (operator tool)

Assumes it is the only process touching the database during migrations.

- create + migrate a new instance (single action)
- backup an instance
- compact an instance
- print the application version
- print the database migration version
- verify a migration
- migrate up (never down)
- create accounts

### `tapp` — application server

- start the server; **opens (never creates)** an instance — persistent, or
  in-memory for testing
- catches signals and shuts down gracefully
- print the application version
- print the database migration version
- authn/authz middleware and a RESTish API (control the server, manage
  sessions, …)
- **no game-engine logic yet**
- owns the database instance while it is running; uses WAL

### `tpty` — client

Drives the server over the RESTish API; the engine logic lives here initially
(in `internal/` packages, so it stays client-agnostic).

- **Raw HTTP verbs** (like `earl`): `get`, `put`, `post`, `patch`, `delete` — a
  positional PATH plus `-d` for a body (`-d @file.json` reads a file). These let
  us work the API directly while iterating; the convenience commands follow.
- **Convenience commands** (sugar over the verbs): `ping`, `whoami`, `login`,
  `logout`. `whoami` ≈ `get /me`; `login` exchanges email + secret for a bearer
  token and saves it; `logout` revokes and forgets it.
- **Engine commands** (still being defined): manage players, report on game
  state, run reports, run the next phase, revert a phase, then push the result
  back to the database.

`tpty` takes only a subset of `earl`: the HTTP verbs plus the
`ping`/`whoami`/`login`/`logout` sugar. `earl`'s admin extras (e.g.
`impersonate`) are deliberately omitted for now.

`tpty` will likely work with our JSON data structures locally, but that is not
committed — we may find a better shape as we iterate.

## Open questions / TODO

- Engine's local store: likely the existing JSON file model, carrying working
  state plus per-step history so a GM can run and roll back before pushing. Not
  committed.
- Two rollback granularities: the high-level game flow
  (`setup -> ((load orders -> execute)+ -> report)+ -> close game`) and the
  local tick loop. "Revert a phase" must undo whatever chunk was snapshotted.
  The simplest model is a snapshot (a folder of JSON) per step — automate that
  away, but keep it as the mental anchor.
- "Phase" vs. "tick": turn-processing.md models a turn as 32 ticks; whether a
  GM-facing "phase" is a tick or a coarser grouping is unsettled.
