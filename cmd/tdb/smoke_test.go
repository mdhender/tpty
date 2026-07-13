// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdhender/tpty/internal/stores/sqlite"
	"golang.org/x/crypto/bcrypt"
	zsqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// TestSmokeEndToEnd drives the tdb operator loop through the real command impl
// functions in a throwaway directory: create → verify → create-account →
// backup → compact → migrate. It also pins the create/migrate new-vs-existing
// guards, the bcrypt hashing and email lowercasing of create-account, and the
// backup target guard.
func TestSmokeEndToEnd(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// create → a fresh instance migrated to the expected version.
	if err := createInstance(ctx, dir); err != nil {
		t.Fatalf("createInstance = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tpty.db")); err != nil {
		t.Fatalf("tpty.db not created: %v", err)
	}

	// create again → refused (an instance already exists).
	if err := createInstance(ctx, dir); err == nil {
		t.Fatal("createInstance on an existing instance = nil, want error")
	}

	// verify → version matches expected.
	if err := verifyInstance(ctx, dir); err != nil {
		t.Fatalf("verifyInstance = %v, want nil", err)
	}

	// create account → the email is lowercased and the secret is bcrypt-hashed.
	if err := createAccount(ctx, dir, "Alice@Example.com", "Alice", true, false, "hunter2"); err != nil {
		t.Fatalf("createAccount = %v, want nil", err)
	}
	hash, isAdmin, displayName, _ := readAccount(t, ctx, dir, "alice@example.com")
	if hash == "" {
		t.Fatal("account alice@example.com not found (email not lowercased?)")
	}
	if isAdmin != 1 {
		t.Errorf("is_admin = %d, want 1", isAdmin)
	}
	if displayName != "Alice" {
		t.Errorf("display_name = %q, want %q", displayName, "Alice")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("hunter2")); err != nil {
		t.Errorf("stored password_hash does not verify against the secret: %v", err)
	}
	if hash == "hunter2" {
		t.Error("password stored in plaintext, want a bcrypt hash")
	}

	// create account → an omitted secret is generated (not an error) and stored
	// as a real bcrypt hash; an omitted display name defaults to "anonymous
	// account".
	if err := createAccount(ctx, dir, "carol@example.com", "", false, false, ""); err != nil {
		t.Fatalf("createAccount with a generated secret = %v, want nil", err)
	}
	carolHash, _, carolName, _ := readAccount(t, ctx, dir, "carol@example.com")
	if !strings.HasPrefix(carolHash, "$2") {
		t.Errorf("generated-secret account hash = %q, want a bcrypt hash", carolHash)
	}
	if carolName != "anonymous account" {
		t.Errorf("default display_name = %q, want %q", carolName, "anonymous account")
	}

	// create account → duplicate email is rejected by the unique constraint.
	if err := createAccount(ctx, dir, "alice@example.com", "", false, false, "x"); err == nil {
		t.Fatal("createAccount with a duplicate email = nil, want error")
	}

	// create account → a missing email is rejected.
	if err := createAccount(ctx, dir, "", "", false, false, "x"); err == nil {
		t.Fatal("createAccount without an email = nil, want error")
	}

	// backup → writes a timestamped tpty.db.<stamp> into the chosen folder; the
	// caller never picks the file name.
	backupDir := t.TempDir()
	if err := backupInstance(ctx, dir, backupDir); err != nil {
		t.Fatalf("backupInstance = %v, want nil", err)
	}
	backups, err := filepath.Glob(filepath.Join(backupDir, "tpty.db.*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("found %d backup files matching tpty.db.*, want 1: %v", len(backups), backups)
	}
	if fi, err := os.Stat(backups[0]); err != nil {
		t.Fatalf("stat backup: %v", err)
	} else if fi.Size() == 0 {
		t.Fatalf("backup file %s is empty", backups[0])
	}

	// backup with no folder → defaults to the database's own folder.
	if err := backupInstance(ctx, dir, ""); err != nil {
		t.Fatalf("backupInstance with default folder = %v, want nil", err)
	}
	if got, err := filepath.Glob(filepath.Join(dir, "tpty.db.*")); err != nil {
		t.Fatalf("glob: %v", err)
	} else if len(got) != 1 {
		t.Fatalf("default-folder backup wrote %d files, want 1: %v", len(got), got)
	}

	// backup into a folder that does not exist → refused, nothing created.
	missing := filepath.Join(t.TempDir(), "missing")
	if err := backupInstance(ctx, dir, missing); err == nil {
		t.Fatal("backupInstance into a missing folder = nil, want error")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("backup created folder %s; it must not", missing)
	}

	// compact → succeeds and leaves the instance verifiable.
	if err := compactInstance(ctx, dir); err != nil {
		t.Fatalf("compactInstance = %v, want nil", err)
	}
	if err := verifyInstance(ctx, dir); err != nil {
		t.Fatalf("verifyInstance after compact = %v, want nil", err)
	}

	// migrate → an existing instance migrates up (a no-op at the current version).
	if err := migrateInstance(ctx, dir); err != nil {
		t.Fatalf("migrateInstance = %v, want nil", err)
	}

	// version → the application version, no instance needed.
	if err := showVersion(); err != nil {
		t.Fatalf("showVersion = %v, want nil", err)
	}

	// migrate version → the database schema version.
	if err := showSchemaVersion(ctx, dir); err != nil {
		t.Fatalf("showSchemaVersion = %v, want nil", err)
	}
}

// TestMigrateAndVerifyRejectMissing pins that operations needing an existing
// instance fail on an empty directory.
func TestMigrateAndVerifyRejectMissing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	if err := migrateInstance(ctx, dir); err == nil {
		t.Error("migrateInstance on an empty directory = nil, want error")
	}
	if err := verifyInstance(ctx, dir); err == nil {
		t.Error("verifyInstance on an empty directory = nil, want error")
	}
	if err := createAccount(ctx, dir, "a@example.com", "", false, false, "x"); err == nil {
		t.Error("createAccount on an empty directory = nil, want error")
	}
}

