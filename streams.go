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

// Stream returns a deterministic PRNG for the given key and leaf.
//
// A stream is derived by hashing the master seeds together with the key (a
// string naming the purpose of the stream) and the leaf (values identifying the
// specific item within that purpose, e.g. a province's coordinates). The hash
// seeds a PCG source. Because a stream is fully determined by its key and leaf,
// draws never depend on iteration order.
func (s Seeds) Stream(key string, leaf ...int64) *rand.Rand {
	h := sha256.New()

	var buf [8]byte
	putUint64 := func(v uint64) {
		binary.BigEndian.PutUint64(buf[:], v)
		_, _ = h.Write(buf[:])
	}

	// Master seeds.
	putUint64(s.Seed1)
	putUint64(s.Seed2)

	// Length-prefixed key, so distinct (key, leaf) pairs cannot collide.
	putUint64(uint64(len(key)))
	_, _ = h.Write([]byte(key))

	// Length-prefixed leaf values.
	putUint64(uint64(len(leaf)))
	for _, v := range leaf {
		putUint64(uint64(v))
	}

	sum := h.Sum(nil)
	return rand.New(rand.NewPCG(
		binary.BigEndian.Uint64(sum[0:8]),
		binary.BigEndian.Uint64(sum[8:16]),
	))
}
