# MVP Burndown

The MVP is the full game loop:

> create a game → add players → accept orders → process a turn → advance the
> turn counter → send out turn reports.

This list is the ordered work to get there. It follows the repo's rules
(`CLAUDE.md`): **every feature ties back to a reference document**, so the
reference is written (or extended) *before* its code. Items are in dependency
order — later items assume earlier ones are done.

## Scope decisions

- **Model layer.** The domain gains a layer:
  `games ──o< players ──o< factions ──o< entities`. A game has many players, a
  player has many factions, a faction has many entities. Orders authenticate as
  a **player** but act on **entities**, which are owned through **factions**.
- **Parse every order; execute a subset.** The MVP **accepts and parses all**
  orders from the original rules (IDs 0–29): Hold, Move, Attack, Use, Take,
  Drop, Join, Study, Work, Buy, Sell, Follow, Explore, Persuade, Swear, Pay,
  Declare, Recruit, Form, Pillage/Tax, Execute, Terrorize, Wait, Armor, Tell,
  Garrison. The format of every order is known, so complete parsing and
  validation is achievable now. **Execution is a mix of real and stub**: some
  orders do their real work, others are accepted and parsed but stubbed at
  execution. **Which is which is decided per order, when we write that order's
  reference** — the reference tells us whether we can implement it yet.
- **Execution ordering** within a turn (which orders resolve when, the day/week
  cycle) is open design work, called out below.
- **Delivery is manual.** The GM pulls players' emailed orders by hand and mails
  reports out by hand. So "accept orders" = the GM drops a saved orders file in
  and runs a command; "send reports" = the engine writes per-player report files
  the GM then emails. **No email integration is in scope.**

## Already in place (context, not work)

- `tpty game create` — writes `game.json` (id, master seeds, `turn: 0`, file map).
- `tpty world generate` / `world render` / `world starting-provinces …`.
- `tpty player create | list | show | remove | reactivate | reset-password`.
- `Game.Turn` field exists in the manifest; nothing yet reads or advances it.

Of the six MVP verbs, **create a game** and **add players** are done.

## Reference docs (write first — nothing below can start without these)

1. **`reference/factions.md`.** Define a faction: it belongs to a player, a
   player may have many, and it owns entities. Its identity and its place
   between player and entity in the model.
2. **`reference/entities.md`.** Define the entity: id, owning faction, location
   (a province), and the fields needed by the orders implemented for real (add
   more as later orders leave stub state). `players.md` already says the
   player's first entity is created when the game begins — pin that down (turn 1
   seeds each active player a faction + starting entity).
