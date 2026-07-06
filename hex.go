// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import "fmt"

// Hex is a position on the world grid in axial coordinates.
//
// See content/docs/reference/world-generation.md for the coordinate system.
type Hex struct {
	Q, R int
}

// String returns the hex's coordinates in the canonical compact form "(q,r)",
// with no spaces (for example, "(-1,0)"). This is the form used in data files,
// program input and output, and messages about a game; the spaced form "(q, r)"
// is only for prose. See content/docs/reference/world-generation.md.
func (h Hex) String() string {
	return fmt.Sprintf("(%d,%d)", h.Q, h.R)
}

// The six neighbor directions, as axial vectors. Listed clockwise from north.
var (
	DirN  = Hex{Q: 0, R: -1}
	DirNE = Hex{Q: +1, R: -1}
	DirSE = Hex{Q: +1, R: 0}
	DirS  = Hex{Q: 0, R: +1}
	DirSW = Hex{Q: -1, R: +1}
	DirNW = Hex{Q: -1, R: 0}
)

// Add returns the hex offset from h by d.
func (h Hex) Add(d Hex) Hex {
	return Hex{Q: h.Q + d.Q, R: h.R + d.R}
}

// Scale returns h scaled by k.
func (h Hex) Scale(k int) Hex {
	return Hex{Q: h.Q * k, R: h.R * k}
}

// Distance returns the number of steps from the origin to h.
func (h Hex) Distance() int {
	s := -h.Q - h.R
	return (iabs(h.Q) + iabs(h.R) + iabs(s)) / 2
}

// ringStepDirs are the step directions used to walk a ring clockwise, starting
// at the north corner. Each side of the ring is walked in one direction.
var ringStepDirs = [6]Hex{DirSE, DirS, DirSW, DirNW, DirN, DirNE}

// Ring returns the hexes of ring k, in clockwise order starting at the hex
// directly north of the origin, (0, -k). Ring 0 is the single origin hex; ring
// k (for k > 0) contains 6k hexes.
func Ring(k int) []Hex {
	if k <= 0 {
		return []Hex{{Q: 0, R: 0}}
	}
	hexes := make([]Hex, 0, 6*k)
	h := DirN.Scale(k) // the north corner, (0, -k)
	for _, dir := range ringStepDirs {
		for range k {
			hexes = append(hexes, h)
			h = h.Add(dir)
		}
	}
	return hexes
}

func iabs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
