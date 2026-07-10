// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mdhender/tpty/internal/prng"
)

// testSeeds are fixed master seeds for player tests.
var testSeeds = prng.Seeds{Seed1: 1, Seed2: 2}

// newTestStore returns a store with one player already created, for tests that
// exercise uniqueness and id assignment.
func newTestStore(t *testing.T) *PlayerStore {
	t.Helper()
	s := NewPlayerStore()
	if _, err := s.Create(testSeeds, "Alice@Example.com", "alice", "(0,0)"); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	return s
}

func TestPlayerStoreAssignsSequentialIDs(t *testing.T) {
	s := NewPlayerStore()
	for i, handle := range []string{"aa", "bb", "cc"} {
		p, err := s.Create(testSeeds, handle+"@x.com", handle, "(0,0)")
		if err != nil {
			t.Fatalf("Create(%q): %v", handle, err)
		}
		if want := i + 1; p.ID != want {
			t.Errorf("Create(%q) id = %d, want %d", handle, p.ID, want)
		}
	}
	if s.NextID != 4 {
		t.Errorf("NextID = %d, want 4", s.NextID)
	}
}

func TestPlayerStoreLowercasesEmail(t *testing.T) {
	s := NewPlayerStore()
	p, err := s.Create(testSeeds, "Alice@Example.COM", "alice", "(0,0)")
	if err != nil {
		t.Fatal(err)
	}
	if p.Email != "alice@example.com" {
		t.Errorf("stored email = %q, want %q", p.Email, "alice@example.com")
	}
	// Lookups match regardless of the queried case.
	if _, ok := s.ByEmail("ALICE@example.com"); !ok {
		t.Error("ByEmail did not match a differently-cased address")
	}
}

func TestPlayerStoreRejectsDuplicateEmail(t *testing.T) {
	s := newTestStore(t)
	// Same address, different case and handle: still a duplicate.
	_, err := s.Create(testSeeds, "alice@example.com", "alice2", "(0,0)")
	if !errors.Is(err, ErrDuplicateEmail) {
		t.Errorf("Create with duplicate email: err = %v, want ErrDuplicateEmail", err)
	}
}

func TestPlayerStoreRejectsDuplicateHandle(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Create(testSeeds, "other@example.com", "alice", "(0,0)")
	if !errors.Is(err, ErrDuplicateHandle) {
		t.Errorf("Create with duplicate handle: err = %v, want ErrDuplicateHandle", err)
	}
}

func TestPlayerStoreHandleUniquenessIsCaseSensitive(t *testing.T) {
	s := newTestStore(t) // has handle "alice"
	// "Alice" differs from "alice" and is allowed.
	if _, err := s.Create(testSeeds, "cap@example.com", "Alice", "(0,0)"); err != nil {
		t.Errorf("Create(%q) should be allowed alongside %q: %v", "Alice", "alice", err)
	}
}

func TestPlayerStoreRejectsMissingEmail(t *testing.T) {
	s := NewPlayerStore()
	_, err := s.Create(testSeeds, "   ", "alice", "(0,0)")
	if !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("Create with blank email: err = %v, want ErrInvalidEmail", err)
	}
}

func TestPlayerStoreRejectsBadHandle(t *testing.T) {
	s := NewPlayerStore()
	_, err := s.Create(testSeeds, "a@example.com", "1nope", "(0,0)")
	if !errors.Is(err, ErrInvalidHandle) {
		t.Errorf("Create with bad handle: err = %v, want ErrInvalidHandle", err)
	}
}

func TestPlayerStoreRejectsNonCanonicalProvince(t *testing.T) {
	s := NewPlayerStore()
	for _, prov := range []string{"(0, 0)", "0,0", "(0,0) ", "(+1,0)", "(0,0)x", "north"} {
		if _, err := s.Create(testSeeds, "a@example.com", "alice", prov); !errors.Is(err, ErrInvalidProvince) {
			t.Errorf("Create with province %q: err = %v, want ErrInvalidProvince", prov, err)
		}
	}
}

func TestPlayerStoreAcceptsCanonicalProvince(t *testing.T) {
	s := NewPlayerStore()
	p, err := s.Create(testSeeds, "a@example.com", "alice", "(-1,2)")
	if err != nil {
		t.Fatal(err)
	}
	if p.StartingProvince != "(-1,2)" {
		t.Errorf("stored province = %q, want %q", p.StartingProvince, "(-1,2)")
	}
}

