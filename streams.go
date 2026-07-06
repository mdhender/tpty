// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand/v2"
)

// Seeds are the two master seeds that make a game deterministic. The same seeds
// always produce the same game.
//
// See content/docs/reference/world-generation.md for the observable guarantee
// and CLAUDE.md for the implementation constraints.
type Seeds struct {
	Seed1 uint64 `json:"seed1"`
	Seed2 uint64 `json:"seed2"`
}

// Key is one element of a stream's key path. The first element of a path is a
// domain tag (a named constant naming the stream's purpose); the remaining
// elements identify the specific instance — coerce identifiers such as
// coordinates or ids to Key.
type Key int64

// Domain tags identify the purpose of a stream and are the first element of a
// key path.
//
// This block is APPEND-ONLY and starts at 1 (0 is reserved as invalid). Never
// reorder or insert constants: iota would renumber the rest and silently change
// every existing game. See CLAUDE.md and
// content/docs/explanation/counter-based-prng.md.
const (
	KeyTerrain            Key = iota + 1 // world terrain generation
	KeyPlayerSeeds                       // a player's private seeds, keyed by a hash of the handle
	KeyPlayerSecret                      // a player's password, keyed by the starting province (q, r)
	KeyWorldSeeds                        // the world's private seeds, derived from the game's
	KeyPlayerPasswordReset               // a player's reset password, keyed by the current turn
)

// Stream returns a deterministic PRNG for the given key path.
//
// A stream is derived by hashing the master seeds together with the key path via
// SHA-256, then seeding a PCG source with the digest. Because a stream is fully
// determined by its key path, draws never depend on iteration order.
func (s Seeds) Stream(path ...Key) *rand.Rand {
	h := sha256.New()

	var buf [8]byte
	putUint64 := func(v uint64) {
		binary.BigEndian.PutUint64(buf[:], v)
		_, _ = h.Write(buf[:])
	}

	// Master seeds.
	putUint64(s.Seed1)
	putUint64(s.Seed2)

	// Length-prefixed key path, so paths of different lengths cannot collide.
	putUint64(uint64(len(path)))
	for _, k := range path {
		putUint64(uint64(k))
	}

	sum := h.Sum(nil)
	return rand.New(rand.NewPCG(
		binary.BigEndian.Uint64(sum[0:8]),
		binary.BigEndian.Uint64(sum[8:16]),
	))
}

// Derive returns child seeds derived from s along the given key path. It gives a
// subsystem (a world, a player) its own master seeds from a parent's,
// deterministically, so the subsystem can carry its own randomness.
//
// The derivation is a frozen compatibility surface: see CLAUDE.md.
func (s Seeds) Derive(path ...Key) Seeds {
	r := s.Stream(path...)
	return Seeds{Seed1: r.Uint64(), Seed2: r.Uint64()}
}
