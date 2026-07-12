// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package sqlite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mdhender/tpty/internal/cerrs"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitemigration"
	"zombiezen.com/go/sqlite/sqlitex"
)

const (
	// ErrInvalidPath is returned when a directory argument is missing or is not a
	// directory. The store never creates directories, so a bad path is a hard
	// error, never something to create.
	ErrInvalidPath = cerrs.Error("invalid path")
	// ErrNotExist is returned when a directory exists but holds no instance — by
	// OpenPersistent, Backup, and Compact, which all operate on an existing
	// instance. Bringing one into being is CreatePersistent's job. It is wrapped
	// with the directory (see requireInstance), so callers see "no instance in
	// <dir>".
	ErrNotExist = cerrs.Error("no instance")
)

// dbFilename is the file name the store owns under a persistent instance's
// directory. Callers pass the directory; the package owns the name (and WAL's
// companion -wal / -shm files that appear beside it).
const dbFilename = "tpty.db"

// InstanceExists reports whether a persistent instance already exists in dir. It
// lets a caller distinguish "create a new instance" from "operate on an existing
// one" without knowing the file name the package owns.
func InstanceExists(dir string) (bool, error) {
	_, err := os.Stat(filepath.Join(dir, dbFilename))
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, fmt.Errorf("sqlite: %w", err)
	}
}

// Open flags per mode. Every mode sets OpenURI so the URI query string is
// honored. On-disk instances use WAL; the in-memory temporary instances do not
// (WAL has no meaning for an in-memory database).
//
// Only createFlags and temporaryFlags include OpenCreate. persistentFlags and
// nonMigratingFlags deliberately do NOT — an on-disk instance is only ever
// brought into being by CreatePersistent.
const (
	// persistentFlags open an EXISTING on-disk instance read-write and migrate it
	// up. No OpenCreate: OpenPersistent never creates a file.
	persistentFlags = sqlite.OpenReadWrite | sqlite.OpenWAL | sqlite.OpenURI
	// createFlags additionally create the database file. Used only by
	// CreatePersistent, the sole function that may bring an instance into being.
	createFlags = persistentFlags | sqlite.OpenCreate
	// temporaryFlags open an in-memory database (mode=memory in the URI).
	temporaryFlags = sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenURI
	// nonMigratingFlags open an existing file without creating or migrating it;
	// used by the tdb commands that must not alter the instance.
	nonMigratingFlags = sqlite.OpenReadWrite | sqlite.OpenWAL | sqlite.OpenURI
)

// openTimeout bounds the initial open + migration of an on-disk instance.
// sqlitemigration retries a failing open indefinitely (a server-startup
// convenience) rather than returning the error, so without a deadline a corrupt
// or foreign database file would hang the caller. Real migrations here complete
// in milliseconds; this is only a safety net against a doomed open. It is a var
// solely so tests can shorten it.
var openTimeout = 30 * time.Second

// pool is the subset of *sqlitemigration.Pool and *sqlitex.Pool that DB needs.
// Both concrete pools satisfy it (their Get signatures differ, but Take, Put,
// and Close are identical), so one DB type covers every open mode.
type pool interface {
	Take(ctx context.Context) (*sqlite.Conn, error)
	Put(conn *sqlite.Conn)
	Close() error
}

// DB is a handle to a T'Pty SQLite instance and its connection pool. It is safe
// for concurrent use: borrow a connection with Get, return it with Put, and
// close the pool with Close when done.
type DB struct {
	pool pool
}

// OpenPersistent opens the EXISTING instance held in dir and migrates it up. dir
// is the directory that holds the instance, NOT a file name — the package owns
// the file name (dbFilename) under it.
//
// OpenPersistent never creates anything: not the directory, not the database
// file. The directory must already exist (else ErrInvalidPath) and must already
// hold an instance (else ErrNotExist) — an absent database file is an error
// rather than a fresh, empty instance. Bringing a new instance into being is the
// sole responsibility of CreatePersistent. Every connection runs PRAGMA
// foreign_keys = ON and uses WAL.
//
// Open fails if the on-disk migration version is NEWER than this binary's
// ExpectedVersion: sqlitemigration only migrates up (it no-ops a database at or
// above the versions it knows), so running against a future schema would
// silently misbehave. That is caught by a post-open user_version guard.
func OpenPersistent(ctx context.Context, dir string) (*DB, error) {
	if err := requireDir(dir); err != nil {
		return nil, err
	}
	// The instance file must already exist. Beyond honoring "never create", this
	// keeps a missing file away from sqlitemigration, which retries a failing
	// open indefinitely (a server-startup convenience) rather than returning the
	// error — so a doomed URI would hang, not fail.
	if err := requireInstance(dir); err != nil {
		return nil, err
	}
	return openMigrating(ctx, dir, persistentFlags)
}

