// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package prng

// The domain-tag registry: the leading element of every key path names the
// purpose of a draw, providing domain separation so two purposes can never
// share a stream. This is the single, authoritative place tags are defined.
//
// FROZEN SURFACE — APPEND ONLY. The block starts at 1 (0 is invalid, so a
// forgotten tag is an obvious bug rather than a silent alias). Never insert or
// reorder a constant: iota would renumber every tag after it and silently
// rewrite every live game. To add a tag, append it to the END of this block and
// pin a golden vector for its stream.
//
// The numbering is inherited unchanged from the engine's original registry (the
// retired streams.go), so every game generated before this package existed
// reproduces byte-for-byte.
const (
	_                      Key = iota // 0 is invalid — never use as a domain tag
	TagTerrain                        // 1: world terrain generation, addressed by (q, r)
	TagPlayerSeeds                    // 2: a player's private seeds, keyed by a hash of the handle
	TagPlayerSecret                   // 3: a player's creation password, keyed by the starting province (q, r)
	TagWorldSeeds                     // 4: the world's private seeds, derived from the game's
	TagPlayerPasswordReset            // 5: a player's reset password, keyed by the current turn
)
