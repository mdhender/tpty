---
title: Configuration
weight: 13
---

The `tpty` command is configured from **`TPTY_`-prefixed environment variables**
and, optionally, from **`.env` files** in the working directory. This page
describes how those sources combine.

## Flags and environment variables

Every `tpty` flag can also be set from a `TPTY_`-prefixed environment variable:
the flag `--data` is filled from `TPTY_DATA`, `--foo` from `TPTY_FOO`, and so on.
A value given on the command line takes precedence over the environment variable.

## `TPTY_ENV` and `.env` files

`TPTY_ENV` selects which `.env` files are loaded before any flag is parsed. It is
read straight from the operating-system environment — **not** a flag — because it
must be known before flags are resolved.

- If `TPTY_ENV` is **not set**, no `.env` files are loaded. Configuration then
  comes only from the real environment and the command line.
- If set, it must be one of `development`, `test`, `production`, or `claude`
  (`claude` is reserved for the coding agent's local work). Any other value is an
  error.

### File precedence

When `TPTY_ENV` is set, these files load from the working directory, highest
precedence first. A variable already set — by the real environment or by a
higher-precedence file — is **not** overwritten, so the first file to define a
variable wins.

| Priority | File | Git-ignored? | Secrets? | Purpose |
|---|---|---|---|---|
| Highest | `.env.<env>.local` | Yes | Yes | Environment-specific local overrides |
| 2nd | `.env.local` | Yes | Yes | Local overrides |
| 3rd | `.env.<env>` | No | Never | Shared, environment-specific variables |
| Lowest | `.env` | No | Never | Shared across all environments |

`<env>` is the value of `TPTY_ENV`. A missing file is skipped, not an error.

> Committed files (`.env`, `.env.<env>`) must never hold secrets; keep secrets in
> the git-ignored `.local` files.

Note: loading `.env.local` in the `test` environment is a deliberate departure
from `bkeepers/dotenv`.

## Resolution order

For any single setting, the effective value is the first of:

1. the command-line flag;
2. the `TPTY_`-prefixed environment variable — whether exported directly or
   sourced from a `.env` file (per the precedence table above);
3. the flag's default.
