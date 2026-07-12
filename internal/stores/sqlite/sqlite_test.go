// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// seedGameSQL builds one FK-complete game (game_id = 1) touching every
// game-scoped table, so a teardown test can prove the whole graph cascades. It
// respects the schema's foreign keys statement-by-statement (foreign_keys is ON,
// so each is checked immediately).
const seedGameSQL = `
INSERT INTO terrains (code, name, worldographer_tile) VALUES (1, 'Plains', 'Plains');

INSERT INTO accounts (id, email) VALUES (1, 'a@example.com');

INSERT INTO games (id, code) VALUES (1, 'G1');

INSERT INTO game_engine_state (game_id, seed1, seed2, current_turn) VALUES (1, 10, 20, 1);

INSERT INTO worlds (game_id, seed1, seed2, rings) VALUES (1, 30, 40, 10);
INSERT INTO provinces (game_id, q, r, terrain) VALUES (1, 0, 0, 1);
INSERT INTO starting_provinces (game_id, q, r) VALUES (1, 0, 0);

INSERT INTO memberships (id, account_id, game_id, is_gm) VALUES (1, 1, 1, 0);
INSERT INTO players (id, game_id, display_name, start_q, start_r, password, seed1, seed2)
    VALUES (1, 1, 'P1', 0, 0, 'secret', 50, 60);

INSERT INTO factions (id, game_id, display_name, controller_kind, controller_id)
    VALUES (1, 1, 'F1', 'player', 1);
INSERT INTO entities (id, game_id, display_name, faction_id, loc_q, loc_r)
    VALUES (1, 1, 'E1', 1, 0, 0);

INSERT INTO order_submissions (game_id, turn, player_id, raw) VALUES (1, 1, 1, 'orders');
INSERT INTO parsed_orders (game_id, turn, player_id, entity_id, seq, command_id, word, line, col)
    VALUES (1, 1, 1, 1, 1, 0, 'noop', 1, 1);
INSERT INTO parsed_order_args (game_id, turn, player_id, entity_id, seq, arg_index, value)
    VALUES (1, 1, 1, 1, 1, 0, 'arg');

INSERT INTO turn_results (game_id, turn) VALUES (1, 1);
INSERT INTO turn_outcomes (game_id, turn, seq, entity_id, command_id, word, args_text, stub, message)
    VALUES (1, 1, 1, 1, 0, 'noop', '', 1, 'ok');
INSERT INTO turn_carryover (game_id, turn, entity_id, active, ticks_left) VALUES (1, 1, 1, 0, 0);
INSERT INTO turn_carryover_orders (game_id, turn, entity_id, seq, command_id, word, args_text, line, col)
    VALUES (1, 1, 1, 1, 0, 'noop', '', 1, 1);
INSERT INTO turn_log (game_id, turn, seq, message) VALUES (1, 1, 1, 'log line');
`

// gameScopedTables are every table a game owns; all must be empty after the
// game is torn down.
var gameScopedTables = []string{
	"games", "game_engine_state", "worlds", "provinces", "starting_provinces",
	"memberships", "players", "factions", "entities",
	"order_submissions", "parsed_orders", "parsed_order_args",
	"turn_results", "turn_outcomes", "turn_carryover", "turn_carryover_orders", "turn_log",
}

// TestFullGameTeardown pins the NO ACTION same-game composite foreign keys
// (players -> memberships, entities -> factions, order_submissions -> players):
// DELETE FROM games must cascade every game-scoped table clean, while a direct
// delete of a still-referenced parent must be blocked. See the callout in
// content/docs/reference/sql-schema.md.
func TestFullGameTeardown(t *testing.T) {
	t.Run("delete games cascades every table", func(t *testing.T) {
		db := openTempT(t, "")
		seed(t, db)

		execT(t, db, "DELETE FROM games WHERE id = 1;")

		for _, table := range gameScopedTables {
			if n := countT(t, db, table); n != 0 {
				t.Errorf("after DELETE FROM games, %s has %d rows, want 0", table, n)
			}
		}
	})

	t.Run("direct delete of a referenced parent is blocked", func(t *testing.T) {
		db := openTempT(t, "")
		seed(t, db)

		// players row 1 references memberships row 1 via the NO ACTION composite
		// FK, so deleting the membership directly must fail.
		if err := exec(db, "DELETE FROM memberships WHERE id = 1;"); err == nil {
			t.Fatal("deleting a membership a player still references should be blocked, got no error")
		}

		// The graph is intact: nothing was deleted.
		if n := countT(t, db, "memberships"); n != 1 {
			t.Errorf("memberships has %d rows after a blocked delete, want 1", n)
		}
		if n := countT(t, db, "players"); n != 1 {
			t.Errorf("players has %d rows after a blocked delete, want 1", n)
		}
	})
}

