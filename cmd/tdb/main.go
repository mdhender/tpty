// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Command tdb administers a T'Pty SQLite database. It is the operator tool:
// it creates and migrates instances, verifies and reports their version, backs
// them up, compacts them, and creates accounts. It assumes it is the only
// process touching the database during a migration.
package main

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"os"
	"strings"

	"github.com/mdhender/tpty"
	"github.com/mdhender/tpty/internal/dotenv"
	"github.com/mdhender/tpty/internal/phrases"
	"github.com/mdhender/tpty/internal/stores/sqlite"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Load .env files before parsing flags so ff reads TDB_* variables sourced
	// from them (e.g. TDB_SECRET, TDB_PATH). TDB_ENV selects which files load
	// (see dotenv) and is read straight from the environment — not a flag —
	// because it must be known before any flag is parsed.
	if env, ok := os.LookupEnv("TDB_ENV"); ok {
		if err := dotenv.Load(env); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: %q: %v\n", env, err)
			os.Exit(1)
		}
	}

	root := newRootCommand()

	// Resolve flags from TDB_-prefixed environment variables when not given on
	// the command line (e.g. --secret from TDB_SECRET). Command-line flags win.
	err := root.ParseAndRun(context.Background(), os.Args[1:], ff.WithEnvVarPrefix("TDB"))
	switch {
	case errors.Is(err, ff.ErrHelp):
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", ffhelp.Command(root))
		os.Exit(0)
	case err != nil:
		_, _ = fmt.Fprintf(os.Stderr, "tdb: %v\n", err)
		os.Exit(1)
	}
}

// newRootCommand builds the tdb command tree:
//
//	tdb
//	├── create
//	│   ├── database    create and migrate a new database
//	│   └── account     insert an account with a bcrypt password hash
//	├── update
//	│   └── account     change an existing account's fields
//	├── migrate
//	│   ├── up          migrate an existing database up
//	│   ├── verify      check the schema version equals the expected version
//	│   └── version     show the database schema version
//	├── backup          back up an instance (non-mutating)
//	├── compact         VACUUM an instance (non-mutating; offline)
//	└── version         print the application version
//
// --path (the directory holding the instance) is a global flag shared by every
// subcommand. Commands take flags only; positional arguments are rejected.
func newRootCommand() *ff.Command {
	rootFlags := ff.NewFlagSet("tdb")
	version := rootFlags.BoolLong("version", "print version information and exit")
	// path is a global flag: the directory that holds the instance. The store
	// owns the file name beneath it.
	path := rootFlags.StringLong("path", "", "`path` to the directory holding the instance")

	root := &ff.Command{
		Name:      "tdb",
		Usage:     "tdb [FLAGS] SUBCOMMAND ...",
		ShortHelp: "administer a T'Pty database",
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
		newCreateCommand(rootFlags, path),
		newUpdateCommand(rootFlags, path),
		newMigrateCommand(rootFlags, path),
		newBackupCommand(rootFlags, path),
		newCompactCommand(rootFlags, path),
		newVersionCommand(rootFlags),
	}
	return root
}

// noArgs rejects positional arguments; tdb commands take flags only.
func noArgs(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
	}
	return nil
}

// requirePath rejects an empty --path.
func requirePath(path string) error {
	if path == "" {
		return fmt.Errorf("--path is required")
	}
	return nil
}

// isFlagSet reports whether the named flag was actually set (on the command line
// or from its environment variable), as opposed to holding its default. It lets
// a command tell "the operator asked for this value" from "this is the default".
func isFlagSet(fs *ff.FlagSet, name string) bool {
	f, ok := fs.GetFlag(name)
	return ok && f.IsSet()
}

// newCreateCommand builds the "create" resource group and its subcommands.
func newCreateCommand(parent *ff.FlagSet, path *string) *ff.Command {
	createFlags := ff.NewFlagSet("create").SetParent(parent)
	create := &ff.Command{
		Name:      "create",
		Usage:     "tdb create [FLAGS] SUBCOMMAND ...",
		ShortHelp: "create a database or an account",
		Flags:     createFlags,
		Exec: func(ctx context.Context, args []string) error {
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}
	create.Subcommands = []*ff.Command{
		newCreateDatabaseCommand(createFlags, path),
		newCreateAccountCommand(createFlags, path),
	}
	return create
}

func newCreateDatabaseCommand(parent *ff.FlagSet, path *string) *ff.Command {
	fs := ff.NewFlagSet("database").SetParent(parent)
	return &ff.Command{
		Name:      "database",
		Usage:     "tdb create database [FLAGS]",
		ShortHelp: "create and migrate a new database",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := noArgs(args); err != nil {
				return err
			}
			if err := requirePath(*path); err != nil {
				return err
			}
			return createInstance(ctx, *path)
		},
	}
}

