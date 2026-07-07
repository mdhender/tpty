// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"errors"
	"reflect"
	"testing"
)

func TestStartingProvinceSetAdd(t *testing.T) {
	s := NewStartingProvinceSet()

	// A non-canonical province is rejected and nothing is added.
	if _, err := s.Add("(0, 0)"); !errors.Is(err, ErrInvalidProvince) {
		t.Errorf("Add non-canonical: err = %v, want ErrInvalidProvince", err)
	}
	if s.Len() != 0 {
		t.Errorf("Len after failed Add = %d, want 0", s.Len())
	}

	// Adds return the canonical form and preserve insertion order.
	for i, p := range []string{"(0,0)", "(1,-1)", "(-2,0)"} {
		got, err := s.Add(p)
		if err != nil {
			t.Fatalf("Add(%q): %v", p, err)
		}
		if got != p {
			t.Errorf("Add(%q) = %q, want %q", p, got, p)
		}
		if s.Len() != i+1 {
			t.Errorf("Len after Add(%q) = %d, want %d", p, s.Len(), i+1)
		}
	}

	// A duplicate is rejected.
	if _, err := s.Add("(1,-1)"); !errors.Is(err, ErrDuplicateStartingProvince) {
		t.Errorf("Add duplicate: err = %v, want ErrDuplicateStartingProvince", err)
	}

	want := []string{"(0,0)", "(1,-1)", "(-2,0)"}
	if got := s.List(); !reflect.DeepEqual(got, want) {
		t.Errorf("List = %v, want %v (insertion order)", got, want)
	}
}

func TestStartingProvinceSetRemove(t *testing.T) {
	s, err := ParseStartingProvinceSet([]string{"(0,0)", "(1,-1)", "(-2,0)"})
	if err != nil {
		t.Fatal(err)
	}

	// Removing an absent province errors and changes nothing.
	if _, err := s.Remove("(9,9)"); !errors.Is(err, ErrUnknownStartingProvince) {
		t.Errorf("Remove absent: err = %v, want ErrUnknownStartingProvince", err)
	}
	// A non-canonical province is rejected before the membership check.
	if _, err := s.Remove("(0, 0)"); !errors.Is(err, ErrInvalidProvince) {
		t.Errorf("Remove non-canonical: err = %v, want ErrInvalidProvince", err)
	}
	if s.Len() != 3 {
		t.Errorf("Len after failed Remove = %d, want 3", s.Len())
	}

	// Removing the middle entry keeps the order of the rest.
	got, err := s.Remove("(1,-1)")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got != "(1,-1)" {
		t.Errorf("Remove returned %q, want %q", got, "(1,-1)")
	}
	if s.Contains("(1,-1)") {
		t.Error("Contains true for a removed province")
	}
	want := []string{"(0,0)", "(-2,0)"}
	if got := s.List(); !reflect.DeepEqual(got, want) {
		t.Errorf("List after Remove = %v, want %v", got, want)
	}
}

