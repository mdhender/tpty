// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mdhender/tpty/internal/cerrs"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// Row-level errors for the server-facing store methods. They report conditions
// on the rows within an already-open instance, distinct from the file-level
// errors (ErrInvalidPath, ErrNotExist) that report a missing directory or
// instance. The application server maps them onto HTTP status codes (see
// internal/server).
const (
	// ErrRecordNotFound is returned when a requested row does not exist.
	ErrRecordNotFound = cerrs.Error("record not found")
	// ErrConflict is returned when a write violates a uniqueness constraint —
	// a duplicate account email, for example.
	ErrConflict = cerrs.Error("conflict")
)

// Account is the server-side account projection the application API serves. It
// mirrors the accounts row: IsActive is the negation of the stored inactive
// flag, and PasswordHash is the bcrypt hash of the login secret (never the
// plaintext). The zero CreatedAt/UpdatedAt map to a stored 0.
type Account struct {
	ID           int64
	Email        string
	DisplayName  string
	PasswordHash string
	IsAdmin      bool
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Session is a server-side bearer session. ID is the opaque public identifier
// used in the API to address a session; Token is the bearer credential the
// client presents, stored as-is (not hashed) and resolved by equality. RevokedAt
// is the zero time while the session is active.
type Session struct {
	ID        string
	AccountID int64
	Token     string
	IssuedAt  time.Time
	ExpiresAt time.Time
	RevokedAt time.Time
}

// Revoked reports whether the session has been revoked.
func (s Session) Revoked() bool { return !s.RevokedAt.IsZero() }

// MyGame projects a game together with the caller's seat in it, for the
// per-account "my games" listing (GET /me/games). PlayerID is the seat id
// (memberships.id, the engine's player_id) and IsGM whether the caller is that
// game's GM. It carries only game identity (games.code), no engine state.
type MyGame struct {
	GameID   int64
	Code     string
	PlayerID int64
	IsGM     bool
}

// isConstraint reports whether err is a SQLite constraint violation (a UNIQUE,
// CHECK, NOT NULL, or foreign-key failure), which the server-facing writers map
// to ErrConflict.
func isConstraint(err error) bool {
	return sqlite.ErrCode(err).ToPrimary() == sqlite.ResultConstraint
}

// unixOrZero converts a stored Unix-seconds column to a UTC time.Time, mapping a
// stored 0 to the zero Time (an absent timestamp).
func unixOrZero(sec int64) time.Time {
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

// unixSeconds converts a time.Time to the Unix-seconds form stored in a column,
// mapping the zero Time to 0.
func unixSeconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// accountColumns is the fixed SELECT list scanAccount expects, in order.
const accountColumns = "id, email, display_name, password_hash, is_admin, inactive, created_at, updated_at"

// scanAccount reads an account from the current row, whose columns are the
// accountColumns in order. The stored inactive flag is negated into IsActive.
func scanAccount(stmt *sqlite.Stmt) Account {
	return Account{
		ID:           stmt.ColumnInt64(0),
		Email:        stmt.ColumnText(1),
		DisplayName:  stmt.ColumnText(2),
		PasswordHash: stmt.ColumnText(3),
		IsAdmin:      stmt.ColumnBool(4),
		IsActive:     !stmt.ColumnBool(5),
		CreatedAt:    unixOrZero(stmt.ColumnInt64(6)),
		UpdatedAt:    unixOrZero(stmt.ColumnInt64(7)),
	}
}

// GetAccountByID returns the account with the given id, or ErrRecordNotFound.
func (db *DB) GetAccountByID(ctx context.Context, id int64) (Account, error) {
	conn, err := db.Get(ctx)
	if err != nil {
		return Account{}, err
	}
	defer db.Put(conn)
	return getAccountWhere(conn, "id = ?", id)
}

// GetAccountByEmail returns the account with the given email (matched
// case-insensitively against the lowercased stored value), or ErrRecordNotFound.
// It is the lookup behind login.
func (db *DB) GetAccountByEmail(ctx context.Context, email string) (Account, error) {
	conn, err := db.Get(ctx)
	if err != nil {
		return Account{}, err
	}
	defer db.Put(conn)
	return getAccountWhere(conn, "email = ?", strings.ToLower(email))
}

// getAccountWhere reads the single account matching one WHERE predicate (id or
// email), returning ErrRecordNotFound when there is no match.
func getAccountWhere(conn *sqlite.Conn, where string, arg any) (Account, error) {
	var (
		a     Account
		found bool
	)
	err := sqlitex.Execute(conn,
		"SELECT "+accountColumns+" FROM accounts WHERE "+where, &sqlitex.ExecOptions{
			Args:       []any{arg},
			ResultFunc: func(stmt *sqlite.Stmt) error { a = scanAccount(stmt); found = true; return nil },
		})
	if err != nil {
		return Account{}, fmt.Errorf("get account: %w", err)
	}
	if !found {
		return Account{}, ErrRecordNotFound
	}
	return a, nil
}

// ListAccounts returns every account, active and inactive, ordered by id.
func (db *DB) ListAccounts(ctx context.Context) ([]Account, error) {
	conn, err := db.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Put(conn)

	var accounts []Account
	err = sqlitex.Execute(conn,
		"SELECT "+accountColumns+" FROM accounts ORDER BY id", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error { accounts = append(accounts, scanAccount(stmt)); return nil },
		})
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	return accounts, nil
}

// InsertAccount inserts a new account against the open instance and returns its
// assigned id. Email is lowercased; a duplicate email returns ErrConflict. The
// caller supplies the already-bcrypt-hashed secret (the store persists
// credentials, it does not hash them) and stores IsActive as the negated
// inactive flag the schema expects.
func (db *DB) InsertAccount(ctx context.Context, a Account) (int64, error) {
	conn, err := db.Get(ctx)
	if err != nil {
		return 0, err
	}
	defer db.Put(conn)

	err = sqlitex.Execute(conn,
		"INSERT INTO accounts (email, display_name, password_hash, is_admin, inactive) VALUES (?, ?, ?, ?, ?)",
		&sqlitex.ExecOptions{Args: []any{
			strings.ToLower(a.Email), a.DisplayName, a.PasswordHash, boolToInt(a.IsAdmin), boolToInt(!a.IsActive),
		}})
	if err != nil {
		if isConstraint(err) {
			return 0, fmt.Errorf("create account %q: %w", a.Email, ErrConflict)
		}
		return 0, fmt.Errorf("create account: %w", err)
	}
	return conn.LastInsertRowID(), nil
}

// SaveAccount writes the mutable fields (email, display name, password hash,
// admin, active) of the account identified by a.ID and bumps updated_at. Email
// is lowercased; a duplicate email returns ErrConflict and an unknown id returns
// ErrRecordNotFound. It backs both the admin account update and the self-service
// email/secret changes.
func (db *DB) SaveAccount(ctx context.Context, a Account) error {
	conn, err := db.Get(ctx)
	if err != nil {
		return err
	}
	defer db.Put(conn)

	err = sqlitex.Execute(conn, `
		UPDATE accounts
		SET email = ?, display_name = ?, password_hash = ?, is_admin = ?, inactive = ?, updated_at = unixepoch()
		WHERE id = ?`, &sqlitex.ExecOptions{Args: []any{
		strings.ToLower(a.Email), a.DisplayName, a.PasswordHash, boolToInt(a.IsAdmin), boolToInt(!a.IsActive), a.ID,
	}})
	if err != nil {
		if isConstraint(err) {
			return fmt.Errorf("update account %d: %w", a.ID, ErrConflict)
		}
		return fmt.Errorf("update account %d: %w", a.ID, err)
	}
	if conn.Changes() == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// ListMyGames returns the games the account holds an active seat in, each with
// the account's seat id (memberships.id) and whether it is the GM, ordered by
// game id. Inactive seats are omitted.
func (db *DB) ListMyGames(ctx context.Context, accountID int64) ([]MyGame, error) {
	conn, err := db.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Put(conn)

	var games []MyGame
	err = sqlitex.Execute(conn, `
		SELECT g.id, g.code, m.id, m.is_gm
		FROM memberships m
		JOIN games g ON g.id = m.game_id
		WHERE m.account_id = ? AND m.inactive = 0
		ORDER BY g.id`, &sqlitex.ExecOptions{
		Args: []any{accountID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			games = append(games, MyGame{
				GameID:   stmt.ColumnInt64(0),
				Code:     stmt.ColumnText(1),
				PlayerID: stmt.ColumnInt64(2),
				IsGM:     stmt.ColumnBool(3),
			})
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list games for account %d: %w", accountID, err)
	}
	return games, nil
}
