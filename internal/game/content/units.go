// Package content is the canonical, single-source-of-truth registry for
// game content: unit stats, research nodes, and building definitions. It
// holds pure data only - no database access, no Telegram/telebot
// dependency - so it can be safely imported by handlers, the tick engine,
// and (eventually) any tooling/tests without pulling in unrelated deps.
//
// This package is the SpaceHunt-revival content spine: every new
// fleet unit, research node, or building added going forward should be
// defined here first, then wired into the relevant handler/engine code -
// rather than duplicating stat tables inline like the earliest slices did.
package content

// UnitRole describes what combat role a unit fills, used by the combat
// engine to apply the right situational bonuses (e.g. anti-air, siege).
type UnitRole string

const (
	RoleInfantry UnitRole = "infantry" // baseline ground troop
	RoleArmor    UnitRole = "armor"    // heavy armored attacker
	RoleAntiAir  UnitRole = "anti_air" // hard counter vs drones/jets
	RoleSiege    UnitRole = "siege"    // hard counter vs turrets/buildings
	RoleRecon    UnitRole = "recon"    // scouting/utility, minimal combat power
	RoleCapital  UnitRole = "capital"  // top-tier, expensive, high-impact
)

// Unit is the canonical definition of one fleet/roster unit.
type Unit struct {
	Key             string             // stable identifier, matches workshop_inventory column
	Column          string             // workshop_inventory column name (usually == Key + "s")
	Emoji           string             // display glyph
	Title           string             // display name
	Role            UnitRole           // combat role
	AttackRating    float64            // base attack contribution per unit
	Cost            map[string]float64 // resource_column -> amount
	DeconstructRate float64            // fraction of Cost refunded on deconstruction
	Flavor          string             // one-line flavor/description text for panels
}

// DeconstructRefund returns the resource_column -> amount map refunded when
// a single unit of this type is scrapped.
func (u Unit) DeconstructRefund() map[string]float64 {
	refund := make(map[string]float64, len(u.Cost))
	for res, amt := range u.Cost {
		refund[res] = amt * u.DeconstructRate
	}
	return refund
}

// Units is the full canonical fleet roster. Existing units (soldier,
// drone, mech, nuke, buggy, ship, jet, hauler, tanker, rig, destroyer,
// bomber) are already wired into factory.go/deconstruct.go/combat.go
// directly and are NOT yet migrated to read from here, to avoid touching
// already-shipped, tested code paths without a functional reason. New
// units from this point forward are defined here first.
var Units = []Unit{
	{
		Key:             "scout",
		Column:          "scouts",
		Emoji:           "🛵",
		Title:           "Scout Walker",
		Role:            RoleRecon,
		AttackRating:    2.0,
		Cost:            map[string]float64{"metal": 70.0},
		DeconstructRate: 0.40,
		Flavor:          "🔍 Cheap, fast recon walker. Extends Radar range and shortens enemy incoming-raid warning delay when garrisoned.",
	},
	{
		Key:             "battlecruiser",
		Column:          "battlecruisers",
		Emoji:           "🚢",
		Title:           "Battlecruiser",
		Role:            RoleCapital,
		AttackRating:    60.0,
		Cost:            map[string]float64{"metal": 3000.0, "crystal": 405.0},
		DeconstructRate: 0.40,
		Flavor:          "👑 Top-tier capital warship. Massive raw attack rating, but painfully expensive - the Deathstar-tier flex unit.",
	},
	{
		Key:             "deathstar",
		Column:          "deathstars",
		Emoji:           "🌑💀",
		Title:           "Doomsday Rig",
		Role:            RoleCapital,
		AttackRating:    500.0,
		Cost:            map[string]float64{"metal": 25000.0, "crystal": 7200.0, "neuro_cores": 500.0},
		DeconstructRate: 0.25,
		Flavor:          "☠️👑 THE ultimate superweapon. A single Doomsday Rig outweighs an entire fleet - but the cost is catastrophic, and losing one in a failed raid is devastating.",
	},
}

// FindUnit looks up a unit by its Key. Returns (Unit{}, false) if not found.
func FindUnit(key string) (Unit, bool) {
	for _, u := range Units {
		if u.Key == key {
			return u, true
		}
	}
	return Unit{}, false
}

// MustFindUnit is like FindUnit but panics if the key doesn't exist. Safe
// to use in package-level var initializers (e.g. other handlers building
// their own tables from this registry at startup) since a missing key
// there is a programming error, not a runtime condition to handle gracefully.
func MustFindUnit(key string) Unit {
	u, ok := FindUnit(key)
	if !ok {
		panic("content: unknown unit key: " + key)
	}
	return u
}
