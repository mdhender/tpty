// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdhender/tpty"
)

// TestLoadStartingProvincesMissingFile verifies that an absent file yields an
// empty, non-nil set and no error, so createPlayer can distinguish "no
// provinces defined" from a genuine read failure.
func TestLoadStartingProvincesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	set, err := loadStartingProvinces(path)
	if err != nil {
		t.Fatalf("loadStartingProvinces(missing) error = %v, want nil", err)
	}
	if set == nil {
		t.Fatal("loadStartingProvinces(missing) set = nil, want empty non-nil set")
	}
	if len(set) != 0 {
		t.Errorf("loadStartingProvinces(missing) len = %d, want 0", len(set))
	}
}

// TestLoadStartingProvincesEmptyArray verifies that an explicit empty array is
// treated the same as a missing file: an empty set with no error.
func TestLoadStartingProvincesEmptyArray(t *testing.T) {
	path := writeTempFile(t, "[]")
	set, err := loadStartingProvinces(path)
	if err != nil {
		t.Fatalf("loadStartingProvinces([]) error = %v, want nil", err)
	}
	if len(set) != 0 {
		t.Errorf("loadStartingProvinces([]) len = %d, want 0", len(set))
	}
}

// TestLoadStartingProvincesValid verifies that valid entries are parsed into the
// set keyed by their canonical compact form.
func TestLoadStartingProvincesValid(t *testing.T) {
	path := writeTempFile(t, `["(0,0)","(1,-1)"]`)
	set, err := loadStartingProvinces(path)
	if err != nil {
		t.Fatalf("loadStartingProvinces(valid) error = %v, want nil", err)
	}
	for _, want := range []string{"(0,0)", "(1,-1)"} {
		if !set[want] {
			t.Errorf("loadStartingProvinces(valid) missing %q; set = %v", want, set)
		}
	}
	if len(set) != 2 {
		t.Errorf("loadStartingProvinces(valid) len = %d, want 2", len(set))
	}
}

// TestLoadStartingProvincesErrors verifies that malformed JSON and invalid
// province strings still surface as errors, so the missing-file special case
// does not mask genuine problems.
func TestLoadStartingProvincesErrors(t *testing.T) {
	tests := map[string]string{
		"malformed json":   `{`,
		"not a string":     `[123]`,
		"invalid province": `["not-a-province"]`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := writeTempFile(t, content)
			if _, err := loadStartingProvinces(path); err == nil {
				t.Errorf("loadStartingProvinces(%q) error = nil, want an error", content)
			}
		})
	}
}

// writeTempFile writes content to a file in a fresh temp dir and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "starting-provinces.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// setupGameDir creates a data directory with a game.json and, when rings > 0, a
// world.json of that many rings. When rings <= 0 no world file is written, so
// the world-absent path can be exercised.
func setupGameDir(t *testing.T, rings int) string {
	t.Helper()
	dir := t.TempDir()
	game, err := tpty.NewGame("test-game", tpty.Seeds{Seed1: 1, Seed2: 2})
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := writeJSON(filepath.Join(dir, "game.json"), game); err != nil {
		t.Fatalf("write game.json: %v", err)
	}
	if rings > 0 {
		world := map[string]any{"rings": rings, "provinces": []any{}}
		if err := writeJSON(filepath.Join(dir, "world.json"), world); err != nil {
			t.Fatalf("write world.json: %v", err)
		}
	}
	return dir
}

// readProvinces reads and decodes the starting-provinces.json in dir.
func readProvinces(t *testing.T, dir string) []string {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join(dir, "starting-provinces.json"))
	if err != nil {
		t.Fatalf("read starting-provinces.json: %v", err)
	}
	var got []string
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatalf("decode starting-provinces.json: %v", err)
	}
	return got
}

// captureOutput runs fn with os.Stdout and os.Stderr redirected to pipes and
// returns whatever each received.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	outC, errC := make(chan string, 1), make(chan string, 1)
	go func() { var b bytes.Buffer; _, _ = io.Copy(&b, outR); outC <- b.String() }()
	go func() { var b bytes.Buffer; _, _ = io.Copy(&b, errR); errC <- b.String() }()

	fn()

	_ = outW.Close()
	_ = errW.Close()
	return <-outC, <-errC
}

