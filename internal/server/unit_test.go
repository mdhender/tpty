// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"net/http"
	"testing"
)

func TestBearerToken(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"", ""},
		{"Bearer abc123", "abc123"},
		{"bearer abc123", "abc123"},     // scheme is case-insensitive
		{"BEARER   spaced  ", "spaced"}, // surrounding space trimmed
		{"Basic abc123", ""},            // wrong scheme
		{"abc123", ""},                  // no scheme
	}
	for _, tc := range cases {
		r, _ := http.NewRequest(http.MethodGet, "/", nil)
		if tc.header != "" {
			r.Header.Set("Authorization", tc.header)
		}
		if got := bearerToken(r); got != tc.want {
			t.Errorf("bearerToken(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}

func TestVerifySecretRejectsFailClosedDefault(t *testing.T) {
	// '*' is the schema's fail-closed default password_hash; it is not a valid
	// bcrypt hash, so nothing verifies against it.
	if verifySecret("*", "anything") {
		t.Error("verifySecret against '*' returned true, want false")
	}
	s, err := New(nil, "t", nil, WithBcryptCost(4))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	hash, err := s.hashSecret("correct horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !verifySecret(hash, "correct horse") {
		t.Error("verifySecret with the right secret = false, want true")
	}
	if verifySecret(hash, "wrong") {
		t.Error("verifySecret with the wrong secret = true, want false")
	}
}

func TestNewTokenUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		tok, err := newToken()
		if err != nil {
			t.Fatalf("newToken: %v", err)
		}
		if seen[tok] {
			t.Fatalf("newToken produced a duplicate on iteration %d", i)
		}
		seen[tok] = true
	}
}

// TestOperationAuthFailsClosed pins that the admin and authenticated operations
// are not accidentally marked public: only the three genuinely public operations
// carry levelPublic, and the map's default (levelAuthed) is fail-closed.
func TestOperationAuthFailsClosed(t *testing.T) {
	public := map[string]bool{"GetHealth": true, "GetVersion": true, "Login": true}
	for op, lvl := range operationAuth {
		if lvl == levelPublic && !public[op] {
			t.Errorf("operation %q is marked public but must not be", op)
		}
	}
	for op := range public {
		if operationAuth[op] != levelPublic {
			t.Errorf("operation %q should be public", op)
		}
	}
	// A route absent from the map defaults to authed (fail-closed).
	if operationAuth["SomethingBrandNew"] != levelAuthed {
		t.Error("default auth level is not levelAuthed (fail-open)")
	}
}