// TestStartingProvinceSetRemoveThenReAdd confirms a removed province may be added
// again — it goes to the end, since order is insertion order.
func TestStartingProvinceSetRemoveThenReAdd(t *testing.T) {
	s, err := ParseStartingProvinceSet([]string{"(0,0)", "(1,-1)"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Remove("(0,0)"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("(0,0)"); err != nil {
		t.Fatal(err)
	}
	want := []string{"(1,-1)", "(0,0)"}
	if got := s.List(); !reflect.DeepEqual(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestParseStartingProvinceSet(t *testing.T) {
	// Valid input round-trips, preserving order.
	in := []string{"(2,-2)", "(0,0)", "(-1,1)"}
	s, err := ParseStartingProvinceSet(in)
	if err != nil {
		t.Fatalf("ParseStartingProvinceSet(valid): %v", err)
	}
	if got := s.List(); !reflect.DeepEqual(got, in) {
		t.Errorf("List = %v, want %v", got, in)
	}

	// A non-canonical entry is rejected.
	if _, err := ParseStartingProvinceSet([]string{"(0,0)", "north"}); !errors.Is(err, ErrInvalidProvince) {
		t.Errorf("ParseStartingProvinceSet(bad): err = %v, want ErrInvalidProvince", err)
	}
	// A duplicate entry is rejected.
	if _, err := ParseStartingProvinceSet([]string{"(0,0)", "(0,0)"}); !errors.Is(err, ErrDuplicateStartingProvince) {
		t.Errorf("ParseStartingProvinceSet(dup): err = %v, want ErrDuplicateStartingProvince", err)
	}
}

// TestStartingProvinceSetListIsACopy confirms List returns a fresh slice that the
// caller can mutate without corrupting the set.
func TestStartingProvinceSetListIsACopy(t *testing.T) {
	s, err := ParseStartingProvinceSet([]string{"(0,0)", "(1,-1)"})
	if err != nil {
		t.Fatal(err)
	}
	got := s.List()
	got[0] = "(9,9)"
	if s.List()[0] != "(0,0)" {
		t.Error("mutating List's result changed the set")
	}
}

func TestDefaultStartingProvinceRing(t *testing.T) {
	// ceil(rings/2), halfway out toward the outermost ring.
	for _, tc := range []struct{ rings, want int }{
		{1, 1}, {2, 1}, {3, 2}, {4, 2}, {5, 3}, {6, 3}, {7, 4},
	} {
		if got := DefaultStartingProvinceRing(tc.rings); got != tc.want {
			t.Errorf("DefaultStartingProvinceRing(%d) = %d, want %d", tc.rings, got, tc.want)
		}
	}
}

func TestStartingProvincesWorkedExample(t *testing.T) {
	// The reference's worked example: rings=3, default d=2.
	got, err := StartingProvinces(2)
	if err != nil {
		t.Fatalf("StartingProvinces(2): %v", err)
	}
	want := []string{"(0,-2)", "(2,-2)", "(2,0)", "(0,2)", "(-2,2)", "(-2,0)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StartingProvinces(2) = %v, want %v (N, NE, SE, S, SW, NW order)", got, want)
	}
}

func TestStartingProvincesDistanceOne(t *testing.T) {
	// d=1 places the six at the origin's immediate neighbors.
	got, err := StartingProvinces(1)
	if err != nil {
		t.Fatalf("StartingProvinces(1): %v", err)
	}
	want := []string{"(0,-1)", "(1,-1)", "(1,0)", "(0,1)", "(-1,1)", "(-1,0)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StartingProvinces(1) = %v, want %v", got, want)
	}
}

func TestStartingProvincesAlwaysSixDistinctAtDistance(t *testing.T) {
	// For any positive distance the six are distinct and every one sits exactly
	// that many steps from the origin.
	for d := 1; d <= 20; d++ {
		got, err := StartingProvinces(d)
		if err != nil {
			t.Fatalf("StartingProvinces(%d): %v", d, err)
		}
		if len(got) != 6 {
			t.Fatalf("StartingProvinces(%d): %d provinces, want 6", d, len(got))
		}
		seen := make(map[string]bool, 6)
		for _, s := range got {
			if seen[s] {
				t.Errorf("StartingProvinces(%d): duplicate province %s", d, s)
			}
			seen[s] = true
			h, err := canonicalProvince(s)
			if err != nil {
				t.Errorf("StartingProvinces(%d): %q is not canonical: %v", d, s, err)
			}
			if dist := h.Distance(); dist != d {
				t.Errorf("StartingProvinces(%d): %s is at distance %d, want %d", d, s, dist, d)
			}
		}
	}
}

func TestStartingProvincesRejectsNonPositiveDistance(t *testing.T) {
	for _, d := range []int{0, -1, -5} {
		if _, err := StartingProvinces(d); err == nil {
			t.Errorf("StartingProvinces(%d) = nil error, want error", d)
		}
	}
}
