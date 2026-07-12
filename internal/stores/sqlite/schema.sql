-- Copyright (c) 2026 Michael D Henderson. All rights reserved.

-- Schema for the T'Pty game engine's SQLite storage backend.
--
-- One shared database holds every game; each per-game row carries a game_id
-- (the surrogate games.id, not the human-facing code). The design mirrors the
-- JSON-file model documented under content/docs/reference/ (games.md,
-- world-generation.md, players.md, factions.md, entities.md, orders/, turns.md,
-- turn-processing.md, reports.md).
--
-- Frozen surfaces (see CLAUDE.md) stored here but NEVER renumbered: the terrain
-- enum (terrains.code 0..6), order command ids (0..29), and the PRNG seeds.
--
-- Seeds are uint64 in the engine but SQLite INTEGER is signed 64-bit; store the
-- bit-cast int64 (Go: int64(u) on write, uint64(i) on read) so no bits are lost.
--
-- Foreign keys are declared but SQLite only enforces them when the connection
-- runs `PRAGMA foreign_keys = ON;` — the store must set that on every connection.
--
-- The migration version is SQLite's PRAGMA user_version, managed by the ZombieZen
-- sqlitemigration package (it equals the number of migrations applied). There is
-- deliberately no version table in this schema.

-- ---------------------------------------------------------------------------
-- Global / static
-- ---------------------------------------------------------------------------

-- terrains is the frozen terrain enum and its Worldographer tile mapping. It is
-- global (not game-scoped) and seeded once. code matches the engine's Terrain
-- enum (Mountain=0 .. Badlands=6) and must never be renumbered.
CREATE TABLE terrains (
    code               INTEGER PRIMARY KEY,   -- 0..6, matches Terrain enum
    name               TEXT NOT NULL UNIQUE,  -- "Mountain", "Plains", ...
    worldographer_tile TEXT NOT NULL          -- for terrain-translation export
);

-- ---------------------------------------------------------------------------
-- Accounts & sessions (server authentication / authorization)
-- ---------------------------------------------------------------------------

-- accounts authenticate a person with the game server. An account is
-- server-level, not scoped to any one game. email is forced to lowercase by the
-- application before saving and must be unique. display_name is how the person
-- wants to be addressed (a convenience for administrators). inactive and
-- is_admin are booleans stored as INTEGER (0 = false, 1 = true). Timestamps are
-- Unix seconds (UTC).
CREATE TABLE accounts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,                    -- lowercased before saving
    display_name  TEXT NOT NULL DEFAULT '',               -- how the person wants to be addressed
    password_hash TEXT NOT NULL DEFAULT '*',               -- '*' is not a valid hash, so it fails every login
    inactive      INTEGER NOT NULL DEFAULT 0 CHECK (inactive IN (0, 1)),
    is_admin      INTEGER NOT NULL DEFAULT 0 CHECK (is_admin IN (0, 1)),
    created_at    INTEGER NOT NULL DEFAULT (unixepoch()),  -- Unix seconds (UTC)
    updated_at    INTEGER NOT NULL DEFAULT (unixepoch())   -- app sets on every update
);

-- sessions authenticate API requests. A session is minted at login; the client
-- presents its token as an opaque bearer credential. The token is a hex-encoded
-- random N-bit value — high enough entropy that it is stored as-is (NOT hashed)
-- and looked up directly. id is a separate, public session identifier used to
-- address a session in the API (e.g. to revoke one). Revocation records WHEN via
-- revoked_at (NULL = active) rather than the inactive flag the other tables use,
-- because the API reports on and reasons about session lifetime. Timestamps are
-- Unix seconds (UTC).
CREATE TABLE sessions (
    id         TEXT NOT NULL PRIMARY KEY,                -- opaque public session id
    account_id INTEGER NOT NULL REFERENCES accounts(id), -- the effective identity
    token      TEXT NOT NULL UNIQUE,                     -- hex-encoded random bearer token (stored as-is)
    issued_at  INTEGER NOT NULL,                         -- Unix seconds (UTC)
    expires_at INTEGER NOT NULL,                         -- Unix seconds (UTC)
    revoked_at INTEGER                                   -- NULL = active; set = revoked
);

CREATE INDEX sessions_by_account ON sessions(account_id);

