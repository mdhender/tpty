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
	"sort"
	"text/tabwriter"

	"github.com/mdhender/tpty"
	"github.com/mdhender/tpty/internal/dotenv"
	"github.com/mdhender/tpty/internal/orders"
	"github.com/mdhender/tpty/internal/prng"
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
//	├── orders
//	│   ├── list
//	│   └── submit
//	├── player
//	│   ├── create
//	│   ├── list
//	│   ├── reactivate
//	│   ├── remove
//	│   ├── reset-password
//	│   └── show
//	├── turn
//	│   └── process
//	└── world
//	    ├── generate
//	    ├── render
//	    └── starting-provinces
//	        ├── add
//	        ├── generate
//	        ├── list
//	        └── remove
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
		newOrdersCommand(rootFlags, data),
		newPlayerCommand(rootFlags, data),
		newTurnCommand(rootFlags, data),
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

			return createGame(*data, *id, prng.New(s1, s2))
		},
	}
}

// createGame writes a new game.json manifest into the data directory. It refuses
// to overwrite an existing game.
func createGame(data, id string, seeds prng.Seeds) error {
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

// newOrdersCommand builds the "orders" resource command and its subcommands.
func newOrdersCommand(parent *ff.FlagSet, data *string) *ff.Command {
	ordersFlags := ff.NewFlagSet("orders").SetParent(parent)

	cmd := &ff.Command{
		Name:      "orders",
		Usage:     "tpty orders [FLAGS] SUBCOMMAND ...",
		ShortHelp: "submit and manage player orders",
		Flags:     ordersFlags,
		Exec: func(ctx context.Context, args []string) error {
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}

	cmd.Subcommands = []*ff.Command{
		newOrdersListCommand(ordersFlags, data),
		newOrdersSubmitCommand(ordersFlags, data),
	}
	return cmd
}

// newOrdersListCommand builds the "orders list" command, which shows, for the
// game's current turn, which active players have submitted orders and which have
// not, so the GM can tell when everyone is in and it is time to process the turn.
//
// See content/docs/reference/turns.md, "Per-turn lifecycle".
func newOrdersListCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("list").SetParent(parent)

	return &ff.Command{
		Name:      "list",
		Usage:     "tpty orders list [FLAGS]",
		ShortHelp: "show which active players have submitted orders for the current turn",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			return listOrders(*data)
		},
	}
}

// listOrders prints, for the game's current turn, a table of every active player
// and whether they have submitted orders (submitted / not submitted), with a
// summary count, so the GM can tell when everyone is in and it is time to process
// the turn.
//
// A player counts as "submitted" when a stored orders file exists for them under
// the current turn's order directory (as written by "orders submit"). A missing
// turn directory — nobody has submitted yet — is not an error: every active
// player is then "not submitted".
//
// See content/docs/reference/turns.md, "Per-turn lifecycle", and
// content/docs/reference/orders/_index.md.
func listOrders(data string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}

	// Orders are collected for the current turn; turn 0 is setup and has no play.
	// Match "orders submit", which likewise refuses at turn 0.
	if game.Turn < 1 {
		return fmt.Errorf("the game is at turn 0 (setup); orders are collected once play begins at turn 1")
	}

	store, err := loadPlayers(files.Players)
	if err != nil {
		return err
	}

	// A player is "submitted" when a stored orders file exists for them at the
	// current turn. Stat each active player's path rather than reading the turn
	// directory, so status is reported against the active set (a stale file from a
	// since-removed player is ignored).
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tHANDLE\tSTATUS")
	active, submitted := 0, 0
	for _, p := range store.Players {
		if !p.Active() {
			continue
		}
		active++
		status := "not submitted"
		if _, statErr := os.Stat(tpty.PlayerOrdersPath(files.Orders, game.Turn, p.ID)); statErr == nil {
			status = "submitted"
			submitted++
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("stat orders for player %d: %w", p.ID, statErr)
		}
		fmt.Fprintf(w, "%d\t%s\t%s\n", p.ID, p.Handle, status)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	if active == 0 {
		fmt.Printf("game %q has no active players\n", game.ID)
		return nil
	}
	fmt.Printf("%d of %d active player(s) have submitted orders for turn %d\n", submitted, active, game.Turn)
	return nil
}