// newMigrateCommand builds the "migrate" resource group and its subcommands.
func newMigrateCommand(parent *ff.FlagSet, path *string) *ff.Command {
	migrateFlags := ff.NewFlagSet("migrate").SetParent(parent)
	migrate := &ff.Command{
		Name:      "migrate",
		Usage:     "tdb migrate [FLAGS] SUBCOMMAND ...",
		ShortHelp: "migrate a database up or show its schema version",
		Flags:     migrateFlags,
		Exec: func(ctx context.Context, args []string) error {
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}
	migrate.Subcommands = []*ff.Command{
		newMigrateUpCommand(migrateFlags, path),
		newMigrateVerifyCommand(migrateFlags, path),
		newMigrateVersionCommand(migrateFlags, path),
	}
	return migrate
}

func newMigrateUpCommand(parent *ff.FlagSet, path *string) *ff.Command {
	fs := ff.NewFlagSet("up").SetParent(parent)
	return &ff.Command{
		Name:      "up",
		Usage:     "tdb migrate up [FLAGS]",
		ShortHelp: "migrate an existing database up",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := noArgs(args); err != nil {
				return err
			}
			if err := requirePath(*path); err != nil {
				return err
			}
			return migrateInstance(ctx, *path)
		},
	}
}

func newMigrateVersionCommand(parent *ff.FlagSet, path *string) *ff.Command {
	fs := ff.NewFlagSet("version").SetParent(parent)
	return &ff.Command{
		Name:      "version",
		Usage:     "tdb migrate version [FLAGS]",
		ShortHelp: "show the database schema version",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := noArgs(args); err != nil {
				return err
			}
			if err := requirePath(*path); err != nil {
				return err
			}
			return showSchemaVersion(ctx, *path)
		},
	}
}

func newMigrateVerifyCommand(parent *ff.FlagSet, path *string) *ff.Command {
	fs := ff.NewFlagSet("verify").SetParent(parent)
	return &ff.Command{
		Name:      "verify",
		Usage:     "tdb migrate verify [FLAGS]",
		ShortHelp: "check the schema version equals the expected version",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := noArgs(args); err != nil {
				return err
			}
			if err := requirePath(*path); err != nil {
				return err
			}
			return verifyInstance(ctx, *path)
		},
	}
}

func newBackupCommand(parent *ff.FlagSet, path *string) *ff.Command {
	fs := ff.NewFlagSet("backup").SetParent(parent)
	outputPath := fs.StringLong("output-path", "", "destination `folder` for the backup (default: the database's own folder)")
	return &ff.Command{
		Name:      "backup",
		Usage:     "tdb backup [FLAGS]",
		ShortHelp: "back up an instance",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := noArgs(args); err != nil {
				return err
			}
			if err := requirePath(*path); err != nil {
				return err
			}
			return backupInstance(ctx, *path, *outputPath)
		},
	}
}

func newCompactCommand(parent *ff.FlagSet, path *string) *ff.Command {
	fs := ff.NewFlagSet("compact").SetParent(parent)
	return &ff.Command{
		Name:      "compact",
		Usage:     "tdb compact [FLAGS]",
		ShortHelp: "VACUUM an instance (offline)",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := noArgs(args); err != nil {
				return err
			}
			if err := requirePath(*path); err != nil {
				return err
			}
			return compactInstance(ctx, *path)
		},
	}
}

func newCreateAccountCommand(parent *ff.FlagSet, path *string) *ff.Command {
	fs := ff.NewFlagSet("account").SetParent(parent)
	email := fs.StringLong("email", "", "the account's `email` (lowercased before saving)")
	displayName := fs.StringLong("display-name", "", "how the person wants to be addressed (default: anonymous account)")
	admin := fs.BoolLong("is-admin", "make the account an administrator (default: false)")
	inactive := fs.BoolLong("is-inactive", "create the account inactive (default: false)")
	// secret resolves from TDB_SECRET when not passed on the command line; when
	// still empty a passphrase is generated and printed.
	secret := fs.StringLong("secret", "", "the account's password `secret` (or set TDB_SECRET; generated if omitted)")
	return &ff.Command{
		Name:      "account",
		Usage:     "tdb create account [FLAGS]",
		ShortHelp: "create an account with a bcrypt password hash",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := noArgs(args); err != nil {
				return err
			}
			if err := requirePath(*path); err != nil {
				return err
			}
			return createAccount(ctx, *path, *email, *displayName, *admin, *inactive, *secret)
		},
	}
}

