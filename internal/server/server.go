// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Package server implements the T'Pty application server's REST API: the
// authentication and account-administration surface defined by api/openapi.yaml.
// It is deliberately client-agnostic and holds no game-engine logic — handlers
// are thin adapters over the sqlite store (internal/stores/sqlite) and the
// generated wire contract (internal/api).
//
// The routes are driven by the generated code: the Server implements the
// generated StrictServerInterface (one method per spec operationId), and Handler
// mounts them onto a net/http ServeMux via the generated RegisterHandlers path.
// Adding or renaming a route in the spec regenerates the interface and the
// compiler reports what to implement, keeping the handlers bound to the contract.
package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/mdhender/tpty/internal/api"
	"github.com/mdhender/tpty/internal/stores/sqlite"
	"golang.org/x/crypto/bcrypt"
)

// loginDecoySecret is hashed once per server (in New, at the configured bcrypt
// cost) into decoyHash. Verifying a presented secret against it on a login for an
// unknown account does the same bcrypt work as a real check, so a caller cannot
// distinguish "no such account" from "wrong secret" by response time.
const loginDecoySecret = "decoy-secret-for-constant-time-login"

// Server serves the application API over net/http. Construct it with New and
// mount it with Handler.
type Server struct {
	db      *sqlite.DB
	log     *slog.Logger
	version string // application version reported by GET /version

	// bcryptCost is the cost used to hash account secrets (hashSecret). Tests set
	// it to bcrypt.MinCost to keep hashing fast; production uses DefaultCost.
	bcryptCost int
	// decoyHash is a valid bcrypt hash at bcryptCost, verified against on a login
	// for an unknown account so timing matches a real check (see Login).
	decoyHash string
	// now returns the current time; a field only so tests can control the clock.
	now func() time.Time
}

// Option customizes a Server built by New.
type Option func(*Server)

// WithBcryptCost sets the bcrypt cost used to hash account secrets. It is for
// tests, which drop to bcrypt.MinCost so hashing does not dominate the run.
func WithBcryptCost(cost int) Option {
	return func(s *Server) { s.bcryptCost = cost }
}

// WithClock overrides the server's clock, for tests.
func WithClock(now func() time.Time) Option {
	return func(s *Server) { s.now = now }
}

// New builds a Server over an already-open store (cmd/tapp opens it; the server
// never creates one). version is the application version string reported by
// GET /version. log may be nil, in which case slog's default is used.
//
// New computes the login decoy hash once, at the resolved bcrypt cost, and fails
// if it cannot (bcrypt rejects a cost above bcrypt.MaxCost). A blank decoy would
// verify with no bcrypt work, reintroducing the account-enumeration timing side
// channel it exists to close, so New refuses rather than silently degrade it.
func New(db *sqlite.DB, version string, log *slog.Logger, opts ...Option) (*Server, error) {
	s := &Server{
		db:         db,
		log:        log,
		version:    version,
		bcryptCost: bcrypt.DefaultCost,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	decoy, err := bcrypt.GenerateFromPassword([]byte(loginDecoySecret), s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("new server: decoy hash at cost %d: %w", s.bcryptCost, err)
	}
	s.decoyHash = string(decoy)
	return s, nil
}

// Handler builds the routed http.Handler. The generated RegisterHandlers path
// (api.HandlerFromMux) mounts every spec operation onto a net/http ServeMux,
// wrapping the Server's StrictServerInterface with the auth middleware. /healthz
// stays a plain stdlib handler (the liveness probe behaves exactly as the
// bootstrap's did — text/plain "ok"), so it is dropped from the generated set to
// avoid a duplicate registration. Cross-origin protection (stdlib CSRF) and the
// request-id/logging/recovery chain wrap the whole mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// The liveness probe: a plain stdlib handler, unchanged from the bootstrap.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})

	// The generated strict handler: our StrictServerInterface wrapped with the
	// per-operation auth middleware, plus JSON error envelopes for the body-decode
	// and response failures the generated code surfaces.
	strict := api.NewStrictHandlerWithOptions(
		s,
		[]api.StrictMiddlewareFunc{s.authMiddleware},
		api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  s.strictRequestError,
			ResponseErrorHandlerFunc: s.strictResponseError,
		},
	)
	// Mount the generated routes onto mux, skipping /healthz (registered above as
	// the plain liveness probe) so the ServeMux is not asked to bind it twice.
	api.HandlerFromMux(strict, skipHealthzMux{mux})

	// A catch-all so an unknown path returns the JSON error envelope rather than
	// net/http's plain-text 404.
	mux.HandleFunc("/", s.handleNotFound)

	// Cross-origin protection (stdlib CSRF): reject non-safe cross-origin browser
	// requests. Non-browser clients (no Origin/Sec-Fetch-Site) are unaffected, so
	// CLI/script/bot callers pass through. A denied request gets our 403 envelope.
	csrf := http.NewCrossOriginProtection()
	csrf.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusForbidden, codeForbidden, "cross-origin request rejected")
	}))

	// Base middleware, outermost first: assign a request id, log the request,
	// recover panics, then apply CSRF. Recovery sits inside logging so a recovered
	// panic is still logged with its final (500) status.
	return chain(csrf.Handler(mux), withRequestID, withLogging(s.log), withRecovery)
}

// skipHealthzMux adapts an *http.ServeMux for api.HandlerFromMux so the generated
// GET /healthz registration is dropped: /healthz is served by the plain liveness
// handler registered separately (see Handler), and a ServeMux panics on a
// duplicate pattern. Every other generated route is forwarded unchanged.
type skipHealthzMux struct{ *http.ServeMux }

func (m skipHealthzMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	if pattern == "GET /healthz" {
		return
	}
	m.ServeMux.HandleFunc(pattern, handler)
}

// handleNotFound renders the standard 404 envelope for any unrouted path.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotFound, codeNotFound, "resource not found")
}

// strictRequestError renders a malformed request body (the generated strict
// handler could not decode JSON into the operation's request type) as the
// standard 400 envelope, rather than the generated default's plain text.
func (s *Server) strictRequestError(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, r, http.StatusBadRequest, codeBadRequest, "request body is not valid JSON")
}

// strictResponseError renders a response-encoding failure as the standard 500
// envelope. It is reached only when a handler returns an error or an unexpected
// response type — a server bug — so the detail is logged, not returned.
func (s *Server) strictResponseError(w http.ResponseWriter, r *http.Request, err error) {
	logger(r).ErrorContext(r.Context(), "response: strict handler", "err", err)
	writeError(w, r, http.StatusInternalServerError, codeInternal, "internal server error")
}

// schemaVersion returns the open database's schema version (SQLite user_version),
// which GET /version reports. The schema is immutable for the process lifetime
// (migrations complete at store open), but the read is cheap and infrequent, so
// it is not cached here.
func (s *Server) schemaVersion(ctx context.Context) (int, error) {
	return s.db.UserVersion(ctx)
}