// newOrdersSubmitCommand builds the "orders submit" command, which ingests a
// player's saved orders file into the current turn's order store.
//
// See the reference documentation at content/docs/reference/orders/_index.md.
func newOrdersSubmitCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("submit").SetParent(parent)
	file := fs.StringLong("file", "", "`path` to the player's saved orders file to ingest")

	return &ff.Command{
		Name:      "submit",
		Usage:     "tpty orders submit [FLAGS]",
		ShortHelp: "submit a player's orders for the current turn",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			if *file == "" {
				return fmt.Errorf("--file is required")
			}
			return submitOrders(*data, *file)
		},
	}
}

// submitOrders ingests a player's saved orders file into the current turn's
// order store. It parses the file, authenticates its opening record against the
// game and its players, checks that each entity block names an entity the player
// owns, and stores the raw submission verbatim under the game's orders directory.
//
// Authentication failure rejects the file in full and stores nothing (a non-zero
// exit). When authentication succeeds the submission is accepted and stored even
// if it carries warnings — parse errors and ownership problems are reported but
// do not block acceptance; those orders simply will not be executed.
//
// See content/docs/reference/orders/_index.md, "Authentication", "Ownership",
// and "Rejected orders".
func submitOrders(data, file string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}

	// Orders are collected for the current turn; turn 0 is setup and has no play.
	if game.Turn < 1 {
		return fmt.Errorf("the game is at turn 0 (setup); orders are collected once play begins at turn 1")
	}

	buf, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read orders file %s: %w", file, err)
	}

	f, parseErrs := orders.Parse(string(buf))

	store, err := loadPlayers(files.Players)
	if err != nil {
		return err
	}

	player, err := game.AuthenticateOrders(f, store)
	if err != nil {
		// A malformed opening record is the most likely cause of an authentication
		// failure with no parsed opening; surface the parse errors that explain it.
		if f.Opening == nil && len(parseErrs) > 0 {
			for _, pe := range parseErrs {
				fmt.Printf("  %s\n", pe.Error())
			}
		}
		return fmt.Errorf("orders rejected: %w", err)
	}

	factions, err := loadFactions(files.Factions)
	if err != nil {
		return err
	}
	entities, err := loadEntities(files.Entities)
	if err != nil {
		return err
	}

	ownErrs := tpty.CheckOrderOwnership(f, player.ID, factions, entities)

	path := tpty.PlayerOrdersPath(files.Orders, game.Turn, player.ID)
	_, statErr := os.Stat(path)
	replaced := statErr == nil

	stored := tpty.StoredOrders{Turn: game.Turn, PlayerID: player.ID, Raw: string(buf)}
	if err := writeJSON(path, stored); err != nil {
		return fmt.Errorf("write orders: %w", err)
	}

	totalOrders := 0
	for _, block := range f.Entities {
		totalOrders += len(block.Orders)
	}

	fmt.Printf("accepted orders for player %d (%q) in game %q, turn %d\n", player.ID, player.Handle, game.ID, game.Turn)
	fmt.Printf("  parsed %d entity block(s), %d order(s)\n", len(f.Entities), totalOrders)

	// Combine parse and ownership problems into one list, sorted by position.
	// These orders will not be executed, but the rest of the submission stands.
	warnings := make([]orders.Error, 0, len(parseErrs)+len(ownErrs))
	warnings = append(warnings, parseErrs...)
	warnings = append(warnings, ownErrs...)
	sort.SliceStable(warnings, func(i, j int) bool {
		if warnings[i].Line != warnings[j].Line {
			return warnings[i].Line < warnings[j].Line
		}
		return warnings[i].Col < warnings[j].Col
	})
	if len(warnings) > 0 {
		fmt.Printf("warnings (%d): the following will not be executed; the rest is accepted\n", len(warnings))
		for _, w := range warnings {
			fmt.Printf("  %s\n", w.Error())
		}
	}

	if replaced {
		fmt.Printf("replaced previous submission\n")
	}
	fmt.Printf("wrote %s\n", path)
	return nil
}

