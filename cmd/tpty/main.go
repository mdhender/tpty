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
//	└── world
//	    └── generate
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

	root.Subcommands = []*ff.Command{newWorldCommand(rootFlags, data)}
	return root
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
	}
	return world
}

// newWorldRenderCommand builds the "world render" command, which renders a
// generated world to a Worldographer .wxx file in the data directory.
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

// renderWorld reads world.json and terrain-translation.json from the data
// directory and writes world.wxx (Worldographer) alongside them.
func renderWorld(data string) error {
	worldPath := filepath.Join(data, "world.json")
	worldBuf, err := os.ReadFile(worldPath)
	if err != nil {
		return fmt.Errorf("read world: %w", err)
	}
	var world struct {
		Provinces []worldographer.Province `json:"provinces"`
	}
	if err := json.Unmarshal(worldBuf, &world); err != nil {
		return fmt.Errorf("parse %s: %w", worldPath, err)
	}

	ttPath := filepath.Join(data, "terrain-translation.json")
	ttBuf, err := os.ReadFile(ttPath)
	if err != nil {
		return fmt.Errorf("read terrain translation: %w", err)
	}
	var translation map[string]string
	if err := json.Unmarshal(ttBuf, &translation); err != nil {
		return fmt.Errorf("parse %s: %w", ttPath, err)
	}

	outPath := filepath.Join(data, "world.wxx")
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

// newWorldGenerateCommand builds the "world generate" command.
//
// See the reference documentation at
// content/docs/reference/world-generation.md for the rules this implements.
func newWorldGenerateCommand(parent *ff.FlagSet, data *string) *ff.Command {
	fs := ff.NewFlagSet("generate").SetParent(parent)
	rings := fs.IntLong("rings", 0, "number of `rings` to generate (0 < rings < 100)")
	seed1 := fs.Uint64Long("seed1", 0, "first master `seed` (0 = choose at random)")
	seed2 := fs.Uint64Long("seed2", 0, "second master `seed` (0 = choose at random)")

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

			// Resolve master seeds, choosing random values where unset. The
			// resolved seeds are reported so the world can be reproduced.
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
			fmt.Printf("seeds: seed1=%d seed2=%d\n", s1, s2)

			return generateWorld(*rings, *data, tpty.Seeds{Seed1: s1, Seed2: s2})
		},
	}
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

// generateWorld generates a world of the given number of rings from the master
// seeds and writes it as JSON into the engine's data directory.
func generateWorld(rings int, data string, seeds tpty.Seeds) error {
	world, err := tpty.GenerateWorld(seeds, rings)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(data, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	path := filepath.Join(data, "world.json")
	buf, err := json.MarshalIndent(world, "", "  ")
	if err != nil {
		return fmt.Errorf("encode world: %w", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("write world: %w", err)
	}

	// Write the terrain-to-Worldographer tile translation alongside the world,
	// so the world can be imported into Worldographer.
	ttPath := filepath.Join(data, "terrain-translation.json")
	ttBuf, err := json.MarshalIndent(tpty.TerrainTranslation(), "", "  ")
	if err != nil {
		return fmt.Errorf("encode terrain translation: %w", err)
	}
	if err := os.WriteFile(ttPath, ttBuf, 0o644); err != nil {
		return fmt.Errorf("write terrain translation: %w", err)
	}

	fmt.Printf("wrote %d provinces to %s\n", len(world.Provinces), path)
	fmt.Printf("wrote terrain translation to %s\n", ttPath)
	return nil
}
