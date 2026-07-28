package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/NomadDigita/The-Vagabond/internal/engine/world"
)

// continentCoordinate rolls a random (x, y) inside the correct quadrant
// for a given continent, matching the quadrant convention established at
// onboarding: Africa is +x+y, Europe -x+y, Asia +x-y, Americas -x-y.
// Every per-continent system in the game (world events/weather via
// internal/engine/world.Continents, and anything built on top of it
// later) assumes a base's region string is always one of exactly these
// four - see allocateCoordinate's doc comment for why that matters.
func continentCoordinate(r *rand.Rand, continent string) (x, y int) {
	switch continent {
	case "Africa":
		return r.Intn(991) + 10, r.Intn(991) + 10
	case "Europe":
		return -(r.Intn(991) + 10), r.Intn(991) + 10
	case "Asia":
		return r.Intn(991) + 10, -(r.Intn(991) + 10)
	default: // Americas
		return -(r.Intn(991) + 10), -(r.Intn(991) + 10)
	}
}

// allocateCoordinate rolls a fresh (x, y) for the given continent,
// retrying on the rare (x, y) collision, and returns the new
// coordinates row's UUID ready to attach to an encampment.
//
// continent MUST be one of world.Continents. Passing anything else
// (the literal string "Unknown Sector" was the previous bug in
// /newjobteleport and /ghostprotocol) silently excludes that base from
// every per-continent system forever - weather events are looked up by
// exact continent name, so a base with a bogus region just never
// matches any of them again.
//
// seed lets callers vary determinism: onboarding seeds per-player (a
// stable, evenly-spread initial placement), while /newjobteleport and
// /ghostprotocol want a fresh independent reroll each time, so they
// pass a time-based seed instead.
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

// randomContinent picks one of the four playable continents uniformly
// at random, for callers (teleport, Ghost Protocol) that want a genuine
// reroll rather than onboarding's per-player deterministic spread.
func randomContinent(r *rand.Rand) string {
	return world.Continents[r.Intn(len(world.Continents))]
}

// flavorTowns and flavorCountriesByContinent back flavorLocation below.
// These are deliberately fictional (no real-world country names), to
// keep the tone consistent with the game's post-collapse setting.
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

// flavorLocation deterministically derives a cosmetic town + country
// pair for a coordinate. It's a pure function of (x, y) rather than a
// stored value - deliberately not a new DB column - so the same
// coordinate always describes itself the same way on every screen
// (onboarding, the returning-player dashboard, teleport, Ghost
// Protocol) without a migration or any risk of the display drifting
// from the stored coordinate.
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

// locationDescriptor renders the standard "you are here" line used
// everywhere a player's base location is shown. HTML-safe as-is: every
// input is either a fixed continent name or drawn from the fixed
// flavorTowns/flavorCountriesByContinent pools above, never player-
// authored text, so nothing here needs htmlEscape.
func locationDescriptor(x, y int, continent string) string {
	town, country := flavorLocation(x, y, continent)
	return fmt.Sprintf("%s, %s (%s Territory)", htmlBold(town), htmlCode(country), continent)
}
