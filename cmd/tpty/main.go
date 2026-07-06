// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Command tpty is the command-line interface to the T'Pty game engine.
package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	"github.com/mdhender/tpty"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

func main() {
	root := newRootCommand()

	err := root.ParseAndRun(context.Background(), os.Args[1:])
	switch {
	case errors.Is(err, ff.ErrHelp):
		fmt.Fprintf(os.Stderr, "%s\n", ffhelp.Command(root))
		os.Exit(0)
	case err != nil:
		fmt.Fprintf(os.Stderr, "tpty: %v\n", err)
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

	world.Subcommands = []*ff.Command{newWorldGenerateCommand(worldFlags, data)}
	return world
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

			return generateWorld(*rings, *data, s1, s2)
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

// generateWorld generates a world of the given number of rings and writes it
// into the engine's data directory, using the two master seeds.
//
// TODO: implement world generation per content/docs/reference/world-generation.md.
func generateWorld(rings int, data string, seed1, seed2 uint64) error {
	return fmt.Errorf("world generation is not yet implemented (rings=%d, data=%q)", rings, data)
}