// TestNoCommandCreatesDirectories pins the invariant that no tdb command brings
// a directory into being: create/migrate/verify/create-account/backup/compact
// against paths whose directories do not exist must all fail without creating
// anything.
func TestNoCommandCreatesDirectories(t *testing.T) {
	ctx := context.Background()

	t.Run("create refuses a missing directory", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope")
		if err := createInstance(ctx, missing); err == nil {
			t.Fatal("createInstance on a missing directory = nil, want error")
		}
		mustNotExist(t, missing)
	})

	t.Run("backup refuses a missing output folder and creates nothing", func(t *testing.T) {
		src := t.TempDir()
		if err := createInstance(ctx, src); err != nil {
			t.Fatalf("createInstance: %v", err)
		}
		outDir := filepath.Join(t.TempDir(), "missing")

		if err := backupInstance(ctx, src, outDir); err == nil {
			t.Fatal("backupInstance into a missing folder = nil, want error")
		}
		mustNotExist(t, outDir)
	})

	t.Run("backup refuses a missing source directory", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nosrc")
		if err := backupInstance(ctx, missing, t.TempDir()); err == nil {
			t.Fatal("backupInstance from a missing source = nil, want error")
		}
		mustNotExist(t, missing)
	})
}

// mustNotExist fails the test if path exists.
func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s exists; a tdb command must not create it", path)
	}
}

// readAccount returns the password_hash, is_admin, display_name, and inactive of
// the account with the given email, or ("", 0, "", 0) if none exists.
func readAccount(t *testing.T, ctx context.Context, dir, email string) (hash string, isAdmin int, displayName string, inactive int) {
	t.Helper()
	db, err := sqlite.OpenNonMigrating(ctx, dir)
	if err != nil {
		t.Fatalf("OpenNonMigrating: %v", err)
	}
	defer db.Close()

	conn, err := db.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer db.Put(conn)

	err = sqlitex.ExecuteTransient(conn,
		"SELECT password_hash, is_admin, display_name, inactive FROM accounts WHERE email = ?;",
		&sqlitex.ExecOptions{
			Args: []any{email},
			ResultFunc: func(stmt *zsqlite.Stmt) error {
				hash = stmt.ColumnText(0)
				isAdmin = stmt.ColumnInt(1)
				displayName = stmt.ColumnText(2)
				inactive = stmt.ColumnInt(3)
				return nil
			},
		})
	if err != nil {
		t.Fatalf("read account: %v", err)
	}
	return hash, isAdmin, displayName, inactive
}

// runTDB drives the real command tree for a single invocation, exercising flag
// parsing (including ff's IsSet, which update account relies on).
func runTDB(t *testing.T, args ...string) error {
	t.Helper()
	return newRootCommand().ParseAndRun(context.Background(), args)
}

