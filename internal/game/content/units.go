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

// MaxDoomsdayRigs returns how many Doomsday Rigs a player at the given
// Outpost level may own at once. Phase 7 rebalance: the old hard cap of
// exactly 1 made the unit feel irrelevant at scale (a single 500-rating
// unit is negligible next to a garrison of thousands of Soldiers), so
// ownership now grows with Outpost level instead - one Rig baseline,
// plus one more every 5 levels, capped at 10 so it never becomes a
// second army in its own right.
func MaxDoomsdayRigs(outpostLevel int) int {
	if outpostLevel < 1 {
		outpostLevel = 1
	}
	max := 1 + outpostLevel/5
	if max > 10 {
		max = 10
	}
	return max
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
		AttackRating:    650.0,
		Cost:            map[string]float64{"metal": 25000.0, "crystal": 7200.0, "neuro_cores": 500.0},
		DeconstructRate: 0.25,
		Flavor:          "☠️👑 THE ultimate superweapon. A single Doomsday Rig outweighs an entire fleet - but the cost is catastrophic, and losing one in a failed raid is devastating. Ownership scales with Outpost level - see /factory for your current cap.",
	},
	{
		Key:             "liberator",
		Column:          "liberators",
		Emoji:           "🦅",
		Title:           "Liberator",
		Role:            RoleCapital,
		AttackRating:    35.0,
		Cost:            map[string]float64{"metal": 1800.0, "crystal": 150.0},
		DeconstructRate: 0.40,
		Flavor:          "🦅 Mid-tier capital gunship - the accessible stepping stone between Bombers and the Battlecruiser. Flat, reliable attack rating with no situational bonus.",
	},
	{
		Key:             "wraith",
		Column:          "wraiths",
		Emoji:           "👻",
		Title:           "Wraith",
		Role:            RoleAntiAir,
		AttackRating:    25.0,
		Cost:            map[string]float64{"metal": 900.0, "crystal": 80.0},
		DeconstructRate: 0.40,
		Flavor:          "👻 Stealth strike fighter. Its cloaking field partially blinds the target's Defense Grid, weakening the turret-derived defense bonus on any raid it's part of.",
	},
	{
		Key:             "observer",
		Column:          "observers",
		Emoji:           "👁️",
		Title:           "Observer",
		Role:            RoleRecon,
		AttackRating:    0.0,
		Cost:            map[string]float64{"metal": 250.0, "crystal": 20.0},
		DeconstructRate: 0.40,
		Flavor:          "👁️ Garrison-only recon satellite. Stronger than a Scout Walker - extends early-warning range and boosts counter-espionage odds when stationed at home.",
	},
	{
		Key:             "guardian",
		Column:          "guardians",
		Emoji:           "🛡️🤖",
		Title:           "Guardian",
		Role:            RoleArmor,
		AttackRating:    0.0,
		Cost:            map[string]float64{"metal": 1400.0, "crystal": 60.0},
		DeconstructRate: 0.40,
		Flavor:          "🛡️🤖 Garrison-only heavy defense walker. Adds directly to defense rating and hard-counters Bombers - the higher your Guardian count, the less a Bomber-heavy raid gains from your Defense Grid's fortification.",
	},
	{
		Key:             "piercing_missile",
		Column:          "piercing_missiles",
		Emoji:           "🎯☢️",
		Title:           "Piercing Missile",
		Role:            RoleSiege,
		AttackRating:    0.0,
		Cost:            map[string]float64{"metal": 3200.0, "crystal": 620.0},
		DeconstructRate: 0.25,
		Flavor:          "🎯☢️ Silo-launched warhead built to punch through defenses rather than raid strength - it strips Defense Grid turret levels directly and is far harder for an Anti-Missile Battery to intercept than a standard Nuke.",
	},
	{
		Key:             "cargo_mk1",
		Column:          "cargo_mk1",
		Emoji:           "🚚",
		Title:           "Cargo Ship Mk I",
		Role:            RoleRecon,
		AttackRating:    0.0,
		Cost:            map[string]float64{"metal": 400.0},
		DeconstructRate: 0.40,
		Flavor:          "🚚 Entry-tier logistics hauler. Further reduces the return-march loot weight penalty on top of a Resource Hauler.",
	},
	{
		Key:             "cargo_mk2",
		Column:          "cargo_mk2",
		Emoji:           "🚚🚚",
		Title:           "Cargo Ship Mk II",
		Role:            RoleRecon,
		AttackRating:    0.0,
		Cost:            map[string]float64{"metal": 900.0, "crystal": 40.0},
		DeconstructRate: 0.40,
		Flavor:          "🚚🚚 Mid-tier logistics hauler. Substantially reduces the return-march loot weight penalty.",
	},
	{
		Key:             "cargo_mk3",
		Column:          "cargo_mk3",
		Emoji:           "🚚🚚🚚",
		Title:           "Cargo Ship Mk III",
		Role:            RoleRecon,
		AttackRating:    0.0,
		Cost:            map[string]float64{"metal": 1800.0, "crystal": 100.0},
		DeconstructRate: 0.40,
		Flavor:          "🚚🚚🚚 Top-tier logistics hauler. Massively reduces the return-march loot weight penalty - the backbone of a serious plunder run.",
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
