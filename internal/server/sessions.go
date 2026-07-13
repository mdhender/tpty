// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/mdhender/tpty/internal/api"
	"github.com/mdhender/tpty/internal/stores/sqlite"
)

// ListMySessions serves GET /me/sessions (openapi.yaml: listMySessions). It
// returns the caller's active sessions (neither revoked nor expired), newest
// first, with the session behind the current request marked current: true.
func (s *Server) ListMySessions(ctx context.Context, request api.ListMySessionsRequestObject) (api.ListMySessionsResponseObject, error) {
	account, ok := accountFromContext(ctx)
	if !ok {
		return api.ListMySessions401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "authentication required"))}, nil
	}
	current, _ := sessionFromContext(ctx)

	sessions, err := s.db.ListActiveSessionsByAccount(ctx, account.ID, s.now())
	if err != nil {
		return nil, fmt.Errorf("sessions: list mine: %w", err)
	}
	out := make([]api.Session, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, toSessionDTO(sess, sess.ID == current.ID))
	}
	return api.ListMySessions200JSONResponse{Sessions: out}, nil
}

// RevokeMySession serves DELETE /me/sessions/{sessionId} (openapi.yaml:
// revokeMySession) — the "log out this device" counterpart to logout. Only the
// caller's own sessions are revocable: an unknown session, or one owned by another
// account, yields 404 rather than revealing its existence. Idempotent while the
// record persists.
func (s *Server) RevokeMySession(ctx context.Context, request api.RevokeMySessionRequestObject) (api.RevokeMySessionResponseObject, error) {
	account, ok := accountFromContext(ctx)
	if !ok {
		return api.RevokeMySession401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "authentication required"))}, nil
	}

	sess, err := s.db.GetSession(ctx, request.SessionId)
	if err != nil {
		if errors.Is(err, sqlite.ErrRecordNotFound) {
			return api.RevokeMySession404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "session not found"))}, nil
		}
		return nil, fmt.Errorf("sessions: get mine: %w", err)
	}
	// A session owned by another account is invisible to the caller: 404, not 403,
	// so its existence is not revealed cross-account.
	if sess.AccountID != account.ID {
		return api.RevokeMySession404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "session not found"))}, nil
	}
	if err := s.db.RevokeSession(ctx, request.SessionId, s.now()); err != nil {
		if errors.Is(err, sqlite.ErrRecordNotFound) {
			return api.RevokeMySession404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "session not found"))}, nil
		}
		return nil, fmt.Errorf("sessions: revoke mine: %w", err)
	}
	return api.RevokeMySession204Response{}, nil
}

// ListAccountSessions serves GET /accounts/{accountId}/sessions (openapi.yaml:
// listAccountSessions). Admin only. It lists the target account's active
// sessions; an unknown account is 404. The current marker is meaningless here (the
// admin is not the subject), so it is omitted.
func (s *Server) ListAccountSessions(ctx context.Context, request api.ListAccountSessionsRequestObject) (api.ListAccountSessionsResponseObject, error) {
	if _, err := s.db.GetAccountByID(ctx, request.AccountId); err != nil {
		if errors.Is(err, sqlite.ErrRecordNotFound) {
			return api.ListAccountSessions404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "account not found"))}, nil
		}
		return nil, fmt.Errorf("sessions: load account: %w", err)
	}
	sessions, err := s.db.ListActiveSessionsByAccount(ctx, request.AccountId, s.now())
	if err != nil {
		return nil, fmt.Errorf("sessions: list for account: %w", err)
	}
	out := make([]api.Session, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, toSessionDTO(sess, false))
	}
	return api.ListAccountSessions200JSONResponse{Sessions: out}, nil
}

// RevokeAccountSessions serves DELETE /accounts/{accountId}/sessions (openapi.yaml:
// revokeAccountSessions) — the admin "force logout everywhere" for a compromised
// or deactivated account. Admin only; an unknown account is 404. Immediate and
// idempotent (revoking with nothing active is a no-op 204).
func (s *Server) RevokeAccountSessions(ctx context.Context, request api.RevokeAccountSessionsRequestObject) (api.RevokeAccountSessionsResponseObject, error) {
	if _, err := s.db.GetAccountByID(ctx, request.AccountId); err != nil {
		if errors.Is(err, sqlite.ErrRecordNotFound) {
			return api.RevokeAccountSessions404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "account not found"))}, nil
		}
		return nil, fmt.Errorf("sessions: load account: %w", err)
	}
	if _, err := s.db.RevokeAccountSessions(ctx, request.AccountId, s.now()); err != nil {
		return nil, fmt.Errorf("sessions: revoke account: %w", err)
	}
	return api.RevokeAccountSessions204Response{}, nil
}

// RevokeAccountSession serves DELETE /accounts/{accountId}/sessions/{sessionId}
// (openapi.yaml: revokeAccountSession). Admin only. It revokes a single session of
// the target account; a session that does not belong to the account (or does not
// exist) is 404. Immediate and idempotent while the record persists.
func (s *Server) RevokeAccountSession(ctx context.Context, request api.RevokeAccountSessionRequestObject) (api.RevokeAccountSessionResponseObject, error) {
	sess, err := s.db.GetSession(ctx, request.SessionId)
	if err != nil {
		if errors.Is(err, sqlite.ErrRecordNotFound) {
			return api.RevokeAccountSession404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "session not found"))}, nil
		}
		return nil, fmt.Errorf("sessions: get for account: %w", err)
	}
	if sess.AccountID != request.AccountId {
		return api.RevokeAccountSession404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "session not found"))}, nil
	}
	if err := s.db.RevokeSession(ctx, request.SessionId, s.now()); err != nil {
		if errors.Is(err, sqlite.ErrRecordNotFound) {
			return api.RevokeAccountSession404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "session not found"))}, nil
		}
		return nil, fmt.Errorf("sessions: revoke for account: %w", err)
	}
	return api.RevokeAccountSession204Response{}, nil
}

// PurgeSessions serves POST /admin/sessions/purge (openapi.yaml: purgeSessions).
// Admin only. It hard-deletes session records that have already expired and
// returns the number removed. Expired sessions no longer authenticate, so this
// affects neither active sessions nor revocation.
func (s *Server) PurgeSessions(ctx context.Context, request api.PurgeSessionsRequestObject) (api.PurgeSessionsResponseObject, error) {
	purged, err := s.db.PurgeExpiredSessions(ctx, s.now())
	if err != nil {
		return nil, fmt.Errorf("sessions: purge expired: %w", err)
	}
	return api.PurgeSessions200JSONResponse{Purged: purged}, nil
}