// TestCreateAccountInactive covers the --is-inactive flag on create account.
func TestCreateAccountInactive(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := runTDB(t, "create", "database", "--path", dir); err != nil {
		t.Fatalf("create database: %v", err)
	}
	if err := runTDB(t, "create", "account", "--path", dir, "--email", "ghost@example.com", "--secret", "pw", "--is-inactive"); err != nil {
		t.Fatalf("create account --is-inactive: %v", err)
	}
	if _, _, _, inactive := readAccount(t, ctx, dir, "ghost@example.com"); inactive != 1 {
		t.Errorf("inactive = %d, want 1", inactive)
	}
}

// TestUpdateAccountCommand drives update account end-to-end: the mutually
// exclusive status flags, the tri-state (unset leaves status alone), and each
// field change, verified by reading the row back.
func TestUpdateAccountCommand(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := runTDB(t, "create", "database", "--path", dir); err != nil {
		t.Fatalf("create database: %v", err)
	}
	if err := runTDB(t, "create", "account", "--path", dir, "--email", "boss@example.com", "--display-name", "Boss", "--secret", "startpw"); err != nil {
		t.Fatalf("create account: %v", err)
	}

	// --active and --inactive together are rejected.
	if err := runTDB(t, "update", "account", "--path", dir, "--email", "boss@example.com", "--active", "--inactive"); err == nil {
		t.Error("update account --active --inactive = nil, want mutually-exclusive error")
	}

	// No change requested is rejected.
	if err := runTDB(t, "update", "account", "--path", dir, "--email", "boss@example.com"); err == nil {
		t.Error("update account with no changes = nil, want error")
	}

	// --inactive deactivates; a later field-only update leaves status alone.
	if err := runTDB(t, "update", "account", "--path", dir, "--email", "boss@example.com", "--inactive"); err != nil {
		t.Fatalf("update --inactive: %v", err)
	}
	if _, _, _, inactive := readAccount(t, ctx, dir, "boss@example.com"); inactive != 1 {
		t.Fatalf("after --inactive, inactive = %d, want 1", inactive)
	}
	if err := runTDB(t, "update", "account", "--path", dir, "--email", "boss@example.com", "--new-display-name", "Big Boss"); err != nil {
		t.Fatalf("update --new-display-name: %v", err)
	}
	if _, _, dn, inactive := readAccount(t, ctx, dir, "boss@example.com"); dn != "Big Boss" || inactive != 1 {
		t.Errorf("after display-only update: display=%q inactive=%d, want \"Big Boss\" and 1 (status untouched)", dn, inactive)
	}

	// --active reactivates.
	if err := runTDB(t, "update", "account", "--path", dir, "--email", "boss@example.com", "--active"); err != nil {
		t.Fatalf("update --active: %v", err)
	}
	if _, _, _, inactive := readAccount(t, ctx, dir, "boss@example.com"); inactive != 0 {
		t.Errorf("after --active, inactive = %d, want 0", inactive)
	}

	// An invalid new display name is rejected.
	if err := runTDB(t, "update", "account", "--path", dir, "--email", "boss@example.com", "--new-display-name", "1bad"); err == nil {
		t.Error("update with an invalid display name = nil, want error")
	}

	// A new email moves the row; the old address no longer resolves.
	if err := runTDB(t, "update", "account", "--path", dir, "--email", "boss@example.com", "--new-email", "Chief@Example.com"); err != nil {
		t.Fatalf("update --new-email: %v", err)
	}
	if h, _, _, _ := readAccount(t, ctx, dir, "chief@example.com"); h == "" {
		t.Error("account not found under the new (lowercased) email")
	}
	if h, _, _, _ := readAccount(t, ctx, dir, "boss@example.com"); h != "" {
		t.Error("account still resolves under the old email")
	}

	// A new secret replaces the hash.
	if err := runTDB(t, "update", "account", "--path", dir, "--email", "chief@example.com", "--new-secret", "freshpw"); err != nil {
		t.Fatalf("update --new-secret: %v", err)
	}
	hash, _, _, _ := readAccount(t, ctx, dir, "chief@example.com")
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("freshpw")); err != nil {
		t.Errorf("new secret does not verify: %v", err)
	}

	// Updating an account that does not exist is an error.
	if err := runTDB(t, "update", "account", "--path", dir, "--email", "nobody@example.com", "--inactive"); err == nil {
		t.Error("update of a missing account = nil, want error")
	}
}
