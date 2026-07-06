// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import "fmt"

// startingProvinceDirs are the six flat-top directions, in the deterministic
// order the default starting provinces are always listed: N, NE, SE, S, SW, NW.
// The order is a frozen output contract (see content/docs/reference/world-generation.md).
var startingProvinceDirs = [6]Hex{DirN, DirNE, DirSE, DirS, DirSW, DirNW}

// DefaultStartingProvinceRing returns the default ring distance for a world of
// the given ring count: ceil(rings / 2), halfway out toward the outermost ring.
// See content/docs/reference/world-generation.md.
func DefaultStartingProvinceRing(rings int) int {
	return (rings + 1) / 2
}

// StartingProvinces returns the six default starting provinces at the given ring
// distance from the origin: one in each flat-top direction, each the direction
// vector scaled by distance, as canonical compact "(q,r)" strings in the order
// N, NE, SE, S, SW, NW. The distance must be positive.
//
// The selection is purely geometric; terrain is not consulted. See
// content/docs/reference/world-generation.md for the rule.
func StartingProvinces(distance int) ([]string, error) {
	if distance <= 0 {
		return nil, fmt.Errorf("starting-province distance must be > 0, got %d", distance)
	}
	out := make([]string, 0, len(startingProvinceDirs))
	for _, dir := range startingProvinceDirs {
		out = append(out, dir.Scale(distance).String())
	}
	return out, nil
}
