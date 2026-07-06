// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"reflect"
	"testing"
)

func TestGenerateWorldRejectsBadRings(t *testing.T) {
	for _, rings := range []int{-1, 0, 100, 200} {
		if _, err := GenerateWorld(Seeds{Seed1: 1, Seed2: 2}, rings); err == nil {
			t.Errorf("GenerateWorld(rings=%d) = nil error, want error", rings)
		}
	}
}

func TestGenerateWorldProvinceCount(t *testing.T) {
	seeds := Seeds{Seed1: 1, Seed2: 2}
	for _, tc := range []struct{ rings, want int }{
		{1, 7}, {2, 19}, {3, 37}, {5, 91},
	} {
		w, err := GenerateWorld(seeds, tc.rings)
		if err != nil {
			t.Fatalf("GenerateWorld(rings=%d): %v", tc.rings, err)
		}
		if got := len(w.Provinces); got != tc.want {
			t.Errorf("rings=%d: %d provinces, want %d (1 + 3n(n+1))", tc.rings, got, tc.want)
		}
	}
}

func TestGenerateWorldOriginIsMountain(t *testing.T) {
	w, err := GenerateWorld(Seeds{Seed1: 42, Seed2: 99}, 4)
	if err != nil {
		t.Fatal(err)
	}
	origin := w.Provinces[0]
	if origin.Q != 0 || origin.R != 0 || origin.Terrain != Mountain {
		t.Errorf("origin province = %+v, want {0 0 Mountain}", origin)
	}
	// Mountain must not appear anywhere else (it is not on the 1d6 table).
	for _, p := range w.Provinces[1:] {
		if p.Terrain == Mountain {
			t.Errorf("province %v is Mountain but is not the origin", p)
		}
		if p.Terrain < Plains || p.Terrain > Badlands {
			t.Errorf("province %v has terrain outside the 1d6 table: %v", p, p.Terrain)
		}
	}
}

func TestGenerateWorldIsDeterministic(t *testing.T) {
	seeds := Seeds{Seed1: 7, Seed2: 13}
	a, err := GenerateWorld(seeds, 6)
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateWorld(seeds, 6)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Error("two generations with the same seeds differ")
	}
}

func TestDifferentSeedsDifferentWorlds(t *testing.T) {
	a, _ := GenerateWorld(Seeds{Seed1: 1, Seed2: 2}, 6)
	b, _ := GenerateWorld(Seeds{Seed1: 1, Seed2: 3}, 6)
	if reflect.DeepEqual(a.Provinces, b.Provinces) {
		t.Error("different seeds produced identical terrain")
	}
}

func TestTerrainStreamIsPositionKeyed(t *testing.T) {
	// A province's terrain depends only on the seeds and its coordinates, not on
	// how many rings were generated around it.
	small, _ := GenerateWorld(Seeds{Seed1: 5, Seed2: 8}, 3)
	large, _ := GenerateWorld(Seeds{Seed1: 5, Seed2: 8}, 9)

	terrainAt := func(w *World) map[Hex]Terrain {
		m := make(map[Hex]Terrain, len(w.Provinces))
		for _, p := range w.Provinces {
			m[Hex{Q: p.Q, R: p.R}] = p.Terrain
		}
		return m
	}
	big := terrainAt(large)
	for h, terr := range terrainAt(small) {
		if big[h] != terr {
			t.Errorf("province %v: terrain %v in 3-ring world but %v in 9-ring world", h, terr, big[h])
		}
	}
}
