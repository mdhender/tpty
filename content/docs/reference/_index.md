---
title: Reference
weight: 4
sidebar:
  open: false
---

Information-oriented reference material — the rules of the game, described as they are.

This section is being built up one rule at a time. The complete draft lives
under [Resources]({{< relref "/docs/resources" >}}) until each rule has been
reviewed and moved here.

{{< cards >}}
  {{< card link="games" title="Games" subtitle="The top-level game, its master seeds, and the game.json manifest" >}}
  {{< card link="turns" title="Turns" subtitle="How a game advances: turn numbering and the per-turn lifecycle" >}}
  {{< card link="turn-processing" title="Turn Processing" subtitle="How the engine processes a turn: the tick timeline, order queue, and scheduler" >}}
  {{< card link="world-generation" title="World Generation" subtitle="How the hex world is generated from the master seeds" >}}
  {{< card link="determinism" title="Determinism" subtitle="Master seeds, streams, and the key-path addressing that keeps a game reproducible" >}}
  {{< card link="hex-geometry" title="Hex Geometry" subtitle="The apothem and the hex measurements derived from it" >}}
  {{< card link="players" title="Players" subtitle="Players in a game: identity, starting province, and order password" >}}
  {{< card link="factions" title="Factions" subtitle="Groups of entities under one controller — a player or an NPC" >}}
  {{< card link="entities" title="Entities" subtitle="Actors in the world: identity, owning faction, and location" >}}
  {{< card link="orders" title="Orders" subtitle="The orders file format, authentication, and the command summary for every order" >}}
  {{< card link="reports" title="Reports" subtitle="The per-player turn report: its contents and the start-of-turn snapshot it describes" >}}
  {{< card link="sql-schema" title="SQL Schema" subtitle="The SQLite storage backend: its tables and the server/engine boundary" >}}
  {{< card link="engine-storage" title="Engine Storage" subtitle="The engine's local JSON file store: the game.json manifest and file layout" >}}
  {{< card link="configuration" title="Configuration" subtitle="How tpty reads TPTY_ environment variables and .env files" >}}
  {{< card link="glossary" title="Glossary" subtitle="Terms used across the reference" >}}
{{< /cards >}}