// newTurnCommand builds the "turn" resource command and its subcommands.
func newTurnCommand(parent *ff.FlagSet, data *string) *ff.Command {
	turnFlags := ff.NewFlagSet("turn").SetParent(parent)

	cmd := &ff.Command{
		Name:      "turn",
		Usage:     "tpty turn [FLAGS] SUBCOMMAND ...",
		ShortHelp: "process and advance turns",
		Flags:     turnFlags,
		Exec: func(ctx context.Context, args []string) error {
			// No subcommand selected; show help.
			return ff.ErrHelp
		},
	}

	cmd.Subcommands = []*ff.Command{
		newTurnProcessCommand(turnFlags, data),
	}
	return cmd
}

// newTurnProcessCommand builds the "turn process" command, which runs the
// turn-execution engine over the current turn's submitted orders, enforces the
// turn guards, and persists the results. It does not advance the turn.
//
// See content/docs/reference/turns.md ("Per-turn lifecycle") and
// content/docs/reference/turn-processing.md.
func newTurnProcessCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("process").SetParent(parent)

	return &ff.Command{
		Name:      "process",
		Usage:     "tpty turn process [FLAGS]",
		ShortHelp: "process the current turn's orders and persist the results",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			return processTurn(*data)
		},
	}
}

// processTurn runs the turn-execution engine over the game's current turn. It
// loads the game and the current turn's submitted orders, enforces the three
// guards (turn 0 has no play; there must be orders to process; a turn is
// processed at most once), carries in the previous turn's unfinished order
// queues, runs tpty.ProcessTurn with the default dispatch, and persists the
// mutated entities file and this turn's TurnResult. The result file's existence
// is the "already processed" marker. The current turn is not advanced —
// advancing to turn N+1 is a separate command.
//
// See content/docs/reference/turns.md ("Per-turn lifecycle", the guards) and
// content/docs/reference/turn-processing.md.
func processTurn(data string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}

	// Guard — turn 0 is setup and has no play. Match "orders submit"/"orders
	// list", which likewise refuse at turn 0.
	if game.Turn < 1 {
		return fmt.Errorf("the game is at turn 0 (setup); turns are processed once play begins at turn 1")
	}

	// Guard — there must be orders to process. A missing turn directory yields an
	// empty slice, so this also catches "nobody has submitted yet".
	subs, err := tpty.LoadTurnOrders(files.Orders, game.Turn)
	if err != nil {
		return fmt.Errorf("load submitted orders: %w", err)
	}
	if len(subs) == 0 {
		return fmt.Errorf("no orders collected for turn %d", game.Turn)
	}

	// Guard — a turn is processed at most once. The result file's existence marks
	// the turn as already processed (double-processing is an error; there is no
	// --force in this pass).
	if _, statErr := os.Stat(tpty.TurnResultPath(files.Turns, game.Turn)); statErr == nil {
		return fmt.Errorf("turn %d already processed", game.Turn)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat turn result: %w", statErr)
	}

	factions, err := loadFactions(files.Factions)
	if err != nil {
		return err
	}
	entities, err := loadEntities(files.Entities)
	if err != nil {
		return err
	}

	// Carry in the previous turn's unfinished order queues, if that turn was
	// processed. The first processed turn has none.
	var carryover []tpty.EntityQueue
	if prev, prevErr := tpty.LoadTurnResult(files.Turns, game.Turn-1); prevErr == nil {
		carryover = prev.Carryover
	} else if !errors.Is(prevErr, tpty.ErrTurnNotProcessed) {
		return fmt.Errorf("load previous turn carryover: %w", prevErr)
	}

	result := tpty.ProcessTurn(tpty.TurnInput{
		State:     tpty.GameState{Game: game, Entities: entities, Factions: factions},
		Turn:      game.Turn,
		Submitted: subs,
		Carryover: carryover,
		Dispatch:  tpty.NewDispatch(),
	})

	// Persist the mutated entities (Move updates locations in the store) and the
	// turn result. The turn is NOT advanced.
	if err := writeJSON(files.Entities, entities); err != nil {
		return fmt.Errorf("write entities: %w", err)
	}
	if err := tpty.SaveTurnResult(files.Turns, result); err != nil {
		return fmt.Errorf("write turn result: %w", err)
	}

	executed, stubbed := 0, 0
	for _, o := range result.Outcomes {
		if o.Stub {
			stubbed++
		} else {
			executed++
		}
	}

	resultPath := tpty.TurnResultPath(files.Turns, game.Turn)
	fmt.Printf("processed turn %d of game %q\n", game.Turn, game.ID)
	fmt.Printf("  %d submission(s), %d order outcome(s): %d executed, %d stubbed\n",
		len(subs), len(result.Outcomes), executed, stubbed)
	fmt.Printf("  %d carryover queue(s) for turn %d\n", len(result.Carryover), game.Turn+1)
	fmt.Printf("wrote %s\n", resultPath)
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
		newPlayerReactivateCommand(playerFlags, data),
		newPlayerRemoveCommand(playerFlags, data),
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
	if allowed.Len() == 0 {
		return fmt.Errorf("no starting provinces are defined for this game; add one with 'tpty world starting-provinces add --province %s' or generate the defaults", canonical)
	}
	if !allowed.Contains(canonical) {
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
	all := fs.BoolLong("all", "include removed (inactive) players")

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
			return listPlayers(*data, *all)
		},
	}
}

