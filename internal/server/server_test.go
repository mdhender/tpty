// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/mdhender/tpty/internal/stores/sqlite"
	"golang.org/x/crypto/bcrypt"
	"zombiezen.com/go/sqlite/sqlitex"
)

// harness is a running server over a fresh in-memory store, with a real HTTP
// listener (httptest) so the tests exercise the actual wire path.
type harness struct {
	t   *testing.T
	db  *sqlite.DB
	srv *Server
	ts  *httptest.Server
}

// newHarness builds a server over a fresh in-memory instance and starts it. It
// hashes at bcrypt.MinCost so the test is not dominated by bcrypt.
func newHarness(t *testing.T) *harness {
	t.Helper()
	db, err := sqlite.OpenTemporary(context.Background(), "")
	if err != nil {
		t.Fatalf("open temporary db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	srv, err := New(db, "9.9.9-test", nil, WithBcryptCost(bcrypt.MinCost))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return &harness{t: t, db: db, srv: srv, ts: ts}
}

// seedAccount inserts an account with the given secret (bcrypt at MinCost) and
// returns its id.
func (h *harness) seedAccount(email, secret, displayName string, isAdmin, isActive bool) int64 {
	h.t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.MinCost)
	if err != nil {
		h.t.Fatalf("hash: %v", err)
	}
	id, err := h.db.InsertAccount(context.Background(), sqlite.Account{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		IsAdmin:      isAdmin,
		IsActive:     isActive,
	})
	if err != nil {
		h.t.Fatalf("seed account %q: %v", email, err)
	}
	return id
}

// seedSeat inserts a game and an active membership for accountID, returning the
// game id and the seat (player) id. It writes the rows directly so the /me/games
// projection has something to return.
func (h *harness) seedSeat(accountID int64, code string, isGM bool) (gameID, playerID int64) {
	h.t.Helper()
	conn, err := h.db.Get(context.Background())
	if err != nil {
		h.t.Fatalf("get conn: %v", err)
	}
	defer h.db.Put(conn)

	if err := sqlitex.Execute(conn, "INSERT INTO games (code) VALUES (?)", &sqlitex.ExecOptions{Args: []any{code}}); err != nil {
		h.t.Fatalf("insert game: %v", err)
	}
	gameID = conn.LastInsertRowID()
	gm := 0
	if isGM {
		gm = 1
	}
	if err := sqlitex.Execute(conn, "INSERT INTO memberships (account_id, game_id, is_gm) VALUES (?, ?, ?)",
		&sqlitex.ExecOptions{Args: []any{accountID, gameID, gm}}); err != nil {
		h.t.Fatalf("insert membership: %v", err)
	}
	return gameID, conn.LastInsertRowID()
}

// login exchanges credentials for a token, failing the test on a non-200.
func (h *harness) login(email, secret string) string {
	h.t.Helper()
	var out struct {
		Token string `json:"token"`
	}
	code := h.do(http.MethodPost, "/auth/login", "", map[string]any{"email": email, "secret": secret}, &out)
	if code != http.StatusOK {
		h.t.Fatalf("login %q = %d, want 200", email, code)
	}
	if out.Token == "" {
		h.t.Fatalf("login %q returned empty token", email)
	}
	return out.Token
}

// do issues a request with an optional bearer token and JSON body, decoding a
// JSON response into out (when out is non-nil), and returns the status code.
func (h *harness) do(method, path, token string, body, out any) int {
	h.t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, h.ts.URL+path, rdr)
	if err != nil {
		h.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := h.ts.Client().Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
			h.t.Fatalf("decode %s %s: %v", method, path, err)
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	return resp.StatusCode
}

func TestHealthzPlainText(t *testing.T) {
	h := newHarness(t)
	resp, err := h.ts.Client().Get(h.ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("get healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("healthz content-type = %q, want text/plain", ct)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "ok\n" {
		t.Errorf("healthz body = %q, want %q", b, "ok\n")
	}
}

func TestVersionPublic(t *testing.T) {
	h := newHarness(t)
	var out struct {
		Application string `json:"application"`
		Database    struct {
			SchemaVersion int `json:"schemaVersion"`
		} `json:"database"`
	}
	if code := h.do(http.MethodGet, "/version", "", nil, &out); code != http.StatusOK {
		t.Fatalf("version = %d, want 200", code)
	}
	if out.Application != "9.9.9-test" {
		t.Errorf("application = %q, want 9.9.9-test", out.Application)
	}
	if out.Database.SchemaVersion < 1 {
		t.Errorf("schemaVersion = %d, want >= 1", out.Database.SchemaVersion)
	}
}

func TestLoginPaths(t *testing.T) {
	h := newHarness(t)
	h.seedAccount("admin@x.test", "supersecret", "Admin", true, true)
	h.seedAccount("dormant@x.test", "supersecret", "Dormant", false, false) // inactive

	// Wrong secret, unknown account, and inactive account all yield the same 401.
	for _, tc := range []struct {
		name          string
		email, secret string
	}{
		{"wrong secret", "admin@x.test", "nope"},
		{"unknown email", "ghost@x.test", "whatever"},
		{"inactive account", "dormant@x.test", "supersecret"},
		{"empty secret", "admin@x.test", ""},
	} {
		code := h.do(http.MethodPost, "/auth/login", "", map[string]any{"email": tc.email, "secret": tc.secret}, nil)
		if code != http.StatusUnauthorized {
			t.Errorf("login %s = %d, want 401", tc.name, code)
		}
	}

	// A good login mints a usable token: case-insensitive email match.
	tok := h.login("ADMIN@x.test", "supersecret")
	if code := h.do(http.MethodGet, "/me", tok, nil, nil); code != http.StatusOK {
		t.Errorf("GET /me with fresh token = %d, want 200", code)
	}
}

func TestMeRequiresAuth(t *testing.T) {
	h := newHarness(t)
	if code := h.do(http.MethodGet, "/me", "", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("GET /me without token = %d, want 401", code)
	}
	if code := h.do(http.MethodGet, "/me", "garbage-token", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("GET /me with bad token = %d, want 401", code)
	}
}

func TestAdminGatingAndAccountsCRUD(t *testing.T) {
	h := newHarness(t)
	h.seedAccount("admin@x.test", "supersecret", "Admin", true, true)
	h.seedAccount("user@x.test", "supersecret", "User", false, true)
	admin := h.login("admin@x.test", "supersecret")
	user := h.login("user@x.test", "supersecret")

	// A non-admin is forbidden on the admin surface.
	if code := h.do(http.MethodGet, "/accounts", user, nil, nil); code != http.StatusForbidden {
		t.Errorf("non-admin GET /accounts = %d, want 403", code)
	}

	// Admin creates an account with no secret: email lowercased, secret returned once.
	var created struct {
		Account struct {
			Id       int64  `json:"id"`
			Email    string `json:"email"`
			IsActive bool   `json:"isActive"`
			IsAdmin  bool   `json:"isAdmin"`
		} `json:"account"`
		GeneratedSecret *string `json:"generatedSecret"`
	}
	if code := h.do(http.MethodPost, "/accounts", admin, map[string]any{"email": "New@X.test", "isActive": true}, &created); code != http.StatusCreated {
		t.Fatalf("create account = %d, want 201", code)
	}
	if created.Account.Email != "new@x.test" {
		t.Errorf("created email = %q, want lowercased new@x.test", created.Account.Email)
	}
	if created.GeneratedSecret == nil || *created.GeneratedSecret == "" {
		t.Errorf("create without secret: generatedSecret = %v, want a value", created.GeneratedSecret)
	}

	// The generated secret actually logs in.
	h.login("new@x.test", *created.GeneratedSecret)

	// A duplicate email is 409.
	if code := h.do(http.MethodPost, "/accounts", admin, map[string]any{"email": "new@x.test", "secret": "anothersecret"}, nil); code != http.StatusConflict {
		t.Errorf("duplicate create = %d, want 409", code)
	}

	// Get the account, then unknown id is 404.
	if code := h.do(http.MethodGet, "/accounts/"+itoa(created.Account.Id), admin, nil, nil); code != http.StatusOK {
		t.Errorf("get account = %d, want 200", code)
	}
	if code := h.do(http.MethodGet, "/accounts/999999", admin, nil, nil); code != http.StatusNotFound {
		t.Errorf("get unknown account = %d, want 404", code)
	}

	// Patch the display name.
	var patched struct {
		Account struct {
			DisplayName string `json:"displayName"`
		} `json:"account"`
	}
	if code := h.do(http.MethodPatch, "/accounts/"+itoa(created.Account.Id), admin, map[string]any{"displayName": "Renamed"}, &patched); code != http.StatusOK {
		t.Fatalf("patch account = %d, want 200", code)
	}
	if patched.Account.DisplayName != "Renamed" {
		t.Errorf("patched display name = %q, want Renamed", patched.Account.DisplayName)
	}

	// An empty patch is rejected.
	if code := h.do(http.MethodPatch, "/accounts/"+itoa(created.Account.Id), admin, map[string]any{}, nil); code != http.StatusBadRequest {
		t.Errorf("empty patch = %d, want 400", code)
	}
}

func TestSessionLifecycle(t *testing.T) {
	h := newHarness(t)
	h.seedAccount("user@x.test", "supersecret", "User", false, true)
	tokA := h.login("user@x.test", "supersecret")
	tokB := h.login("user@x.test", "supersecret")

	// The self listing marks the current session and lists both.
	var listed struct {
		Sessions []struct {
			Id      string `json:"id"`
			Current *bool  `json:"current"`
		} `json:"sessions"`
	}
	if code := h.do(http.MethodGet, "/me/sessions", tokA, nil, &listed); code != http.StatusOK {
		t.Fatalf("list sessions = %d, want 200", code)
	}
	if len(listed.Sessions) != 2 {
		t.Fatalf("session count = %d, want 2", len(listed.Sessions))
	}
	currentCount := 0
	var otherID string
	for _, s := range listed.Sessions {
		if s.Current != nil && *s.Current {
			currentCount++
		} else {
			otherID = s.Id
		}
	}
	if currentCount != 1 {
		t.Errorf("current sessions = %d, want exactly 1", currentCount)
	}

	// Revoking another of my sessions works; reusing that token then fails.
	if code := h.do(http.MethodDelete, "/me/sessions/"+otherID, tokA, nil, nil); code != http.StatusNoContent {
		t.Errorf("revoke my session = %d, want 204", code)
	}
	if code := h.do(http.MethodGet, "/me", tokB, nil, nil); code != http.StatusUnauthorized {
		t.Errorf("revoked token still works = %d, want 401", code)
	}

	// Revoking an unknown session is 404.
	if code := h.do(http.MethodDelete, "/me/sessions/deadbeef", tokA, nil, nil); code != http.StatusNotFound {
		t.Errorf("revoke unknown session = %d, want 404", code)
	}

	// Logout revokes the current session immediately.
	if code := h.do(http.MethodPost, "/auth/logout", tokA, nil, nil); code != http.StatusNoContent {
		t.Errorf("logout = %d, want 204", code)
	}
	if code := h.do(http.MethodGet, "/me", tokA, nil, nil); code != http.StatusUnauthorized {
		t.Errorf("token after logout = %d, want 401", code)
	}
}

func TestSecretChangeRevokesOthers(t *testing.T) {
	h := newHarness(t)
	h.seedAccount("user@x.test", "supersecret", "User", false, true)
	keep := h.login("user@x.test", "supersecret")
	other := h.login("user@x.test", "supersecret")

	if code := h.do(http.MethodPost, "/me/secret", keep, map[string]any{"currentSecret": "supersecret", "newSecret": "brandnewsecret"}, nil); code != http.StatusNoContent {
		t.Fatalf("change secret = %d, want 204", code)
	}
	// The session that made the change survives; the other is revoked.
	if code := h.do(http.MethodGet, "/me", keep, nil, nil); code != http.StatusOK {
		t.Errorf("current session after secret change = %d, want 200", code)
	}
	if code := h.do(http.MethodGet, "/me", other, nil, nil); code != http.StatusUnauthorized {
		t.Errorf("other session after secret change = %d, want 401", code)
	}
	// A wrong current secret is rejected.
	if code := h.do(http.MethodPost, "/me/secret", keep, map[string]any{"currentSecret": "wrong", "newSecret": "yetanothersecret"}, nil); code != http.StatusUnauthorized {
		t.Errorf("wrong current secret = %d, want 401", code)
	}
	// Too-short new secret is a 400.
	if code := h.do(http.MethodPost, "/me/secret", keep, map[string]any{"currentSecret": "brandnewsecret", "newSecret": "short"}, nil); code != http.StatusBadRequest {
		t.Errorf("short new secret = %d, want 400", code)
	}
}

func TestEmailChangeAndConflict(t *testing.T) {
	h := newHarness(t)
	h.seedAccount("user@x.test", "supersecret", "User", false, true)
	h.seedAccount("taken@x.test", "supersecret", "Taken", false, true)
	tok := h.login("user@x.test", "supersecret")

	// Wrong secret is rejected.
	if code := h.do(http.MethodPost, "/me/email", tok, map[string]any{"currentSecret": "wrong", "newEmail": "moved@x.test"}, nil); code != http.StatusUnauthorized {
		t.Errorf("email change wrong secret = %d, want 401", code)
	}
	// A collision with an existing email is 409.
	if code := h.do(http.MethodPost, "/me/email", tok, map[string]any{"currentSecret": "supersecret", "newEmail": "taken@x.test"}, nil); code != http.StatusConflict {
		t.Errorf("email change conflict = %d, want 409", code)
	}
	// A clean change succeeds and the session survives (secret unchanged).
	if code := h.do(http.MethodPost, "/me/email", tok, map[string]any{"currentSecret": "supersecret", "newEmail": "Moved@X.test"}, nil); code != http.StatusOK {
		t.Errorf("email change = %d, want 200", code)
	}
	if code := h.do(http.MethodGet, "/me", tok, nil, nil); code != http.StatusOK {
		t.Errorf("session after email change = %d, want 200", code)
	}
	// The new (lowercased) email now logs in.
	h.login("moved@x.test", "supersecret")
}

func TestMyGamesProjection(t *testing.T) {
	h := newHarness(t)
	uid := h.seedAccount("player@x.test", "supersecret", "Player", false, true)
	_, playerID := h.seedSeat(uid, "ALPHA", false)
	tok := h.login("player@x.test", "supersecret")

	var out struct {
		Games []struct {
			Code     string `json:"code"`
			PlayerId int64  `json:"playerId"`
			IsGm     bool   `json:"isGm"`
		} `json:"games"`
	}
	if code := h.do(http.MethodGet, "/me/games", tok, nil, &out); code != http.StatusOK {
		t.Fatalf("me/games = %d, want 200", code)
	}
	if len(out.Games) != 1 {
		t.Fatalf("games = %d, want 1", len(out.Games))
	}
	if out.Games[0].Code != "ALPHA" || out.Games[0].PlayerId != playerID || out.Games[0].IsGm {
		t.Errorf("game = %+v, want {ALPHA, %d, false}", out.Games[0], playerID)
	}
}

func TestAdminSessionManagementAndPurge(t *testing.T) {
	h := newHarness(t)
	h.seedAccount("admin@x.test", "supersecret", "Admin", true, true)
	uid := h.seedAccount("user@x.test", "supersecret", "User", false, true)
	admin := h.login("admin@x.test", "supersecret")
	h.login("user@x.test", "supersecret") // give the user a live session

	// Admin lists the user's sessions.
	var listed struct {
		Sessions []struct {
			Id string `json:"id"`
		} `json:"sessions"`
	}
	if code := h.do(http.MethodGet, "/accounts/"+itoa(uid)+"/sessions", admin, nil, &listed); code != http.StatusOK {
		t.Fatalf("admin list sessions = %d, want 200", code)
	}
	if len(listed.Sessions) != 1 {
		t.Fatalf("user sessions = %d, want 1", len(listed.Sessions))
	}
	// Sessions of an unknown account are 404.
	if code := h.do(http.MethodGet, "/accounts/999999/sessions", admin, nil, nil); code != http.StatusNotFound {
		t.Errorf("sessions of unknown account = %d, want 404", code)
	}
	// Force-logout everywhere.
	if code := h.do(http.MethodDelete, "/accounts/"+itoa(uid)+"/sessions", admin, nil, nil); code != http.StatusNoContent {
		t.Errorf("revoke all sessions = %d, want 204", code)
	}

	// Purge is admin-only and returns a count.
	var purged struct {
		Purged int64 `json:"purged"`
	}
	if code := h.do(http.MethodPost, "/admin/sessions/purge", admin, nil, &purged); code != http.StatusOK {
		t.Fatalf("purge = %d, want 200", code)
	}
}

func TestErrorEnvelopeAndRequestID(t *testing.T) {
	h := newHarness(t)
	req, _ := http.NewRequest(http.MethodGet, h.ts.URL+"/me", nil)
	resp, err := h.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("missing X-Request-Id response header")
	}
	var env struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestId string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Error.Code != codeUnauthorized {
		t.Errorf("error code = %q, want %q", env.Error.Code, codeUnauthorized)
	}
	if env.Error.RequestId == "" {
		t.Error("error envelope missing requestId")
	}
	if env.Error.RequestId != resp.Header.Get("X-Request-Id") {
		t.Errorf("envelope requestId %q != header %q", env.Error.RequestId, resp.Header.Get("X-Request-Id"))
	}
}

func TestCSRFRejectsCrossOriginBrowserPost(t *testing.T) {
	h := newHarness(t)
	// A cross-site browser POST (Sec-Fetch-Site: cross-site) is rejected...
	req, _ := http.NewRequest(http.MethodPost, h.ts.URL+"/auth/login", strings.NewReader(`{"email":"x@x.test","secret":"y"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Origin", "http://evil.example")
	resp, err := h.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site login = %d, want 403", resp.StatusCode)
	}
	// ...while a same-origin POST (no cross-origin signal) passes CSRF (and gets a
	// normal 401 for the bogus credentials).
	if code := h.do(http.MethodPost, "/auth/login", "", map[string]any{"email": "x@x.test", "secret": "y"}, nil); code != http.StatusUnauthorized {
		t.Errorf("same-origin bad login = %d, want 401 (CSRF should allow it)", code)
	}
}

// itoa is a tiny int64→string helper for building path ids in tests.
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
