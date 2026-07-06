// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Command tpty is the command-line interface to the T'Pty game engine.
package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/mdhender/tpty"
	"github.com/mdhender/tpty/dotenv"
	"github.com/mdhender/tpty/worldographer"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

func main() {
	// Load .env files before parsing flags so ff reads TPTY_* variables sourced
	// from them. TPTY_ENV selects which files load (see dotenv) and is read
	// straight from the environment — not a flag — because it must be known
	// before any flag is parsed. It defaults to development.
	env := os.Getenv("TPTY_ENV")
	if env == "" {
		env = "development"
	}
	if err := dotenv.Load(env); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: load %q environment: %v\n", env, err)
		os.Exit(1)
	}

	root := newRootCommand()

	// Resolve flags from TPTY_-prefixed environment variables (sourced from the
	// .env files loaded above) when not given on the command line. For example,
	// --data is filled from TPTY_DATA. Command-line flags take precedence.
	err := root.ParseAndRun(context.Background(), os.Args[1:], ff.WithEnvVarPrefix("TPTY"))
	switch {
	case errors.Is(err, ff.ErrHelp):
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", ffhelp.Command(root))
		os.Exit(0)
	case err != nil:
		_, _ = fmt.Fprintf(os.Stderr, "tpty: %v\n", err)
		os.Exit(1)
	}
}

// newRootCommand builds the tpty command tree:
//
//	tpty
//	├── game
//	│   └── create
//	├── player
//	│   ├── create
//	│   ├── list
//	│   ├── reset-password
//	│   └── show
//	└── world
//	    ├── generate
//	    ├── render
//	    └── starting-provinces
//	        └── generate
//
// Commands are noun-verb (resource then action). All commands take flags only;
// positional arguments are rejected.
func newRootCommand() *ff.Command {
	rootFlags := ff.NewFlagSet("tpty")
	version := rootFlags.BoolLong("version", "print version information and exit")
	// data is a global flag: the path to the engine's data directory. It is
	// shared with all subcommands, which read and write the engine's data files
	// beneath it.
	data := rootFlags.StringLong("data", "", "`path` to the engine's data directory")

	root := &ff.Command{
		Name:      "tpty",
		Usage:     "tpty [FLAGS] SUBCOMMAND ...",
		ShortHelp: "the T'Pty game engine",
		Flags:     rootFlags,
		Exec: func(ctx context.Context, args []string) error {
			if *version {
				fmt.Println(tpty.Version())
				return nil
			}
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}

	root.Subcommands = []*ff.Command{
		newGameCommand(rootFlags, data),
		newPlayerCommand(rootFlags, data),
		newWorldCommand(rootFlags, data),
	}
	return root
}

// newGameCommand builds the "game" resource command and its subcommands.
func newGameCommand(parent *ff.FlagSet, data *string) *ff.Command {
	gameFlags := ff.NewFlagSet("game").SetParent(parent)

	game := &ff.Command{
		Name:      "game",
		Usage:     "tpty game [FLAGS] SUBCOMMAND ...",
		ShortHelp: "create and inspect games",
		Flags:     gameFlags,
		Exec: func(ctx context.Context, args []string) error {
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}

	game.Subcommands = []*ff.Command{
		newGameCreateCommand(gameFlags, data),
	}
	return game
}

// newGameCreateCommand builds the "game create" command, which writes a new
// game.json manifest into the data directory.
//
// See the reference documentation at content/docs/reference/games.md.
func newGameCreateCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("create").SetParent(parent)
	id := fs.StringLong("game-id", "", "the game's `id` (a slug naming the game)")
	seed1 := fs.Uint64Long("seed1", 0, "first master `seed` (0 = choose at random)")
	seed2 := fs.Uint64Long("seed2", 0, "second master `seed` (0 = choose at random)")

	return &ff.Command{
		Name:      "create",
		Usage:     "tpty game create [FLAGS]",
		ShortHelp: "create a new game",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			if err := tpty.ValidateGameID(*id); err != nil {
				return err
			}

			// Resolve master seeds, choosing random values where unset.
			s1, s2 := *seed1, *seed2
			if s1 == 0 {
				var err error
				if s1, err = randomSeed(); err != nil {
					return err
				}
			}
			if s2 == 0 {
				var err error
				if s2, err = randomSeed(); err != nil {
					return err
				}
			}

			return createGame(*data, *id, tpty.Seeds{Seed1: s1, Seed2: s2})
		},
	}
}

