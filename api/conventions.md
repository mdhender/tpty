# API conventions

Cross-cutting rules for the T'Pty application server's REST surface. The
per-endpoint contract lives in [openapi.yaml](openapi.yaml); this page covers
what applies across all of it. The spec is the **source of truth** — change it
first, then the code (the server handlers arrive in a follow-up; this pass is the
contract only).

## Authentication

Clients authenticate with an opaque, server-side **session token**.
`POST /auth/login` exchanges an account's **email + secret** for a session token;
every subsequent request presents it as `Authorization: Bearer <token>` (the
`sessionToken` security scheme in `openapi.yaml`). The token is **not** a JWT — it
carries no claims and is resolved against the `sessions` table on each request, so
revoking a session (or deactivating the account) takes effect immediately, on the
next call.

Two distinct secrets, stored two different ways (see the SQL Schema reference):

- The **account secret** is stored as a **bcrypt** hash (`accounts.password_hash`).
  It is verified on login and on the sensitive self-service routes
  (`POST /me/email`, `POST /me/secret`).
- The **session token** is high-entropy and stored **as-is, not hashed**
  (`sessions.token`); the server resolves it by equality.

Email is stored lowercased and matched case-insensitively.

## Authorization

Two authorization levels appear on this surface:

- **Server admin** — `accounts.is_admin`. The `/accounts` and `/admin` routes
  require it; a non-admin caller gets `403`.
- **Per-game role** — `memberships.is_gm`. The application surface only *reads*
  this today, via `GET /me/games` (`isGm` per seat). Roster mutation and the GM
  authority that goes with it are deferred to the engine surface.

## Routing and versioning

Routes carry **no version segment** — there is no `/v1` in the path — and are
served from the **root** (`GET /healthz`, `POST /auth/login`, …), matching the
liveness probe the server bootstrap already exposes. For the beta the surface is
unversioned and churn-friendly; if versioning is ever needed it will be handled
another way (e.g. a header), decided then, rather than by forking the path.

## Errors

A single error envelope across all endpoints:

```json
{
  "error": {
    "code": "some_stable_code",
    "message": "human-readable text",
    "requestId": "e2b1c0d4f5a6..."
  }
}
```

`code` is stable and machine-readable; `message` is for humans and may change;
`requestId` is the request's correlation id. The codes in use today:

| Code             | Typical status | Meaning                                             |
|------------------|----------------|-----------------------------------------------------|
| `bad_request`    | 400            | Malformed body, missing/invalid field, bad path id  |
| `unauthorized`   | 401            | Missing, malformed, expired, or invalid credentials |
| `forbidden`      | 403            | Authenticated but not allowed                       |
| `not_found`      | 404            | No such resource (or hidden from the caller)        |
| `conflict`       | 409            | Conflicts with current state (e.g. duplicate email) |
| `internal_error` | 500            | Unexpected server-side failure                      |

Codes are additive: new ones may be appended as endpoints land, but an existing
code's meaning does not change.

## Idempotency

The application surface needs no idempotency-key mechanism: its writes are already
safe to retry. `PATCH` updates are naturally idempotent; the `DELETE`-style
session revocations return `204` whether or not anything was still active;
creating a resource that would collide (a duplicate account email) returns `409`
rather than silently creating a second one.

Order submission and turn processing have a genuine idempotency requirement, but
those belong to the **game engine** surface, which is deferred and out of scope
for the application server.

## Scope

This contract is the **application** surface only, grounded in the server-side
`accounts` and `sessions` tables and the `memberships` boundary table. The
**game engine** surface — creating games, rosters, turns, orders, and the world
read model — is intentionally not part of it yet. The one membership-facing route
here, `GET /me/games`, is a read-only projection of the caller's own seats.