// TestPlayerStoreIDsSurviveReload confirms ids are not reused after a store is
// saved and reloaded: NextID persists, so the next id continues past the highest
// id ever assigned rather than restarting or reusing.
func TestPlayerStoreIDsSurviveReload(t *testing.T) {
	s := NewPlayerStore()
	for _, h := range []string{"aa", "bb", "cc"} {
		if _, err := s.Create(testSeeds, h+"@x.com", h, "(0,0)"); err != nil {
			t.Fatal(err)
		}
	}

	// Round-trip through JSON, as the command persists the store.
	buf, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded PlayerStore
	if err := json.Unmarshal(buf, &reloaded); err != nil {
		t.Fatal(err)
	}

	// Simulate removing a player, then creating a new one; the id must continue.
	reloaded.Players = reloaded.Players[:2]
	p, err := reloaded.Create(testSeeds, "dd@x.com", "dd", "(0,0)")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != 4 {
		t.Errorf("id after reload = %d, want 4 (ids are never reused)", p.ID)
	}
}

// TestPlayerCreateIsDeterministic confirms that the same master seeds and inputs
// reproduce an identical player — including derived seeds and password.
func TestPlayerCreateIsDeterministic(t *testing.T) {
	a := NewPlayerStore()
	pa, err := a.Create(testSeeds, "x@y.com", "alice", "(1,2)")
	if err != nil {
		t.Fatal(err)
	}
	b := NewPlayerStore()
	pb, err := b.Create(testSeeds, "x@y.com", "alice", "(1,2)")
	if err != nil {
		t.Fatal(err)
	}
	if pa != pb {
		t.Errorf("same seeds and inputs produced different players:\n a=%+v\n b=%+v", pa, pb)
	}
	if pa.Password == "" || pa.Seeds == (prng.Seeds{}) {
		t.Errorf("expected a generated password and non-zero seeds, got %+v", pa)
	}
}

// TestPlayerPasswordIsJSONSafe confirms the generated password carries no space
// and no character that would require JSON escaping.
func TestPlayerPasswordIsJSONSafe(t *testing.T) {
	s := NewPlayerStore()
	p, err := s.Create(testSeeds, "a@x.com", "alice", "(0,0)")
	if err != nil {
		t.Fatal(err)
	}
	if p.Password == "" {
		t.Fatal("password is empty")
	}
	if strings.ContainsAny(p.Password, " \t\n\r\"\\") {
		t.Errorf("password %q contains a space or JSON-escaping character", p.Password)
	}
}

// TestPlayerSeedsDependOnHandle confirms a player's derived seeds (and therefore
// password) are tied to the handle: this is why handles cannot change in a game.
func TestPlayerSeedsDependOnHandle(t *testing.T) {
	s := NewPlayerStore()
	alice, err := s.Create(testSeeds, "a@x.com", "alice", "(0,0)")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := s.Create(testSeeds, "b@x.com", "bob", "(0,0)")
	if err != nil {
		t.Fatal(err)
	}
	if alice.Seeds == bob.Seeds {
		t.Error("different handles derived identical seeds")
	}
	if alice.Password == bob.Password {
		t.Error("different handles produced identical passwords")
	}
}

// TestPlayerSeedsDependOnMaster confirms different games (master seeds) derive
// different player seeds for the same handle.
func TestPlayerSeedsDependOnMaster(t *testing.T) {
	a := NewPlayerStore()
	pa, _ := a.Create(prng.Seeds{Seed1: 1, Seed2: 2}, "a@x.com", "alice", "(0,0)")
	b := NewPlayerStore()
	pb, _ := b.Create(prng.Seeds{Seed1: 3, Seed2: 4}, "a@x.com", "alice", "(0,0)")
	if pa.Seeds == pb.Seeds {
		t.Error("different master seeds derived identical player seeds")
	}
}

// TestPlayerPasswordDependsOnProvince confirms the starting province keys the
// password: same master seeds and handle (so same player seeds), but a different
// province, yields a different password.
func TestPlayerPasswordDependsOnProvince(t *testing.T) {
	a := NewPlayerStore()
	pa, _ := a.Create(testSeeds, "a@x.com", "alice", "(0,0)")
	b := NewPlayerStore()
	pb, _ := b.Create(testSeeds, "a@x.com", "alice", "(1,1)")
	if pa.Seeds != pb.Seeds {
		t.Fatalf("seeds should depend only on the handle: %v vs %v", pa.Seeds, pb.Seeds)
	}
	if pa.Password == pb.Password {
		t.Error("different starting provinces produced identical passwords")
	}
}