// listPlayers prints a table of the game's players (id, handle, email, starting
// province, and status). By default only active players are listed; passing all
// includes removed (inactive) players. Passwords are shown only by "player show".
func listPlayers(data string, all bool) error {
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
	fmt.Fprintln(w, "ID\tHANDLE\tEMAIL\tSTARTING PROVINCE\tSTATUS")
	shown, hidden := 0, 0
	for _, p := range store.Players {
		if !all && !p.Active() {
			hidden++
			continue
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", p.ID, p.Handle, p.Email, p.StartingProvince, playerStatus(p))
		shown++
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if shown == 0 {
		fmt.Printf("game %q has no active players\n", game.ID)
	}
	if hidden > 0 {
		fmt.Printf("(%d inactive player(s) hidden; pass --all to include them)\n", hidden)
	}
	return nil
}

// playerStatus renders a player's active state for display.
func playerStatus(p tpty.Player) string {
	if p.Active() {
		return "active"
	}
	return "inactive"
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
	fmt.Printf("  status:   %s\n", playerStatus(player))
	return nil
}

// newPlayerRemoveCommand builds the "player remove" command, which removes a
// player by marking them inactive. The player is retained and can be restored
// with "player reactivate". The player is looked up by id or handle.
//
// See the reference documentation at content/docs/reference/players.md.
func newPlayerRemoveCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("remove").SetParent(parent)
	id := fs.IntLong("id", 0, "the player's `id`")
	handle := fs.StringLong("handle", "", "the player's `handle`")

	return &ff.Command{
		Name:      "remove",
		Usage:     "tpty player remove [FLAGS]",
		ShortHelp: "remove (deactivate) a player",
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
			return setPlayerActive(*data, *id, *handle, false)
		},
	}
}

