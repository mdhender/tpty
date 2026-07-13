// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/mdhender/tpty/internal/api"
	"github.com/mdhender/tpty/internal/stores/sqlite"
)

// generatedSecretBytes is the entropy (in bytes) of an auto-generated account
// secret, base64url-encoded into the plaintext returned once at creation.
const generatedSecretBytes = 12

// generateSecret mints a random plaintext secret (base64url) for an account
// created without one; the plaintext is returned to the admin once and only its
// bcrypt hash is stored.
func generateSecret() (string, error) {
	b := make([]byte, generatedSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ListAccounts serves GET /accounts (openapi.yaml: listAccounts). Admin only
// (enforced by the auth middleware). Returns every account ordered by id.
func (s *Server) ListAccounts(ctx context.Context, request api.ListAccountsRequestObject) (api.ListAccountsResponseObject, error) {
	accounts, err := s.db.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("accounts: list: %w", err)
	}
	out := make([]api.Account, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, toAccountDTO(a))
	}
	return api.ListAccounts200JSONResponse{Accounts: out}, nil
}

// CreateAccount serves POST /accounts (openapi.yaml: createAccount). Admin only.
// The account is inactive and non-admin unless isActive/isAdmin are set; email is
// lowercased and must be unique (a duplicate is 409). When secret is omitted a
// random one is generated and returned once in generatedSecret.
func (s *Server) CreateAccount(ctx context.Context, request api.CreateAccountRequestObject) (api.CreateAccountResponseObject, error) {
	if request.Body == nil {
		return api.CreateAccount400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "request body is required"))}, nil
	}
	req := request.Body
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	if email == "" {
		return api.CreateAccount400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "email is required"))}, nil
	}
	displayName := derefOr(req.DisplayName, "")
	if displayName != "" {
		if err := sqlite.ValidateDisplayName(displayName); err != nil {
			return api.CreateAccount400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, err.Error()))}, nil
		}
	}

	var (
		secret    string
		generated *string
	)
	if req.Secret != nil {
		if len(*req.Secret) < minSecretLen {
			return api.CreateAccount400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "secret must be at least 8 characters"))}, nil
		}
		secret = *req.Secret
	} else {
		gen, err := generateSecret()
		if err != nil {
			return nil, fmt.Errorf("accounts: generate secret: %w", err)
		}
		secret = gen
		generated = &gen
	}
	hashed, err := s.hashSecret(secret)
	if err != nil {
		return nil, fmt.Errorf("accounts: hash secret: %w", err)
	}

	id, err := s.db.InsertAccount(ctx, sqlite.Account{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: hashed,
		IsAdmin:      derefOr(req.IsAdmin, false),
		IsActive:     derefOr(req.IsActive, false),
	})
	if err != nil {
		if errors.Is(err, sqlite.ErrConflict) {
			return api.CreateAccount409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(errorBody(ctx, codeConflict, "an account with that email already exists"))}, nil
		}
		return nil, fmt.Errorf("accounts: create: %w", err)
	}
	// Re-read so the response carries the stored created_at/updated_at.
	created, err := s.db.GetAccountByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("accounts: reload created: %w", err)
	}
	return api.CreateAccount201JSONResponse{
		Account:         toAccountDTO(created),
		GeneratedSecret: generated,
	}, nil
}

// GetAccount serves GET /accounts/{accountId} (openapi.yaml: getAccount). Admin
// only. An unknown id is 404.
func (s *Server) GetAccount(ctx context.Context, request api.GetAccountRequestObject) (api.GetAccountResponseObject, error) {
	a, err := s.db.GetAccountByID(ctx, request.AccountId)
	if err != nil {
		if errors.Is(err, sqlite.ErrRecordNotFound) {
			return api.GetAccount404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "account not found"))}, nil
		}
		return nil, fmt.Errorf("accounts: get: %w", err)
	}
	return api.GetAccount200JSONResponse{Account: toAccountDTO(a)}, nil
}

// UpdateAccount serves PATCH /accounts/{accountId} (openapi.yaml: updateAccount).
// Admin only, partial update: a present field is applied, an absent field left
// unchanged, and at least one field is required. email must stay unique (409); a
// new secret must be at least 8 characters. This is the admin-only
// credential-recovery path.
func (s *Server) UpdateAccount(ctx context.Context, request api.UpdateAccountRequestObject) (api.UpdateAccountResponseObject, error) {
	if request.Body == nil {
		return api.UpdateAccount400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "request body is required"))}, nil
	}
	req := request.Body
	if req.DisplayName == nil && req.Email == nil && req.IsActive == nil && req.IsAdmin == nil && req.Secret == nil {
		return api.UpdateAccount400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "at least one field is required"))}, nil
	}

	a, err := s.db.GetAccountByID(ctx, request.AccountId)
	if err != nil {
		if errors.Is(err, sqlite.ErrRecordNotFound) {
			return api.UpdateAccount404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "account not found"))}, nil
		}
		return nil, fmt.Errorf("accounts: get for update: %w", err)
	}

	if req.DisplayName != nil {
		if *req.DisplayName != "" {
			if verr := sqlite.ValidateDisplayName(*req.DisplayName); verr != nil {
				return api.UpdateAccount400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, verr.Error()))}, nil
			}
		}
		a.DisplayName = *req.DisplayName
	}
	if req.Email != nil {
		email := strings.ToLower(strings.TrimSpace(string(*req.Email)))
		if email == "" {
			return api.UpdateAccount400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "email must not be empty"))}, nil
		}
		a.Email = email
	}
	if req.IsActive != nil {
		a.IsActive = *req.IsActive
	}
	if req.IsAdmin != nil {
		a.IsAdmin = *req.IsAdmin
	}
	if req.Secret != nil {
		if len(*req.Secret) < minSecretLen {
			return api.UpdateAccount400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(errorBody(ctx, codeBadRequest, "secret must be at least 8 characters"))}, nil
		}
		hashed, herr := s.hashSecret(*req.Secret)
		if herr != nil {
			return nil, fmt.Errorf("accounts: hash secret: %w", herr)
		}
		a.PasswordHash = hashed
	}

	if err := s.db.SaveAccount(ctx, a); err != nil {
		if errors.Is(err, sqlite.ErrConflict) {
			return api.UpdateAccount409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(errorBody(ctx, codeConflict, "an account with that email already exists"))}, nil
		}
		if errors.Is(err, sqlite.ErrRecordNotFound) {
			return api.UpdateAccount404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(errorBody(ctx, codeNotFound, "account not found"))}, nil
		}
		return nil, fmt.Errorf("accounts: update: %w", err)
	}
	// Re-read so the response carries the bumped updated_at.
	updated, err := s.db.GetAccountByID(ctx, request.AccountId)
	if err != nil {
		return nil, fmt.Errorf("accounts: reload updated: %w", err)
	}
	return api.UpdateAccount200JSONResponse{Account: toAccountDTO(updated)}, nil
}

// derefOr returns *p when p is non-nil, otherwise def.
func derefOr[T any](p *T, def T) T {
	if p != nil {
		return *p
	}
	return def
}
