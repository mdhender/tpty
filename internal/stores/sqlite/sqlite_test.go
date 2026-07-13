// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestCreateAndOpenPersistent covers the split between the sole creator and the
// open-existing path: CreatePersistent brings a fresh instance into being under
// an existing directory and migrates it to ExpectedVersion; OpenPersistent then
// re-opens that existing instance.
func TestCreateAndOpenPersistent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := CreatePersistent(ctx, dir)
	if err != nil {
		t.Fatalf("CreatePersistent: %v", err)
	}

	if v, err := db.UserVersion(ctx); err != nil {
		t.Fatalf("UserVersion: %v", err)
	} else if v != ExpectedVersion() {
		t.Errorf("user_version = %d, want ExpectedVersion %d", v, ExpectedVersion())
	}
	if _, err := os.Stat(filepath.Join(dir, dbFilename)); err != nil {
		t.Errorf("expected %s on disk: %v", dbFilename, err)
	}
	// The schema migrated: a known table exists.
	if !tableExists(t, db, "games") {
		t.Error("games table missing after migrate")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// OpenPersistent re-opens the now-existing instance.
	reopened, err := OpenPersistent(ctx, dir)
	if err != nil {
		t.Fatalf("OpenPersistent on an existing instance: %v", err)
	}
	defer reopened.Close()
	if v, err := reopened.UserVersion(ctx); err != nil {
		t.Fatalf("UserVersion: %v", err)
	} else if v != ExpectedVersion() {
		t.Errorf("reopened user_version = %d, want %d", v, ExpectedVersion())
	}
}

// TestOpenPersistentNeverCreatesFile is the core guarantee: given an existing
// but empty directory, OpenPersistent must fail rather than create a fresh
// database file. Creation belongs solely to CreatePersistent.
func TestOpenPersistentNeverCreatesFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir() // exists, but holds no instance

	db, err := OpenPersistent(ctx, dir)
	if err == nil {
		db.Close()
		t.Fatal("OpenPersistent on an empty directory = nil error, want ErrNotExist")
	}
	if !errors.Is(err, ErrNotExist) {
		t.Errorf("error = %v, want ErrNotExist", err)
	}

	// Nothing may have been created — not the database file nor its WAL companions.
	for _, name := range []string{dbFilename, dbFilename + "-wal", dbFilename + "-shm"} {
		if _, statErr := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(statErr) {
			t.Errorf("OpenPersistent created %s; it must never create a file", name)
		}
	}
}

// TestCreatePersistent covers the creator's own guards: it refuses to overwrite
// an existing instance, and it will not create the directory.
func TestCreatePersistent(t *testing.T) {
	ctx := context.Background()

	t.Run("refuses an existing instance", func(t *testing.T) {
		dir := t.TempDir()
		db, err := CreatePersistent(ctx, dir)
		if err != nil {
			t.Fatalf("CreatePersistent: %v", err)
		}
		db.Close()

		if again, err := CreatePersistent(ctx, dir); err == nil {
			again.Close()
			t.Fatal("CreatePersistent over an existing instance = nil error, want failure")
		}
	})

	t.Run("does not create the directory", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope")
		if _, err := CreatePersistent(ctx, missing); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("CreatePersistent on a missing directory = %v, want ErrInvalidPath", err)
		}
		if _, statErr := os.Stat(missing); !os.IsNotExist(statErr) {
			t.Errorf("CreatePersistent created %s; it must not create the directory", missing)
		}
	})
}

