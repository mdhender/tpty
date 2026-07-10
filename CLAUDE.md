# CLAUDE.md

Guidance for Claude Code and other agents working in this repository.

## Documentation structure

Docs live in `content/docs/`, organized with the [Diataxis](https://diataxis.fr)
framework: `tutorials/`, `how-to/`, `explanation/`, and `reference/`.

`reference/` is authoritative for the rules of the game.

## Do not consult `content/docs/resources/`

The `content/docs/resources/` directory holds unofficial, historical, and
background material (e.g. the original T'Nyc rules). It is **not** part of the
game and is likely to contradict the current rules.

**Never read, cite, or draw on anything under `content/docs/resources/` when
answering questions or generating content about T'Pty.** Treat `reference/` as
the source of truth instead.

## Development rules

These rules are strict. Do not deviate from them without explicit approval.

They govern **game features** — the code and rules that make up T'Pty itself.
The standalone tools under `cmd/` (e.g. `mapgen`, `hexover`) are orthogonal
scratch tooling and are out of scope for these rules for now.

No game rules exist yet (Reference is being built up rule by rule), so there is
nothing to test at present. Once rules land, "green tests" means `go test ./...`
passes.

1. **Every feature must tie back to a reference document.** Before implementing a
   feature, identify the rule under `content/docs/reference/` that it implements.
   If no such reference exists, stop and write (or ask for) the reference first —
   do not build features that are not grounded in `reference/`.

2. **Fix all bugs before introducing new features.** Known bugs take priority
   over new work. Do not start a new feature while there are open, unfixed bugs.

3. **All tests must be green before we push.** Run the full test suite and
   confirm it passes before any `git push`. Never push with failing, skipped, or
   unrun tests.

4. **Verify `go.mod` before every push.** This repo shares one `go.mod` between
   the Go code and the Hugo site, which uses the Hextra theme as a Hugo module
   (`github.com/imfing/hextra`, see `hugo.yaml`). `go mod tidy` REMOVES the
   `github.com/imfing/hextra` require, because Hextra ships no importable Go
   package. A local `hugo` build silently re-adds it, but pushing a `go.mod`
   without that require breaks the Hugo build for everyone. Before pushing,
   confirm `go.mod` still contains `github.com/imfing/hextra`; if a `go mod tidy`
   dropped it, run any `hugo` build to restore it, then commit. Do NOT try to
   fix this with a blank-import file (e.g. `resources/hextra.go`) — Hextra has no
   Go package, so that only breaks `go build`.

## Engine determinism (implementation)

The engine must be deterministic: the same master seeds always reproduce the
same game. This is an implementation constraint on engine code; the
[Determinism reference](content/docs/reference/determinism.md) is the
authoritative description of the mechanism (seeds, streams, key paths, and the
domain tags). Keep this section and that reference in sync.

This is a counter-based / spawn-keyed PRNG. See the
[Counter-Based PRNGs](content/docs/explanation/counter-based-prng.md)
explanation for the why; the reference for what it is; the rules below are the
constraints agents must hold to.

- A game records two `uint64` master seeds, `seed1` and `seed2` (`prng.Seeds`).
  The whole mechanism lives in the `github.com/mdhender/tpty/internal/prng`
  package.
- A stream is addressed by a **key path**: `Seeds.Stream(path ...Key)` (see
  `internal/prng/prng.go`), where `type Key int64`. It hashes the master seeds
  with the key path via SHA-256 and seeds a PCG source (`math/rand/v2`) with the
  digest. `Stream` returns a `*prng.Stream` (a `math/rand/v2.Source`); wrap it in
  `rand.New(...)` for the full `*rand.Rand` API.
- The first element of a key path is a **domain tag** — a named constant naming
  the stream's purpose, defined in `internal/prng/tags.go`. The remaining
  elements identify the specific instance; coerce identifiers (coordinates, ids)
  to `Key`. For example:

  ```go
  const TagTerrain Key = iota + 1 // domain tags: one enumerated block
  stream := rand.New(seeds.Stream(TagTerrain, Key(h.Q), Key(h.R)))
  ```

- **Keep every domain tag in one enumerated block, starting at 1** (reserve 0 as
  invalid). The block is **append-only**: never reorder or insert constants —
  `iota` would renumber the rest and silently change every existing game.
- **Do not change how an address is built** once games exist (the order of a key
  path's elements, the coercions, the length-prefixing). The key-path encoding
  is a frozen compatibility surface, like a save format.
- The hash **length-prefixes the key path**, so `[K, q]` and `[K, q, r]` cannot
  collide. Keep that.
- **Address a stream by the item's own identity** (coordinates, id), not by
  visit order, so draws are order-independent. Never range over a Go map where
  the iteration order would change the order of random draws.
