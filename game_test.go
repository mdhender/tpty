// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mdhender/tpty/internal/prng"
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
	if _, err := NewGame("bad id", prng.Seeds{Seed1: 1, Seed2: 2}); !errors.Is(err, ErrInvalidGameID) {
		t.Errorf("NewGame with bad id: err = %v, want ErrInvalidGameID", err)
	}
	g, err := NewGame("ok", prng.Seeds{Seed1: 1, Seed2: 2})
	if err != nil {
		t.Fatal(err)
	}
	if g.Files != DefaultGameFiles() {
		t.Errorf("NewGame files = %+v, want defaults %+v", g.Files, DefaultGameFiles())
	}
}

func TestGameJSONRoundTrip(t *testing.T) {
	g, err := NewGame("smoke-test-1", prng.Seeds{Seed1: 12345, Seed2: 67890})
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

// TestNewGameStartsAtTurnZero confirms a new game begins at turn 0 (setup — no
// turn), the zero value.
func TestNewGameStartsAtTurnZero(t *testing.T) {
	g, err := NewGame("ok", prng.Seeds{Seed1: 1, Seed2: 2})
	if err != nil {
		t.Fatal(err)
	}
	if g.Turn != 0 {
		t.Errorf("new game Turn = %d, want 0", g.Turn)
	}
}

// TestGameTurnRoundTrips confirms a non-zero turn survives a JSON round-trip and
// is written under the "turn" key.
func TestGameTurnRoundTrips(t *testing.T) {
	g, err := NewGame("smoke-test-1", prng.Seeds{Seed1: 1, Seed2: 2})
	if err != nil {
		t.Fatal(err)
	}
	g.Turn = 5
	buf, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var got Game
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatal(err)
	}
	if got.Turn != 5 {
		t.Errorf("round-trip Turn = %d, want 5", got.Turn)
	}
	var raw map[string]any
	_ = json.Unmarshal(buf, &raw)
	if _, ok := raw["turn"]; !ok {
		t.Errorf("expected a \"turn\" key in %s", buf)
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
	game := prng.Seeds{Seed1: 7, Seed2: 13}
	w, err := GenerateWorld(game, 3)
	if err != nil {
		t.Fatal(err)
	}
	if w.Seeds == game {
		t.Error("world seeds equal the game seeds; expected a derived pair")
	}
	if want := game.Derive(prng.TagWorldSeeds); w.Seeds != want {
		t.Errorf("world seeds = %+v, want derived %+v", w.Seeds, want)
	}
}
