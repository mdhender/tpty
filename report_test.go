// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/mdhender/tpty/internal/prng"
)

// twoPlayerGame builds a game with two active players, each controlling one
// faction that owns one entity, plus optionally an inactive third player. It
// returns the stores so tests can exercise GenerateReports.
func twoPlayerGame(t *testing.T) (*Game, *PlayerStore, *FactionStore, *EntityStore) {
	t.Helper()
	seeds := prng.Seeds{Seed1: 1, Seed2: 2}
	game, err := NewGame("report-test", seeds)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	game.Turn = 1

	players := NewPlayerStore()
	if _, err := players.Create(seeds, "alice@x.com", "alice", "(1,-1)"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := players.Create(seeds, "bob@x.com", "bob", "(2,-1)"); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	factions := NewFactionStore()
	entities := NewEntityStore()
	if _, err := SeedTurnOne(players, factions, entities); err != nil {
		t.Fatalf("SeedTurnOne: %v", err)
	}
	return game, players, factions, entities
}

// TestGenerateReportsOnePerActivePlayer checks that one report is produced per
// active player, in ascending player-id order.
func TestGenerateReportsOnePerActivePlayer(t *testing.T) {
	game, players, factions, entities := twoPlayerGame(t)

	reports := GenerateReports(game, players, factions, entities, game.Turn)
	if len(reports) != 2 {
		t.Fatalf("got %d reports, want 2", len(reports))
	}
	if reports[0].PlayerID != 1 || reports[1].PlayerID != 2 {
		t.Fatalf("report player ids = %d,%d, want 1,2 (ascending)", reports[0].PlayerID, reports[1].PlayerID)
	}
	for _, r := range reports {
		if r.Turn != game.Turn {
			t.Errorf("report for player %d has turn %d, want %d", r.PlayerID, r.Turn, game.Turn)
		}
	}
}

// TestGenerateReportsIsolation is the key test: player A's report carries only
// A's faction and entity, never B's. It proves the per-player scoping.
func TestGenerateReportsIsolation(t *testing.T) {
	game, players, factions, entities := twoPlayerGame(t)

	reports := GenerateReports(game, players, factions, entities, game.Turn)
	if len(reports) != 2 {
		t.Fatalf("got %d reports, want 2", len(reports))
	}

	alice, _ := players.ByID(1)
	bob, _ := players.ByID(2)

	for _, r := range reports {
		if r.PlayerID != 1 && r.PlayerID != 2 {
			t.Fatalf("unexpected player id %d", r.PlayerID)
		}
		if len(r.Factions) != 1 {
			t.Fatalf("player %d has %d factions, want 1", r.PlayerID, len(r.Factions))
		}
		if len(r.Entities) != 1 {
			t.Fatalf("player %d has %d entities, want 1", r.PlayerID, len(r.Entities))
		}
		// The faction must be controlled by this player.
		f := r.Factions[0]
		if f.Controller.Kind != ControllerPlayer || f.Controller.ID != r.PlayerID {
			t.Errorf("player %d report faction controller = %+v, want player %d", r.PlayerID, f.Controller, r.PlayerID)
		}
		// The entity must belong to this player's faction.
		e := r.Entities[0]
		if e.FactionID != f.ID {
			t.Errorf("player %d report entity faction = %d, want %d", r.PlayerID, e.FactionID, f.ID)
		}
		// The entity's location must be the player's starting province.
		wantLoc := alice.StartingProvince
		wantHandle := alice.Handle
		wantEmail := alice.Email
		if r.PlayerID == 2 {
			wantLoc = bob.StartingProvince
			wantHandle = bob.Handle
			wantEmail = bob.Email
		}
		if e.Location != wantLoc {
			t.Errorf("player %d report entity location = %q, want %q", r.PlayerID, e.Location, wantLoc)
		}
		if r.PlayerHandle != wantHandle {
			t.Errorf("player %d report handle = %q, want %q", r.PlayerID, r.PlayerHandle, wantHandle)
		}
		if r.PlayerEmail != wantEmail {
			t.Errorf("player %d report email = %q, want %q", r.PlayerID, r.PlayerEmail, wantEmail)
		}
	}

	// Cross-check: alice's report must not contain bob's faction or entity ids.
	aliceReport, bobReport := reports[0], reports[1]
	if aliceReport.Factions[0].ID == bobReport.Factions[0].ID {
		t.Error("alice and bob share a faction id in their reports")
	}
	if aliceReport.Entities[0].ID == bobReport.Entities[0].ID {
		t.Error("alice and bob share an entity id in their reports")
	}
}

// TestGenerateReportsInactivePlayerSkipped verifies a removed player gets no
// report even though their faction and entity records persist.
func TestGenerateReportsInactivePlayerSkipped(t *testing.T) {
	game, players, factions, entities := twoPlayerGame(t)

	if _, err := players.Deactivate(2); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	reports := GenerateReports(game, players, factions, entities, game.Turn)
	if len(reports) != 1 {
		t.Fatalf("got %d reports, want 1 after deactivating player 2", len(reports))
	}
	if reports[0].PlayerID != 1 {
		t.Errorf("remaining report is for player %d, want 1", reports[0].PlayerID)
	}
}

// TestGenerateReportsDeterministic verifies the output is identical across
// repeated calls (ordering does not depend on iteration order).
func TestGenerateReportsDeterministic(t *testing.T) {
	game, players, factions, entities := twoPlayerGame(t)

	first := GenerateReports(game, players, factions, entities, game.Turn)
	second := GenerateReports(game, players, factions, entities, game.Turn)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("GenerateReports not deterministic:\nfirst  = %+v\nsecond = %+v", first, second)
	}
}

// TestReportPathHelpers checks the zero-padded path helpers.
func TestReportPathHelpers(t *testing.T) {
	if got, want := ReportsTurnDir("reports", 1), "reports/turn-0001"; got != want {
		t.Errorf("ReportsTurnDir = %q, want %q", got, want)
	}
	if got, want := PlayerReportFilename(3), "player-0003.json"; got != want {
		t.Errorf("PlayerReportFilename = %q, want %q", got, want)
	}
	if got, want := PlayerReportPath("reports", 12, 7), "reports/turn-0012/player-0007.json"; got != want {
		t.Errorf("PlayerReportPath = %q, want %q", got, want)
	}
}

// TestSaveReportRoundTrip writes a report and reads it back, confirming the JSON
// round-trips to an equal value at the expected path.
func TestSaveReportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	report := Report{
		PlayerID:     1,
		PlayerHandle: "alice",
		PlayerEmail:  "alice@x.com",
		Turn:         1,
		Factions:     []Faction{{ID: 1, Name: "Faction 1", Controller: Controller{Kind: ControllerPlayer, ID: 1}}},
		Entities:     []Entity{{ID: 1, Name: "Entity 1", FactionID: 1, Location: "(1,-1)"}},
	}

	if err := SaveReport(dir, report); err != nil {
		t.Fatalf("SaveReport: %v", err)
	}

	path := PlayerReportPath(dir, report.Turn, report.PlayerID)
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved report: %v", err)
	}
	var got Report
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatalf("decode saved report: %v", err)
	}
	if !reflect.DeepEqual(got, report) {
		t.Errorf("round-trip mismatch:\ngot  = %+v\nwant = %+v", got, report)
	}
}
