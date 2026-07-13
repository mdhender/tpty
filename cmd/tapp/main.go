// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Command tapp is the T'Pty application server. It owns the database instance
// while running and has no game-engine logic. This is the server bootstrap: it
// opens the database, starts an HTTP server exposing an unauthenticated
// liveness endpoint, and shuts down gracefully on SIGINT/SIGTERM.
//
// The RESTish API surface and authn/authz live in a separate ticket (#76,
// blocked by the OpenAPI spec #71); this binary otherwise starts with an empty
// router (requests 404).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdhender/tpty"
	"github.com/mdhender/tpty/internal/dotenv"
	"github.com/mdhender/tpty/internal/stores/sqlite"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"golang.org/x/crypto/bcrypt"
	"zombiezen.com/go/sqlite/sqlitex"
)

const (
	// memoryDBPath is the sentinel --db-path value that selects a temporary,
	// in-memory instance seeded with a well-known dev admin account.
	memoryDBPath = ":memory:"

	// The well-known dev admin credentials seeded into a :memory: instance. They
	// are fixed (not secret) and logged at startup so an operator can use them
	// against a throwaway database. They exist only for the in-memory dev mode; a
	// persistent instance carries the accounts tdb created.
	devAdminEmail  = "admin@tpty.local"
	devAdminSecret = "tpty-dev-admin"
	devAdminName   = "dev admin"

	// shutdownTimeout bounds the graceful shutdown: in-flight requests have this
	// long to finish before the server forces them closed.
	shutdownTimeout = 10 * time.Second
)

func main() {
	// Load .env files before parsing flags so ff reads TAPP_* variables sourced
	// from them. TAPP_ENV selects which files load (see dotenv) and is read
	// straight from the environment — not a flag — because it must be known
	// before any flag is parsed.
	if env, ok := os.LookupEnv("TAPP_ENV"); ok {
		if err := dotenv.Load(env); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: %q: %v\n", env, err)
			os.Exit(1)
		}
	}

	root := newRootCommand()

	// Resolve flags from TAPP_-prefixed environment variables when not given on
	// the command line. Command-line flags win.
	err := root.ParseAndRun(context.Background(), os.Args[1:], ff.WithEnvVarPrefix("TAPP"))
	switch {
	case errors.Is(err, ff.ErrHelp):
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", ffhelp.Command(root))
		os.Exit(0)
	case err != nil:
		_, _ = fmt.Fprintf(os.Stderr, "tapp: %v\n", err)
		os.Exit(1)
	}
}

// newRootCommand builds the tapp command tree:
//
//	tapp
//	├── serve       open the database and run the HTTP server
//	└── version     print the application version
//
// Commands take flags only; positional arguments are rejected.
func newRootCommand() *ff.Command {
	rootFlags := ff.NewFlagSet("tapp")
	version := rootFlags.BoolLong("version", "print version information and exit")

	root := &ff.Command{
		Name:      "tapp",
		Usage:     "tapp [FLAGS] SUBCOMMAND ...",
		ShortHelp: "run the T'Pty application server",
		Flags:     rootFlags,
		Exec: func(ctx context.Context, args []string) error {
			if *version {
				return showVersion()
			}
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}

	root.Subcommands = []*ff.Command{
		newServeCommand(rootFlags),
		newVersionCommand(rootFlags),
	}
	return root
}

// noArgs rejects positional arguments; tapp commands take flags only.
func noArgs(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
	}
	return nil
}

// serveOptions collects the resolved flags for the serve command.
type serveOptions struct {
	dbPath string // the store directory, or memoryDBPath for the ephemeral dev instance
	host   string // interface to bind
	port   string // TCP port to listen on
}

