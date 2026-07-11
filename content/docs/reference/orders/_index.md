---
title: Orders
weight: 7
sidebar:
  open: false
---

An **orders file** is how a player tells the engine what their entities do in a
turn. A player prepares one orders file per turn and sends it to the GM, who
submits it to the engine (see [Turns]({{< relref "/docs/reference/turns.md" >}})
for where this falls in the per-turn lifecycle).

This page describes the orders file itself — its format, how it is
authenticated, and the full set of orders with their parameters. It is the
authoritative description of what the engine parses. The detailed behaviour of
each individual order is described in that order's own entry under this section,
added as the order is implemented.

## The orders file

An orders file is plain text. It has an **opening record** that authenticates
the file, followed by one **entity block** for each entity the player is issuing
orders to. Records are separated by blank lines; leading and trailing whitespace
on a line is not significant.

### Text fields

Some fields are **text**: the game id and password in the opening record, the
entity name on a header line, and the names of formed units in a names section.

A text field may be written with or without surrounding double quotes. Quotes
are required only when the text contains a space (or another character that
could be confused with a field separator); otherwise they are optional — the
engine accepts `Conan` and `"Conan"` alike.

The convention — and the form the engine uses when it generates a default order
set for a player — is to **quote every text field**, even one with no spaces.
The examples in this page follow that convention.

### Opening record

The first non-blank line is the opening record. It has three fields, separated
by whitespace:

1. the **game id**, as quoted text (see
   [Games]({{< relref "/docs/reference/games.md" >}}));
2. the **player id**, a positive integer (see
   [Players]({{< relref "/docs/reference/players.md" >}}));
3. the player's **password**, as quoted text (see
   [Players]({{< relref "/docs/reference/players.md#password" >}})).

For example:

```
"smoke-test-1" 3 "k9m2qphtx7"
```

The game id and password are text fields, shown quoted here per the convention
above. Because they are quoted text they never contain a space, so their quotes
are optional — but quoting them is recommended. The player id is a number, not a
text field, so it is never quoted.

### Entity blocks

Each entity block gives the orders for one entity. A block is:

1. a **header line**, `entity <id>, <name>`, naming the entity by its id and its
   name;
2. zero or more **order lines**, one order each.

For example:

```
entity 101, "Conan the Copyright"
    drop  102
    move  1  2  3  2

entity 102, "Sendya"
    work  57  10
```

The `entity` keyword is matched case-insensitively, and order lines are
conventionally indented beneath the header as shown; the indentation is cosmetic
and not significant to the parser. The name is a text field — quote it per the
convention above — and is informational: the entity is identified by its id.

### Names section

An entity block that forms new units (see the `Form` order) may end with a
**names section**: the line `names:` (matched case-insensitively) followed by one line per newly formed unit,
each giving the forming entity's id and the name to give the unit it forms:

```
entity 204, "King Loric the Dread"
    study 39 14
    form  9 16 2000 2
    buy   40 40 1 1
    study 9 45
names:
    204 "The Slaves of Darkness"
```

## Order lines

An order line is a **command word** followed by its **parameters**, separated by
whitespace:

