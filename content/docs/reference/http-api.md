---
title: HTTP API
weight: 13
---

The application server exposes a RESTish HTTP API for authentication, the
authenticated account's own context, and account administration. The
authoritative contract is the OpenAPI document at `api/openapi.yaml`; the
handlers, clients, and request validation are generated from and checked against
it. This page describes the surface; the spec is the source of truth for the
exact request and response shapes.

The API is grounded in the server-side tables of the
[SQL Schema]({{< relref "/docs/reference/sql-schema.md" >}}): `accounts` and
`sessions`, plus the `memberships` boundary table. It does **not** cover the game
engine (games, turns, orders, the world) — that surface lands with the engine.

## Base URL and versioning

Routes are served from the **root** with **no version segment** (`GET /healthz`,
`POST /auth/login`, …). The surface is unversioned for the beta.

## Authentication

`POST /auth/login` exchanges an account's **email + secret** for an opaque
**session token**. Every subsequent request presents it as
`Authorization: Bearer <token>`.

- The **account secret** is verified against the bcrypt `accounts.password_hash`.
- The **session token** is high-entropy; only its **SHA-256 hash** is stored, in
  `sessions.hashed_token` (the raw token is shown once, at login, and never
  stored). The server hashes a presented token and resolves it by equality on
  every request, so revocation (and account deactivation) takes effect
  immediately.

A session authenticates while `revoked_at IS NULL AND expires_at > now`.

## Authorization

- **Server admin** (`accounts.is_admin`) gates the `/accounts` and `/admin`
  routes; a non-admin caller gets `403`.
- **Per-game role** (`memberships.is_gm`) is surfaced read-only through
  `GET /me/games`.

## Endpoints

### System

| Method & path | Auth | Purpose |
|---|---|---|
| `GET /healthz` | none | Liveness probe; `200` with plain-text `ok`. |
| `GET /version` | none | Application version and database schema version (SQLite `user_version`). |

### Auth and session lifecycle

| Method & path | Auth | Purpose |
|---|---|---|
| `POST /auth/login` | none | Exchange email + secret for a session token. |
| `POST /auth/logout` | session | Revoke the current session (or all of the account's). |
| `GET /me/sessions` | session | List the caller's active sessions. |
| `DELETE /me/sessions/{sessionId}` | session | Revoke one of the caller's sessions. |

### The authenticated account (`/me`)

| Method & path | Auth | Purpose |
|---|---|---|
| `GET /me` | session | The caller's account. |
| `PATCH /me` | session | Update the caller's own `displayName`. |
| `POST /me/email` | session | Change email (requires the current secret). |
| `POST /me/secret` | session | Change the secret (requires the current secret; revokes other sessions). |
| `GET /me/games` | session | The games the caller holds a seat in, with `playerId` and `isGm` per seat. |

### Account administration (admin only)

| Method & path | Auth | Purpose |
|---|---|---|
| `GET /accounts` | admin | List accounts. |
| `POST /accounts` | admin | Create an account (optionally returns a generated secret once). |
| `GET /accounts/{accountId}` | admin | Get an account. |
| `PATCH /accounts/{accountId}` | admin | Update an account (also the account-recovery path). |
| `GET /accounts/{accountId}/sessions` | admin | List an account's active sessions. |
| `DELETE /accounts/{accountId}/sessions` | admin | Revoke all of an account's sessions. |
| `DELETE /accounts/{accountId}/sessions/{sessionId}` | admin | Revoke one of an account's sessions. |
| `POST /admin/sessions/purge` | admin | Delete expired session records. |

## Errors

Every error uses one envelope — a stable machine-readable `code`, a human
`message`, and the request's `requestId`:

```json
{ "error": { "code": "unauthorized", "message": "…", "requestId": "…" } }
```

The codes today are `bad_request` (400), `unauthorized` (401), `forbidden`
(403), `not_found` (404), `conflict` (409), and `internal_error` (500). New
codes may be appended; an existing code's meaning does not change.