-- ---------------------------------------------------------------------------
-- Games
-- ---------------------------------------------------------------------------

-- games is the top-level unit of play and the shared identity that every game_id
-- foreign key targets. id is an auto-assigned surrogate key; code is the game's
-- human-facing slug (the id the GM chooses and orders files carry): unique, 1..6
-- characters, uppercase letters and digits only. The engine's per-game state
-- (master seeds, current turn) lives in game_engine_state, NOT here, so engine
-- state stays out of the application/server tables. See
-- content/docs/reference/games.md.
CREATE TABLE games (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,  -- surrogate, globally unique, never reused
    code TEXT NOT NULL UNIQUE,               -- slug; the game's human-facing id
    CHECK (length(code) BETWEEN 1 AND 6),    -- 1..6 characters wide
    CHECK (code NOT GLOB '*[^A-Z0-9]*')      -- uppercase letters and digits only
);

-- ---------------------------------------------------------------------------
-- Memberships (server <-> engine boundary)
-- ---------------------------------------------------------------------------

-- memberships gives an account a seat at a game's table: the authorization that
-- an account may participate in a game, and in which role. is_gm = 1 marks the
-- game master; is_gm = 0 an ordinary player. This is the authorization layer and
-- is distinct from the game-domain players table below (display_name, seeds,
-- provinces). A GM holds a seat but gets no engine player record — a GM controls
-- nothing in the game. Removal is a soft delete (inactive = 1); the UNIQUE key
-- spans active + inactive, so re-seating reactivates the same row.
--
-- id is also the engine's player_id: globally unique (AUTOINCREMENT, never
-- reused), NOT a per-game sequence. The players table below keys off it, so a
-- player is identified the same way across the whole database.
CREATE TABLE memberships (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    game_id    INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    is_gm      INTEGER NOT NULL DEFAULT 0 CHECK (is_gm    IN (0, 1)),
    inactive   INTEGER NOT NULL DEFAULT 0 CHECK (inactive IN (0, 1)),
    UNIQUE (account_id, game_id),
    UNIQUE (game_id, id)   -- FK target for players (same-game enforcement)
);

-- ---------------------------------------------------------------------------
-- Engine per-game state
-- ---------------------------------------------------------------------------

-- game_engine_state is the engine's root state for one game: its two uint64
-- master seeds (bit-cast) and its current turn. It is deliberately separate from
-- the application games row (which carries only identity), keeping engine state
-- out of the application and server tables. One row per game. See
-- content/docs/reference/games.md and determinism.md.
CREATE TABLE game_engine_state (
    game_id      INTEGER NOT NULL PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
    seed1        INTEGER NOT NULL,            -- uint64 master seed (bit-cast)
    seed2        INTEGER NOT NULL,            -- uint64 master seed (bit-cast)
    current_turn INTEGER NOT NULL DEFAULT 0   -- 0 = setup; play begins at 1
);

-- ---------------------------------------------------------------------------
-- World
-- ---------------------------------------------------------------------------

-- worlds is the one generated world per game. Its seeds are derived from the
-- game's master seeds (TagWorldSeeds). See world-generation.md.
CREATE TABLE worlds (
    game_id INTEGER PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
    seed1   INTEGER NOT NULL,             -- world's derived seeds
    seed2   INTEGER NOT NULL,
    rings   INTEGER NOT NULL CHECK (rings > 0 AND rings < 100)
);

-- provinces is every hex of a world and its assigned terrain.
CREATE TABLE provinces (
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    q       INTEGER NOT NULL,
    r       INTEGER NOT NULL,
    terrain INTEGER NOT NULL REFERENCES terrains(code),
    PRIMARY KEY (game_id, q, r)
);

-- starting_provinces is a game's allowed starting-province set: the provinces a
-- player may be placed on. Entries are unique (the primary key). A starting
-- province MUST name a province of the generated world; the composite foreign
-- key into provinces enforces that (and, being ON DELETE CASCADE, drops a
-- starting entry if its province is removed). The set is unordered — list it by
-- (q, r).
CREATE TABLE starting_provinces (
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    q       INTEGER NOT NULL,
    r       INTEGER NOT NULL,
    PRIMARY KEY (game_id, q, r),
    FOREIGN KEY (game_id, q, r) REFERENCES provinces(game_id, q, r) ON DELETE CASCADE
);