// newPlayerReactivateCommand builds the "player reactivate" command, which
// restores a removed (inactive) player by marking them active again. The player
// is looked up by id or handle.
//
// See the reference documentation at content/docs/reference/players.md.
func newPlayerReactivateCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("reactivate").SetParent(parent)
	id := fs.IntLong("id", 0, "the player's `id`")
	handle := fs.StringLong("handle", "", "the player's `handle`")

	return &ff.Command{
		Name:      "reactivate",
		Usage:     "tpty player reactivate [FLAGS]",
		ShortHelp: "reactivate a removed player",
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
			return setPlayerActive(*data, *id, *handle, true)
		},
	}
}

// setPlayerActive removes (active == false) or reactivates (active == true) the
// player identified by id (when id != 0) or by handle, writes the updated
// players file, and reports the change. Looking the player up by id or handle
// resolves it to its id before the state change, so both commands share one path.
func setPlayerActive(data string, id int, handle string, active bool) error {
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

	verb := "removed"
	if active {
		player, err = store.Reactivate(player.ID)
		verb = "reactivated"
	} else {
		player, err = store.Deactivate(player.ID)
	}
	if err != nil {
		return err
	}

	if err := writeJSON(files.Players, store); err != nil {
		return fmt.Errorf("write players: %w", err)
	}

	fmt.Printf("%s player %d in game %q\n", verb, player.ID, game.ID)
	fmt.Printf("  handle:   %s\n", player.Handle)
	fmt.Printf("  email:    %s\n", player.Email)
	fmt.Printf("  status:   %s\n", playerStatus(player))
	fmt.Printf("wrote %s\n", files.Players)
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
// which manages the game's allowed starting provinces: "generate" writes the
// default set, and "add", "remove", and "list" maintain it afterward.
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
		newWorldStartingProvincesAddCommand(spFlags, data),
		newWorldStartingProvincesGenerateCommand(spFlags, data),
		newWorldStartingProvincesListCommand(spFlags, data),
		newWorldStartingProvincesRemoveCommand(spFlags, data),
	}
	return sp
}

// newWorldStartingProvincesAddCommand builds the
// "world starting-provinces add" command, which appends one province to the
// game's allowed starting provinces.
//
// See content/docs/reference/world-generation.md for the rules.
func newWorldStartingProvincesAddCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("add").SetParent(parent)
	province := fs.StringLong("province", "", "the `province` to add, in compact form, e.g. (-1,0)")

	return &ff.Command{
		Name:      "add",
		Usage:     "tpty world starting-provinces add [FLAGS]",
		ShortHelp: "add a starting province",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			if *province == "" {
				return fmt.Errorf("--province is required")
			}
			return addStartingProvince(*data, *province)
		},
	}
}

// newWorldStartingProvincesRemoveCommand builds the
// "world starting-provinces remove" command, which removes one province from the
// game's allowed starting provinces.
//
// See content/docs/reference/world-generation.md for the rules.
func newWorldStartingProvincesRemoveCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("remove").SetParent(parent)
	province := fs.StringLong("province", "", "the `province` to remove, in compact form, e.g. (-1,0)")

	return &ff.Command{
		Name:      "remove",
		Usage:     "tpty world starting-provinces remove [FLAGS]",
		ShortHelp: "remove a starting province",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			if *province == "" {
				return fmt.Errorf("--province is required")
			}
			return removeStartingProvince(*data, *province)
		},
	}
}

// newWorldStartingProvincesListCommand builds the
// "world starting-provinces list" command, which prints the game's allowed
// starting provinces.
func newWorldStartingProvincesListCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("list").SetParent(parent)

	return &ff.Command{
		Name:      "list",
		Usage:     "tpty world starting-provinces list [FLAGS]",
		ShortHelp: "list the game's starting provinces",
		Flags:     fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q: this command takes flags only, no positional arguments", args[0])
			}
			if *data == "" {
				return fmt.Errorf("--data is required")
			}
			return listStartingProvinces(*data)
		},
	}
}

