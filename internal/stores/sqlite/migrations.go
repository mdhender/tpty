// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package sqlite

import _ "embed"

// AppID identifies a T'Pty database file. It is written to SQLite's
// application_id header by sqlitemigration, and the migration package refuses to
// open a file carrying a different id — a guard against pointing the store at an
// unrelated SQLite database. The value is the ASCII bytes "tpty" (0x74707479),
// so it is recognizable in a hex dump of the file. It must never change.
const AppID int32 = 0x74707479 // "tpty"

// schemaSQL is the authoritative DDL, migration 0001. While the schema is still
// iterating we keep a single migration file and re-baseline it; append-only,
// never-edit migrations begin once we ship a real database. See
// internal/stores/sqlite/README.md.
//
//go:embed schema.sql
var schemaSQL string

// Migrations is the ordered list of SQL scripts that build the schema, one per
// version. The migration version is SQLite's PRAGMA user_version and equals the
// number of migrations applied (there is no version table). schema.sql is
// migration 0001, so a fully-migrated database reports user_version == 1.
var Migrations = []string{
	schemaSQL, // 0001
}

// ExpectedVersion is the migration version a binary built against this package
// expects: the number of migrations it carries. A persistent database whose
// on-disk user_version exceeds this was written by a newer binary, and the
// migrating opens refuse it (sqlitemigration only migrates up).
func ExpectedVersion() int {
	return len(Migrations)
}
