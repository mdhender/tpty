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

The game id and password are quoted because they are quoted text: they contain
no characters that require JSON escaping and none that could be confused with a
space. The player id is a bare integer.

### Entity blocks

Each entity block gives the orders for one entity. A block is:

1. a **header line**, `Entity <id>, <name>`, naming the entity by its id and
   its name;
2. a **count line**, `Orders: <n>`, giving the number of order lines that
   follow;
3. exactly `<n>` **order lines**, one order each.

For example:

```
Entity 101, Conan the Copyright
Orders: 2
drop  102
move  1  2  3  2

Entity 102, Sendya
Orders: 1
work  57  10
```

The name on the header line is informational; the entity is identified by its
id. The count must equal the number of order lines in the block.

### Names section

An entity block that forms new units (see the `Form` order) may end with a
**names section**: the line `Names:` followed by one line per newly formed unit,
each giving the forming entity's id and the name to give the unit it forms:

```
Entity 204, King Loric the Dread
Orders: 4
study 39 14
form  9 16 2000 2
buy   40 40 1 1
study 9 45
Names:
204 The Slaves of Darkness
```

## Order lines

An order line is a **command word** followed by its **parameters**, separated by
whitespace:

- The command word is one of the orders in the [command summary](#command-summary)
  below. It is matched case-insensitively.
- Parameters are integers. Game objects — entities, skills, things, provinces,
  directions — are referred to by their number. Which numbers are valid for a
  given parameter is defined by that object's own reference (for example, the
  directions of movement, or the skill list) and by the individual order's entry.
- Some orders take an optional or a variable-length parameter list. `Move`, for
  example, takes a path of one or more direction numbers. The
  [command summary](#command-summary) marks optional parameters with `[ ]`.

Use spaces to separate fields; do not use tab characters. Spacing is otherwise
at the writer's discretion.

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
- An unknown command word, a malformed order line, a count that does not match
  the number of order lines, or an order given to an unowned entity is reported.

Every order in the file is parsed and validated regardless of whether the engine
can yet execute it: an order that parses successfully is accepted even when its
effect is not yet implemented.

## Command summary

The tables below list every order, its parameters, and its base time cost in
days. This is the parsing specification: it covers all orders. Detailed
behaviour lives in each order's own entry, added as the order is implemented.

Conventions:

- Parameters in `[ ]` are optional; a trailing `…` marks a variable-length list.
- `varies` means the time cost depends on the order's parameters or outcome; it
  is defined in the order's own entry.

### Movement & position

| ID | Command  | Parameters       | Time (days) |
|----|----------|------------------|-------------|
| 0  | Hold     |                  | 7           |
| 1  | Move     | Direction…       | varies      |
| 2  | Attack   | [Direction…]     | varies      |
| 14 | Explore  |                  | 7           |
| 26 | Wait     | [Days]           | varies      |
| 29 | Garrison | [State]          | 0           |

### Stacks

| ID | Command | Parameters    | Time (days) |
|----|---------|---------------|-------------|
| 4  | Take    | Unit          | 7           |
| 5  | Drop    | [Unit]        | 0           |
| 6  | Join    | Stack         | 0           |
| 12 | Follow  | [Entity]      | 28          |
| 16 | Swear   | [Lord-Entity] | 0           |

### Skills & work

| ID | Command | Parameters              | Time (days) |
|----|---------|-------------------------|-------------|
| 3  | Use     | [Skill] [Target] [Modifier] | varies  |
| 8  | Study   | Skill [Days]            | varies      |
| 9  | Work    | [Skill] [Options]       | 7           |

### Goods & money

| ID | Command      | Parameters                | Time (days) |
|----|--------------|---------------------------|-------------|
| 10 | Buy          | Thing [From] Offer [Number] | 7         |
| 11 | Sell         | Thing [To] Price [Number] | 7           |
| 18 | Pay          | Entity Money MoneyLeft    | 0           |
| 23 | Pillage / Tax | Province [Severity]      | 7           |

### Followers & social

| ID | Command  | Parameters                        | Time (days) |
|----|----------|-----------------------------------|-------------|
| 15 | Persuade | Entity [Skill] [Bribe]            | 7           |
| 19 | Declare  | [Entity] Opinion                  | 0           |
| 20 | Recruit  | NumberSought PayOffered           | 14          |
| 21 | Form     | Armor [SpeciesHired] [Amount] [NumOrders] | 0   |
| 28 | Tell     | [Entity] Y/N/Number [Number]      | 0+          |

### Force & control

| ID | Command   | Parameters                    | Time (days) |
|----|-----------|-------------------------------|-------------|
| 24 | Execute   | Captive                       | 28          |
| 25 | Terrorize | [Province] [Severity] [Mode]  | 7           |
| 27 | Armor     | [New Rating]                  | varies      |

The ids `7`, `13`, `17`, and `22` are unused.

## See also

- [Turns]({{< relref "/docs/reference/turns.md" >}})
- [Players]({{< relref "/docs/reference/players.md" >}})
- [Games]({{< relref "/docs/reference/games.md" >}})
- [Glossary]({{< relref "/docs/reference/glossary.md" >}})
