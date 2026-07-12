---
title: Administer a database with tdb
weight: 11
---

This guide walks an [operator]({{< relref "/docs/reference/glossary.md" >}})
through administering a T'Pty SQLite database with `tdb`: creating and migrating
a database, checking its versions, creating an account, backing it up, and
compacting it. For the relational model behind the database, see
[SQL Schema]({{< relref "/docs/reference/sql-schema.md" >}}).

Two conventions run through every command:

- `--path` is the **directory** that holds the instance, not a file name. `tdb`
  owns the file (`tpty.db`) beneath it. The directory must already exist — `tdb`
  never creates directories.
- `tdb` assumes it is the **only** process touching the database during a
  migration or compaction. Stop the server first for `migrate up` and `compact`.

Every flag also resolves from a `TDB_`-prefixed environment variable (for
example `--secret` from `TDB_SECRET`); a flag given on the command line wins.

## Create a database

Create and migrate a new database in an existing directory:

```sh
tdb create database --path path/to/instance
```

This creates `tpty.db` under the directory and migrates it to the current
schema. It refuses to run if an instance is already there — use `migrate up` to
bring an existing one forward.

## Migrate an existing database up

After deploying a binary that carries new migrations, bring an existing database
up to the current schema:

```sh
tdb migrate up --path path/to/instance
```

There is **no migrate-down**. To go back, restore from a backup — so take one
first (see [Back up a database](#back-up-a-database)). Stop the server before
migrating.

## Check versions

Print the application (binary) version:

```sh
tdb version
```

Print the database's on-disk schema version alongside the version the binary
expects, so you can see whether a `migrate up` is due:

```sh
tdb migrate version --path path/to/instance
```

To assert the two match — for a deploy script or health check — use `verify`,
which exits non-zero when they differ:

```sh
tdb migrate verify --path path/to/instance
```

## Create an account

An account is a person's **server login**. Create one with:

```sh
tdb create account --path path/to/instance --email alice@example.com
```

- `--email` is required and is lowercased before saving; it must be unique.
- `--secret` sets the password (or set `TDB_SECRET`). If you omit it, `tdb`
  generates a passphrase and **prints it once** — copy it then, as it is not
  stored in the clear or shown again.
- `--display-name` defaults to `anonymous account`.
- `--is-admin` makes the account an administrator (default: not).

The account password is bcrypt-hashed and is separate from a player's in-game
order password — see [Players]({{< relref "/docs/reference/players.md" >}}) and
[SQL Schema]({{< relref "/docs/reference/sql-schema.md" >}}).

## Back up a database

Write a consistent, compacted copy of the database into a folder:

```sh
tdb backup --path path/to/instance --output-path path/to/backups
```

The backup is named `tpty.db.<timestamp-utc>` (for example
`tpty.db.20260712T213106Z`); you choose only the folder, never the file name.
`--output-path` must already exist and defaults to the database's own folder
when omitted. The backup does not modify the source, so it is safe to run while
the server is up.

## Compact a database

Reclaim free space left by deletions:

```sh
tdb compact --path path/to/instance
```

This runs `VACUUM`, rewriting the database in place. It is an **offline**
operation — stop the server first.
