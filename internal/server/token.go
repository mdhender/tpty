// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// A session has two independent opaque strings, both drawn from crypto/rand:
//
//   - the token, the bearer credential the client presents. Only its SHA-256 hash
//     is stored (sessions.hashed_token); the raw token is shown to the client once,
//     at login, and never persisted. It is resolved on each request by hashing the
//     presented value and matching by equality.
//   - the id, the session's public handle used in /me/sessions URLs, which is not
//     a credential and can be listed freely.
//
// The token is high-entropy (256 bits), so a fast hash (SHA-256) is the correct
// choice: there is nothing to brute-force, and resolving a bearer credential on
// every request must be cheap. This is why the token uses SHA-256 while an
// account secret uses bcrypt.
const (
	// tokenBytes is the token's entropy in bytes (256 bits).
	tokenBytes = 32
	// sessionIDBytes is the public session id's entropy in bytes (128 bits).
	sessionIDBytes = 16
)

// newToken mints a fresh opaque session token — the raw bearer credential shown
// to the client once — as a URL-safe base64 string.
func newToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("new token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns the hex-encoded SHA-256 of a raw token — the form stored in
// sessions.hashed_token and looked up on each authenticated request. Hashing is
// deterministic, so the same token always maps to the same stored value.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// newSessionID mints a fresh opaque public session id (not a credential).
func newSessionID() (string, error) {
	b := make([]byte, sessionIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("new session id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashSecret derives a bcrypt hash of a plaintext secret to store in
// accounts.password_hash, at the server's configured cost (Server.bcryptCost).
// It is a method so the cost travels with the server: tests drop to
// bcrypt.MinCost while production stays at bcrypt.DefaultCost.
func (s *Server) hashSecret(plaintext string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plaintext), s.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash secret: %w", err)
	}
	return string(h), nil
}

// verifySecret reports whether plaintext matches an encoded bcrypt hash. The cost
// is carried in the hash, so verification needs no server setting. A stored '*'
// (the fail-closed default for an account created without a secret) is not a
// valid bcrypt hash, so every verification against it fails.
func verifySecret(encoded, plaintext string) bool {
	return bcrypt.CompareHashAndPassword([]byte(encoded), []byte(plaintext)) == nil
}