// TestResetPasswordDiffersFromCreationAndIsStored confirms a reset produces a
// value different from the creation password and writes it onto the stored
// player (the value authentication validates against).
func TestResetPasswordDiffersFromCreationAndIsStored(t *testing.T) {
	s := NewPlayerStore()
	created, err := s.Create(testSeeds, "a@x.com", "alice", "(0,0)")
	if err != nil {
		t.Fatal(err)
	}

	reset, err := s.ResetPassword("a@x.com", 0)
	if err != nil {
		t.Fatal(err)
	}
	if reset.Password == created.Password {
		t.Error("reset password equals the creation password")
	}
	if reset.Password == "" {
		t.Fatal("reset password is empty")
	}
	// The new value is persisted on the stored player.
	stored, ok := s.ByEmail("a@x.com")
	if !ok {
		t.Fatal("player vanished after reset")
	}
	if stored.Password != reset.Password {
		t.Errorf("stored password = %q, want the reset value %q", stored.Password, reset.Password)
	}
	// Only the password changed.
	if stored.ID != created.ID || stored.Handle != created.Handle ||
		stored.Email != created.Email || stored.StartingProvince != created.StartingProvince ||
		stored.Seeds != created.Seeds {
		t.Errorf("reset changed more than the password:\n created=%+v\n stored=%+v", created, stored)
	}
}

// TestResetPasswordVariesByTurn confirms two resets in the same turn reproduce
// the same value, while resets in different turns differ.
func TestResetPasswordVariesByTurn(t *testing.T) {
	s := NewPlayerStore()
	if _, err := s.Create(testSeeds, "a@x.com", "alice", "(0,0)"); err != nil {
		t.Fatal(err)
	}

	turn0a, err := s.ResetPassword("a@x.com", 0)
	if err != nil {
		t.Fatal(err)
	}
	turn0b, err := s.ResetPassword("a@x.com", 0)
	if err != nil {
		t.Fatal(err)
	}
	turn1, err := s.ResetPassword("a@x.com", 1)
	if err != nil {
		t.Fatal(err)
	}

	if turn0a.Password != turn0b.Password {
		t.Errorf("same-turn reset was not reproducible: %q vs %q", turn0a.Password, turn0b.Password)
	}
	if turn0a.Password == turn1.Password {
		t.Error("reset did not vary across turns")
	}
}

// TestResetPasswordEmailLookupIsCaseInsensitive confirms the email key is matched
// after lowercasing, like every other email comparison.
func TestResetPasswordEmailLookupIsCaseInsensitive(t *testing.T) {
	s := NewPlayerStore()
	if _, err := s.Create(testSeeds, "Alice@Example.com", "alice", "(0,0)"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResetPassword("ALICE@EXAMPLE.COM", 0); err != nil {
		t.Errorf("case-insensitive email lookup failed: %v", err)
	}
}

// TestResetPasswordUnknownEmail confirms an email that matches no player is a
// distinct, testable error.
func TestResetPasswordUnknownEmail(t *testing.T) {
	s := newTestStore(t) // has alice@example.com
	if _, err := s.ResetPassword("nobody@example.com", 0); !errors.Is(err, ErrUnknownEmail) {
		t.Errorf("ResetPassword with unknown email: err = %v, want ErrUnknownEmail", err)
	}
}

// TestResetPasswordRejectsMissingEmail confirms a blank email is rejected before
// any lookup.
func TestResetPasswordRejectsMissingEmail(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ResetPassword("   ", 0); !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("ResetPassword with blank email: err = %v, want ErrInvalidEmail", err)
	}
}

// TestResetPasswordIsJSONSafe confirms a reset password, like a creation
// password, carries no space or JSON-escaping character.
func TestResetPasswordIsJSONSafe(t *testing.T) {
	s := NewPlayerStore()
	if _, err := s.Create(testSeeds, "a@x.com", "alice", "(0,0)"); err != nil {
		t.Fatal(err)
	}
	reset, err := s.ResetPassword("a@x.com", 3)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(reset.Password, " \t\n\r\"\\") {
		t.Errorf("reset password %q contains a space or JSON-escaping character", reset.Password)
	}
}

// TestPlayerCreatedActive confirms a newly created player is active and its
// zero-value Inactive flag is omitted from JSON.
func TestPlayerCreatedActive(t *testing.T) {
	s := NewPlayerStore()
	p, err := s.Create(testSeeds, "a@x.com", "alice", "(0,0)")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Active() {
		t.Error("a newly created player should be active")
	}
	buf, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(buf), "inactive") {
		t.Errorf("active player JSON should omit the inactive key: %s", buf)
	}
}

// TestPlayerDeactivateReactivate confirms removing a player marks it inactive
// without dropping the record, and reactivating restores it.
func TestPlayerDeactivateReactivate(t *testing.T) {
	s := newTestStore(t) // player 1 is "alice"

	removed, err := s.Deactivate(1)
	if err != nil {
		t.Fatal(err)
	}
	if removed.Active() {
		t.Error("player should be inactive after Deactivate")
	}
	// The record is retained, not dropped.
	if got, ok := s.ByID(1); !ok || got.Active() {
		t.Errorf("removed player must remain in the store, inactive: got %+v, ok=%v", got, ok)
	}
	if len(s.Players) != 1 {
		t.Errorf("Deactivate must not drop the record: len = %d, want 1", len(s.Players))
	}

	back, err := s.Reactivate(1)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Active() {
		t.Error("player should be active after Reactivate")
	}
	if got, _ := s.ByID(1); !got.Active() {
		t.Error("stored player should be active after Reactivate")
	}
}

