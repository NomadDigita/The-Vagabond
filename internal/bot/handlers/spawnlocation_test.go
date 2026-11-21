package handlers

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/engine/world"
)

func TestContinentCoordinate_QuadrantMatchesConvention(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	cases := []struct {
		continent string
		xPositive bool
		yPositive bool
	}{
		{"Africa", true, true},
		{"Europe", false, true},
		{"Asia", true, false},
		{"Americas", false, false},
	}
	for _, tc := range cases {
		for i := 0; i < 50; i++ {
			x, y := continentCoordinate(r, tc.continent)
			if (x > 0) != tc.xPositive {
				t.Fatalf("%s: expected x positive=%v, got x=%d", tc.continent, tc.xPositive, x)
			}
			if (y > 0) != tc.yPositive {
				t.Fatalf("%s: expected y positive=%v, got y=%d", tc.continent, tc.yPositive, y)
			}
		}
	}
}

func TestRandomContinent_AlwaysValid(t *testing.T) {
	valid := make(map[string]bool, len(world.Continents))
	for _, c := range world.Continents {
		valid[c] = true
	}
	r := rand.New(rand.NewSource(2))
	for i := 0; i < 100; i++ {
		got := randomContinent(r)
		if !valid[got] {
			t.Fatalf("randomContinent returned %q, not one of world.Continents %v", got, world.Continents)
		}
	}
}

func TestFlavorLocation_Deterministic(t *testing.T) {
	town1, country1 := flavorLocation(123, 456, "Africa")
	town2, country2 := flavorLocation(123, 456, "Africa")
	if town1 != town2 || country1 != country2 {
		t.Fatalf("flavorLocation not deterministic for (123, 456, Africa): got (%q, %q) then (%q, %q)", town1, country1, town2, country2)
	}
}

func TestFlavorLocation_UnknownContinentFallback(t *testing.T) {
	town, country := flavorLocation(1, 1, "Nowhere")
	if town == "" {
		t.Fatalf("expected a non-empty town even for an unrecognized continent, got empty")
	}
	if country == "" {
		t.Fatalf("expected a non-empty fallback country for an unrecognized continent, got empty")
	}
}

func TestFlavorCountriesByContinent_CoversEveryContinent(t *testing.T) {
	for _, c := range world.Continents {
		if len(flavorCountriesByContinent[c]) == 0 {
			t.Fatalf("flavorCountriesByContinent has no entries for continent %q", c)
		}
	}
}

func TestOutpostNameRegex_ValidatesConsistentlyWithRename(t *testing.T) {
	valid := []string{"Ashford", "Camp 7", "New-Haven", "Outpost123"}
	invalid := []string{"Bad<Name>", "Emoji😀Camp", "Semi;colon", "Quote\"Mark"}

	for _, name := range valid {
		if !outpostNameRegex.MatchString(name) {
			t.Fatalf("expected %q to be a valid outpost name", name)
		}
	}
	for _, name := range invalid {
		if outpostNameRegex.MatchString(name) {
			t.Fatalf("expected %q to be rejected as an invalid outpost name", name)
		}
	}
}

func TestLocationDescriptor_NoRawHTMLFromUserInput(t *testing.T) {
	for _, continent := range world.Continents {
		desc := locationDescriptor(1, 1, continent)
		if strings.Contains(desc, "&amp;") || strings.Contains(desc, "&lt;") {
			t.Fatalf("locationDescriptor(%q) unexpectedly contained escaped entities: %q", continent, desc)
		}
	}
}
