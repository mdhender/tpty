// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
	"slices"

	"github.com/mdhender/tpty/internal/cerrs"
)

// Errors returned when managing a game's allowed starting provinces.
const (
	ErrDuplicateStartingProvince = cerrs.Error("duplicate starting province")
	ErrUnknownStartingProvince   = cerrs.Error("unknown starting province")
)

// StartingProvinceSet is a game's allowed starting provinces: the provinces a
// player may be placed on. It keeps entries unique and in the order they were
// added, and is the in-memory form of the manifest's starting-provinces.json.
//
// Each province is a canonical compact "(q,r)" string. Entries are validated for
// canonical form and uniqueness only; a starting province is not required to
// name a province of the generated world.
//
// See content/docs/reference/world-generation.md for the rules.
type StartingProvinceSet struct {
	provinces []string
}

// NewStartingProvinceSet returns an empty set.
func NewStartingProvinceSet() *StartingProvinceSet {
	return &StartingProvinceSet{}
}

// ParseStartingProvinceSet builds a set from a list of province strings,
// preserving order. Each entry must be in canonical compact form and no entry
// may repeat; a non-canonical province is rejected with ErrInvalidProvince and a
// repeat with ErrDuplicateStartingProvince.
func ParseStartingProvinceSet(list []string) (*StartingProvinceSet, error) {
	s := &StartingProvinceSet{provinces: make([]string, 0, len(list))}
	for _, p := range list {
		if _, err := s.Add(p); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Add validates province and appends it to the set, returning the canonical form
// stored. A non-canonical province is rejected with ErrInvalidProvince; one
// already in the set with ErrDuplicateStartingProvince.
func (s *StartingProvinceSet) Add(province string) (string, error) {
	canonical, err := ParseProvince(province)
	if err != nil {
		return "", err
	}
	if s.Contains(canonical) {
		return "", fmt.Errorf("%s: %w", canonical, ErrDuplicateStartingProvince)
	}
	s.provinces = append(s.provinces, canonical)
	return canonical, nil
}

// Remove validates province and removes it from the set, returning the canonical
// form removed and preserving the order of the rest. A non-canonical province is
// rejected with ErrInvalidProvince; a province not in the set with
// ErrUnknownStartingProvince.
func (s *StartingProvinceSet) Remove(province string) (string, error) {
	canonical, err := ParseProvince(province)
	if err != nil {
		return "", err
	}
	for i, p := range s.provinces {
		if p == canonical {
			s.provinces = append(s.provinces[:i], s.provinces[i+1:]...)
			return canonical, nil
		}
	}
	return "", fmt.Errorf("%s: %w", canonical, ErrUnknownStartingProvince)
}

// Contains reports whether province (a canonical compact string) is in the set.
func (s *StartingProvinceSet) Contains(province string) bool {
	return slices.Contains(s.provinces, province)
}

// List returns the set's provinces in order as a fresh slice the caller may
// modify without affecting the set.
func (s *StartingProvinceSet) List() []string {
	out := make([]string, len(s.provinces))
	copy(out, s.provinces)
	return out
}

// Len returns the number of provinces in the set.
func (s *StartingProvinceSet) Len() int {
	return len(s.provinces)
}

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