// newUpdateCommand builds the "update" resource group and its subcommands.
func newUpdateCommand(parent *ff.FlagSet, path *string) *ff.Command {
	updateFlags := ff.NewFlagSet("update").SetParent(parent)
	update := &ff.Command{
		Name:      "update",
		Usage:     "tdb update [FLAGS] SUBCOMMAND ...",
		ShortHelp: "update an account",
		Flags:     updateFlags,
		Exec: func(ctx context.Context, args []string) error {
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}
	update.Subcommands = []*ff.Command{
		newUpdateAccountCommand(updateFlags, path),
	}
	return update
}

func newUpdateAccountCommand(parent *ff.FlagSet, path *string) *ff.Command {
	fs := ff.NewFlagSet("account").SetParent(parent)
	email := fs.StringLong("email", "", "the `email` identifying the account to update")
	newEmail := fs.StringLong("new-email", "", "a new `email` (lowercased before saving)")
	newDisplayName := fs.StringLong("new-display-name", "", "a new display `name`")
	newSecret := fs.StringLong("new-secret", "", "a new password `secret`")
	// active/inactive are ordinary bool flags (a bare flag sets it true). We do
	// not rely on a default value — the account's status changes only when the
	// operator sets one of them, detected via isFlagSet — so a missing flag never
	// forces active vs inactive.
	active := fs.BoolLong("active", "mark the account active (mutually exclusive with --inactive)")
	inactive := fs.BoolLong("inactive", "mark the account inactive (mutually exclusive with --active)")
	return &ff.Command{
		Name:      "account",
		Usage:     "tdb update account [FLAGS]",
		ShortHelp: "change an existing account's fields",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := noArgs(args); err != nil {
				return err
			}
			if err := requirePath(*path); err != nil {
				return err
			}
			// --active and --inactive are complementary (active == !inactive) and
			// mutually exclusive. A flag's default value is not an instruction: the
			// account's status changes only when the operator actually sets one of
			// them, which ff reports via IsSet.
			activeSet, inactiveSet := isFlagSet(fs, "active"), isFlagSet(fs, "inactive")
			var setInactive *bool
			switch {
			case activeSet && inactiveSet:
				return fmt.Errorf("--active and --inactive are mutually exclusive")
			case inactiveSet:
				v := *inactive
				setInactive = &v
			case activeSet:
				v := !*active
				setInactive = &v
			}
			return updateAccount(ctx, *path, *email, *newEmail, *newDisplayName, *newSecret, setInactive)
		},
	}
}

