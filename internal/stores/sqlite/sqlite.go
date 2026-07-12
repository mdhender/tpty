// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package sqlite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitemigration"
	"zombiezen.com/go/sqlite/sqlitex"
)

// dbFilename is the file name the store owns under a persistent instance's
// directory. Callers pass the directory; the package owns the name (and WAL's
// companion -wal / -shm files that appear beside it).
const dbFilename = "tpty.db"

// Open flags per mode. Every mode sets OpenURI so the URI query string is
// honored. Persistent and non-migrating instances use WAL; the in-memory
// temporary instances do not (WAL has no meaning for an in-memory database).
const (
	// persistentFlags create the file if absent and migrate it up.
	persistentFlags = sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenWAL | sqlite.OpenURI
	// temporaryFlags open an in-memory database (mode=memory in the URI).
	temporaryFlags = sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenURI
	// nonMigratingFlags open an existing file without creating or migrating it;
	// used by the tdb commands that must not alter the instance.
	nonMigratingFlags = sqlite.OpenReadWrite | sqlite.OpenWAL | sqlite.OpenURI
)

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

// OpenPersistent opens the instance held in dir, creating and migrating it up as
// needed. dir is the directory that holds the instance, NOT a file name — the
// package owns the file name (dbFilename) under it. The directory is created if
// absent. Every connection runs PRAGMA foreign_keys = ON and uses WAL.
//
// Open fails if the on-disk migration version is NEWER than this binary's
// ExpectedVersion: sqlitemigration only migrates up (it no-ops a database at or
// above the versions it knows), so running against a future schema would
// silently misbehave. That is caught by a post-open user_version guard.
func OpenPersistent(ctx context.Context, dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}
	uri := "file:" + filepath.Join(dir, dbFilename)
	p := sqlitemigration.NewPool(uri, schema(), sqlitemigration.Options{
		Flags:       persistentFlags,
		PrepareConn: prepareConn,
	})
	db := &DB{pool: p}
	if err := db.checkVersion(ctx); err != nil {
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
