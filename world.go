// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
)

// Terrain is the kind of land assigned to a province.
type Terrain int

const (
	Mountain Terrain = iota
	Plains
	Forests
	Desert
	Hills
	Lake
	Badlands
)

// terrainNames maps each terrain to its name. It is used only for display and
// JSON encoding; it is never ranged over in a way that affects PRNG draws.
var terrainNames = map[Terrain]string{
	Mountain: "Mountain",
	Plains:   "Plains",
	Forests:  "Forests",
	Desert:   "Desert",
	Hills:    "Hills",
	Lake:     "Lake",
	Badlands: "Badlands",
}

// String returns the terrain's name.
func (t Terrain) String() string {
	if name, ok := terrainNames[t]; ok {
		return name
	}
	return fmt.Sprintf("Terrain(%d)", int(t))
}

// MarshalJSON encodes the terrain as its name.
func (t Terrain) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// worldographerTiles maps each terrain to its Worldographer tile name. It is
// the source of truth for the terrain-translation.json export.
var worldographerTiles = map[Terrain]string{
	Badlands: "Classic/Other Badlands",
	Desert:   "Classic/Flat Desert Sandy",
	Forests:  "Classic/Flat Forest Deciduous Heavy",
	Hills:    "Classic/Hills",
	Lake:     "Classic/Water Sea",
	Mountain: "Classic/Mountains",
	Plains:   "Classic/Flat Farmland",
}

// TerrainTranslation returns the mapping from each terrain's name to its
// Worldographer tile name, used to import a generated world into Worldographer.
func TerrainTranslation() map[string]string {
	m := make(map[string]string, len(worldographerTiles))
	for terrain, tile := range worldographerTiles {
		m[terrain.String()] = tile
	}
	return m
}

// Province is a single hex and its assigned terrain.
type Province struct {
	Q       int     `json:"q"`
	R       int     `json:"r"`
	Terrain Terrain `json:"terrain"`
}

// World is a generated world.
type World struct {
	Seeds     Seeds      `json:"seeds"`
	Rings     int        `json:"rings"`
	Provinces []Province `json:"provinces"`
}

// terrainStreamKey names the stream used to roll a province's terrain.
const terrainStreamKey = "world.terrain"

// GenerateWorld generates a world of the given number of rings from the master
// seeds. The number of rings must satisfy 0 < rings < 100.
//
// See content/docs/reference/world-generation.md for the rules.
func GenerateWorld(seeds Seeds, rings int) (*World, error) {
	if rings <= 0 || rings >= 100 {
		return nil, fmt.Errorf("rings must be > 0 and < 100, got %d", rings)
	}

	w := &World{
		Seeds:     seeds,
		Rings:     rings,
		Provinces: make([]Province, 0, 1+3*rings*(rings+1)),
	}

	// The origin is always a mountain.
	w.Provinces = append(w.Provinces, Province{Q: 0, R: 0, Terrain: Mountain})

	// Each remaining province rolls 1d6 for its terrain from a stream keyed by
	// its own coordinates, so the result is independent of iteration order.
	for k := 1; k <= rings; k++ {
		for _, h := range Ring(k) {
			stream := seeds.Stream(terrainStreamKey, int64(h.Q), int64(h.R))
			w.Provinces = append(w.Provinces, Province{Q: h.Q, R: h.R, Terrain: rollTerrain(stream)})
		}
	}

	return w, nil
}

// rollTerrain rolls 1d6 and maps the result to a terrain, per the reference.
func rollTerrain(r *rand.Rand) Terrain {
	switch r.IntN(6) + 1 {
	case 1:
		return Plains
	case 2:
		return Forests
	case 3:
		return Desert
	case 4:
		return Hills
	case 5:
		return Lake
	case 6:
		return Badlands
	default:
		panic("unreachable: 1d6 out of range")
	}
}