// newServeCommand builds "tapp serve". --db-path names the store directory (the
// flag is --db-path, not --path, which is reserved to tdb). --db-path :memory:
// selects a temporary in-memory instance seeded with a dev admin account.
func newServeCommand(parent *ff.FlagSet) *ff.Command {
	fs := ff.NewFlagSet("serve").SetParent(parent)
	dbPath := fs.StringLong("db-path", "", "`path` to the store directory, or :memory: for an ephemeral dev instance")
	host := fs.StringLong("host", "localhost", "`host` interface to bind")
	port := fs.StringLong("port", "8080", "`port` to listen on")
	return &ff.Command{
		Name:      "serve",
		Usage:     "tapp serve [FLAGS]",
		ShortHelp: "open the database and run the HTTP server",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := noArgs(args); err != nil {
				return err
			}
			if *dbPath == "" {
				return fmt.Errorf("--db-path is required")
			}
			// Cancel the context on SIGINT/SIGTERM so serve unblocks and shuts the
			// server down gracefully.
			ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
			defer stop()
			return serve(ctx, serveOptions{dbPath: *dbPath, host: *host, port: *port})
		},
	}
}

func newVersionCommand(parent *ff.FlagSet) *ff.Command {
	fs := ff.NewFlagSet("version").SetParent(parent)
	return &ff.Command{
		Name:      "version",
		Usage:     "tapp version",
		ShortHelp: "print the application version",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := noArgs(args); err != nil {
				return err
			}
			return showVersion()
		},
	}
}

// serve opens the database (seeding a dev admin for a :memory: instance), binds
// the listener, and runs the HTTP server until ctx is cancelled, then shuts down
// gracefully. It owns the database instance for the life of the server.
func serve(ctx context.Context, opts serveOptions) error {
	db, err := openServeDB(ctx, opts.dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	addr := net.JoinHostPort(opts.host, opts.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	return serveOn(ctx, ln, newMux())
}

// openServeDB opens the database for the given --db-path. The sentinel
// memoryDBPath opens a fresh temporary in-memory instance and seeds the
// well-known dev admin account, logging its credentials so the operator can use
// them; any other path opens an EXISTING persistent instance (never creating
// one) and migrates it up. The caller owns the returned DB and must Close it.
func openServeDB(ctx context.Context, dbPath string) (*sqlite.DB, error) {
	if dbPath == memoryDBPath {
		db, err := sqlite.OpenTemporary(ctx, "")
		if err != nil {
			return nil, err
		}
		if err := seedDevAdmin(ctx, db); err != nil {
			_ = db.Close()
			return nil, err
		}
		slog.Warn("using an in-memory development database; data is NOT persisted",
			"email", devAdminEmail, "password", devAdminSecret)
		return db, nil
	}
	return sqlite.OpenPersistent(ctx, dbPath)
}

// seedDevAdmin inserts the well-known dev admin account into db, bcrypt-hashing
// the fixed dev password. It is used only for the ephemeral :memory: instance so
// that a throwaway server has a usable admin login.
func seedDevAdmin(ctx context.Context, db *sqlite.DB) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(devAdminSecret), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash dev admin password: %w", err)
	}
	conn, err := db.Get(ctx)
	if err != nil {
		return err
	}
	defer db.Put(conn)
	if err := sqlitex.ExecuteTransient(conn,
		"INSERT INTO accounts (email, display_name, password_hash, is_admin) VALUES (?, ?, ?, 1);",
		&sqlitex.ExecOptions{Args: []any{devAdminEmail, devAdminName, string(hash)}},
	); err != nil {
		return fmt.Errorf("seed dev admin: %w", err)
	}
	return nil
}

// newMux builds the HTTP router. For now it exposes only GET /healthz, an
// unauthenticated liveness check; every other request 404s. The RESTish API and
// auth arrive in #76.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})
	return mux
}

// serveOn runs an HTTP server for handler on ln until ctx is cancelled, then
// shuts it down gracefully within shutdownTimeout. It returns nil on a clean
// shutdown and a non-nil error only if the server failed to serve or the
// graceful shutdown did not complete in time.
func serveOn(ctx context.Context, ln net.Listener, handler http.Handler) error {
	srv := &http.Server{Handler: handler}

	// Serve in the background; a real serve error (anything but the expected
	// ErrServerClosed from Shutdown) lands on errc.
	errc := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", ln.Addr().String())
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
			return
		}
		errc <- nil
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received; shutting down gracefully")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		slog.Info("server stopped")
		return nil
	}
}

// showVersion prints the application version.
func showVersion() error {
	fmt.Printf("tapp %s\n", tpty.Version())
	return nil
}
