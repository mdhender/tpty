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
same game. This is an implementation constraint on engine code (the reference
only documents the observable guarantee).

- A game records two `uint64` master seeds, `seed1` and `seed2`, used to seed a
  PCG source (`math/rand/v2`).
- Derive PRNG streams from the master seeds rather than reusing one source.
  A stream is obtained by hashing the master seeds with a key path and a leaf
  identifier, then seeding a PCG source with the result. (TODO: confirm the
  "key"/"leaf" naming and the exact derivation once the seeding code exists.)
- Never range over a Go map where the iteration order would change the order of
  random draws. Sort keys or use an ordered structure instead.