-- ---------------------------------------------------------------------------
-- Players, factions, entities
-- ---------------------------------------------------------------------------

-- players are the engine attributes of a player seat. id is the global player_id
-- — it IS the membership's id (not a per-game sequence), so only a member with
-- is_gm = 0 gets a players row. game_id is kept as a plain column for querying
-- (which game this player is in); the (game_id, id) foreign key keeps it aligned
-- with the membership's game. Email is NOT stored here — it is an account
-- attribute, reached via the membership (players.id -> memberships.account_id ->
-- accounts.email). Removal is a soft delete (inactive = 1); the record and its
-- unique display_name are retained. See players.md.
CREATE TABLE players (
    id           INTEGER NOT NULL PRIMARY KEY,
    game_id      INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,               -- the player's in-game handle (seeds derive from it)
    start_q      INTEGER NOT NULL,            -- starting province
    start_r      INTEGER NOT NULL,
    password     TEXT NOT NULL,               -- plaintext shared secret
    seed1        INTEGER NOT NULL,            -- player's private seeds
    seed2        INTEGER NOT NULL,
    inactive     INTEGER NOT NULL DEFAULT 0,  -- 0 = active, 1 = removed
    UNIQUE (game_id, display_name),
    UNIQUE (game_id, id),                                          -- FK target for order_submissions
    FOREIGN KEY (game_id, id) REFERENCES memberships(game_id, id)  -- id = membership id, same game
);

-- factions are groups of entities under a single controller (a player or an
-- NPC). id is globally unique (AUTOINCREMENT, never reused), aligning with the
-- player_id; game_id is kept as a plain column for querying. controller_id is
-- scoped to controller_kind, so player ids and npc ids cannot be confused; for a
-- player controller it is the global player_id (memberships.id / players.id).
-- The UNIQUE (game_id, id) is redundant (id is already unique) but exists so
-- entities can FK (game_id, faction_id), forcing an entity to share its faction's
-- game. See factions.md.
CREATE TABLE factions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id         INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    display_name    TEXT NOT NULL,
    controller_kind TEXT NOT NULL CHECK (controller_kind IN ('player','npc')),
    controller_id   INTEGER NOT NULL CHECK (controller_id >= 1),
    UNIQUE (game_id, display_name),
    UNIQUE (game_id, id)   -- FK target for entities (same-game enforcement)
    -- controller_id names a player or npc; it cannot be a single FK target.
);

-- entities are the actors orders act on. id is globally unique (AUTOINCREMENT,
-- never reused); game_id is kept as a plain column for querying. Each belongs to
-- one faction and occupies one province (which may lie off the generated map).
-- The FK is on (game_id, faction_id) so an entity and its faction must share a
-- game. See entities.md.
CREATE TABLE entities (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id      INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,
    faction_id   INTEGER NOT NULL,
    loc_q        INTEGER NOT NULL,          -- occupied province
    loc_r        INTEGER NOT NULL,
    FOREIGN KEY (game_id, faction_id) REFERENCES factions(game_id, id)
);

-- ---------------------------------------------------------------------------
-- Orders: raw submissions + normalized parse tree
-- ---------------------------------------------------------------------------

-- order_submissions is one player's verbatim orders text for one turn. The raw
-- text is the source of truth and is re-parsed by the engine. See orders/.
CREATE TABLE order_submissions (
    game_id   INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    turn      INTEGER NOT NULL,
    player_id INTEGER NOT NULL,
    raw       TEXT NOT NULL,
    PRIMARY KEY (game_id, turn, player_id),
    FOREIGN KEY (game_id, player_id) REFERENCES players(game_id, id)  -- same game as the player
);

-- parsed_orders is the normalized parse of a submission: one row per order line,
-- grouped by the entity block it appeared in and ordered by seq.
CREATE TABLE parsed_orders (
    game_id    INTEGER NOT NULL,
    turn       INTEGER NOT NULL,
    player_id  INTEGER NOT NULL,
    entity_id  INTEGER NOT NULL,          -- from the entity block header
    seq        INTEGER NOT NULL,          -- order within the submission
    command_id INTEGER NOT NULL,          -- CommandID 0..29 (frozen)
    word       TEXT NOT NULL,             -- command word as written
    line       INTEGER NOT NULL,          -- 1-based source line
    col        INTEGER NOT NULL,          -- 1-based source column
    PRIMARY KEY (game_id, turn, player_id, entity_id, seq),
    FOREIGN KEY (game_id, turn, player_id)
        REFERENCES order_submissions(game_id, turn, player_id) ON DELETE CASCADE
);

