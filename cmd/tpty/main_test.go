// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package main

import (
	"os"
	"path/filepath"
	"testing"
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
