// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Package sqlite is the SQLite storage backend for T'Pty. It manages the
// connection pool and migrations for the one shared database that holds every
// game, and is the foundation both the tdb operator CLI and the tapp server
// build on.
//
// # Opening an instance
//
// Three open modes cover the callers' needs. All of them enable foreign-key
// enforcement (PRAGMA foreign_keys = ON) on every connection, which the schema's
// foreign keys require and SQLite leaves off by default.
//
//   - [OpenPersistent] opens the on-disk instance in a directory (the package
//     owns the file name under it), creating and migrating it up as needed, in
//     WAL mode.
//   - [OpenTemporary] opens an in-memory instance and migrates it up — unique
//     when unnamed, or shared by name — chiefly for tests.
//   - [OpenNonMigrating] opens an existing file without migrating it, for the tdb
//     commands that must not alter the instance (backup, compact, version).
//
// All three return a [DB]: borrow a connection with [DB.Get], return it with
// [DB.Put], and [DB.Close] the pool when done.
//
// # Migrations and versioning
//
// The migration version is SQLite's user_version pragma, managed by the
// zombiezen sqlitemigration package; it equals the number of migrations applied,
// and there is no version table. [Migrations] is the ordered list of SQL scripts
// (schema.sql is migration 0001), and [ExpectedVersion] is the version a binary
// built against this package expects.
//
// sqlitemigration only migrates up: a database already at or above the versions
// it knows is left untouched, with no error. The migrating opens therefore add a
// post-open guard that refuses an instance whose on-disk user_version is newer
// than [ExpectedVersion] — running against a future schema would silently
// misbehave. There is deliberately no migrate-down; to go back, the operator
// restores from a backup.
//
// The relational model is documented in content/docs/reference/sql-schema.md and
// the design notes in this package's README.md; schema.sql is the authoritative
// DDL.
package sqlite
