---
title: Run the application server with tapp
weight: 12
---

This guide walks an [operator]({{< relref "/docs/reference/glossary.md" >}})
through running the T'Pty application server, `tapp`: starting it against a
persistent database, using an ephemeral in-memory database for development,
checking that it is alive, and shutting it down cleanly.

`tapp` owns the database instance while it runs. It **opens** a database — it
never creates one. Bring an instance into being with `tdb` first (see
[Administer a database with tdb]({{< relref "/docs/how-to/administer-a-database.md" >}})).

Two conventions carry over from the rest of the toolchain:

- `--db-path` is the **directory** that holds the instance, not a file name. The
  store owns the file (`tpty.db`) beneath it. (The flag is `--db-path`, not
  `--path` — that spelling is reserved to `tdb`.)
- Every flag also resolves from a `TAPP_`-prefixed environment variable; a flag
  given on the command line wins.

## Start the server

Point `tapp` at an existing database directory and choose an interface and port
to listen on:

```sh
tapp serve --db-path path/to/instance --host localhost --port 8080
```

`--host` defaults to `localhost` and `--port` to `8080`, so both may be omitted.
`--db-path` is **required**. The server opens the instance, migrates it up to the
schema the binary expects, and starts listening. It refuses to start if the
directory holds no instance, or if the on-disk schema is **newer** than the
binary expects — deploy the matching binary, or migrate with `tdb` first.

The server logs its lifecycle through `log/slog`:

```
INFO server listening addr=127.0.0.1:8080
```

## Run an in-memory database for development

For quick local work, pass the sentinel `:memory:` as the database path:

```sh
tapp serve --db-path :memory:
```

This starts a **temporary** in-memory instance that is created fresh and **not
persisted** — everything is discarded when the server stops. To make it usable,
`tapp` seeds a well-known admin account and logs its credentials at startup:

```
WARN using an in-memory development database; data is NOT persisted email=admin@tpty.local password=tpty-dev-admin
```

Use that email and password to sign in while developing. Because the credentials
are fixed and printed in the clear, use `:memory:` for development only, never
for anything reachable by others.

## Check that the server is alive

The server exposes one unauthenticated liveness endpoint, `GET /healthz`, which
returns `200 OK` once it is up:

```sh
curl -i http://localhost:8080/healthz
```

```
HTTP/1.1 200 OK

ok
```

Use it as a readiness/liveness probe. No other routes exist yet — the RESTish
API and authentication arrive in a later release — so every other path returns
`404 Not Found`.

## Shut the server down

Send the process an interrupt (`Ctrl-C`) or a `SIGTERM`. `tapp` stops accepting
new connections and lets in-flight requests finish before exiting:

```
INFO shutdown signal received; shutting down gracefully
INFO server stopped
```

If a request does not finish within the shutdown timeout, the server stops
waiting and exits anyway.

## Check the version

Print the application (binary) version — no database needed:

```sh
tapp version
```