// TestPlayerDeactivateErrors confirms the guard-rail errors: an unknown id and a
// double-deactivate.
func TestPlayerDeactivateErrors(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Deactivate(999); !errors.Is(err, ErrUnknownPlayer) {
		t.Errorf("Deactivate(unknown): err = %v, want ErrUnknownPlayer", err)
	}
	if _, err := s.Deactivate(1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Deactivate(1); !errors.Is(err, ErrAlreadyInactive) {
		t.Errorf("Deactivate(already inactive): err = %v, want ErrAlreadyInactive", err)
	}
}

// TestPlayerReactivateErrors confirms the guard-rail errors: an unknown id and
// reactivating an already-active player.
func TestPlayerReactivateErrors(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Reactivate(999); !errors.Is(err, ErrUnknownPlayer) {
		t.Errorf("Reactivate(unknown): err = %v, want ErrUnknownPlayer", err)
	}
	if _, err := s.Reactivate(1); !errors.Is(err, ErrAlreadyActive) {
		t.Errorf("Reactivate(already active): err = %v, want ErrAlreadyActive", err)
	}
}

// TestInactivePlayerStillHoldsEmailAndHandle confirms uniqueness spans inactive
// players: a removed player still occupies its email and handle, so neither can
// be reused.
func TestInactivePlayerStillHoldsEmailAndHandle(t *testing.T) {
	s := newTestStore(t) // alice@example.com / "alice"
	if _, err := s.Deactivate(1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(testSeeds, "alice@example.com", "someoneelse", "(0,0)"); !errors.Is(err, ErrDuplicateEmail) {
		t.Errorf("reusing an inactive player's email: err = %v, want ErrDuplicateEmail", err)
	}
	if _, err := s.Create(testSeeds, "new@example.com", "alice", "(0,0)"); !errors.Is(err, ErrDuplicateHandle) {
		t.Errorf("reusing an inactive player's handle: err = %v, want ErrDuplicateHandle", err)
	}
}

// TestPlayerJSONBackCompatDefaultsActive confirms a players.json written before
// the inactive field existed (no "inactive" key) reads as active.
func TestPlayerJSONBackCompatDefaultsActive(t *testing.T) {
	const legacy = `{"id":1,"handle":"alice","email":"a@x.com","starting_province":"(0,0)","password":"x","seeds":{"seed1":1,"seed2":2}}`
	var p Player
	if err := json.Unmarshal([]byte(legacy), &p); err != nil {
		t.Fatal(err)
	}
	if !p.Active() {
		t.Error("a player record with no inactive key should read as active")
	}
}

func TestParseProvince(t *testing.T) {
	for _, s := range []string{"(0,0)", "(-1,0)", "(2,-3)"} {
		got, err := ParseProvince(s)
		if err != nil || got != s {
			t.Errorf("ParseProvince(%q) = (%q, %v), want (%q, nil)", s, got, err, s)
		}
	}
	for _, s := range []string{"(0, 0)", "0,0", "(+1,0)", "north", ""} {
		if _, err := ParseProvince(s); !errors.Is(err, ErrInvalidProvince) {
			t.Errorf("ParseProvince(%q) err = %v, want ErrInvalidProvince", s, err)
		}
	}
}

func TestValidateHandle(t *testing.T) {
	valid := []string{"ab", "Al", "player-1", "j.doe", "a_b.c-d", "Bo Peep"}
	invalid := []string{"a", "1player", "Bo  Peep", "Bo ", " Bo", "joe!", "", "Bo\tPeep"}
	for _, h := range valid {
		if err := ValidateHandle(h); err != nil {
			t.Errorf("ValidateHandle(%q) = %v, want nil", h, err)
		}
	}
	for _, h := range invalid {
		if err := ValidateHandle(h); !errors.Is(err, ErrInvalidHandle) {
			t.Errorf("ValidateHandle(%q) = %v, want ErrInvalidHandle", h, err)
		}
	}
}

func TestHexString(t *testing.T) {
	for _, tc := range []struct {
		h    Hex
		want string
	}{
		{Hex{0, 0}, "(0,0)"},
		{Hex{-1, 0}, "(-1,0)"},
		{Hex{2, -3}, "(2,-3)"},
	} {
		if got := tc.h.String(); got != tc.want {
			t.Errorf("Hex%v.String() = %q, want %q", tc.h, got, tc.want)
		}
	}
}