// CreatePersistent creates a new on-disk instance in dir and migrates it up. It
// is the ONLY function in the package that may bring an on-disk instance into
// being — and even it will not create the directory: dir must already exist
// (else ErrInvalidPath). It refuses to run when an instance already exists there
// (use OpenPersistent to open one that does).
func CreatePersistent(ctx context.Context, dir string) (*DB, error) {
	if err := requireDir(dir); err != nil {
		return nil, err
	}
	if exists, err := InstanceExists(dir); err != nil {
		return nil, err
	} else if exists {
		return nil, fmt.Errorf("sqlite: an instance already exists in %s", dir)
	}
	return openMigrating(ctx, dir, createFlags)
}

// requireDir returns ErrInvalidPath unless dir exists and is a directory. The
// store never creates directories, so a bad path is a hard error.
func requireDir(dir string) error {
	if sb, err := os.Stat(dir); err != nil || !sb.IsDir() {
		return ErrInvalidPath
	}
	return nil
}

// requireInstance returns ErrNotExist, wrapped with dir, unless dir holds an
// instance. It gives every operation on an existing instance the same
// "no instance in <dir>" error.
func requireInstance(dir string) error {
	if exists, err := InstanceExists(dir); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("%w in %s", ErrNotExist, dir)
	}
	return nil
}

