// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"context"

	"github.com/mdhender/tpty/internal/api"
	"github.com/mdhender/tpty/internal/stores/sqlite"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// toAccountDTO projects a store account onto the wire Account schema. The
// application role is a single isAdmin boolean; per-game roles are never carried
// here (they are surfaced by GET /me/games). displayName and the timestamps are
// omitted when empty/zero.
func toAccountDTO(a sqlite.Account) api.Account {
	dto := api.Account{
		Id:       a.ID,
		Email:    openapi_types.Email(a.Email),
		IsActive: a.IsActive,
		IsAdmin:  a.IsAdmin,
	}
	if a.DisplayName != "" {
		dn := a.DisplayName
		dto.DisplayName = &dn
	}
	if !a.CreatedAt.IsZero() {
		t := a.CreatedAt
		dto.CreatedAt = &t
	}
	if !a.UpdatedAt.IsZero() {
		t := a.UpdatedAt
		dto.UpdatedAt = &t
	}
	return dto
}

// toSessionDTO projects a store session onto the wire Session schema. Raw tokens
// are never included. current marks the session behind the request and is only
// meaningful on self listings; it is omitted (nil) for admin listings.
func toSessionDTO(s sqlite.Session, current bool) api.Session {
	dto := api.Session{
		Id:        s.ID,
		IssuedAt:  s.IssuedAt,
		ExpiresAt: s.ExpiresAt,
	}
	if current {
		c := true
		dto.Current = &c
	}
	return dto
}

// toMyGameDTO projects a store MyGame onto the wire MyGame schema.
func toMyGameDTO(g sqlite.MyGame) api.MyGame {
	return api.MyGame{
		Id:       g.GameID,
		Code:     g.Code,
		PlayerId: g.PlayerID,
		IsGm:     g.IsGM,
	}
}

// errorBody builds the standard error envelope for a strict-handler response,
// stamping the request's correlation id from ctx. Handlers return it wrapped in
// the operation's generated 4xx response type; the auth middleware and the
// catch-all write it directly with writeError instead.
func errorBody(ctx context.Context, code, message string) api.Error {
	var e api.Error
	e.Error.Code = code
	e.Error.Message = message
	if id := requestID(ctx); id != "" {
		e.Error.RequestId = &id
	}
	return e
}