func newVersionCommand(parent *ff.FlagSet) *ff.Command {
	fs := ff.NewFlagSet("version").SetParent(parent)
	return &ff.Command{
		Name:      "version",
		Usage:     "tdb version",
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

// createInstance creates and migrates a new instance. It refuses to run against
// a directory that already holds one — that is migrate's job.
func createInstance(ctx context.Context, path string) error {
	if exists, err := sqlite.InstanceExists(path); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("an instance already exists in %s (use migrate)", path)
	}

	db, err := sqlite.CreatePersistent(ctx, path)
	if err != nil {
		return err
	}
	defer db.Close()

	v, err := db.UserVersion(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("created instance in %s (migration version %d)\n", path, v)
	return nil
}

// migrateInstance migrates an existing instance up. There is no migrate-down; to
// go back the operator restores from a backup.
func migrateInstance(ctx context.Context, path string) error {
	if exists, err := sqlite.InstanceExists(path); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("no instance in %s (use create database)", path)
	}

	db, err := sqlite.OpenPersistent(ctx, path)
	if err != nil {
		return err
	}
	defer db.Close()

	v, err := db.UserVersion(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("instance in %s is at migration version %d\n", path, v)
	return nil
}

// verifyInstance confirms the instance's migration version equals the expected
// version, without migrating it. It does not test the schema itself.
func verifyInstance(ctx context.Context, path string) error {
	if exists, err := sqlite.InstanceExists(path); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("no instance in %s", path)
	}

	db, err := sqlite.OpenNonMigrating(ctx, path)
	if err != nil {
		return err
	}
	defer db.Close()

	v, err := db.UserVersion(ctx)
	if err != nil {
		return err
	}
	expected := sqlite.ExpectedVersion()
	if v != expected {
		return fmt.Errorf("migration version %d does not match expected %d", v, expected)
	}
	fmt.Printf("ok: migration version %d matches expected %d\n", v, expected)
	return nil
}

// backupInstance backs the instance up into the outputPath folder (defaulting to
// the database's own folder) and reports where the copy landed. The store owns
// the backup file name and the mechanics.
func backupInstance(ctx context.Context, path, outputPath string) error {
	target, err := sqlite.Backup(ctx, path, outputPath)
	if err != nil {
		return err
	}
	fmt.Printf("backed up %s to %s\n", path, target)
	return nil
}

// compactInstance VACUUMs the instance in place, reclaiming free space. It is an
// offline operation: the operator must stop the server first.
func compactInstance(ctx context.Context, path string) error {
	if err := sqlite.Compact(ctx, path); err != nil {
		return err
	}
	fmt.Printf("compacted instance in %s\n", path)
	return nil
}

// createAccount inserts an account, hashing secret with bcrypt into
// password_hash. email is required and lowercased before saving (the schema
// enforces its uniqueness). displayName defaults to "anonymous account". When
// secret is empty a passphrase is generated and printed so the operator can
// share it.
//
// Note for future reviewers: this account password (accounts.password_hash) is
// the SERVER login credential and is deliberately bcrypt-hashed. It is separate
// from the player's in-game order password (players.password, a plaintext shared
// secret) and we care more about securing this one — it authenticates a person
// to the server, not a turn's orders to the engine.
func createAccount(ctx context.Context, path, email, displayName string, admin, inactive bool, secret string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return fmt.Errorf("--email is required")
	}
	if displayName == "" {
		displayName = "anonymous account"
	}
	generated := false
	if secret == "" {
		s, err := generateSecret()
		if err != nil {
			return err
		}
		secret, generated = s, true
	}
	if exists, err := sqlite.InstanceExists(path); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("no instance in %s (use create database)", path)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	id, err := sqlite.CreateAccount(ctx, path, email, displayName, string(hash), admin, inactive)
	if err != nil {
		return err
	}

	fmt.Printf("created account %q (id=%d, admin=%t, inactive=%t)\n", email, id, admin, inactive)
	if generated {
		fmt.Printf("generated secret: %s\n", secret)
	}
	return nil
}

// updateAccount applies the requested changes to the account identified by
// email. An empty string flag means "leave unchanged"; setInactive is nil unless
// the operator asked to change the active status. A new secret is bcrypt-hashed
// here before the store persists it (the store persists credentials, it does not
// hash them). The store validates a new display name.
func updateAccount(ctx context.Context, path, email, newEmail, newDisplayName, newSecret string, setInactive *bool) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return fmt.Errorf("--email is required")
	}

	var upd sqlite.AccountUpdate
	if newEmail != "" {
		e := strings.ToLower(strings.TrimSpace(newEmail))
		upd.NewEmail = &e
	}
	if newDisplayName != "" {
		upd.NewDisplayName = &newDisplayName
	}
	if newSecret != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(newSecret), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		h := string(hash)
		upd.NewPasswordHash = &h
	}
	upd.Inactive = setInactive

	if upd.NewEmail == nil && upd.NewDisplayName == nil && upd.NewPasswordHash == nil && upd.Inactive == nil {
		return fmt.Errorf("nothing to update: pass at least one of --new-email, --new-display-name, --new-secret, --active, --inactive")
	}

	if exists, err := sqlite.InstanceExists(path); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("no instance in %s (use create database)", path)
	}

	if err := sqlite.UpdateAccount(ctx, path, email, upd); err != nil {
		return err
	}
	fmt.Printf("updated account %q\n", email)
	return nil
}

// generateSecret returns a fresh passphrase secret, seeded from the OS CSPRNG so
// the credential is unpredictable. Seven words clears 64 bits of entropy.
func generateSecret() (string, error) {
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	r := mrand.New(mrand.NewPCG(
		binary.LittleEndian.Uint64(b[0:8]),
		binary.LittleEndian.Uint64(b[8:16]),
	))
	return phrases.Generate(r, 7), nil
}

// showVersion prints the application version.
func showVersion() error {
	fmt.Printf("tdb %s\n", tpty.Version())
	return nil
}

// showSchemaVersion prints the database's on-disk schema (migration) version,
// alongside the version this binary expects, so the operator can see whether a
// migrate up is needed. It does not migrate the instance.
func showSchemaVersion(ctx context.Context, path string) error {
	if exists, err := sqlite.InstanceExists(path); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("no instance in %s", path)
	}

	db, err := sqlite.OpenNonMigrating(ctx, path)
	if err != nil {
		return err
	}
	defer db.Close()

	v, err := db.UserVersion(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("database schema version %d\n", v)
	fmt.Printf("expected schema version %d\n", sqlite.ExpectedVersion())
	return nil
}