// addStartingProvince appends province to the game's allowed starting provinces
// and writes the updated file. A non-canonical province, or one already in the
// set, is an error and nothing is written.
func addStartingProvince(data, province string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}
	set, err := loadStartingProvinces(files.StartingProvinces)
	if err != nil {
		return err
	}
	canonical, err := set.Add(province)
	if err != nil {
		return err
	}
	if err := writeJSON(files.StartingProvinces, set.List()); err != nil {
		return fmt.Errorf("write starting provinces: %w", err)
	}

	fmt.Printf("added starting province %s to game %q (%d total)\n", canonical, game.ID, set.Len())
	fmt.Printf("wrote %s\n", files.StartingProvinces)
	return nil
}

// removeStartingProvince removes province from the game's allowed starting
// provinces and writes the updated file. A non-canonical province, or one not in
// the set, is an error and nothing is written. If a player is already placed on
// the removed province it warns (but proceeds): that player is left on a province
// no longer allowed, which the GM must resolve.
func removeStartingProvince(data, province string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}
	set, err := loadStartingProvinces(files.StartingProvinces)
	if err != nil {
		return err
	}
	canonical, err := set.Remove(province)
	if err != nil {
		return err
	}

	// Surface players stranded by the removal. loadPlayers yields an empty store
	// when the file is absent, so this is a no-op before any player exists.
	store, err := loadPlayers(files.Players)
	if err != nil {
		return err
	}
	for _, p := range store.Players {
		if p.StartingProvince == canonical {
			fmt.Fprintf(os.Stderr, "warning: player %d (%s) is placed on %s, which is no longer an allowed starting province\n", p.ID, p.Handle, canonical)
		}
	}

	if err := writeJSON(files.StartingProvinces, set.List()); err != nil {
		return fmt.Errorf("write starting provinces: %w", err)
	}

	fmt.Printf("removed starting province %s from game %q (%d remaining)\n", canonical, game.ID, set.Len())
	fmt.Printf("wrote %s\n", files.StartingProvinces)
	return nil
}

// listStartingProvinces prints the game's allowed starting provinces in order.
func listStartingProvinces(data string) error {
	game, files, err := loadGame(data)
	if err != nil {
		return err
	}
	set, err := loadStartingProvinces(files.StartingProvinces)
	if err != nil {
		return err
	}

	if set.Len() == 0 {
		fmt.Printf("game %q has no starting provinces\n", game.ID)
		return nil
	}

	fmt.Printf("game %q has %d starting province(s):\n", game.ID, set.Len())
	for _, p := range set.List() {
		fmt.Printf("  %s\n", p)
	}
	return nil
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

// loadFactions reads the factions file at path into a store. A missing file
// yields a new, empty store, so ownership checks work before any faction exists.
func loadFactions(path string) (*tpty.FactionStore, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return tpty.NewFactionStore(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read factions: %w", err)
	}
	var store tpty.FactionStore
	if err := json.Unmarshal(buf, &store); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &store, nil
}

// loadEntities reads the entities file at path into a store. A missing file
// yields a new, empty store, so ownership checks work before any entity exists.
func loadEntities(path string) (*tpty.EntityStore, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return tpty.NewEntityStore(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read entities: %w", err)
	}
	var store tpty.EntityStore
	if err := json.Unmarshal(buf, &store); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &store, nil
}

// loadStartingProvinces reads the allowed starting provinces at path into a set,
// validating that each entry is a canonical compact province string and that no
// entry repeats. A missing file yields an empty set, so callers can distinguish
// "no provinces defined" (an actionable condition) from a genuine read/parse
// failure.
func loadStartingProvinces(path string) (*tpty.StartingProvinceSet, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return tpty.NewStartingProvinceSet(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read starting provinces: %w", err)
	}
	var list []string
	if err := json.Unmarshal(buf, &list); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	set, err := tpty.ParseStartingProvinceSet(list)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
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
