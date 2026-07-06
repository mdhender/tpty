// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"reflect"
	"testing"
)

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
