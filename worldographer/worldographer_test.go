// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldographer

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/mdhender/ottomap/hex"
	"github.com/mdhender/ottomap/wog"
)

// ring1 is the origin plus the six provinces of ring 1.
func ring1() []Province {
	return []Province{
		{Q: 0, R: 0, Terrain: "Mountain"},
		{Q: 0, R: -1, Terrain: "Plains"},
		{Q: 1, R: -1, Terrain: "Forests"},
		{Q: 1, R: 0, Terrain: "Desert"},
		{Q: 0, R: 1, Terrain: "Hills"},
		{Q: -1, R: 1, Terrain: "Lake"},
		{Q: -1, R: 0, Terrain: "Badlands"},
	}
}

func fullTranslation() map[string]string {
	return map[string]string{
		"Mountain": "Classic/Mountains",
		"Plains":   "Classic/Flat Farmland",
		"Forests":  "Classic/Flat Forest Deciduous Heavy",
		"Desert":   "Classic/Flat Desert Sandy",
		"Hills":    "Classic/Hills",
		"Lake":     "Classic/Water Sea",
		"Badlands": "Classic/Other Badlands",
	}
}

func TestOffsetBoundsPadsMinColToEvenParity(t *testing.T) {
	min, max := offsetBounds(ring1())
	if min.Col&1 != minColParity {
		t.Errorf("min.Col %d has wrong parity", min.Col)
	}
	// ring 1 spans offset cols -1..1, rows -1..1; the odd min col -1 pads to -2.
	if min != (hex.OffsetCoord{Col: -2, Row: -1}) || max != (hex.OffsetCoord{Col: 1, Row: 1}) {
		t.Errorf("bounds = %v..%v, want {-2 -1}..{1 1}", min, max)
	}
}

// TestRenderRoundTrip renders provinces to a .wxx and reads them back, checking
// every province lands at the same axial coordinate with the translated tile.
// This verifies the coordinate conversion is internally consistent (visual
// stagger parity still requires opening the file in Worldographer).
func TestRenderRoundTrip(t *testing.T) {
	provinces := ring1()
	translation := fullTranslation()

	var buf bytes.Buffer
	if err := Render(&buf, provinces, translation); err != nil {
		t.Fatalf("Render: %v", err)
	}

	m, _, err := wog.Read(&buf)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	// The .wxx format re-bases the grid to numberFirstCol/Row = 0, so absolute
	// coordinates shift by -offMin (an even shift, which preserves column
	// parity). Compare the relative offset layout and terrains instead.
	min, _ := offsetBounds(provinces)
	want := map[hex.OffsetCoord]string{}
	for _, p := range provinces {
		oc := hex.Axial{Q: p.Q, R: p.R}.ToOffset(hex.OddQ)
		want[hex.OffsetCoord{Col: oc.Col - min.Col, Row: oc.Row - min.Row}] = translation[p.Terrain]
	}

	got := map[hex.OffsetCoord]string{}
	for c, tile := range m.Tiles() {
		if name := string(tile.Terrain); name != "" && name != "Blank" {
			got[c.ToOffset(hex.OddQ)] = name
		}
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip layout mismatch:\n got  %v\n want %v", got, want)
	}
}

func TestRenderMissingTranslationFails(t *testing.T) {
	// Lake has no translation.
	translation := fullTranslation()
	delete(translation, "Lake")

	var buf bytes.Buffer
	if err := Render(&buf, ring1(), translation); err == nil {
		t.Fatal("Render succeeded, want error for missing translation")
	}
}

func TestRenderEmptyFails(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, nil, fullTranslation()); err == nil {
		t.Fatal("Render(nil) succeeded, want error")
	}
}