func TestGenerateStartingProvincesHardFailsWithoutWorld(t *testing.T) {
	dir := setupGameDir(t, 0) // no world.json
	err := generateStartingProvinces(dir, 0, false)
	if err == nil {
		t.Fatal("generateStartingProvinces without world.json = nil error, want error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "starting-provinces.json")); statErr == nil {
		t.Error("starting-provinces.json was written despite the world being absent")
	}
}

func TestGenerateStartingProvincesDefaultRing(t *testing.T) {
	dir := setupGameDir(t, 3) // default ring = ceil(3/2) = 2
	if _, _, err := runGenerate(t, dir, 0, false); err != nil {
		t.Fatalf("generateStartingProvinces: %v", err)
	}
	want := []string{"(0,-2)", "(2,-2)", "(2,0)", "(0,2)", "(-2,2)", "(-2,0)"}
	if got := readProvinces(t, dir); !equalStrings(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}
}

func TestGenerateStartingProvincesExplicitRing(t *testing.T) {
	dir := setupGameDir(t, 3)
	if _, _, err := runGenerate(t, dir, 1, false); err != nil {
		t.Fatalf("generateStartingProvinces: %v", err)
	}
	want := []string{"(0,-1)", "(1,-1)", "(1,0)", "(0,1)", "(-1,1)", "(-1,0)"}
	if got := readProvinces(t, dir); !equalStrings(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}
}

func TestGenerateStartingProvincesRejectsOutOfRangeRing(t *testing.T) {
	dir := setupGameDir(t, 3)
	for _, ring := range []int{-1, 4, 100} {
		if err := generateStartingProvinces(dir, ring, false); err == nil {
			t.Errorf("--ring %d with a 3-ring world = nil error, want error", ring)
		}
	}
}

func TestGenerateStartingProvincesFailsWhenFileExists(t *testing.T) {
	dir := setupGameDir(t, 3)
	spPath := filepath.Join(dir, "starting-provinces.json")
	if err := os.WriteFile(spPath, []byte(`["(0,0)"]`), 0o644); err != nil {
		t.Fatalf("seed starting-provinces.json: %v", err)
	}
	if err := generateStartingProvinces(dir, 0, false); err == nil {
		t.Fatal("existing file without --overwrite = nil error, want error")
	}
	// The pre-existing file must be untouched.
	if got := readProvinces(t, dir); !equalStrings(got, []string{"(0,0)"}) {
		t.Errorf("existing file was modified: %v", got)
	}
}

func TestGenerateStartingProvincesOverwrite(t *testing.T) {
	dir := setupGameDir(t, 3)
	spPath := filepath.Join(dir, "starting-provinces.json")
	if err := os.WriteFile(spPath, []byte(`["(0,0)"]`), 0o644); err != nil {
		t.Fatalf("seed starting-provinces.json: %v", err)
	}
	if _, _, err := runGenerate(t, dir, 0, true); err != nil {
		t.Fatalf("generateStartingProvinces --overwrite: %v", err)
	}
	want := []string{"(0,-2)", "(2,-2)", "(2,0)", "(0,2)", "(-2,2)", "(-2,0)"}
	if got := readProvinces(t, dir); !equalStrings(got, want) {
		t.Errorf("after overwrite wrote %v, want %v", got, want)
	}
}

func TestGenerateStartingProvincesWarnsWhenPlayersExist(t *testing.T) {
	dir := setupGameDir(t, 3)
	if err := os.WriteFile(filepath.Join(dir, "players.json"), []byte(`{"players":[]}`), 0o644); err != nil {
		t.Fatalf("seed players.json: %v", err)
	}
	_, stderr, err := runGenerate(t, dir, 0, false)
	if err != nil {
		t.Fatalf("generateStartingProvinces with players.json = %v, want success", err)
	}
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "players.json") {
		t.Errorf("stderr = %q, want a warning mentioning players.json", stderr)
	}
	// It still writes the set despite the warning.
	if got := readProvinces(t, dir); len(got) != 6 {
		t.Errorf("wrote %d provinces, want 6", len(got))
	}
}

// runGenerate runs generateStartingProvinces with output captured, returning the
// captured stdout, stderr, and the command's error.
func runGenerate(t *testing.T, dir string, ring int, overwrite bool) (stdout, stderr string, err error) {
	t.Helper()
	stdout, stderr = captureOutput(t, func() { err = generateStartingProvinces(dir, ring, overwrite) })
	return stdout, stderr, err
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
