// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"context"
	"fmt"
	"math"

	"github.com/mdhender/tpty/internal/api"
)

// GetHealth serves GET /healthz (openapi.yaml: getHealth): an unauthenticated
// liveness probe that returns 200 with the plain-text body "ok" and does not
// touch the database.
func (s *Server) GetHealth(ctx context.Context, request api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200TextResponse("ok\n"), nil
}

// GetVersion serves GET /version (openapi.yaml: getVersion). It reports the
// application version and the open database's schema version (SQLite
// user_version). Public; no authentication.
func (s *Server) GetVersion(ctx context.Context, request api.GetVersionRequestObject) (api.GetVersionResponseObject, error) {
	schema, err := s.schemaVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("version: read schema version: %w", err)
	}
	// The wire field is int32 (spec-driven). The schema version is a small
	// non-negative migration count, so an out-of-range value is impossible in
	// practice; guard the narrowing anyway so a corrupt value surfaces as a 500
	// rather than silently wrapping negative.
	if schema < 0 || schema > math.MaxInt32 {
		return nil, fmt.Errorf("version: schema version %d out of range", schema)
	}
	return api.GetVersion200JSONResponse{
		Application: s.version,
		Database:    api.DatabaseVersion{SchemaVersion: int32(schema)},
	}, nil
}
