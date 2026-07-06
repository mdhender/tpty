---
title: Counter-Based PRNGs
weight: 2
---

This page is *about* how the game draws random numbers. The observable promise —
that the same master seeds always produce the same game — is stated in the
[world generation reference]({{< ref "/docs/reference/world-generation.md" >}}).
Here we discuss how that promise is kept, why the design looks the way it does,
and where it sits in the wider literature.

## Two demands that ordinary randomness can't meet

A conventional random number generator has hidden state: each call advances a
sequence, and the number you get depends on how many calls came before. That is
fine for a card game, but it fails two things the engine needs at once.

The first is **reproducibility**. Given the same seeds, the world must come out
identical, every time, on any machine. The second, less obvious, is **order
independence**. We want to ask "what is the terrain of the province at `(q, r)`?"
and get the same answer no matter what else the engine did first, and no matter
what order we visit provinces in. A single advancing sequence entangles
everything: draw terrain and then weather from it, and adding one terrain roll
shifts every weather roll; iterate provinces in a different order and the whole
map changes.

## A stream is a hash of its address

The way out is to stop thinking of randomness as a sequence you consume and
start thinking of it as a *function you query*. Give every draw an **address**,
and derive a private stream by hashing that address together with the master
seeds:

```
stream = PRNG( hash(seed1, seed2, address) )
```

Because the stream is a pure function of the seeds and the address, *when* and
*in what order* you compute it stop mattering. The province at `(q, r)` has one
address, so one stream, so one terrain — fixed forever for a given pair of seeds.
Order independence is not something we carefully maintain; it falls out of the
construction, because nothing in the address depends on iteration order. This is
exactly why terrain is drawn per-province rather than from one sweep around the
map (see the [world generation explanation]({{< ref "/docs/explanation/world-generation.md" >}})).

## This is a known construction

It is worth naming what this is, because we did not invent it and the prior art
carries hard-won lessons. The same idea appears, under different names, across
scientific computing and machine learning:

- **Counter-based random number generators.** A stateless function of a *key* and
  a *counter* whose output is a deterministic, well-mixed hash of the two. The
  Random123 library (Philox, Threefry) popularized these for massively parallel
  simulation, precisely because a `(key, counter)` pair can be evaluated anywhere,
  in any order, with no shared state.
- **Spawn keys.** NumPy's `SeedSequence` addresses independent streams by a
  *spawn key* — a tuple of integers naming a node in a tree of generators —
  which it hashes together with the seed. Our address is a spawn key by another
  name.
- **Fold-in / split.** JAX threads an explicit key and derives child keys by
  *folding in* an integer. Same move: mix an identifier into a key to get an
  independent stream.
- **Domain separation.** The cryptographic practice of prefixing a distinct label
  so that different uses of one hash can never collide. Our rule that each
  address begins with a purpose tag is domain separation, plain and simple.

Recognizing the design as a counter-based, spawn-keyed construction is not just
flattering bookkeeping. It tells us the shape is sound, it lends us a vocabulary,
and — as below — it validates a choice we might otherwise have second-guessed.

## Addressing: key paths and domain tags

An address is a **key path**: a short, flat sequence of integers. By convention
its first element is a **domain tag** — a named constant identifying the purpose
of the draw — and the remaining elements identify the specific instance:

```go
type Key int64

const KeyTerrain Key = iota + 1 // domain tags live in one enumerated block

stream := seeds.Stream(KeyTerrain, Key(q), Key(r))
```

An early draft split this into a string "key" for the purpose and an integer
"leaf" for the instance. Collapsing both into one integer type felt like a hack —
until the literature made clear they were never two different things. A spawn key
is just a path of integers; whether an element names a subsystem or a coordinate
is a matter of *convention and position*, not of type. So a single `Key` type,
with coordinates coerced into it, is not an abuse of the model — it *is* the
model. The cost is honest: the compiler no longer distinguishes a purpose from a
coordinate, so the discipline that keeps streams apart lives in convention, not
in types.

## Why the streams stay independent

Two draws that share an address share a stream, which produces *correlated*
randomness — a quiet, corrosive class of bug. Three properties keep addresses
apart, and each is easier to trust once you see it as domain separation over a
hashed path:

- **Distinct domain tags.** Because every address leads with a purpose constant
  from one enumerated set, two different purposes diverge at the first element.
  Reserving `0` as invalid (starting the enumeration at `1`) turns a forgotten tag
  into an obvious bug rather than a silent alias of the first real domain.
- **Unique instances within a domain.** Inside a purpose, the trailing elements
  must single out the instance. Terrain uses `(q, r)`, unique per province. A
  purpose whose addresses were ambiguous would collide with itself.
- **Length is part of the address.** `[K, q]` and `[K, q, r]` must not hash alike,
  so the construction incorporates the *number* of elements, not only their
  values — the same care NumPy takes to keep different depths of the spawn tree
  distinct.

## An address is a frozen contract

Here is the sharpest consequence, and the one most worth internalizing. Once a
game exists, its outcomes are welded to the exact addresses that produced them.
The numeric values of the domain tags, the order of the trailing elements, the
way a coordinate is turned into an integer — all of it is now part of the game's
compatibility surface, no less than a save-file format.

That makes the domain-tag enumeration **append-only**. Insert a new constant in
the middle of the block and `iota` renumbers everything after it, silently
rewriting every existing world. Change how an address is built and the same seeds
diverge. This is not a rule we impose for tidiness; it is a fact about hashing an
address, and the discipline simply acknowledges it.

## The choices we made, and what we gave up

Two design decisions are worth surfacing rather than leaving implicit.

We **separate the hashing from the generator**. The address is hashed to derive
seed material, and that material seeds a PCG generator that produces the actual
numbers. This mirrors NumPy's deliberate split between `SeedSequence` and its bit
generators, and it is the right layering: the addressing scheme and the generator
can each change without disturbing the other.

We hash with **SHA-256**, which is heavier than necessary. The counter-based
libraries use fast, purpose-built mixers — Threefry, Philox, or cheap integer
hashes with good avalanche — because their workloads draw billions of numbers.
Ours draws thousands, so the cost is invisible, and a cryptographic hash buys
certainty about mixing quality with no analysis on our part. We chose simplicity
and confidence over speed; it is a conscious trade, and one we would revisit only
if profiling ever demanded it.

## What we are deliberately leaving open

The framework above — a hashed key path, a leading domain tag, unique instance
addresses, length included, append-only and frozen — is enough to add any future
stream safely. What it does *not* do is decide, in advance, how each future
subsystem should shape its instance addresses. What identifies a weather roll, a
character, a turn? We will answer those as the subsystems arrive and we see how
their addresses actually want to be built. Terrain's `(q, r)` is the only one we
need today, and the rest can wait without any loss of safety.

## See also

- [World generation reference]({{< ref "/docs/reference/world-generation.md" >}}) — the determinism guarantee this design keeps
- [World generation explanation]({{< ref "/docs/explanation/world-generation.md" >}}) — why terrain is addressed per province
- [Random123: counter-based RNGs (D.E. Shaw Research)](https://github.com/DEShawResearch/random123)
- [NumPy SeedSequence & parallel generation (spawn / spawn_key)](https://numpy.org/doc/stable/reference/random/parallel.html)
- [JAX jax.random (fold_in / split)](https://docs.jax.dev/en/latest/jax.random.html)
