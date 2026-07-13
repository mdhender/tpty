// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mdhender/tpty/internal/api"
	"github.com/mdhender/tpty/internal/stores/sqlite"
)

// sessionTTL is how long a login session stays valid before it expires. Opaque
// server-side sessions are resolved on every request and revoked immediately on
// logout or account deactivation, so the TTL is only a backstop for abandoned
// sessions; a generous 30 days suits the CLI/script/bot clients that submit turns
// over a game's lifetime.
const sessionTTL = 30 * 24 * time.Hour

// Login serves POST /auth/login (openapi.yaml: login). It verifies an email +
// secret against the stored bcrypt hash and, on success, mints an opaque
// server-side session, returning the raw token once. Every credential failure
// returns the same opaque 401 so the response never reveals which accounts exist:
//
//   - A missing or empty credential is denied before any lookup or bcrypt work (a
//     cheap DoS guard; it reveals nothing, being identical for every email).
//   - A present-but-wrong credential — unknown email, wrong secret, inactive
//     account — is denied only after equivalent bcrypt work (the decoy hash for an
//     unknown email), so timing cannot enumerate accounts.
func (s *Server) Login(ctx context.Context, request api.LoginRequestObject) (api.LoginResponseObject, error) {
	if request.Body == nil {
		return api.Login400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "request body is required"))}, nil
	}
	email := strings.ToLower(strings.TrimSpace(string(request.Body.Email)))
	secret := request.Body.Secret
	if email == "" || secret == "" {
		return api.Login401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "invalid email or secret"))}, nil
	}

	account, err := s.db.GetAccountByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, sqlite.ErrRecordNotFound) {
			return nil, fmt.Errorf("login: lookup account: %w", err)
		}
		// Unknown email: do equivalent bcrypt work against the decoy so timing
		// cannot be used to enumerate accounts, then deny.
		_ = verifySecret(s.decoyHash, secret)
		return api.Login401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "invalid email or secret"))}, nil
	}
	if !verifySecret(account.PasswordHash, secret) || !account.IsActive {
		return api.Login401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "invalid email or secret"))}, nil
	}

	token, err := newToken()
	if err != nil {
		return nil, fmt.Errorf("login: mint token: %w", err)
	}
	id, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("login: mint session id: %w", err)
	}
	now := s.now()
	expiresAt := now.Add(sessionTTL)
	if err := s.db.CreateSession(ctx, sqlite.Session{
		ID:        id,
		AccountID: account.ID,
		Token:     token,
		IssuedAt:  now,
		ExpiresAt: expiresAt,
	}); err != nil {
		return nil, fmt.Errorf("login: create session: %w", err)
	}

	return api.Login200JSONResponse{
		Token:     token,
		TokenType: api.Bearer,
		ExpiresAt: expiresAt.UTC(),
	}, nil
}

// Logout serves POST /auth/logout (openapi.yaml: logout). It runs on an
// authenticated route, so the auth middleware has already resolved the caller. It
// revokes the session behind the presented token, or — with allSessions: true —
// every active session for the account. Revocation is immediate. Success is 204.
func (s *Server) Logout(ctx context.Context, request api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	session, ok := sessionFromContext(ctx)
	if !ok {
		return api.Logout401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "authentication required"))}, nil
	}

	now := s.now()
	var err error
	if request.Body != nil && request.Body.AllSessions != nil && *request.Body.AllSessions {
		_, err = s.db.RevokeAccountSessions(ctx, session.AccountID, now)
	} else {
		err = s.db.RevokeSession(ctx, session.ID, now)
	}
	if err != nil && !errors.Is(err, sqlite.ErrRecordNotFound) {
		return nil, fmt.Errorf("logout: revoke session: %w", err)
	}
	return api.Logout204Response{}, nil
}
