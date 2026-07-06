// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
	"path/filepath"

	"github.com/mdhender/tpty/cerrs"
)

// Game is the top-level unit of play. The world and the players belong to it. A
// game is stored as a game.json manifest: its id and master seeds, plus the
// locations of its data files.
//
// See content/docs/reference/games.md for the rules.
type Game struct {
	ID    string    `json:"id"`
	Seeds Seeds     `json:"seeds"`
	Files GameFiles `json:"files"`
}

// GameFiles maps each of a game's data files to a path. Paths are resolved
// relative to the directory that contains game.json and may point outside it, so
// two games can share a file.
type GameFiles struct {
	World              string `json:"world"`
	Players            string `json:"players"`
	StartingProvinces  string `json:"starting-provinces"`
	TerrainTranslation string `json:"terrain-translation"`
}

// ErrInvalidGameID is returned when a game id is empty or contains a character
// that is not allowed.
const ErrInvalidGameID = cerrs.Error("invalid game id")

// DefaultGameFiles returns the default data-file layout, with every file beside
// game.json.
func DefaultGameFiles() GameFiles {
	return GameFiles{
		World:              "./world.json",
		Players:            "./players.json",
		StartingProvinces:  "./starting-provinces.json",
		TerrainTranslation: "./terrain-translation.json",
	}
}

// NewGame returns a game with the given id and seeds and the default file
// layout. The id must be valid (see ValidateGameID).
func NewGame(id string, seeds Seeds) (*Game, error) {
	if err := ValidateGameID(id); err != nil {
		return nil, err
	}
	return &Game{ID: id, Seeds: seeds, Files: DefaultGameFiles()}, nil
}

// ValidateGameID reports whether id is a well-formed game id, wrapping
// ErrInvalidGameID if not. A game id is quoted text with the same restrictions
// as a password: non-empty, and every character is a printable ASCII character
// other than space, double quote, or backslash — so it needs no JSON escaping
// and cannot be confused with a space.
func ValidateGameID(id string) error {
	if id == "" {
		return fmt.Errorf("empty: %w", ErrInvalidGameID)
	}
	for _, r := range id {
		if r <= ' ' || r > '~' || r == '"' || r == '\\' {
			return fmt.Errorf("%q: character %q is not allowed: %w", id, r, ErrInvalidGameID)
		}
	}
	return nil
}

// Resolve returns a copy of f with every path resolved against baseDir. An
// absolute path is left unchanged; a relative path is joined to baseDir.
func (f GameFiles) Resolve(baseDir string) GameFiles {
	resolve := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(baseDir, p)
	}
	return GameFiles{
		World:              resolve(f.World),
		Players:            resolve(f.Players),
		StartingProvinces:  resolve(f.StartingProvinces),
		TerrainTranslation: resolve(f.TerrainTranslation),
	}
}
