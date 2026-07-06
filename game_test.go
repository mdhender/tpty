// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func TestValidateGameID(t *testing.T) {
	valid := []string{"a", "smoke-test-1", "Game_1", "g.1", "ABCxyz-._~"}
	invalid := []string{"", "has space", `has"quote`, `back\slash`, "tab\there", "π-unicode", "trail "}
	for _, id := range valid {
		if err := ValidateGameID(id); err != nil {
			t.Errorf("ValidateGameID(%q) = %v, want nil", id, err)
		}
	}
	for _, id := range invalid {
		if err := ValidateGameID(id); !errors.Is(err, ErrInvalidGameID) {
			t.Errorf("ValidateGameID(%q) = %v, want ErrInvalidGameID", id, err)
		}
	}
}

func TestNewGameValidatesID(t *testing.T) {
	if _, err := NewGame("bad id", Seeds{Seed1: 1, Seed2: 2}); !errors.Is(err, ErrInvalidGameID) {
		t.Errorf("NewGame with bad id: err = %v, want ErrInvalidGameID", err)
	}
	g, err := NewGame("ok", Seeds{Seed1: 1, Seed2: 2})
	if err != nil {
		t.Fatal(err)
	}
	if g.Files != DefaultGameFiles() {
		t.Errorf("NewGame files = %+v, want defaults %+v", g.Files, DefaultGameFiles())
	}
}

func TestGameJSONRoundTrip(t *testing.T) {
	g, err := NewGame("smoke-test-1", Seeds{Seed1: 12345, Seed2: 67890})
	if err != nil {
		t.Fatal(err)
	}
	buf, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var got Game
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatal(err)
	}
	if got != *g {
		t.Errorf("round-trip changed the game:\n got %+v\nwant %+v", got, *g)
	}
	// The hyphenated JSON keys must be present.
	var raw map[string]any
	_ = json.Unmarshal(buf, &raw)
	files, _ := raw["files"].(map[string]any)
	if _, ok := files["starting-provinces"]; !ok {
		t.Errorf("expected a \"starting-provinces\" key in %s", buf)
	}
}

func TestGameFilesResolve(t *testing.T) {
	base := filepath.FromSlash("/games/g1")
	abs := filepath.FromSlash("/shared/world.json")
	f := GameFiles{
		World:              "./world.json",
		Players:            "players.json",
		StartingProvinces:  abs, // absolute: unchanged
		TerrainTranslation: "",  // empty: unchanged
	}
	got := f.Resolve(base)
	if want := filepath.Join(base, "world.json"); got.World != want {
		t.Errorf("World = %q, want %q", got.World, want)
	}
	if want := filepath.Join(base, "players.json"); got.Players != want {
		t.Errorf("Players = %q, want %q", got.Players, want)
	}
	if got.StartingProvinces != abs {
		t.Errorf("StartingProvinces = %q, want %q (absolute paths are unchanged)", got.StartingProvinces, abs)
	}
	if got.TerrainTranslation != "" {
		t.Errorf("TerrainTranslation = %q, want \"\" (empty paths are unchanged)", got.TerrainTranslation)
	}
}

// TestWorldSeedsAreDerived confirms the world derives its own master seeds from
// the game's (they are not the game seeds), deterministically.
func TestWorldSeedsAreDerived(t *testing.T) {
	game := Seeds{Seed1: 7, Seed2: 13}
	w, err := GenerateWorld(game, 3)
	if err != nil {
		t.Fatal(err)
	}
	if w.Seeds == game {
		t.Error("world seeds equal the game seeds; expected a derived pair")
	}
	if want := game.Derive(KeyWorldSeeds); w.Seeds != want {
		t.Errorf("world seeds = %+v, want derived %+v", w.Seeds, want)
	}
}
