// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import "fmt"

// PlayerStore holds the players of a single game. Ids are assigned sequentially
// in increasing order and are never reused within a game, so NextID is persisted
// and never decreases — not even when a player is removed.
//
// See content/docs/reference/players.md for the rules.
type PlayerStore struct {
	NextID  int      `json:"next_id"`
	Players []Player `json:"players"`
}

// NewPlayerStore returns an empty store whose first assigned id will be 1.
func NewPlayerStore() *PlayerStore {
	return &PlayerStore{NextID: 1}
}

// Create validates the given fields, derives the player's private seeds and
// password from the game's master seeds, assigns the next sequential id, and
// adds a new player to the store. On success it returns the created player.
//
// Email is stored lowercased and must be unique within the game (compared after
// lowercasing). Handle is stored as given, must match the handle pattern, and
// must be unique within the game (compared exactly). The starting province must
// be in canonical compact form "(q,r)". The password is generated, not supplied.
//
// Create does not check the starting province against the game's allowed
// starting provinces; that is the caller's responsibility, since the allowed set
// lives in starting-provinces.json alongside the store.
func (s *PlayerStore) Create(master Seeds, email, handle, startingProvince string) (Player, error) {
	email = normalizeEmail(email)
	if email == "" {
		return Player{}, fmt.Errorf("email is required: %w", ErrInvalidEmail)
	}
	if err := ValidateHandle(handle); err != nil {
		return Player{}, err
	}
	province, err := canonicalProvince(startingProvince)
	if err != nil {
		return Player{}, err
	}

	if _, ok := s.ByEmail(email); ok {
		return Player{}, fmt.Errorf("%q: %w", email, ErrDuplicateEmail)
	}
	if _, ok := s.ByHandle(handle); ok {
		return Player{}, fmt.Errorf("%q: %w", handle, ErrDuplicateHandle)
	}

	if s.NextID < 1 {
		s.NextID = 1
	}
	seeds := playerSeeds(master, handle)
	p := Player{
		ID:               s.NextID,
		Handle:           handle,
		Email:            email,
		StartingProvince: province.String(),
		Password:         generatePassword(seeds, province),
		Seeds:            seeds,
	}
	s.Players = append(s.Players, p)
	s.NextID++
	return p, nil
}

// ResetPassword reissues the password of the player registered with the given
// email, drawing a new value from that player's own private stream keyed by the
// current turn, storing it on the player, and returning the updated player.
//
// Lookup is by email only (compared after lowercasing): the distinctive
// registered address is deliberately the sole key, to reduce GM mistakes and
// raise the bar for tricking the GM into resetting the wrong player. An empty
// email is rejected with ErrInvalidEmail; an email that matches no player is
// rejected with ErrUnknownEmail.
//
// Only the stored password changes; the player's id, email, handle, starting
// province, and seeds are untouched. The reset value always differs from the
// creation password (a different stream domain) and differs across turns.
func (s *PlayerStore) ResetPassword(email string, turn int) (Player, error) {
	email = normalizeEmail(email)
	if email == "" {
		return Player{}, fmt.Errorf("email is required: %w", ErrInvalidEmail)
	}
	for i := range s.Players {
		if s.Players[i].Email == email {
			s.Players[i].Password = generateResetPassword(s.Players[i].Seeds, turn)
			return s.Players[i], nil
		}
	}
	return Player{}, fmt.Errorf("%q: %w", email, ErrUnknownEmail)
}

// ByID returns the player with the given id, if any.
func (s *PlayerStore) ByID(id int) (Player, bool) {
	for _, p := range s.Players {
		if p.ID == id {
			return p, true
		}
	}
	return Player{}, false
}

// ByEmail returns the player with the given email, compared after lowercasing.
func (s *PlayerStore) ByEmail(email string) (Player, bool) {
	email = normalizeEmail(email)
	for _, p := range s.Players {
		if p.Email == email {
			return p, true
		}
	}
	return Player{}, false
}

// ByHandle returns the player with the given handle, compared exactly
// (case-sensitively).
func (s *PlayerStore) ByHandle(handle string) (Player, bool) {
	for _, p := range s.Players {
		if p.Handle == handle {
			return p, true
		}
	}
	return Player{}, false
}
