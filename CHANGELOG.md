# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.9.0-beta] - 2026-07-11

Completes [epic #24](https://github.com/mdhender/tpty/issues/24): the MVP game
loop end to end — **create a game → add players → accept orders → process a turn
→ advance the turn counter → send out turn reports** — and moves the release
channel from alpha to beta.

### Added

- **Model layer.** A `games ──o< players ──o< factions ──o< entities` hierarchy.
  Orders authenticate as a player but act on entities, owned through factions.
  - Faction domain type and storage.
  - Entity domain type and storage.
  - Each player is seeded a faction and a starting entity at turn 1.
  - PRNG domain tags appended per consumer (none speculative).
- **Accept orders.**
  - Order parser with player authentication and ownership checks for all
    commands (IDs 0–29); every order's format is parsed and validated, execution
    is a mix of real and stub.
  - Per-turn order storage in the manifest.
  - `tpty orders submit` command.
  - `tpty orders list` status command.
  - Order-parser dev harness and fuzz test.
- **Process a turn.**
  - Turn-execution engine and command dispatch table (stub no-op default).
  - Real `Hold` and `Move` command handlers.
  - `tpty turn process` command.
- **Advance the turn.**
  - Turn-advance logic with guards.
  - `tpty turn advance` command.
- **Reports (manual delivery).**
  - Report generation and outbox file layout.
  - `tpty turn report` command.
- **Reference docs** (per CLAUDE.md rule #1, written before the features):
  `reference/factions.md`, `reference/entities.md`, `reference/reports.md`,
  `reference/turn-processing.md`, and the orders reference as
  `reference/orders/` (landing page plus per-command entries).
- **How-to guides** for the turn loop.
- MVP burndown list (`BURNDOWN.md`).

### Changed

- Release channel moved from **alpha** to **beta**.

[0.9.0-beta]: https://github.com/mdhender/tpty/releases/tag/v0.9.0-beta
