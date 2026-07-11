---
title: Hold
weight: 3
---

`hold` is the explicit "do nothing this week" order. An entity that holds spends
the week idle: it takes no action and changes no world state.

```
hold
```

`hold` takes no parameters.

## Time cost

`hold` costs a fixed **7 days** (one week) — the standard-action cost listed for
it in the [command summary]({{< relref "/docs/reference/orders#command-summary" >}}).
The engine measures this as 7 ticks (see
[Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}})), so a
`hold` that becomes active on a working tick completes seven ticks later.

## Effect

None. `hold` changes nothing: the entity's location, faction, and all other
state are exactly as they were. It is recorded as **executed** — a real,
completed order, not a stub — so the turn report shows the entity chose to hold.

Use `hold` to keep an entity idle for a week, for example to pace a queue of
orders or to wait out another entity's action.

## See also

- [Orders]({{< relref "/docs/reference/orders" >}})
- [Move]({{< relref "/docs/reference/orders/move.md" >}})
- [Turn Processing]({{< relref "/docs/reference/turn-processing.md" >}})
- [Entities]({{< relref "/docs/reference/entities.md" >}})