// TestOpenPersistent covers create + migrate: a fresh directory is migrated to
// ExpectedVersion, the file is created under the directory, and the schema is
// present.
func TestOpenPersistent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := OpenPersistent(ctx, dir)
	if err != nil {
		t.Fatalf("OpenPersistent: %v", err)
	}
	defer db.Close()

	if v, err := db.UserVersion(ctx); err != nil {
		t.Fatalf("UserVersion: %v", err)
	} else if v != ExpectedVersion() {
		t.Errorf("user_version = %d, want ExpectedVersion %d", v, ExpectedVersion())
	}

	if _, err := os.Stat(filepath.Join(dir, dbFilename)); err != nil {
		t.Errorf("expected %s on disk: %v", dbFilename, err)
	}

	// The schema migrated: a known table exists.
	if n := countT(t, db, "sqlite_master"); n == 0 {
		t.Error("sqlite_master is empty; schema did not migrate")
	}
	if !tableExists(t, db, "games") {
		t.Error("games table missing after migrate")
	}
}

// TestOpenTemporaryUnique confirms two unnamed temporary instances are isolated.
func TestOpenTemporaryUnique(t *testing.T) {
	db1 := openTempT(t, "")
	db2 := openTempT(t, "")

	execT(t, db1, "INSERT INTO games (id, code) VALUES (1, 'A');")

	if n := countT(t, db2, "games"); n != 0 {
		t.Errorf("second unique instance sees %d games, want 0 (should be isolated)", n)
	}
	if n := countT(t, db1, "games"); n != 1 {
		t.Errorf("first instance sees %d games, want 1", n)
	}
}

// TestOpenTemporaryShared confirms two instances opened with the same name share
// one database.
func TestOpenTemporaryShared(t *testing.T) {
	db1 := openTempT(t, "shared-db")
	db2 := openTempT(t, "shared-db")

	execT(t, db1, "INSERT INTO games (id, code) VALUES (1, 'A');")

	if n := countT(t, db2, "games"); n != 1 {
		t.Errorf("second shared-by-name instance sees %d games, want 1 (should share)", n)
	}
}

// TestNewerVersionGuard confirms the migrating open refuses a database whose
// on-disk user_version is newer than ExpectedVersion.
func TestNewerVersionGuard(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Create and migrate normally.
	db, err := OpenPersistent(ctx, dir)
	if err != nil {
		t.Fatalf("OpenPersistent: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Forge a newer version with a non-migrating open (which does not touch it).
	bump, err := OpenNonMigrating(ctx, dir)
	if err != nil {
		t.Fatalf("OpenNonMigrating: %v", err)
	}
	execT(t, bump, "PRAGMA user_version = 999;")
	if err := bump.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The migrating open must now refuse it.
	if _, err := OpenPersistent(ctx, dir); err == nil {
		t.Fatal("OpenPersistent accepted a database newer than ExpectedVersion, want error")
	}
}

// TestNonMigratingLeavesUntouched confirms the non-migrating open neither
// migrates the instance nor applies the newer-than guard: a hand-made empty
// (version-0) database opens without error and stays at version 0.
func TestNonMigratingLeavesUntouched(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Create an empty database file at user_version 0, without migrating it.
	conn, err := sqlite.OpenConn(filepath.Join(dir, dbFilename), sqlite.OpenReadWrite|sqlite.OpenCreate)
	if err != nil {
		t.Fatalf("OpenConn: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("conn.Close: %v", err)
	}

	db, err := OpenNonMigrating(ctx, dir)
	if err != nil {
		t.Fatalf("OpenNonMigrating on an empty database: %v", err)
	}
	defer db.Close()

	if v, err := db.UserVersion(ctx); err != nil {
		t.Fatalf("UserVersion: %v", err)
	} else if v != 0 {
		t.Errorf("user_version = %d, want 0 (non-migrating open must not migrate)", v)
	}
}

// --- helpers ---------------------------------------------------------------

// openTempT opens a temporary instance and registers its cleanup.
func openTempT(t *testing.T, name string) *DB {
	t.Helper()
	db, err := OpenTemporary(context.Background(), name)
	if err != nil {
		t.Fatalf("OpenTemporary(%q): %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// seed loads the FK-complete game fixture.
func seed(t *testing.T, db *DB) {
	t.Helper()
	conn, err := db.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer db.Put(conn)
	if err := sqlitex.ExecuteScript(conn, seedGameSQL, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// exec runs a single statement, returning any error (used to assert a blocked
// delete).
func exec(db *DB, query string) error {
	conn, err := db.Get(context.Background())
	if err != nil {
		return err
	}
	defer db.Put(conn)
	return sqlitex.ExecuteTransient(conn, query, nil)
}

// execT runs a single statement and fails the test on error.
func execT(t *testing.T, db *DB, query string) {
	t.Helper()
	if err := exec(db, query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// countT returns the row count of a table.
func countT(t *testing.T, db *DB, table string) int {
	t.Helper()
	conn, err := db.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer db.Put(conn)

	var n int
	err = sqlitex.ExecuteTransient(conn, "SELECT COUNT(*) FROM "+table+";", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			n = stmt.ColumnInt(0)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// tableExists reports whether a user table of the given name exists.
func tableExists(t *testing.T, db *DB, name string) bool {
	t.Helper()
	conn, err := db.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer db.Put(conn)

	var found bool
	err = sqlitex.ExecuteTransient(conn,
		"SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?;",
		&sqlitex.ExecOptions{
			Args:       []any{name},
			ResultFunc: func(stmt *sqlite.Stmt) error { found = true; return nil },
		})
	if err != nil {
		t.Fatalf("tableExists %s: %v", name, err)
	}
	return found
}
