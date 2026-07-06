// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Package worldographer renders a generated world to a Worldographer .wxx file.
//
// It is export tooling: it converts our provinces (axial coordinates plus a
// terrain name) into a Worldographer 2025 COLUMNS map, translating each terrain
// to a Worldographer tile via a translation table.
package worldographer

import (
	"fmt"
	"io"
	"sort"

	"github.com/mdhender/ottomap"
	"github.com/mdhender/ottomap/hex"
	"github.com/mdhender/ottomap/wog"
)

// Province is one hex to render: its axial coordinates and terrain name. The
// JSON tags match the provinces in a generated world.json.
type Province struct {
	Q       int    `json:"q"`
	R       int    `json:"r"`
	Terrain string `json:"terrain"`
}

// Render writes the provinces to w as a Worldographer 2025 COLUMNS (.wxx) map,
// translating each terrain name to a Worldographer tile via translation.
//
// A terrain with no entry in translation is a hard error; the GM must fix the
// translation table.
func Render(w io.Writer, provinces []Province, translation map[string]string) error {
	if len(provinces) == 0 {
		return fmt.Errorf("no provinces to render")
	}

	m := ottomap.NewMap()
	m.SetLayout(hex.OddQ)

	missing := map[string]bool{}
	for _, p := range provinces {
		tile, ok := translation[p.Terrain]
		if !ok {
			missing[p.Terrain] = true
			continue
		}
		m.SetTerrain(hex.Axial{Q: p.Q, R: p.R}, ottomap.Terrain(tile))
	}
	if len(missing) > 0 {
		return fmt.Errorf("no translation for terrain(s): %v", sortedKeys(missing))
	}

	min, max := offsetBounds(provinces)
	m.SetBounds(hex.FromOffset(min, hex.OddQ), hex.FromOffset(max, hex.OddQ))

	return wog.Write(w, m, wog.WriteOptions{
		Version:     wog.V2025,
		Orientation: wog.Columns,
	})
}

// minColParity is the column parity that offsetBounds pads the minimum column
// to. Worldographer staggers a COLUMNS map by array position (the odd-indexed
// emitted column is shifted down); padding the minimum column to even parity
// makes the array parity match odd-q's absolute-column parity, so the origin
// and every province render on the correct stagger.
//
// Even parity was verified by rendering a world and opening it in
// Worldographer; odd parity (1) mirrors the stagger.
const minColParity = 0 // even

// offsetBounds returns the minimal odd-q offset bounding box covering every
// province, with the minimum column adjusted to minColParity. The adjustment
// adds at most one column on the west edge, and only when the minimal column
// has the wrong parity.
func offsetBounds(ps []Province) (min, max hex.OffsetCoord) {
	first := hex.Axial{Q: ps[0].Q, R: ps[0].R}.ToOffset(hex.OddQ)
	min, max = first, first
	for _, p := range ps[1:] {
		oc := hex.Axial{Q: p.Q, R: p.R}.ToOffset(hex.OddQ)
		min.Col = minInt(min.Col, oc.Col)
		min.Row = minInt(min.Row, oc.Row)
		max.Col = maxInt(max.Col, oc.Col)
		max.Row = maxInt(max.Row, oc.Row)
	}
	if min.Col&1 != minColParity {
		min.Col--
	}
	return min, max
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
