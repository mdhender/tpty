// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// testSeeds are fixed master seeds for player tests.
var testSeeds = Seeds{Seed1: 1, Seed2: 2}

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
	if pa.Password == "" || pa.Seeds == (Seeds{}) {
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
	pa, _ := a.Create(Seeds{Seed1: 1, Seed2: 2}, "a@x.com", "alice", "(0,0)")
	b := NewPlayerStore()
	pb, _ := b.Create(Seeds{Seed1: 3, Seed2: 4}, "a@x.com", "alice", "(0,0)")
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
