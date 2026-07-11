---
title: Turn Processing
weight: 9
---

This reference describes how the engine **processes** a turn: the tick timeline,
the order queue each entity carries, and the scheduler that decides the order in
which active orders resolve. It is engine-facing and measures time in **ticks**.
One tick is one day; the day costs players see in
[Orders]({{< relref "/docs/reference/orders" >}}) are the same unit. For the
player-facing view of a turn — numbering and the per-turn lifecycle — see
[Turns]({{< relref "/docs/reference/turns.md" >}}).

Processing is deterministic: the same master seeds and the same orders reproduce
the same turn (see [Determinism]({{< relref "/docs/reference/determinism.md" >}})).

## Ticks

A turn is processed as **32 ticks**, numbered `0`–`31`:

- **Tick 0 — setup.** The engine prepares the turn.
- **Ticks 1–30 — work.** The engine advances active orders; these are the 30
  working ticks.
- **Tick 31 — cleanup.** The engine finalizes the turn. It exists to apply
  effects that complete on tick 30 but land at the start of the next tick.

## Order queue

Each entity has an **order queue** (its order stack): a FIFO list of orders.

- The order at the **front** of the queue is the **active** order; only the
  active order is worked.
- New orders — including a turn's newly submitted orders — are appended to the
  **back** of the queue. So an order carried over from a previous turn finishes
  before any newly submitted order begins.
- The special `stop` order has its own queue behaviour, defined with that order.
  (`stop` is not yet part of these rules.)

## Working an order

The active order carries a **ticks-left-to-complete** counter, set when the
order becomes active to the order's time cost — its cost in days, taken as ticks
(see [Orders]({{< relref "/docs/reference/orders" >}})). A zero-time order has a
ticks-left of `0`.

- Each working tick reduces the active order's ticks-left by one. When ticks-left
  reaches `0` the order is **complete**. A zero-time order completes in the tick
  it becomes active, consuming no working tick.
- On completion, the order's effects apply either **immediately** (most orders)
  or **at the start of the next tick** — for example, a move updates the entity's
  location on the next tick. The completed order is then **popped** from the
  front of the queue, and the next order becomes active.
- An order still carrying ticks-left at the end of the turn stays on the queue
  and **resumes next turn** with its remaining ticks.

For example, a 7-day move starts with ticks-left `7`; when it reaches `0`, the
entity's location updates at the start of the next tick. Tick 31 exists so a move
that completes on tick 30 still applies within the turn.

## The tick scheduler

Within a working tick, many entities have an active order. The engine resolves
them in one total, deterministic order, keyed on three things in turn:

1. **Priority.** Each order type has a priority from `1` (resolved first) to `5`
   (resolved last). Every priority-1 order resolves before any priority-2 order,
   and so on. Priority `0` is a reserved sentinel meaning "priority not set"; an
   order that reaches the scheduler at priority `0` is a defect and the engine
   flags it.
2. **Location.** Within a priority group, orders are grouped by the location of
   the entity issuing them — a province today — and resolved in ascending
   coordinate order: by `q`, then by `r`.
3. **Seniority.** Within a location, orders resolve by the issuing entity's
   [seniority](#seniority): more senior resolves first. Seniority never ties, so
   it is the final, decisive key.

This produces a total ordering with no dependence on how entities happen to be
stored or iterated, which is what keeps processing deterministic.

### Priority by command

Priority is a fixed property of each command. Each command's entry states its
priority; the assignment follows these categories:

| Priority | Commands                                   |
|----------|--------------------------------------------|
| 1        | permission commands (for example admit, hostile) |
| 2        | zero-time commands and `wait`              |
| 3        | `move` and `fly`                           |
| 4        | all other commands                         |
| 5        | `sail`                                      |

Some commands named here are not yet defined; ignore those until they land.

## Seniority

Every province holds a **FIFO list** of the entities (and stacks) in it; every
stack likewise holds a FIFO list of its members. An entity's **seniority** is its
**position** in the list of the location it occupies: the earlier in the list, the
more senior. Because positions within a list are unique, no two entities in the
same location share a seniority — which is why seniority is a tie-free
tie-breaker for the scheduler.

Today all locations are provinces; stacks nest in the same way.

## Randomness

An order whose effect draws randomness seeds its stream from the game's master
seeds, keyed by a domain tag and the entity's identity, and — for most orders —
by the **turn and tick** as well, so a draw is reproducible and tied to when it
happens. See [Determinism]({{< relref "/docs/reference/determinism.md" >}}) for
the key-path mechanism.

## See also

- [Turns]({{< relref "/docs/reference/turns.md" >}})
- [Orders]({{< relref "/docs/reference/orders" >}})
- [Entities]({{< relref "/docs/reference/entities.md" >}})
- [Determinism]({{< relref "/docs/reference/determinism.md" >}})
- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
