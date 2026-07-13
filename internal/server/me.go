// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mdhender/tpty/internal/api"
	"github.com/mdhender/tpty/internal/stores/sqlite"
)

// minSecretLen is the shortest secret accepted on a create or change
// (openapi.yaml: "at least 8 characters"). A generated secret is always longer.
const minSecretLen = 8

// GetMe serves GET /me (openapi.yaml: getMe). It returns the caller's account —
// an application-domain projection only, with no in-game data. The auth
// middleware has already resolved and freshly read the account.
func (s *Server) GetMe(ctx context.Context, request api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	account, ok := accountFromContext(ctx)
	if !ok {
		return api.GetMe401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "authentication required"))}, nil
	}
	return api.GetMe200JSONResponse{Account: toAccountDTO(account)}, nil
}

// UpdateMe serves PATCH /me (openapi.yaml: updateMe): the self-service profile
// update, limited to the non-sensitive displayName. Email and secret changes have
// their own confirmation-guarded routes.
func (s *Server) UpdateMe(ctx context.Context, request api.UpdateMeRequestObject) (api.UpdateMeResponseObject, error) {
	account, ok := accountFromContext(ctx)
	if !ok {
		return api.UpdateMe401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "authentication required"))}, nil
	}
	if request.Body == nil {
		return api.UpdateMe400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "request body is required"))}, nil
	}
	name := request.Body.DisplayName
	if err := sqlite.ValidateDisplayName(name); err != nil {
		return api.UpdateMe400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, err.Error()))}, nil
	}
	account.DisplayName = name
	if err := s.db.SaveAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("me: update profile: %w", err)
	}
	return api.UpdateMe200JSONResponse{Account: toAccountDTO(account)}, nil
}

// UpdateMyEmail serves POST /me/email (openapi.yaml: updateMyEmail). Because email
// is the login identity, the caller must supply the current secret, verified
// before the change. The new email is lowercased and must be unique (409).
// Existing sessions are not revoked — the secret is unchanged.
func (s *Server) UpdateMyEmail(ctx context.Context, request api.UpdateMyEmailRequestObject) (api.UpdateMyEmailResponseObject, error) {
	account, ok := accountFromContext(ctx)
	if !ok {
		return api.UpdateMyEmail401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "authentication required"))}, nil
	}
	if request.Body == nil {
		return api.UpdateMyEmail400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "request body is required"))}, nil
	}
	if !verifySecret(account.PasswordHash, request.Body.CurrentSecret) {
		return api.UpdateMyEmail401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "current secret is incorrect"))}, nil
	}
	email := strings.ToLower(strings.TrimSpace(string(request.Body.NewEmail)))
	if email == "" {
		return api.UpdateMyEmail400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "newEmail is required"))}, nil
	}
	account.Email = email
	if err := s.db.SaveAccount(ctx, account); err != nil {
		if errors.Is(err, sqlite.ErrConflict) {
			return api.UpdateMyEmail409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(errorBody(ctx, codeConflict, "an account with that email already exists"))}, nil
		}
		return nil, fmt.Errorf("me: update email: %w", err)
	}
	return api.UpdateMyEmail200JSONResponse{Account: toAccountDTO(account)}, nil
}

// UpdateMySecret serves POST /me/secret (openapi.yaml: updateMySecret). The caller
// supplies the current secret, verified before the new one is applied; on success
// the account's other sessions are revoked so a stolen session cannot outlive the
// secret it was created under. The auth middleware already re-read the account
// this request and rejected an inactive one, so the account here is fresh and
// active. Success is 204.
func (s *Server) UpdateMySecret(ctx context.Context, request api.UpdateMySecretRequestObject) (api.UpdateMySecretResponseObject, error) {
	account, ok := accountFromContext(ctx)
	if !ok {
		return api.UpdateMySecret401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "authentication required"))}, nil
	}
	session, ok := sessionFromContext(ctx)
	if !ok {
		return api.UpdateMySecret401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "authentication required"))}, nil
	}
	if request.Body == nil {
		return api.UpdateMySecret400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "request body is required"))}, nil
	}
	if !verifySecret(account.PasswordHash, request.Body.CurrentSecret) {
		return api.UpdateMySecret401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "current secret is incorrect"))}, nil
	}
	if len(request.Body.NewSecret) < minSecretLen {
		return api.UpdateMySecret400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "newSecret must be at least 8 characters"))}, nil
	}
	hashed, err := s.hashSecret(request.Body.NewSecret)
	if err != nil {
		return nil, fmt.Errorf("me: hash secret: %w", err)
	}
	account.PasswordHash = hashed
	if err := s.db.SaveAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("me: update secret: %w", err)
	}
	// Revoke the caller's other sessions, sparing the one behind this request. The
	// secret is already changed, so a failure here is logged but not fatal.
	if _, err := s.db.RevokeAccountSessionsExcept(ctx, account.ID, session.ID, s.now()); err != nil {
		s.log.ErrorContext(ctx, "me: revoke other sessions", "err", err)
	}
	return api.UpdateMySecret204Response{}, nil
}

// ListMyGames serves GET /me/games (openapi.yaml: listMyGames). It returns the
// games the caller holds an active seat in, each with the caller's playerId
// (memberships.id) and whether they are the GM. Scoped to the caller.
func (s *Server) ListMyGames(ctx context.Context, request api.ListMyGamesRequestObject) (api.ListMyGamesResponseObject, error) {
	account, ok := accountFromContext(ctx)
	if !ok {
		return api.ListMyGames401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(errorBody(ctx, codeUnauthorized, "authentication required"))}, nil
	}
	games, err := s.db.ListMyGames(ctx, account.ID)
	if err != nil {
		return nil, fmt.Errorf("me: list games: %w", err)
	}
	out := make([]api.MyGame, 0, len(games))
	for _, g := range games {
		out = append(out, toMyGameDTO(g))
	}
	return api.ListMyGames200JSONResponse{Games: out}, nil
}
