// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestAccountStoreCRUD exercises the server-facing account methods: insert (with
// email lowercasing and the negated inactive flag), the id/email lookups, the
// duplicate-email conflict, the missing-row not-found, and a full save.
func TestAccountStoreCRUD(t *testing.T) {
	ctx := context.Background()
	db := openTempT(t, "")

	id, err := db.InsertAccount(ctx, Account{Email: "Alice@Example.com", DisplayName: "Alice", PasswordHash: "h", IsAdmin: true, IsActive: true})
	if err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	a, err := db.GetAccountByID(ctx, id)
	if err != nil {
		t.Fatalf("GetAccountByID: %v", err)
	}
	if a.Email != "alice@example.com" {
		t.Errorf("email = %q, want lowercased alice@example.com", a.Email)
	}
	if !a.IsActive || !a.IsAdmin {
		t.Errorf("flags: IsActive=%v IsAdmin=%v, want true true", a.IsActive, a.IsAdmin)
	}
	if a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() {
		t.Errorf("timestamps not populated: created=%v updated=%v", a.CreatedAt, a.UpdatedAt)
	}

	// Lookup by (case-insensitive) email.
	if byEmail, err := db.GetAccountByEmail(ctx, "ALICE@example.com"); err != nil || byEmail.ID != id {
		t.Errorf("GetAccountByEmail = (%+v, %v), want id %d", byEmail, err, id)
	}

	// A duplicate email is a conflict.
	if _, err := db.InsertAccount(ctx, Account{Email: "alice@example.com", PasswordHash: "h"}); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate insert = %v, want ErrConflict", err)
	}

	// Missing rows are not-found.
	if _, err := db.GetAccountByID(ctx, 9999); !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("GetAccountByID(missing) = %v, want ErrRecordNotFound", err)
	}
	if err := db.SaveAccount(ctx, Account{ID: 9999, Email: "x@y.z", PasswordHash: "h"}); !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("SaveAccount(missing) = %v, want ErrRecordNotFound", err)
	}

	// Save deactivates and renames.
	a.IsActive = false
	a.DisplayName = "Alice II"
	if err := db.SaveAccount(ctx, a); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}
	got, _ := db.GetAccountByID(ctx, id)
	if got.IsActive || got.DisplayName != "Alice II" {
		t.Errorf("after save: IsActive=%v name=%q, want false, Alice II", got.IsActive, got.DisplayName)
	}

	// ListAccounts returns the row.
	if accts, err := db.ListAccounts(ctx); err != nil || len(accts) != 1 {
		t.Errorf("ListAccounts = (%d rows, %v), want 1", len(accts), err)
	}
}

// TestSessionStoreLifecycle exercises the session methods: create, active
// resolution honoring revocation and expiry, listing, single and bulk revocation
// (with the except variant), and the physical purge of expired rows.
func TestSessionStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openTempT(t, "")
	now := time.Now()

	id, err := db.InsertAccount(ctx, Account{Email: "u@x.test", PasswordHash: "h", IsActive: true})
	if err != nil {
		t.Fatalf("InsertAccount: %v", err)
	}

	// The store resolves by equality on the stored value; hashing happens a layer
	// up (the server), so these fixtures use plain strings as the "hashed" tokens.
	mk := func(sid, hashedToken string, expires time.Time) {
		if err := db.CreateSession(ctx, Session{ID: sid, AccountID: id, HashedToken: hashedToken, IssuedAt: now, ExpiresAt: expires}); err != nil {
			t.Fatalf("CreateSession %q: %v", sid, err)
		}
	}
	mk("s1", "tok1", now.Add(time.Hour))
	mk("s2", "tok2", now.Add(time.Hour))
	mk("old", "tokold", now.Add(-time.Hour)) // already expired

	// An active token resolves; an expired one does not.
	if s, err := db.GetActiveSessionByToken(ctx, "tok1", now); err != nil || s.ID != "s1" {
		t.Errorf("resolve tok1 = (%q, %v), want s1", s.ID, err)
	}
	if _, err := db.GetActiveSessionByToken(ctx, "tokold", now); !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("resolve expired = %v, want ErrRecordNotFound", err)
	}

	// The active listing excludes the expired row.
	if sessions, err := db.ListActiveSessionsByAccount(ctx, id, now); err != nil || len(sessions) != 2 {
		t.Fatalf("ListActiveSessionsByAccount = (%d, %v), want 2", len(sessions), err)
	}

	// Revoking s1 makes its token stop resolving; re-revoking is a no-op, and an
	// unknown id is not-found.
	if err := db.RevokeSession(ctx, "s1", now); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	if _, err := db.GetActiveSessionByToken(ctx, "tok1", now); !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("resolve revoked = %v, want ErrRecordNotFound", err)
	}
	if err := db.RevokeSession(ctx, "s1", now); err != nil {
		t.Errorf("re-revoke = %v, want nil (no-op)", err)
	}
	if err := db.RevokeSession(ctx, "nope", now); !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("revoke unknown = %v, want ErrRecordNotFound", err)
	}

	// RevokeAccountSessionsExcept spares s2 and revokes every other not-yet-revoked
	// session. s1 is already revoked, but "old" (expired, never revoked) still
	// carries revoked_at IS NULL, so it is revoked here — one row.
	if n, err := db.RevokeAccountSessionsExcept(ctx, id, "s2", now); err != nil || n != 1 {
		t.Errorf("RevokeAccountSessionsExcept = (%d, %v), want (1, nil)", n, err)
	}
	if s, err := db.GetActiveSessionByToken(ctx, "tok2", now); err != nil || s.ID != "s2" {
		t.Errorf("s2 should survive the except-revoke: (%q, %v)", s.ID, err)
	}

	// Purge removes the expired row only.
	if n, err := db.PurgeExpiredSessions(ctx, now); err != nil || n != 1 {
		t.Errorf("PurgeExpiredSessions = (%d, %v), want (1, nil)", n, err)
	}
	if _, err := db.GetSession(ctx, "old"); !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("purged session still present: %v", err)
	}
}
