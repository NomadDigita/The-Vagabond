package handlers

import (
	"reflect"
	"testing"
)

func TestCraftSpecFor_CoversEveryButtonOnBothPanels(t *testing.T) {
	// Every item key any "Recruit"/"Forge"/"Build"/"Craft" button on
	// HandleRecruitPanel or HandleVehiclesPanel actually sends as its
	// craft_item callback data - if a button's key isn't resolvable here,
	// tapping it silently fails with "❌ Unknown item." This test is the
	// guard against that drifting out of sync as buttons/specs evolve.
	keys := []string{
		"soldier", "drone", "mech", "nuke", "destroyer", "bomber", "scout", "battlecruiser", "deathstar",
		"liberator", "wraith", "observer", "guardian", "piercing_missile",
		"buggy", "ship", "cargo_jet", "jet", "hauler", "tanker", "rig", "cargo_mk1", "cargo_mk2", "cargo_mk3",
	}
	for _, k := range keys {
		spec, ok := craftSpecFor(k)
		if !ok {
			t.Errorf("craftSpecFor(%q) not found - a button sends this key and would silently fail", k)
			continue
		}
		if spec.dbColumn == "" {
			t.Errorf("craftSpecFor(%q) has an empty dbColumn", k)
		}
		if spec.maxPerOrder < 1 {
			t.Errorf("craftSpecFor(%q) has maxPerOrder %d, must be at least 1", k, spec.maxPerOrder)
		}
		if len(spec.cost) == 0 {
			t.Errorf("craftSpecFor(%q) has no cost entries", k)
		}
	}
}

func TestCraftSpecFor_CargoJetAndJetAreTheSameSpec(t *testing.T) {
	cargoJet, ok1 := craftSpecFor("cargo_jet")
	jet, ok2 := craftSpecFor("jet")
	if !ok1 || !ok2 {
		t.Fatalf("expected both aliases to resolve, got ok=%v,%v", ok1, ok2)
	}
	if cargoJet.dbColumn != jet.dbColumn || cargoJet.dbColumn != "jets" {
		t.Errorf("expected cargo_jet and jet to both target the 'jets' column, got %q and %q", cargoJet.dbColumn, jet.dbColumn)
	}
}

func TestCraftSpecFor_UnknownItemFails(t *testing.T) {
	if _, ok := craftSpecFor("not_a_real_unit"); ok {
		t.Error("expected an unknown item key to fail resolution")
	}
}

// TestCraftSpecFor_BulkCeilings is the actual "up to 20x, except for some
// units" policy check - locks in exactly which units get the full 20x
// ceiling and which are deliberately capped lower, so a future edit that
// accidentally removes deathstar/nuke's special case (e.g. a careless
// find-replace) is caught immediately instead of silently allowing a
// 20x Nuclear Device order.
func TestCraftSpecFor_BulkCeilings(t *testing.T) {
	cases := map[string]int{
		"soldier":   20,
		"mech":      20,
		"destroyer": 20,
		"scout":     20,
		"liberator": 20,
		"cargo_mk1": 20,
		"nuke":      3, // extreme power-per-unit, see hardcodedCraftSpecs' comment
		"deathstar": 3, // total-fleet level-cap makes a high per-order ceiling meaningless/misleading
	}
	for item, want := range cases {
		spec, ok := craftSpecFor(item)
		if !ok {
			t.Fatalf("craftSpecFor(%q) not found", item)
		}
		if spec.maxPerOrder != want {
			t.Errorf("craftSpecFor(%q).maxPerOrder = %d, want %d", item, spec.maxPerOrder, want)
		}
	}
}

func TestQuantityOptions(t *testing.T) {
	cases := []struct {
		max  int
		want []int
	}{
		{20, []int{1, 5, 10, 20}},
		{3, []int{1, 3}},
		{1, []int{1}},
		{10, []int{1, 5, 10}},
	}
	for _, tc := range cases {
		got := quantityOptions(tc.max)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("quantityOptions(%d) = %v, want %v", tc.max, got, tc.want)
		}
	}
}

func TestFormatCraftCost(t *testing.T) {
	got := formatCraftCost(map[string]float64{"metal": 1000, "crystal": 70})
	want := "🔩1000 Metal, 🔮70 Crystal"
	if got != want {
		t.Errorf("formatCraftCost = %q, want %q", got, want)
	}

	if got := formatCraftCost(map[string]float64{}); got != "free" {
		t.Errorf("formatCraftCost(empty) = %q, want %q", got, "free")
	}

	// Zero-amount entries (a resource key present but costing 0) must not
	// render as "0 Metal" clutter.
	got = formatCraftCost(map[string]float64{"metal": 0, "crystal": 5})
	if got != "🔮5 Crystal" {
		t.Errorf("formatCraftCost with a zero entry = %q, want %q", got, "🔮5 Crystal")
	}
}
