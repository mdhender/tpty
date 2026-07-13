// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// A session has two independent opaque strings, both drawn from crypto/rand:
//
//   - the token, the bearer credential the client presents. Unlike ecv6, T'Pty
//     stores it AS-IS (not hashed): it is high-entropy and resolved by equality
//     against sessions.token (see api/conventions.md and the SQL schema), so
//     revocation is immediate and no per-request hashing is needed.
//   - the id, the session's public handle used in /me/sessions URLs, which is not
//     a credential and can be listed freely.
const (
	// tokenBytes is the token's entropy in bytes (256 bits).
	tokenBytes = 32
	// sessionIDBytes is the public session id's entropy in bytes (128 bits).
	sessionIDBytes = 16
)

// newToken mints a fresh opaque session token — the raw bearer credential — as a
// URL-safe base64 string. It is stored as-is and shown to the client once, at
// login.
func newToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("new token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
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