// openMigrating opens dir's instance through the migrating pool with the given
// flags and applies the newer-than-expected guard. Callers MUST have already
// validated the directory and the instance file's presence/absence to match
// flags: sqlitemigration retries a failing open indefinitely rather than
// returning the error, so a doomed URI must never reach it.
func openMigrating(ctx context.Context, dir string, flags sqlite.OpenFlags) (*DB, error) {
	uri := "file:" + filepath.Join(dir, dbFilename)
	p := sqlitemigration.NewPool(uri, schema(), sqlitemigration.Options{
		Flags:       flags,
		PrepareConn: prepareConn,
	})
	db := &DB{pool: p}
	// Bound the initial open + migrate so a doomed open errors instead of
	// hanging (see openTimeout).
	vctx, cancel := context.WithTimeout(ctx, openTimeout)
	defer cancel()
	if err := db.checkVersion(vctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// OpenTemporary opens an in-memory instance and migrates it up. It is always
// created fresh. When name is "", the instance is unique and unreachable by any
// other caller; when name is non-empty, another caller passing the same name
// reaches the same database (shared by name). The pool keeps a connection open
// so the in-memory database survives for the life of the DB.
func OpenTemporary(ctx context.Context, name string) (*DB, error) {
	if name == "" {
		// A unique, unguessable name so no other caller can reach this instance.
		var buf [16]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return nil, fmt.Errorf("sqlite: %w", err)
		}
		name = hex.EncodeToString(buf[:])
	}
	uri := fmt.Sprintf("file:%s?mode=memory&cache=shared", name)
	p := sqlitemigration.NewPool(uri, schema(), sqlitemigration.Options{
		Flags:       temporaryFlags,
		PrepareConn: prepareConn,
	})
	db := &DB{pool: p}
	if err := db.checkVersion(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// OpenNonMigrating opens the instance in dir WITHOUT migrating it and WITHOUT the
// newer-than-expected guard, for the tdb commands that must not alter the
// instance (backup, compact, version). It opens an existing file only (it does
// not create one). Every connection still runs PRAGMA foreign_keys = ON and uses
// WAL.
func OpenNonMigrating(ctx context.Context, dir string) (*DB, error) {
	uri := "file:" + filepath.Join(dir, dbFilename)
	p, err := sqlitex.NewPool(uri, sqlitex.PoolOptions{
		Flags:       nonMigratingFlags,
		PrepareConn: prepareConn,
	})
	if err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}
	db := &DB{pool: p}
	// Fail fast if the file cannot be opened, rather than deferring the error to
	// the first real query.
	if err := db.ping(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// backupStampLayout formats (in UTC) the timestamp suffix of a backup file name.
// A literal trailing "Z" is appended separately so Go does not read it as a
// timezone directive.
const backupStampLayout = "20060102T150405"

// Backup writes a consistent, compacted copy of the instance in dir into the
// outputPath folder and returns the path of the file it wrote. The caller
// chooses only the folder — never the file name: the backup is always the
// instance's own file name with a UTC timestamp suffix (e.g.
// "tpty.db.20260712T213106Z"). outputPath defaults to dir when empty and must
// already exist; the store never creates directories. The source is opened
// non-migrating and is not modified.
func Backup(ctx context.Context, dir, outputPath string) (string, error) {
	if err := requireInstance(dir); err != nil {
		return "", err
	}
	if outputPath == "" {
		outputPath = dir
	}
	if err := requireDir(outputPath); err != nil {
		return "", fmt.Errorf("sqlite: backup folder %s: %w", outputPath, err)
	}

	stamp := time.Now().UTC().Format(backupStampLayout) + "Z"
	target := filepath.Join(outputPath, dbFilename+"."+stamp)
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("sqlite: backup target %s already exists", target)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("sqlite: %w", err)
	}

	db, err := OpenNonMigrating(ctx, dir)
	if err != nil {
		return "", err
	}
	defer db.Close()

	conn, err := db.Get(ctx)
	if err != nil {
		return "", err
	}
	defer db.Put(conn)

	// VACUUM INTO reads the source and writes a fresh, compacted copy to the
	// target; it does not modify the source.
	if err := sqlitex.ExecuteTransient(conn, "VACUUM INTO ?;", &sqlitex.ExecOptions{
		Args: []any{target},
	}); err != nil {
		return "", fmt.Errorf("sqlite: backup: %w", err)
	}
	return target, nil
}

// Compact runs VACUUM on the instance in dir, reclaiming free space in place. It
// opens the instance non-migrating (it does not migrate the schema) and is an
// offline operation — the caller must ensure no other process is using the
// instance.
func Compact(ctx context.Context, dir string) error {
	if err := requireInstance(dir); err != nil {
		return err
	}

	db, err := OpenNonMigrating(ctx, dir)
	if err != nil {
		return err
	}
	defer db.Close()

	conn, err := db.Get(ctx)
	if err != nil {
		return err
	}
	defer db.Put(conn)

	if err := sqlitex.ExecuteTransient(conn, "VACUUM;", nil); err != nil {
		return fmt.Errorf("sqlite: compact: %w", err)
	}
	return nil
}

// CreateAccount inserts a server account into the instance in dir and returns
// its new id. It opens the instance migrating (accounts land against the current
// schema). The caller supplies the already-computed passwordHash — the store
// persists credentials, it does not hash them — and a lowercased, validated
// email; the schema enforces email uniqueness. isAdmin is stored as the 0/1
// boolean the schema expects.
func CreateAccount(ctx context.Context, dir, email, displayName, passwordHash string, isAdmin bool) (int64, error) {
	db, err := OpenPersistent(ctx, dir)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	conn, err := db.Get(ctx)
	if err != nil {
		return 0, err
	}
	defer db.Put(conn)

	admin := 0
	if isAdmin {
		admin = 1
	}
	if err := sqlitex.ExecuteTransient(conn,
		"INSERT INTO accounts (email, display_name, password_hash, is_admin) VALUES (?, ?, ?, ?);",
		&sqlitex.ExecOptions{Args: []any{email, displayName, passwordHash, admin}},
	); err != nil {
		return 0, fmt.Errorf("sqlite: create account: %w", err)
	}
	return conn.LastInsertRowID(), nil
}

// Get borrows a connection from the pool, blocking until one is available or ctx
// is done. For a migrating instance it also blocks until the initial migration
// has completed. Return the connection with Put.
func (db *DB) Get(ctx context.Context) (*sqlite.Conn, error) {
	return db.pool.Take(ctx)
}

// Put returns a connection borrowed with Get to the pool.
func (db *DB) Put(conn *sqlite.Conn) {
	db.pool.Put(conn)
}

// Close closes the pool and all of its connections. For a temporary instance
// this also discards the in-memory database.
func (db *DB) Close() error {
	return db.pool.Close()
}

// UserVersion returns the instance's migration version — SQLite's user_version,
// which equals the number of migrations applied. A fully-migrated instance
// reports ExpectedVersion().
func (db *DB) UserVersion(ctx context.Context) (int, error) {
	conn, err := db.pool.Take(ctx)
	if err != nil {
		return 0, err
	}
	defer db.pool.Put(conn)
	return readUserVersion(conn)
}

// checkVersion borrows a connection (forcing the initial migration to run and
// surfacing any migration error) and rejects an instance whose on-disk version
// is newer than this binary expects.
func (db *DB) checkVersion(ctx context.Context) error {
	conn, err := db.pool.Take(ctx)
	if err != nil {
		return fmt.Errorf("sqlite: %w", err)
	}
	defer db.pool.Put(conn)
	v, err := readUserVersion(conn)
	if err != nil {
		return fmt.Errorf("sqlite: %w", err)
	}
	if v > ExpectedVersion() {
		return fmt.Errorf("sqlite: database version %d is newer than expected %d", v, ExpectedVersion())
	}
	return nil
}

// ping borrows and returns a connection, surfacing any open error eagerly.
func (db *DB) ping(ctx context.Context) error {
	conn, err := db.pool.Take(ctx)
	if err != nil {
		return fmt.Errorf("sqlite: %w", err)
	}
	db.pool.Put(conn)
	return nil
}

// schema is the sqlitemigration.Schema this store migrates to: the ordered
// Migrations under the store's AppID.
func schema() sqlitemigration.Schema {
	return sqlitemigration.Schema{
		AppID:      AppID,
		Migrations: Migrations,
	}
}

// prepareConn runs on every connection the pools hand out. It enables foreign
// key enforcement, which SQLite leaves off by default and which the schema's
// foreign keys rely on.
func prepareConn(conn *sqlite.Conn) error {
	return sqlitex.ExecuteTransient(conn, "PRAGMA foreign_keys = ON;", nil)
}

// readUserVersion reads PRAGMA user_version from conn.
func readUserVersion(conn *sqlite.Conn) (int, error) {
	var v int
	err := sqlitex.ExecuteTransient(conn, "PRAGMA user_version;", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			v = stmt.ColumnInt(0)
			return nil
		},
	})
	if err != nil {
		return 0, err
	}
	return v, nil
}