-- parsed_order_args is the ordered raw argument fields of a parsed order.
CREATE TABLE parsed_order_args (
    game_id   INTEGER NOT NULL,
    turn      INTEGER NOT NULL,
    player_id INTEGER NOT NULL,
    entity_id INTEGER NOT NULL,
    seq       INTEGER NOT NULL,
    arg_index INTEGER NOT NULL,           -- 0-based position
    value     TEXT NOT NULL,              -- raw field (quotes removed)
    PRIMARY KEY (game_id, turn, player_id, entity_id, seq, arg_index),
    FOREIGN KEY (game_id, turn, player_id, entity_id, seq)
        REFERENCES parsed_orders(game_id, turn, player_id, entity_id, seq)
        ON DELETE CASCADE
);

-- ---------------------------------------------------------------------------
-- Turn results (denormalized)
-- ---------------------------------------------------------------------------

-- turn_results marks a turn as processed: the existence of the row is the
-- "already processed" marker. See turn-processing.md.
CREATE TABLE turn_results (
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    turn    INTEGER NOT NULL,
    PRIMARY KEY (game_id, turn)
);

-- turn_outcomes is the per-order outcome, in completion order. Order data is
-- flattened (args joined to text) so the report writer needs no join back to
-- parsed_orders.
CREATE TABLE turn_outcomes (
    game_id    INTEGER NOT NULL,
    turn       INTEGER NOT NULL,
    seq        INTEGER NOT NULL,          -- completion order
    entity_id  INTEGER NOT NULL,
    command_id INTEGER NOT NULL,
    word       TEXT NOT NULL,
    args_text  TEXT NOT NULL,             -- denormalized args
    stub       INTEGER NOT NULL,          -- 0/1: handled by no-op stub
    message    TEXT NOT NULL,
    PRIMARY KEY (game_id, turn, seq),
    FOREIGN KEY (game_id, turn)
        REFERENCES turn_results(game_id, turn) ON DELETE CASCADE
);

-- turn_carryover is a per-entity order queue carried into the next turn. active
-- and ticks_left describe the front (active) order.
CREATE TABLE turn_carryover (
    game_id    INTEGER NOT NULL,
    turn       INTEGER NOT NULL,
    entity_id  INTEGER NOT NULL,
    active     INTEGER NOT NULL,          -- 0/1: front order activated?
    ticks_left INTEGER NOT NULL,          -- remaining ticks on the front order
    PRIMARY KEY (game_id, turn, entity_id),
    FOREIGN KEY (game_id, turn)
        REFERENCES turn_results(game_id, turn) ON DELETE CASCADE
);

-- turn_carryover_orders is the queued orders of a carryover queue, flattened and
-- kept in queue position (seq).
CREATE TABLE turn_carryover_orders (
    game_id    INTEGER NOT NULL,
    turn       INTEGER NOT NULL,
    entity_id  INTEGER NOT NULL,
    seq        INTEGER NOT NULL,          -- position in the queue
    command_id INTEGER NOT NULL,
    word       TEXT NOT NULL,
    args_text  TEXT NOT NULL,             -- denormalized args
    line       INTEGER NOT NULL,
    col        INTEGER NOT NULL,
    PRIMARY KEY (game_id, turn, entity_id, seq),
    FOREIGN KEY (game_id, turn, entity_id)
        REFERENCES turn_carryover(game_id, turn, entity_id) ON DELETE CASCADE
);

-- turn_log is the ordered processing log of a turn, for the turn writer.
CREATE TABLE turn_log (
    game_id INTEGER NOT NULL,
    turn    INTEGER NOT NULL,
    seq     INTEGER NOT NULL,
    message TEXT NOT NULL,
    PRIMARY KEY (game_id, turn, seq),
    FOREIGN KEY (game_id, turn)
        REFERENCES turn_results(game_id, turn) ON DELETE CASCADE
);