// TestOpenPersistentRequiresExistingDir confirms OpenPersistent is a hard error
// when the directory does not exist, and — critically — does NOT create it (or
// any parent) on the fly. Auto-creating directories would let a stray path
// scatter empty database instances across the filesystem.
func TestOpenPersistentRequiresExistingDir(t *testing.T) {
	ctx := context.Background()

	t.Run("missing directory", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "does-not-exist")

		db, err := OpenPersistent(ctx, missing)
		if err == nil {
			db.Close()
			t.Fatal("OpenPersistent on a missing directory = nil error, want ErrInvalidPath")
		}
		if !errors.Is(err, ErrInvalidPath) {
			t.Errorf("error = %v, want ErrInvalidPath", err)
		}
		if _, statErr := os.Stat(missing); !os.IsNotExist(statErr) {
			t.Errorf("OpenPersistent created %s; it must not create the directory", missing)
		}
	})

	t.Run("missing nested path is not created", func(t *testing.T) {
		base := t.TempDir()
		nested := filepath.Join(base, "a", "b", "c")

		if _, err := OpenPersistent(ctx, nested); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("OpenPersistent on a missing nested path = %v, want ErrInvalidPath", err)
		}
		if _, statErr := os.Stat(filepath.Join(base, "a")); !os.IsNotExist(statErr) {
			t.Errorf("OpenPersistent created intermediate directories under %s; it must not", base)
		}
	})

	t.Run("path is a file, not a directory", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if _, err := OpenPersistent(ctx, file); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("OpenPersistent on a file path = %v, want ErrInvalidPath", err)
		}
	})
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
	db, err := CreatePersistent(ctx, dir)
	if err != nil {
		t.Fatalf("CreatePersistent: %v", err)
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

// TestBackupAndCompact covers the store-owned backup and compact operations: the
// backup file is named for the instance with a timestamp suffix, lands in the
// chosen (or defaulted) folder, never in a missing one, and compact runs in
// place.
func TestBackupAndCompact(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := CreatePersistent(ctx, dir)
	if err != nil {
		t.Fatalf("CreatePersistent: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	t.Run("backup into an explicit folder", func(t *testing.T) {
		dest := t.TempDir()
		target, err := Backup(ctx, dir, dest)
		if err != nil {
			t.Fatalf("Backup: %v", err)
		}
		// The caller chose only the folder; the store names the file.
		wantPrefix := filepath.Join(dest, dbFilename+".")
		if !strings.HasPrefix(target, wantPrefix) {
			t.Errorf("target = %q, want prefix %q", target, wantPrefix)
		}
		if fi, err := os.Stat(target); err != nil {
			t.Fatalf("stat backup: %v", err)
		} else if fi.Size() == 0 {
			t.Errorf("backup %s is empty", target)
		}
	})

	t.Run("backup defaults to the instance's own folder", func(t *testing.T) {
		target, err := Backup(ctx, dir, "")
		if err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if got := filepath.Dir(target); got != dir {
			t.Errorf("default backup landed in %s, want %s", got, dir)
		}
	})

	t.Run("backup into a missing folder errors and creates nothing", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope")
		if _, err := Backup(ctx, dir, missing); err == nil {
			t.Fatal("Backup into a missing folder = nil, want error")
		}
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Errorf("Backup created %s; it must not", missing)
		}
	})

	t.Run("compact runs in place", func(t *testing.T) {
		if err := Compact(ctx, dir); err != nil {
			t.Fatalf("Compact: %v", err)
		}
	})

	t.Run("backup and compact reject a missing instance", func(t *testing.T) {
		empty := t.TempDir()
		if _, err := Backup(ctx, empty, empty); !errors.Is(err, ErrNotExist) {
			t.Errorf("Backup on an empty dir = %v, want ErrNotExist", err)
		}
		if err := Compact(ctx, empty); !errors.Is(err, ErrNotExist) {
			t.Errorf("Compact on an empty dir = %v, want ErrNotExist", err)
		}
	})
}

// TestOpenTimesOutOnDoomedFile pins the safety net: a file that exists (so it
// passes the precondition checks) but is not a valid database must make the
// migrating open time out and return an error, rather than hang forever on
// sqlitemigration's infinite open-retry.
func TestOpenTimesOutOnDoomedFile(t *testing.T) {
	dir := t.TempDir()
	// A non-SQLite file at the instance's path: preconditions pass, migration fails.
	if err := os.WriteFile(filepath.Join(dir, dbFilename), []byte("not a database"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Shorten the safety-net timeout so the test does not wait 30s.
	restore := openTimeout
	openTimeout = 500 * time.Millisecond
	defer func() { openTimeout = restore }()

	start := time.Now()
	db, err := OpenPersistent(context.Background(), dir)
	if err == nil {
		db.Close()
		t.Fatal("OpenPersistent on a doomed file = nil, want a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("OpenPersistent took %s; the open-timeout did not bound it", elapsed)
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

// TestValidateDisplayName pins the display-name rule from sql-schema.md: a
// leading letter, then letters, digits, spaces, dashes, and apostrophes.
func TestValidateDisplayName(t *testing.T) {
	valid := []string{"Alice", "anonymous account", "O'Brien", "Anne-Marie", "Bob 3rd", "Zoe"}
	for _, s := range valid {
		if err := ValidateDisplayName(s); err != nil {
			t.Errorf("ValidateDisplayName(%q) = %v, want nil", s, err)
		}
	}
	invalid := []string{"", " leading", "3names", "-dash", "bad<script>", "quote\"here", "tab\tchar"}
	for _, s := range invalid {
		if err := ValidateDisplayName(s); !errors.Is(err, ErrInvalidDisplayName) {
			t.Errorf("ValidateDisplayName(%q) = %v, want ErrInvalidDisplayName", s, err)
		}
	}
}

// TestUpdateAccountErrors covers UpdateAccount's own guards: a missing account,
// an invalid new display name, and an empty change set.
func TestUpdateAccountErrors(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := CreatePersistent(ctx, dir)
	if err != nil {
		t.Fatalf("CreatePersistent: %v", err)
	}
	db.Close()
	if _, err := CreateAccount(ctx, dir, "a@example.com", "Alice", "hash", false, false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	inact := true
	if err := UpdateAccount(ctx, dir, "nobody@example.com", AccountUpdate{Inactive: &inact}); !errors.Is(err, ErrNoAccount) {
		t.Errorf("UpdateAccount(missing) = %v, want ErrNoAccount", err)
	}

	bad := "1nope"
	if err := UpdateAccount(ctx, dir, "a@example.com", AccountUpdate{NewDisplayName: &bad}); !errors.Is(err, ErrInvalidDisplayName) {
		t.Errorf("UpdateAccount(bad display) = %v, want ErrInvalidDisplayName", err)
	}

	if err := UpdateAccount(ctx, dir, "a@example.com", AccountUpdate{}); err == nil {
		t.Error("UpdateAccount with no changes = nil, want error")
	}
}