// createGame writes a new game.json manifest into the data directory. It refuses
// to overwrite an existing game.
func createGame(data, id string, seeds tpty.Seeds) error {
	path := filepath.Join(data, "game.json")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	game, err := tpty.NewGame(id, seeds)
	if err != nil {
		return err
	}
	if err := writeJSON(path, game); err != nil {
		return err
	}

	fmt.Printf("created game %q (seed1=%d seed2=%d)\n", id, seeds.Seed1, seeds.Seed2)
	fmt.Printf("wrote %s\n", path)
	return nil
}

// newPlayerCommand builds the "player" resource command and its subcommands.
func newPlayerCommand(parent *ff.FlagSet, data *string) *ff.Command {
	playerFlags := ff.NewFlagSet("player").SetParent(parent)

	player := &ff.Command{
		Name:      "player",
		Usage:     "tpty player [FLAGS] SUBCOMMAND ...",
		ShortHelp: "create and manage players",
		Flags:     playerFlags,
		Exec: func(ctx context.Context, args []string) error {
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}

	player.Subcommands = []*ff.Command{
		newPlayerCreateCommand(playerFlags, data),
		newPlayerListCommand(playerFlags, data),
		newPlayerResetPasswordCommand(playerFlags, data),
		newPlayerShowCommand(playerFlags, data),
	}
	return player
}

// newPlayerCreateCommand builds the "player create" command, which adds a player
// to the game and writes the players file named in the manifest.
//
// See the reference documentation at content/docs/reference/players.md.
func newPlayerCreateCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("create").SetParent(parent)
	email := fs.StringLong("email", "", "the player's `email` address")
	handle := fs.StringLong("handle", "", "the player's `handle`")
	province := fs.StringLong("starting-province", "", "the player's starting `province` in compact form, e.g. (-1,0)")

	return &ff.Command{
		Name:      "create",
		Usage:     "tpty player create [FLAGS]",
		ShortHelp: "create a new player",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			if *email == "" {
				return fmt.Errorf("--email is required")
			}
			if *handle == "" {
				return fmt.Errorf("--handle is required")
			}
			if *province == "" {
				return fmt.Errorf("--starting-province is required")
			}
			return createPlayer(*data, *email, *handle, *province)
		},
	}
}

// createPlayer adds a player to the game. It validates the starting province
// against the game's allowed starting provinces, derives the player's seeds and
// password from the game's master seeds, and writes the updated players file.
func createPlayer(data, email, handle, province string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}

	// The starting province must be one of the game's allowed starting provinces.
	canonical, err := tpty.ParseProvince(province)
	if err != nil {
		return err
	}
	allowed, err := loadStartingProvinces(files.StartingProvinces)
	if err != nil {
		return err
	}
	// An absent or empty allowed set means no player can be placed. Failing is
	// correct, but the GM needs to know what's wrong and how to fix it rather
	// than a raw "no such file" or a misleading per-province "not allowed".
	if len(allowed) == 0 {
		return fmt.Errorf("no starting provinces are defined for this game; create %s (a JSON array of provinces in compact form, e.g. [\"(0,0)\",\"(1,-1)\"])", files.StartingProvinces)
	}
	if !allowed[canonical] {
		return fmt.Errorf("starting province %s is not allowed for this game (see %s)", canonical, files.StartingProvinces)
	}

	store, err := loadPlayers(files.Players)
	if err != nil {
		return err
	}

	player, err := store.Create(game.Seeds, email, handle, canonical)
	if err != nil {
		return err
	}

	if err := writeJSON(files.Players, store); err != nil {
		return fmt.Errorf("write players: %w", err)
	}

	fmt.Printf("created player %d in game %q\n", player.ID, game.ID)
	fmt.Printf("  handle:   %s\n", player.Handle)
	fmt.Printf("  email:    %s\n", player.Email)
	fmt.Printf("  province: %s\n", player.StartingProvince)
	fmt.Printf("  password: %s\n", player.Password)
	fmt.Printf("wrote %s\n", files.Players)
	return nil
}

// newPlayerListCommand builds the "player list" command, which lists the players
// in a game.
func newPlayerListCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("list").SetParent(parent)

	return &ff.Command{
		Name:      "list",
		Usage:     "tpty player list [FLAGS]",
		ShortHelp: "list the players in a game",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			return listPlayers(*data)
		},
	}
}

// listPlayers prints a table of the game's players (id, handle, email, and
// starting province). Passwords are shown only by "player show".
func listPlayers(data string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}
	store, err := loadPlayers(files.Players)
	if err != nil {
		return err
	}

	if len(store.Players) == 0 {
		fmt.Printf("game %q has no players\n", game.ID)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tHANDLE\tEMAIL\tSTARTING PROVINCE")
	for _, p := range store.Players {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", p.ID, p.Handle, p.Email, p.StartingProvince)
	}
	return w.Flush()
}

