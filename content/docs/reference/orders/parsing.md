---
title: Parsing
weight: 2
---

The parser contract: how the engine reads an orders file and reports what it
cannot accept. It defines the error format, the recovery behaviour, and the
division between what the parser checks and what it defers to execution. For the
orders file format itself see [Orders]({{< relref "/docs/reference/orders" >}});
for the per-order parameter lists see
[Grammar]({{< relref "/docs/reference/orders/grammar.md" >}}).

## Error format

Each problem is reported as a single line:

```
line:column: message
```

- `line` is the 1-based source line.
- `column` is the 1-based character (rune) position within that line, pointing
  at the start of the offending word or field.
- `message` states the problem in actionable terms.

For example:

```
3:5: unknown command "movr"
6:13: pillage needs a province like (3,4), got "5"
```

## Multiple errors

The parser reports as many problems as it can find in a single pass. It does not
stop at the first error: a file with several mistakes yields several messages,
one per problem.

## Recovery and sync points

After reporting a problem the parser resumes at the next **sync point**. There
are two:

- **End of line.** A malformed order line is reported and the parser resumes at
  the start of the next line. The rest of the file, including the remaining
  orders in the same entity block, is still parsed.
- **Start of entity.** A malformed entity header is reported and the parser
  resumes at the start of the next entity — the next line whose first word is
  `entity`, the next `names:` line, or the end of the file. Order lines under a
  header that failed to yield a usable id are not attached to any block and
  raise no further errors.

## Fatal versus recoverable

A **fatal** problem rejects the whole file: nothing in it executes.

- A missing or malformed opening record (wrong field count, a non-integer or
  non-positive player id, or an unterminated quote).
- Any authentication failure (see
  [Orders]({{< relref "/docs/reference/orders#authentication" >}})).

A **recoverable** problem is reported against the order or entity it concerns and
does not stop the rest of the file from parsing.

- An unknown command word.
- A wrong argument count.
- A malformed argument.
- An order given to an entity the player does not own (see
  [Orders]({{< relref "/docs/reference/orders#ownership" >}})).

## Checked versus deferred

The parser is **sound**: it never rejects valid input. It validates only what it
can decide from the text alone and defers the rest to execution.

**Checked** at parse time:

- the command word is a known order (matched case-insensitively);
- the number of arguments is within the order's allowed range;
- each argument is a well-formed field — an integer (with an optional leading
  `-`), a province coordinate in the canonical compact form `(q,r)`, or text;
- a required province parameter is a province coordinate. A field that looks
  like a coordinate but is broken (for example `(3,` or `3,4)`) is a malformed
  argument.

**Deferred** to execution:

- whether a number names a real direction, skill, thing, or entity;
- the positional meaning of interleaved optional parameters (for example
  `buy <thing> [from] <offer> [number]`).

An order that parses successfully is accepted even when its effect is not yet
implemented; its arguments are carried through raw for execution to interpret.

## See also

- [Orders]({{< relref "/docs/reference/orders" >}})
- [Grammar]({{< relref "/docs/reference/orders/grammar.md" >}})