3. **`reference/orders/` (landing page + per-command entries).** Too much for
   one doc: the orders machinery is a *file format* plus ~28 discrete commands,
   so the reference mirrors that (see #26). Write **`orders/_index.md`** now and
   complete: the authenticated opening record (game id, player id, password),
   the per-entity blocks in the historical shape (`Entity <id>, <name>` /
   `Orders: <n>` / one command per line), and the **master command summary table
   for all IDs 0–29** (args + time cost, grouped into ~6 families) — this table
   is the parsing spec and covers every order. Plus how auth failures / malformed
   orders / commands on unowned entities are reported. **Per-command entries
   (`orders/<command>.md`, per-command granularity) land later**, one as each
   order leaves stub state (item 15); a still-stubbed order needs no page — the
   summary table covers it.
4. **`reference/reports.md`.** A turn report: game state at the **start** of turn
   `N` (per `turns.md`), scoped to one player — their factions, their entities,
   and the province descriptions those entities see. The report format.
5. **Add `reference/turn-processing.md` (engine-facing, ticks); point `turns.md`
   at it.** The turn-execution model: the 32-tick timeline (0 setup / 1–30 work
   / 31 cleanup), each entity's FIFO order queue with a ticks-left counter, the
   scheduler's total resolution order — priority (1–5) → location → seniority
   (**the "ordering within the set" problem**), carryover of unfinished orders,
   and PRNG seeding by turn + tick + new domain tags. `turns.md` stays
   player-facing (days) with a pointer. The turn guards (no processing before
   orders collected, no double-processing, no advance before processing) are
   enforced by the process/advance commands (items 14, 17).

> Per-command entries under `reference/orders/` (and any subsystem references
> they pull in — skills, things/inventory, stacks, combat) are written **as each
> order is implemented for real** (item 15), not upfront. A stubbed order needs
> only its row in the `orders/_index.md` summary table; a real order needs its
> own `orders/<command>.md` entry plus whatever model it touches. This keeps the
> per-command detail and the subsystems off the critical path for the loop.

## Model layer: factions & entities

6. **PRNG domain tags — appended per consumer, none speculative.** Decided: no
   tags are added up front. `internal/prng/tags.go` is append-only and every new
   tag must ship a golden vector, so a tag is added only when a real consumer
   lands. Order-effect randomness uses a **per-order** tag, added as that order
   is implemented (item 15), keyed `[tag, entityId, turn, tick]` (see
   `turn-processing.md`). Seeded factions/entities have no names yet (no naming
   tags) and carry no private seeds. The key-path encoding itself is already
   frozen in `prng.go`.
7. **Faction domain type + storage** (implements `factions.md`). Struct, add
   file to `GameFiles` / `DefaultGameFiles`, load/save, CRUD as needed.
8. **Entity domain type + storage** (implements `entities.md`) — the minimal
   fields the loop and the first real orders need; grow it as orders leave stub
   state.
9. **Seed each player at turn 1** — on first advance into play, create one
   faction and one starting entity per active player in their starting province.

## Accept orders (parse everything)

10. **Orders parser + authentication** (implements `orders/_index.md`). Parse the file,
    validate the opening record, authenticate against the player record (game id
    match, player id + password; reject inactive players), and parse per-entity
    command blocks for **all** commands, rejecting commands on entities the
    player's factions don't own. Parsing is complete regardless of whether an
    order executes for real.
11. **Order storage in the manifest.** Extend `GameFiles` with a per-turn orders
    location; define the on-disk layout keyed by turn and player/entity.
12. **`tpty orders submit` command** — the GM drops a player's saved orders file
    in; ingest it into the current turn's store; refuse orders for a non-current
    turn; report accepted/rejected (and which orders parsed) clearly.
13. **`tpty orders list` (status) command** — show which active players have
    submitted for the current turn, so the GM knows when to process.

## Process a turn

14. **Turn-execution engine + command dispatch** (implements the processing
    section of `turns.md`). The day/week cycle, the command-resolution ordering,
    and a dispatch table routing every parsed order (IDs 0–29) to a handler —
    with a **stub handler as the default** so an unimplemented order is a no-op
    (recorded, not an error).
15. **Real command handlers, decided per order.** For each order we choose to
    implement: write its `orders/<command>.md` entry (and any subsystem model it
    needs), then replace its stub with the real handler. Deterministic. Track
    per-command; the split of real-vs-stub is made here, order by order, as the
    per-command references land.
16. **`tpty turn process` command** — run the engine for the current turn;
    enforce the guards (orders collected, not already processed); write results.

## Advance the turn counter

17. **Turn-advance logic** — increment `Game.Turn` (`N` → `N+1`) and persist
    `game.json`; refuse to advance an unprocessed turn. Advancing into turn 1
    triggers faction/entity seeding (item 9).
18. **`tpty turn advance` command** — the GM's "commit this turn, move on" action.

## Send out turn reports (manual delivery)

19. **Report generation** (implements `reports.md`) — build each active player's
    start-of-turn report from game state (their factions, entities, and the
    provinces those entities see).
20. **Report output location** in the manifest / file layout (per-player report
    files in an outbox directory).
21. **`tpty turn report` command** — generate and write every active player's
    report for the current turn. The GM emails these out by hand (no email
    integration).

## Close the loop

22. **End-to-end smoke run** on a throwaway game under `games/claude`: create →
    world → starting provinces → players → advance to turn 1 (factions/entities
    appear) → report → submit orders exercising real *and* stubbed commands →
    process → advance → report. Confirm the turn counter moves 0 → 1 → 2 and
    reports reflect start-of-turn state.
23. **Tests green** — `go test ./...` passes (CLAUDE.md rule #3): orders
    parsing/auth for all commands, real per-command behavior, stub no-ops,
    processing determinism, seeding, and the turn guards.
24. **How-to docs** (Diataxis) — "Accept orders", "Process and advance a turn",
    "Generate turn reports", mirroring the existing `how-to/` guides.
25. **Verify `go.mod` still requires `github.com/imfing/hextra`** before the push
    that lands the MVP (CLAUDE.md rule #4).