// newPlayerResetPasswordCommand builds the "player reset-password" command, which
// reissues a player's password. The player is looked up by email only.
//
// See the reference documentation at content/docs/reference/players.md.
func newPlayerResetPasswordCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("reset-password").SetParent(parent)
	email := fs.StringLong("email", "", "the player's `email` address")

	return &ff.Command{
		Name:      "reset-password",
		Usage:     "tpty player reset-password [FLAGS]",
		ShortHelp: "reset a player's password",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			if *email == "" {
				return fmt.Errorf("--email is required")
			}
			return resetPlayerPassword(*data, *email)
		},
	}
}

// resetPlayerPassword reissues the password of the player registered with email,
// drawing a new value from the player's stream keyed by the game's current turn,
// writes the updated players file, and prints the new password for the GM to
// resend.
func resetPlayerPassword(data, email string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}
	store, err := loadPlayers(files.Players)
	if err != nil {
		return err
	}

	player, err := store.ResetPassword(email, game.Turn)
	if err != nil {
		return err
	}

	if err := writeJSON(files.Players, store); err != nil {
		return fmt.Errorf("write players: %w", err)
	}

	fmt.Printf("reset password for player %d in game %q (turn %d)\n", player.ID, game.ID, game.Turn)
	fmt.Printf("  handle:   %s\n", player.Handle)
	fmt.Printf("  email:    %s\n", player.Email)
	fmt.Printf("  password: %s\n", player.Password)
	fmt.Printf("wrote %s\n", files.Players)
	return nil
}

// newPlayerShowCommand builds the "player show" command, which shows one player's
// details, looked up by id or handle.
func newPlayerShowCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("show").SetParent(parent)
	id := fs.IntLong("id", 0, "the player's `id`")
	handle := fs.StringLong("handle", "", "the player's `handle`")

	return &ff.Command{
		Name:      "show",
		Usage:     "tpty player show [FLAGS]",
		ShortHelp: "show a player's details",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			switch {
			case *id != 0 && *handle != "":
				return fmt.Errorf("provide only one of --id or --handle")
			case *id == 0 && *handle == "":
				return fmt.Errorf("provide --id or --handle")
			}
			return showPlayer(*data, *id, *handle)
		},
	}
}

// showPlayer prints one player's details, including the password, looked up by id
// (when id != 0) or by handle.
func showPlayer(data string, id int, handle string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}
	store, err := loadPlayers(files.Players)
	if err != nil {
		return err
	}

	var player tpty.Player
	var ok bool
	if id != 0 {
		if player, ok = store.ByID(id); !ok {
			return fmt.Errorf("no player with id %d", id)
		}
	} else {
		if player, ok = store.ByHandle(handle); !ok {
			return fmt.Errorf("no player with handle %q", handle)
		}
	}

	fmt.Printf("player %d in game %q\n", player.ID, game.ID)
	fmt.Printf("  handle:   %s\n", player.Handle)
	fmt.Printf("  email:    %s\n", player.Email)
	fmt.Printf("  province: %s\n", player.StartingProvince)
	fmt.Printf("  password: %s\n", player.Password)
	return nil
}

// newWorldCommand builds the "world" resource command and its subcommands.
func newWorldCommand(parent *ff.FlagSet, data *string) *ff.Command {
	worldFlags := ff.NewFlagSet("world").SetParent(parent)

	world := &ff.Command{
		Name:      "world",
		Usage:     "tpty world [FLAGS] SUBCOMMAND ...",
		ShortHelp: "generate and inspect worlds",
		Flags:     worldFlags,
		Exec: func(ctx context.Context, args []string) error {
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}

	world.Subcommands = []*ff.Command{
		newWorldGenerateCommand(worldFlags, data),
		newWorldRenderCommand(worldFlags, data),
		newWorldStartingProvincesCommand(worldFlags, data),
	}
	return world
}

// newWorldStartingProvincesCommand builds the "world starting-provinces" group,
// which manages the game's allowed starting provinces. It currently offers the
// "generate" verb; management verbs (list, add, remove) are future siblings.
func newWorldStartingProvincesCommand(parent *ff.FlagSet, data *string) *ff.Command {
	spFlags := ff.NewFlagSet("starting-provinces").SetParent(parent)

	sp := &ff.Command{
		Name:      "starting-provinces",
		Usage:     "tpty world starting-provinces [FLAGS] SUBCOMMAND ...",
		ShortHelp: "manage the game's allowed starting provinces",
		Flags:     spFlags,
		Exec: func(ctx context.Context, args []string) error {
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}

	sp.Subcommands = []*ff.Command{
		newWorldStartingProvincesGenerateCommand(spFlags, data),
	}
	return sp
}

// newWorldStartingProvincesGenerateCommand builds the
// "world starting-provinces generate" command, which writes the default set of
// six starting provinces to the manifest's starting-provinces path.
//
// See the reference documentation at
// content/docs/reference/world-generation.md for the selection rule.
func newWorldStartingProvincesGenerateCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("generate").SetParent(parent)
	// ring defaults to 0, the sentinel for "unset": the ring is then computed
	// from the world's ring count as ceil(rings/2).
	ring := fs.IntLong("ring", 0, "ring distance from the origin (default ceil(worldRings/2); 0 < ring <= worldRings)")
	overwrite := fs.BoolLong("overwrite", "replace an existing starting-provinces file")

	return &ff.Command{
		Name:      "generate",
		Usage:     "tpty world starting-provinces generate [FLAGS]",
		ShortHelp: "write the default six starting provinces",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			return generateStartingProvinces(*data, *ring, *overwrite)
		},
	}
}

