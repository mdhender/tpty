// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"regexp"
	"strings"

	"github.com/mdhender/tpty/internal/cerrs"
	"github.com/mdhender/tpty/internal/prng"
	"github.com/mdhender/tpty/phrases"
)

// Player is a person participating in a game. Players are scoped to a game: a
// player belongs to exactly one game and is identified within it by a
// sequential id, a unique email, and a unique handle.
//
// Seeds is the player's private randomness stream, derived from the game's
// master seeds and the handle. Password is a shared secret used to authenticate
// the player's orders; it is stored in plain text.
//
// Inactive records whether the player has been removed. Players are never
// physically deleted; removing a player sets Inactive, and reactivating clears
// it. The zero value is active, so a newly created player and any player record
// written before this field existed both read as active. Use the Active
// accessor rather than testing Inactive directly.
//
// See content/docs/reference/players.md for the rules.
type Player struct {
	ID               int        `json:"id"`
	Handle           string     `json:"handle"`
	Email            string     `json:"email"`
	StartingProvince string     `json:"starting_province"`
	Password         string     `json:"password"`
	Seeds            prng.Seeds `json:"seeds"`
	Inactive         bool       `json:"inactive,omitempty"`
}

// Active reports whether the player is active — that is, not removed. It is the
// negation of the stored Inactive flag, provided so engine code reads in terms
// of the domain's "active" state instead of a double negative.
func (p Player) Active() bool {
	return !p.Inactive
}

// Errors returned when creating or validating a player.
const (
	ErrInvalidEmail    = cerrs.Error("invalid email")
	ErrInvalidHandle   = cerrs.Error("invalid handle")
	ErrInvalidProvince = cerrs.Error("invalid starting province")
	ErrDuplicateEmail  = cerrs.Error("duplicate email")
	ErrDuplicateHandle = cerrs.Error("duplicate handle")
	ErrUnknownEmail    = cerrs.Error("unknown email")
	ErrUnknownPlayer   = cerrs.Error("unknown player")
	ErrAlreadyInactive = cerrs.Error("player is already inactive")
	ErrAlreadyActive   = cerrs.Error("player is already active")
)

// passwordWordCount is the number of words in a generated password. The phrases
// wordlist yields about 10.3 bits of entropy per word, so seven words give a
// little over 64 bits. See the phrases package.
const passwordWordCount = 7

// handlePattern matches a valid handle: a leading letter (a–z or A–Z) followed
// by one or more letters, digits, or hyphen/underscore/period, with single
// internal ASCII spaces allowed between non-space characters. It rejects a
// leading or trailing space and any run of two or more spaces.
//
// See content/docs/reference/players.md.
var handlePattern = regexp.MustCompile(`^[a-zA-Z]( ?[a-zA-Z0-9_.-])+$`)

// ValidateHandle reports whether handle is well-formed, wrapping ErrInvalidHandle
// if it is not.
func ValidateHandle(handle string) error {
	if !handlePattern.MatchString(handle) {
		return fmt.Errorf("%q: %w", handle, ErrInvalidHandle)
	}
	return nil
}

// normalizeEmail lowercases and trims surrounding space from an email address.
// Emails are stored and compared in lowercase.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ParseProvince validates a province coordinate string in the canonical compact
// form "(q,r)" and returns it unchanged. It is the exported form of the check
// Create applies to a starting province; the command layer uses it to match a
// province against the game's allowed starting provinces. A non-canonical string
// is rejected with ErrInvalidProvince.
func ParseProvince(s string) (string, error) {
	h, err := canonicalProvince(s)
	if err != nil {
		return "", err
	}
	return h.String(), nil
}

// canonicalProvince validates that s is a province coordinate in the canonical
// compact form "(q,r)" (no spaces) and returns the parsed hex. Anything else —
// the spaced form "(q, r)", extra characters, a non-canonical sign or padding —
// is rejected with ErrInvalidProvince.
func canonicalProvince(s string) (Hex, error) {
	var q, r int
	if n, err := fmt.Sscanf(s, "(%d,%d)", &q, &r); err != nil || n != 2 {
		return Hex{}, fmt.Errorf("%q: want compact form like %q: %w", s, "(-1,0)", ErrInvalidProvince)
	}
	// Sscanf tolerates leading spaces, signs, and trailing junk; require the
	// input to already equal the canonical rendering so only the exact compact
	// form is accepted.
	h := Hex{Q: q, R: r}
	if canonical := h.String(); canonical != s {
		return Hex{}, fmt.Errorf("%q is not canonical, want %q: %w", s, canonical, ErrInvalidProvince)
	}
	return h, nil
}

// playerSeeds derives a player's private seeds from the game's master seeds and
// the player's handle. Because the seeds are keyed by the handle, a player's
// randomness is tied to their handle for the life of the game.
//
// The derivation is a frozen compatibility surface (see CLAUDE.md): the key path
// and the handle hash must not change once games exist.
func playerSeeds(gameSeeds prng.Seeds, handle string) prng.Seeds {
	return gameSeeds.Derive(prng.TagPlayerSeeds, prng.Key(hashHandle(handle)))
}

// hashHandle reduces a handle to a stream key using the 64-bit FNV-1a hash. The
// choice of hash is frozen: changing it changes every player's derived seeds.
func hashHandle(handle string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(handle))
	return int64(h.Sum64())
}

// generatePassword generates a player's password from the player's own seeds,
// keyed by the starting province. The result is words joined by periods, so it
// needs no JSON escaping and contains no spaces.
func generatePassword(seeds prng.Seeds, province Hex) string {
	s := rand.New(seeds.Stream(prng.TagPlayerSecret, prng.Key(province.Q), prng.Key(province.R)))
	return phrases.Generate(s, passwordWordCount)
}

// generateResetPassword generates a player's reset password from the player's
// own seeds, keyed by the current turn. It draws from the TagPlayerPasswordReset
// domain — distinct from the TagPlayerSecret domain used by generatePassword —
// so a reset value always differs from the creation password, and the turn
// differentiates successive resets. Like a creation password, the result is
// words joined by periods, so it needs no JSON escaping and contains no spaces.
func generateResetPassword(seeds prng.Seeds, turn int) string {
	s := rand.New(seeds.Stream(prng.TagPlayerPasswordReset, prng.Key(turn)))
	return phrases.Generate(s, passwordWordCount)
}
