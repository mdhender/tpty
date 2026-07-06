---
title: Determinism
weight: 6
---

The engine is deterministic: the same master seeds always reproduce the same
game. Randomness is drawn from **streams** addressed by a **key path**, not from a
single shared sequence, so a draw depends only on its address and never on the
order in which draws are made.

This page describes the mechanism as it is. For why it is built this way — the
prior art, the trade-offs, and the reasoning — see
[Counter-Based PRNGs]({{< relref "/docs/explanation/counter-based-prng.md" >}}).

## Master seeds

A game records two master seeds, `seed1` and `seed2`, both `uint64` (the type
`tpty.Seeds`, with JSON fields `seed1` and `seed2`). Together they are the root of
the game's randomness. See [Games]({{< relref "/docs/reference/games.md" >}}).

## Streams

A stream is a deterministic PRNG addressed by a key path: `Seeds.Stream(path
...Key)`, where `Key` is an `int64`. The first element of the path is a **domain
tag** naming the stream's purpose; the remaining elements identify the specific
instance, with identifiers such as coordinates or ids coerced to `Key`.

```go
stream := seeds.Stream(KeyTerrain, Key(q), Key(r))
```

## Stream derivation

A stream is derived by hashing the master seeds together with the key path using
SHA-256, then seeding a PCG source (`math/rand/v2`) with the digest. The hash
input is, in order:

1. `seed1`, as a big-endian `uint64`;
2. `seed2`, as a big-endian `uint64`;
3. the number of elements in the key path, as a big-endian `uint64`;
4. each element of the key path, in order, each as a big-endian `uint64`.

The first sixteen bytes of the digest seed the PCG source as two big-endian
`uint64` values. Because the length of the key path is hashed before its
elements, paths of different lengths cannot collide — `[K, q]` and `[K, q, r]`
hash differently.

## Child seeds

`Seeds.Derive(path ...Key)` returns a child `Seeds` for a subsystem, such as a
world or a player. It draws two `uint64` values from the stream at that path and
returns them as the child's `seed1` and `seed2`. The subsystem then carries its
own randomness, derived from the parent's seeds but independent of them.

## Domain tags

The domain tag is the first element of every key path. The tags form a single
enumerated block that starts at `1`; `0` is reserved as invalid. The block is
**append-only**.

| Domain tag               | Value | Instance keyed by                        |
|--------------------------|------:|------------------------------------------|
| `KeyTerrain`             |     1 | province coordinates `(q, r)`            |
| `KeyPlayerSeeds`         |     2 | a hash of the player's handle            |
| `KeyPlayerSecret`        |     3 | the starting province `(q, r)`           |
| `KeyWorldSeeds`          |     4 | — (the world's seeds, from the game's)   |
| `KeyPlayerPasswordReset` |     5 | the current turn                         |

## Frozen surfaces

Once a game exists, its outcomes are welded to the addresses that produced them.
The following are a compatibility surface, like a save-file format, and do not
change while any game exists:

- **The key-path encoding** — the order of a path's elements, how identifiers are
  coerced to `Key`, and the length-prefix.
- **The domain-tag numbering** — the block is append-only; constants are never
  reordered or inserted, because `iota` would renumber the rest and silently
  change every existing game.

## Order independence

A stream is addressed by an item's own identity — its coordinates or id — not by
the order in which items are visited, so draws are independent of iteration
order. Code must never range over a Go map where the iteration order would
determine the order of draws.

## See also

- [Games]({{< relref "/docs/reference/games.md" >}}) — where the master seeds are recorded
- [World Generation]({{< relref "/docs/reference/world-generation.md" >}}) — the deterministic world built from the seeds
- [Counter-Based PRNGs]({{< relref "/docs/explanation/counter-based-prng.md" >}}) — why the design looks the way it does
- [Glossary]({{< relref "/docs/reference/glossary.md" >}}) — terms used above