- The command word is one of the orders in the [command summary](#command-summary)
  below. It is matched case-insensitively.
- Parameters identify game objects. Most — entities, skills, things, directions
  — are referred to by their number. A **province** is referred to by its id,
  which is its coordinate in compact form, for example `(-1,0)` (see
  [World Generation]({{< relref "/docs/reference/world-generation.md" >}})). Which
  values are valid for a given parameter is defined by that object's own reference
  (for example, the directions of movement, or the skill list) and by the
  individual order's entry.
- Some orders take an optional or a variable-length parameter list. `Move`, for
  example, takes a path of one or more direction numbers. The
  [command summary](#command-summary) marks required parameters with `< >`,
  optional parameters with `[ ]`, and a repeated (one-or-more) parameter with a
  trailing `…`.

Use spaces to separate fields; do not use tab characters. Spacing is otherwise
at the writer's discretion.

See the [Grammar]({{< relref "/docs/reference/orders/grammar.md" >}}) for a
compact per-order statement of each command word and its parameters.

## Authentication

The opening record authenticates the whole file. The engine validates it against
the [player]({{< relref "/docs/reference/players.md" >}}) record:

- the game id must match the game;
- the player id must name a player in the game;
- the password must match the password stored in that player's record;
- the player must be active.

A file whose opening record fails any of these checks is rejected in full.

## Ownership

Every entity named in an entity block must belong to the authenticated player —
that is, it must be one of the entities owned by one of that player's factions.
An order given to an entity the player does not own is rejected.

## Rejected orders

The engine reports what it could not accept rather than failing silently:

- A malformed opening record, or one that fails authentication, rejects the file.
- An unknown command word, a malformed order line, or an order given to an
  unowned entity is reported.

Every order in the file is parsed and validated regardless of whether the engine
can yet execute it: an order that parses successfully is accepted even when its
effect is not yet implemented.

## Command summary

The tables below list every order, its parameters, and its base time cost in
days. This is the parsing specification: it covers all orders. Detailed
behaviour lives in each order's own entry, added as the order is implemented.

Conventions (the same notation as the
[Grammar]({{< relref "/docs/reference/orders/grammar.md" >}})):

- `<parameter>` is required; `[parameter]` is optional; a trailing `…` marks a
  repeated (one-or-more) parameter.
- `varies` means the time cost depends on the order's parameters or outcome; it
  is defined in the order's own entry.

### Movement & position

| ID | Command  | Parameters        | Time (days) |
|----|----------|-------------------|-------------|
| 0  | Hold     |                   | 7           |
| 1  | Move     | `<direction>…`    | varies      |
| 2  | Attack   | `[direction]`     | varies      |
| 14 | Explore  |                   | 7           |
| 26 | Wait     | `[days]`          | varies      |
| 29 | Garrison | `[state]`         | 0           |

### Stacks

| ID | Command | Parameters     | Time (days) |
|----|---------|----------------|-------------|
| 4  | Take    | `<unit>`       | 7           |
| 5  | Drop    | `[unit]`       | 0           |
| 6  | Join    | `<stack>`      | 0           |
| 12 | Follow  | `[entity]`     | 28          |
| 16 | Swear   | `[lordEntity]` | 0           |

### Skills & work

| ID | Command | Parameters                      | Time (days) |
|----|---------|---------------------------------|-------------|
| 3  | Use     | `[skill] [target] [modifier]`   | varies      |
| 8  | Study   | `<skill> [days]`                | varies      |
| 9  | Work    | `[skill] [options]`             | 7           |

### Goods & money

| ID | Command       | Parameters                        | Time (days) |
|----|---------------|-----------------------------------|-------------|
| 10 | Buy           | `<thing> [from] <offer> [number]` | 7           |
| 11 | Sell          | `<thing> [to] <price> [number]`   | 7           |
| 18 | Pay           | `<entity> <money> <moneyLeft>`    | 0           |
| 23 | Pillage / Tax | `<provinceId> [severity]`         | 7           |

### Followers & social

| ID | Command  | Parameters                                    | Time (days) |
|----|----------|-----------------------------------------------|-------------|
| 15 | Persuade | `<entity> [skill] [bribe]`                    | 7           |
| 19 | Declare  | `[entity] <opinion>`                          | 0           |
| 20 | Recruit  | `<numberSought> <payOffered>`                 | 14          |
| 21 | Form     | `<armor> [speciesHired] [amount] [numOrders]` | 0           |
| 28 | Tell     | `[entity] <yesNoNumber> [number]`             | 0+          |

### Force & control

| ID | Command   | Parameters                       | Time (days) |
|----|-----------|----------------------------------|-------------|
| 24 | Execute   | `<captive>`                      | 28          |
| 25 | Terrorize | `[provinceId] [severity] [mode]` | 7           |
| 27 | Armor     | `[newRating]`                    | varies      |

The ids `7`, `13`, `17`, and `22` are unused.

## See also

- [Turns]({{< relref "/docs/reference/turns.md" >}})
- [Players]({{< relref "/docs/reference/players.md" >}})
- [Games]({{< relref "/docs/reference/games.md" >}})
- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