// generateStartingProvinces writes the default set of six starting provinces to
// the manifest's starting-provinces path. It hard-fails when world.json is
// absent (the ring count comes from it), refuses to clobber an existing file
// unless overwrite is set, and warns (but proceeds) when players.json exists.
//
// A ring of 0 means "unset": the distance defaults to ceil(worldRings/2). An
// explicit ring must satisfy 0 < ring <= worldRings.
func generateStartingProvinces(data string, ring int, overwrite bool) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}

	// The world must exist: it supplies the ring count and the provinces name
	// real hexes. Fail clearly rather than with a raw "no such file".
	worldRings, err := loadWorldRings(files.World)
	if err != nil {
		return err
	}

	if ring == 0 {
		ring = tpty.DefaultStartingProvinceRing(worldRings)
	}
	if ring <= 0 || ring > worldRings {
		return fmt.Errorf("--ring must be > 0 and <= %d (the world's ring count), got %d", worldRings, ring)
	}

	provinces, err := tpty.StartingProvinces(ring)
	if err != nil {
		return err
	}

	// Default is to refuse to clobber: a pre-existing file most likely means the
	// GM is recreating or cloning a game, so overwriting must be explicit.
	if !overwrite {
		if _, err := os.Stat(files.StartingProvinces); err == nil {
			return fmt.Errorf("%s already exists; pass --overwrite to replace it", files.StartingProvinces)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", files.StartingProvinces, err)
		}
	}

	// A footgun-with-a-warning: existing players are not re-validated against the
	// new set (that is a management concern). Flag it, but proceed.
	if _, err := os.Stat(files.Players); err == nil {
		fmt.Fprintf(os.Stderr, "warning: %s exists; existing players are not re-validated against the new starting provinces\n", files.Players)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", files.Players, err)
	}

	if err := writeJSON(files.StartingProvinces, provinces); err != nil {
		return fmt.Errorf("write starting provinces: %w", err)
	}

	fmt.Printf("generated %d starting provinces for game %q (ring %d)\n", len(provinces), game.ID, ring)
	for _, p := range provinces {
		fmt.Printf("  %s\n", p)
	}
	fmt.Printf("wrote %s\n", files.StartingProvinces)
	return nil
}

// loadWorldRings reads the world file named in the manifest and returns its ring
// count. A missing file is a hard failure: starting provinces are meaningful
// only once the world exists.
func loadWorldRings(path string) (int, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, fmt.Errorf("%s does not exist; generate the world first (tpty world generate)", path)
	}
	if err != nil {
		return 0, fmt.Errorf("read world: %w", err)
	}
	var world struct {
		Rings int `json:"rings"`
	}
	if err := json.Unmarshal(buf, &world); err != nil {
		return 0, fmt.Errorf("parse %s: %w", path, err)
	}
	if world.Rings <= 0 {
		return 0, fmt.Errorf("%s has an invalid ring count %d", path, world.Rings)
	}
	return world.Rings, nil
}

// newWorldGenerateCommand builds the "world generate" command. It reads the
// game's master seeds from the game.json manifest in the data directory and
// writes the generated world to the manifest's world path.
//
// See the reference documentation at
// content/docs/reference/world-generation.md for the rules this implements.
func newWorldGenerateCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("generate").SetParent(parent)
	rings := fs.IntLong("rings", 0, "number of `rings` to generate (0 < rings < 100)")

	return &ff.Command{
		Name:      "generate",
		Usage:     "tpty world generate [FLAGS]",
		ShortHelp: "generate a new world",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			// This command takes flags only; reject positional arguments.
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *rings <= 0 || *rings >= 100 {
				return fmt.Errorf("--rings must be > 0 and < 100, got %d", *rings)
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			return generateWorld(*rings, *data)
		},
	}
}

