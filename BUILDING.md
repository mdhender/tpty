# Building T'Pty

This repo contains both the Go game engine (`cmd/tpty` and the `tpty` package)
and the Hugo documentation site (`content/`, built with the Hextra theme).

## Prerequisites

- Go (see the version in `go.mod`)
- Hugo (extended), for the documentation site

## Build and test

```sh
go build ./...   # build the engine and commands
go test ./...    # run the engine tests
go vet ./...     # static checks
hugo             # build the documentation site into public/
```

All of these must be green before pushing. See `CLAUDE.md` for the full
development rules, including the pre-push check that `go.mod` still requires
`github.com/imfing/hextra` (a `go mod tidy` will drop it and break the Hugo
build).

## Generating a world

`tpty world generate` writes a world as JSON into the directory given by the
global `--data` flag. Rings must be greater than 0 and less than 100. Seeds are
optional; if omitted (or 0) they are chosen at random and reported so the world
can be reproduced.

```sh
go run ./cmd/tpty world generate --rings 5 --data games/alpha --seed1 7 --seed2 13
```

## The `games/` directory

`games/` holds generated engine data. Its contents are git-ignored (via
`games/.gitignore`), and it is split by owner:

- **`games/alpha/`** — for the maintainer's own testing.
- **`games/claude/`** — owned by the coding agent (Claude). The agent may leave
  artifacts here for review and clear it out as needed.

Use `--data games/alpha` or `--data games/claude` accordingly.
