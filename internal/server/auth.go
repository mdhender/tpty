// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/mdhender/tpty/internal/api"
	"github.com/mdhender/tpty/internal/stores/sqlite"
)

// authKey types the context value carrying the resolved caller.
type authKey struct{}

// authInfo is the resolved identity for an authenticated request: the effective
// account (the session's subject) and the session behind the presented token.
// Handlers read it with accountFromContext / sessionFromContext.
type authInfo struct {
	account sqlite.Account
	session sqlite.Session
}

// accountFromContext returns the authenticated account and true when the request
// passed the auth middleware, or the zero account and false otherwise.
func accountFromContext(ctx context.Context) (sqlite.Account, bool) {
	info, ok := ctx.Value(authKey{}).(authInfo)
	return info.account, ok
}

// sessionFromContext returns the session behind the presented token and true
// when the request passed the auth middleware, or the zero session and false.
func sessionFromContext(ctx context.Context) (sqlite.Session, bool) {
	info, ok := ctx.Value(authKey{}).(authInfo)
	return info.session, ok
}

// bearerToken extracts the raw token from an "Authorization: Bearer <token>"
// header, returning "" when the header is absent or not a well-formed bearer
// credential. The scheme is matched case-insensitively per RFC 7235.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// authLevel is a route's authentication requirement, keyed by operationId.
type authLevel int

const (
	levelAuthed authLevel = iota // a valid session is required (the fail-closed default)
	levelPublic                  // no authentication
	levelAdmin                   // a valid session for a server admin
)

// operationAuth maps each generated operationId (the StrictServerInterface method
// name) to its authentication requirement. Any operation absent from the map
// defaults to levelAuthed, so a newly added route is authenticated by default
// (fail-closed) until its requirement is stated here.
var operationAuth = map[string]authLevel{
	// System + login are public (api/openapi.yaml: security: []).
	"GetHealth":  levelPublic,
	"GetVersion": levelPublic,
	"Login":      levelPublic,

	// Account administration and admin server operations require a server admin.
	"ListAccounts":          levelAdmin,
	"CreateAccount":         levelAdmin,
	"GetAccount":            levelAdmin,
	"UpdateAccount":         levelAdmin,
	"ListAccountSessions":   levelAdmin,
	"RevokeAccountSessions": levelAdmin,
	"RevokeAccountSession":  levelAdmin,
	"PurgeSessions":         levelAdmin,
}

// authMiddleware is the strict-server middleware that enforces each operation's
// authentication requirement (operationAuth) before its handler runs. For a
// public operation it passes straight through. Otherwise it resolves the bearer
// token to an active session and a fresh, active account — so a revoked session
// or a since-deactivated account fails immediately — and stores both on the
// context for the handler. A missing, malformed, unknown, revoked, or expired
// credential (and a deactivated account) all yield the same opaque 401. An admin
// route additionally requires accounts.is_admin, else 403.
//
// On denial it writes the standard error envelope directly and returns a nil
// response with a nil error, which the generated strict handler treats as
// "already written" — the operation's own handler never runs.
func (s *Server) authMiddleware(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		level := operationAuth[operationID]
		if level == levelPublic {
			return f(ctx, w, r, request)
		}

		raw := bearerToken(r)
		if raw == "" {
			writeError(w, r, http.StatusUnauthorized, codeUnauthorized, "authentication required")
			return nil, nil
		}
		session, err := s.db.GetActiveSessionByToken(ctx, raw, s.now())
		if err != nil {
			if !errors.Is(err, sqlite.ErrRecordNotFound) {
				logger(r).ErrorContext(ctx, "auth: resolve session", "err", err)
			}
			writeError(w, r, http.StatusUnauthorized, codeUnauthorized, "invalid or expired session")
			return nil, nil
		}
		account, err := s.db.GetAccountByID(ctx, session.AccountID)
		if err != nil {
			if !errors.Is(err, sqlite.ErrRecordNotFound) {
				logger(r).ErrorContext(ctx, "auth: load account", "err", err)
			}
			writeError(w, r, http.StatusUnauthorized, codeUnauthorized, "invalid or expired session")
			return nil, nil
		}
		if !account.IsActive {
			writeError(w, r, http.StatusUnauthorized, codeUnauthorized, "invalid or expired session")
			return nil, nil
		}
		if level == levelAdmin && !account.IsAdmin {
			writeError(w, r, http.StatusForbidden, codeForbidden, "admin privileges required")
			return nil, nil
		}

		ctx = context.WithValue(ctx, authKey{}, authInfo{account: account, session: session})
		return f(ctx, w, r, request)
	}
}