// generateWorld reads game.json from the data directory, generates a world from
// the game's master seeds, and writes world.json and terrain-translation.json to
// the locations named in the manifest.
func generateWorld(rings int, data string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}

	world, err := tpty.GenerateWorld(game.Seeds, rings)
	if err != nil {
		return err
	}

	if err := writeJSON(files.World, world); err != nil {
		return fmt.Errorf("write world: %w", err)
	}
	if err := writeJSON(files.TerrainTranslation, tpty.TerrainTranslation()); err != nil {
		return fmt.Errorf("write terrain translation: %w", err)
	}

	fmt.Printf("generated world for game %q (seed1=%d seed2=%d)\n", game.ID, game.Seeds.Seed1, game.Seeds.Seed2)
	fmt.Printf("wrote %d provinces to %s\n", len(world.Provinces), files.World)
	fmt.Printf("wrote terrain translation to %s\n", files.TerrainTranslation)
	return nil
}

// newWorldRenderCommand builds the "world render" command, which renders a
// generated world to a Worldographer .wxx file beside the world file.
func newWorldRenderCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("render").SetParent(parent)

	return &ff.Command{
		Name:      "render",
		Usage:     "tpty world render [FLAGS]",
		ShortHelp: "render a generated world to Worldographer",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			return renderWorld(*data)
		},
	}
}

// renderWorld reads the world and terrain-translation files named in the game's
// manifest and writes world.wxx (Worldographer) beside the world file.
func renderWorld(data string) error {
	_, files, err := loadGame(data)
	if err != nil {
		return err
	}

	worldBuf, err := os.ReadFile(files.World)
	if err != nil {
		return fmt.Errorf("read world: %w", err)
	}
	var world struct {
		Provinces []worldographer.Province `json:"provinces"`
	}
	if err := json.Unmarshal(worldBuf, &world); err != nil {
		return fmt.Errorf("parse %s: %w", files.World, err)
	}

	ttBuf, err := os.ReadFile(files.TerrainTranslation)
	if err != nil {
		return fmt.Errorf("read terrain translation: %w", err)
	}
	var translation map[string]string
	if err := json.Unmarshal(ttBuf, &translation); err != nil {
		return fmt.Errorf("parse %s: %w", files.TerrainTranslation, err)
	}

	outPath := filepath.Join(filepath.Dir(files.World), "world.wxx")
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer f.Close()
	if err := worldographer.Render(f, world.Provinces, translation); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", outPath, err)
	}

	fmt.Printf("wrote %d provinces to %s\n", len(world.Provinces), outPath)
	return nil
}

// loadGame reads game.json from the data directory and returns the game together
// with its data-file paths resolved against the data directory.
func loadGame(data string) (*tpty.Game, tpty.GameFiles, error) {
	path := filepath.Join(data, "game.json")
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, tpty.GameFiles{}, fmt.Errorf("read game: %w", err)
	}
	var game tpty.Game
	if err := json.Unmarshal(buf, &game); err != nil {
		return nil, tpty.GameFiles{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return &game, game.Files.Resolve(data), nil
}

// loadPlayers reads the players file at path into a store. A missing file yields
// a new, empty store, so the first player can be created before the file exists.
func loadPlayers(path string) (*tpty.PlayerStore, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return tpty.NewPlayerStore(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read players: %w", err)
	}
	var store tpty.PlayerStore
	if err := json.Unmarshal(buf, &store); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &store, nil
}

// loadStartingProvinces reads the allowed starting provinces at path into a set,
// validating that each entry is a canonical compact province string. A missing
// file yields an empty set, so callers can distinguish "no provinces defined"
// (an actionable condition) from a genuine read/parse failure.
func loadStartingProvinces(path string) (map[string]bool, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read starting provinces: %w", err)
	}
	var list []string
	if err := json.Unmarshal(buf, &list); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	set := make(map[string]bool, len(list))
	for _, p := range list {
		canonical, err := tpty.ParseProvince(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		set[canonical] = true
	}
	return set, nil
}

// writeJSON encodes v as indented JSON and writes it to path, creating parent
// directories as needed.
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// randomSeed returns a non-zero master seed drawn from a cryptographic source.
func randomSeed() (uint64, error) {
	var b [8]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			return 0, fmt.Errorf("read random seed: %w", err)
		}
		if s := binary.LittleEndian.Uint64(b[:]); s != 0 {
			return s, nil
		}
	}
}
