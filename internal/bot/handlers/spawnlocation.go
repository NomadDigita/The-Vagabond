package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/NomadDigita/The-Vagabond/internal/engine/world"
)

func continentCoordinate(r *rand.Rand, continent string) (x, y int) {
	switch continent {
	case "Africa":
		return r.Intn(991) + 10, r.Intn(991) + 10
	case "Europe":
		return -(r.Intn(991) + 10), r.Intn(991) + 10
	case "Asia":
		return r.Intn(991) + 10, -(r.Intn(991) + 10)
	default:
		return -(r.Intn(991) + 10), -(r.Intn(991) + 10)
	}
}

func allocateCoordinate(ctx context.Context, tx *sql.Tx, seed int64, continent string) (coordID string, x, y int, err error) {
	r := rand.New(rand.NewSource(seed))
	biomes := []string{"wasteland", "irradiated_zone", "scrapyard", "ashfields", "frozen_tundra", "ruins"}
	terrains := []string{"flat", "mountainous", "coastal", "urban_ruins"}

	for attempt := 0; attempt < 15; attempt++ {
		x, y = continentCoordinate(r, continent)
		biome := biomes[r.Intn(len(biomes))]
		terrain := terrains[r.Intn(len(terrains))]
		err = tx.QueryRowContext(ctx, `
			INSERT INTO coordinates (x, y, biome, danger_level, region, terrain)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (x, y) DO NOTHING
			RETURNING id`, x, y, biome, r.Intn(5)+1, continent, terrain).Scan(&coordID)
		if err == nil {
			return coordID, x, y, nil
		}
	}
	return "", 0, 0, fmt.Errorf("allocateCoordinate: exhausted retries for continent %s", continent)
}

func randomContinent(r *rand.Rand) string {
	return world.Continents[r.Intn(len(world.Continents))]
}

var flavorTowns = []string{
	"Ashford Hollow", "Ironhaven", "Rustmoor", "Cinderfall", "Driftwood Landing",
	"Emberreach", "Graywater Crossing", "Hollow Creek", "Ashen Vale", "Scraptown",
	"Copperbrook", "Thistledown", "Wrenfield", "Blackpine Hollow", "Sunken Marsh",
	"Farrow's End", "Millstone Junction", "Bramblewick", "Windmere", "Duskford",
	"Stonegate", "Emberfall Reach", "Coldharbor", "Saltmarsh Landing", "Thornbury",
}

var flavorCountriesByContinent = map[string][]string{
	"Africa":   {"the Sahel Compact", "the Kalahari Reaches", "the Nile Remnant", "the Sun-Scoured Union", "New Serengeti", "the Zambezi Territories"},
	"Europe":   {"the Old Rhine Territories", "the Alpine Remnant", "the North Sea Compact", "New Iberia", "the Danube Reaches", "the Carpathian Union"},
	"Asia":     {"the Steppe Union", "the Monsoon Remnant", "the Himalayan Reaches", "New Indochina", "the Gobi Compact", "the Mekong Territories"},
	"Americas": {"the Rust Belt Union", "the Amazon Remnant", "the Prairie Compact", "New Patagonia", "the Rocky Reaches", "the Great Lakes Territories"},
}

func flavorLocation(x, y int, continent string) (town, country string) {
	seed := int64(x)*1_000_003 + int64(y)*97 + 17
	r := rand.New(rand.NewSource(seed))
	town = flavorTowns[r.Intn(len(flavorTowns))]
	countries := flavorCountriesByContinent[continent]
	if len(countries) == 0 {
		return town, "an uncharted territory"
	}
	country = countries[r.Intn(len(countries))]
	return town, country
}

func locationDescriptor(x, y int, continent string) string {
	town, country := flavorLocation(x, y, continent)
	return fmt.Sprintf("%s, %s (%s Territory)", htmlBold(town), htmlCode(country), continent)
}
