// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import "testing"

func TestRingZeroIsOrigin(t *testing.T) {
	got := Ring(0)
	if len(got) != 1 || got[0] != (Hex{Q: 0, R: 0}) {
		t.Fatalf("Ring(0) = %v, want [{0 0}]", got)
	}
}

func TestRingSizeDistanceAndUniqueness(t *testing.T) {
	for k := 1; k <= 20; k++ {
		ring := Ring(k)

		if len(ring) != 6*k {
			t.Errorf("Ring(%d) has %d hexes, want %d", k, len(ring), 6*k)
		}

		// The walk starts directly north of the origin.
		if ring[0] != (Hex{Q: 0, R: -k}) {
			t.Errorf("Ring(%d) starts at %v, want %v", k, ring[0], Hex{Q: 0, R: -k})
		}

		seen := make(map[Hex]bool, len(ring))
		for _, h := range ring {
			if d := h.Distance(); d != k {
				t.Errorf("Ring(%d) hex %v is at distance %d, want %d", k, h, d, k)
			}
			if seen[h] {
				t.Errorf("Ring(%d) hex %v is duplicated", k, h)
			}
			seen[h] = true
		}
	}
}

func TestRingIsClockwiseContiguous(t *testing.T) {
	// Each consecutive hex in a ring must be a neighbor of the previous one, and
	// the ring must close back to the start.
	neighbors := map[Hex]bool{DirN: true, DirNE: true, DirSE: true, DirS: true, DirSW: true, DirNW: true}
	for k := 1; k <= 10; k++ {
		ring := Ring(k)
		for i := range ring {
			cur := ring[i]
			next := ring[(i+1)%len(ring)]
			step := Hex{Q: next.Q - cur.Q, R: next.R - cur.R}
			if !neighbors[step] {
				t.Errorf("Ring(%d): step from %v to %v is not a single neighbor move (%v)", k, cur, next, step)
			}
		}
	}
}
