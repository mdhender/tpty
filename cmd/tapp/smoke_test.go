// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mdhender/tpty/internal/stores/sqlite"
	"golang.org/x/crypto/bcrypt"
	zsqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// TestOpenServeDBMemorySeedsAdmin pins that --db-path :memory: opens a temporary
// instance and seeds the well-known dev admin: is_admin = 1 and the logged
// password verifies against the stored bcrypt hash (not plaintext).
func TestOpenServeDBMemorySeedsAdmin(t *testing.T) {
	ctx := context.Background()

	db, err := openServeDB(ctx, memoryDBPath)
	if err != nil {
		t.Fatalf("openServeDB(:memory:) = %v, want nil", err)
	}
	defer db.Close()

	hash, isAdmin := readAccount(t, ctx, db, devAdminEmail)
	if hash == "" {
		t.Fatalf("dev admin %q not seeded", devAdminEmail)
	}
	if isAdmin != 1 {
		t.Errorf("dev admin is_admin = %d, want 1", isAdmin)
	}
	if hash == devAdminSecret {
		t.Error("dev admin password stored in plaintext, want a bcrypt hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(devAdminSecret)); err != nil {
		t.Errorf("logged dev admin password does not verify against the stored hash: %v", err)
	}
}

// TestOpenServeDBPersistentRejectsMissing pins that a non-:memory: path opens an
// EXISTING instance and never creates one: an empty directory yields ErrNotExist.
func TestOpenServeDBPersistentRejectsMissing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir() // exists, but holds no instance

	if _, err := openServeDB(ctx, dir); !errors.Is(err, sqlite.ErrNotExist) {
		t.Fatalf("openServeDB(empty dir) = %v, want ErrNotExist", err)
	}
}

// TestHealthzAndRouting pins that GET /healthz returns 200 and any other route
// 404s (the router is otherwise empty until #76).
func TestHealthzAndRouting(t *testing.T) {
	mux := newMux()

	if code := record(mux, http.MethodGet, "/healthz"); code != http.StatusOK {
		t.Errorf("GET /healthz = %d, want 200", code)
	}

	if code := record(mux, http.MethodGet, "/nope"); code != http.StatusNotFound {
		t.Errorf("GET /nope = %d, want 404", code)
	}

	// /healthz is liveness-only: a non-GET method must not match.
	if code := record(mux, http.MethodPost, "/healthz"); code != http.StatusMethodNotAllowed && code != http.StatusNotFound {
		t.Errorf("POST /healthz = %d, want 405 or 404", code)
	}
}

// TestServeOnGracefulShutdown drives the real server lifecycle: bind an
// ephemeral port, confirm /healthz answers over the wire, then cancel the
// context and confirm serveOn returns nil (a clean graceful shutdown) promptly.
func TestServeOnGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveOn(ctx, ln, newMux()) }()

	// The server is up: hit /healthz over the real listener.
	url := "http://" + ln.Addr().String() + "/healthz"
	resp := httpGet(t, url)
	if resp != http.StatusOK {
		t.Errorf("GET %s = %d, want 200", url, resp)
	}

	// Cancel (stands in for SIGINT/SIGTERM) → graceful shutdown, serveOn returns.
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serveOn after shutdown = %v, want nil", err)
		}
	case <-time.After(shutdownTimeout + 2*time.Second):
		t.Fatal("serveOn did not return after context cancel; graceful shutdown hung")
	}

	// After shutdown the listener is closed: a new connection is refused.
	if _, err := net.DialTimeout("tcp", ln.Addr().String(), 200*time.Millisecond); err == nil {
		t.Error("listener still accepting connections after shutdown")
	}
}

// TestServeRequiresDBPath and TestServeRejectsArgs pin the command guards.
func TestServeRequiresDBPath(t *testing.T) {
	if err := runTAPP(t, "serve"); err == nil {
		t.Fatal("serve without --db-path = nil, want error")
	}
}

func TestServeRejectsPositionalArgs(t *testing.T) {
	if err := runTAPP(t, "serve", "--db-path", memoryDBPath, "extra"); err == nil {
		t.Fatal("serve with a positional argument = nil, want error")
	}
}

// TestVersion drives the version command through the real tree.
func TestVersion(t *testing.T) {
	if err := runTAPP(t, "version"); err != nil {
		t.Fatalf("version = %v, want nil", err)
	}
}

// runTAPP drives the real command tree for a single invocation, exercising flag
// parsing and the command guards.
func runTAPP(t *testing.T, args ...string) error {
	t.Helper()
	return newRootCommand().ParseAndRun(context.Background(), args)
}

// readAccount returns the password_hash and is_admin of the account with the
// given email on the open db, or ("", 0) if none exists.
func readAccount(t *testing.T, ctx context.Context, db *sqlite.DB, email string) (hash string, isAdmin int) {
	t.Helper()
	conn, err := db.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer db.Put(conn)

	err = sqlitex.ExecuteTransient(conn,
		"SELECT password_hash, is_admin FROM accounts WHERE email = ?;",
		&sqlitex.ExecOptions{
			Args: []any{email},
			ResultFunc: func(stmt *zsqlite.Stmt) error {
				hash = stmt.ColumnText(0)
				isAdmin = stmt.ColumnInt(1)
				return nil
			},
		})
	if err != nil {
		t.Fatalf("read account: %v", err)
	}
	return hash, isAdmin
}

// record serves method+target against h and returns the response status code.
func record(h http.Handler, method, target string) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec.Code
}

// httpGet issues a real GET to url and returns the status code.
func httpGet(t *testing.T, url string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}
