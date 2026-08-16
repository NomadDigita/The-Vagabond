// Package gear is NEXT_GENERATION_FEATURES_PLAN.md Phase 3: Crafting &
// Gear. Definitions (which gear exists, its slot/rarity/cost/bonus)
// are static Go data, matching the same convention every other
// definition catalog in this codebase uses (progression.Achievements,
// weather.go's eventPool, relicconvoys.go's relicNames) - only
// per-player ownership and the crafting queue are persisted.
//
// Scope note on gear's effect (see NEXT_GENERATION_FEATURES_PLAN.md's
// own Phase 3 status line for the full writeup): equipped gear grants
// a passive resource-generation bonus, applied at the exact same
// call site internal/engine/resource/resource.go already uses for
// Seasonal Wasteland Blooms and every other generation multiplier -
// deliberately NOT wired into internal/engine/tick's raid-combat power
// calculation. That calculation is large, deeply interconnected (hero
// superpowers, capital-unit tech multipliers, road-combat power) and
// this project has an established precedent of not touching it
// directly for additive features (see factiondefeat.go's own
// documented rationale for staying out of resolveRaidCombats). A
// resource-generation bonus is a fully real, felt stat modifier -
// just a safer one to integrate than a raw combat-power edit.
package gear

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
)

// Querier is satisfied by both *sql.DB and *sql.Tx, matching the same
// convention progression.Querier/notifications.querier/storagecap.Queryer
// already establish.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type Slot string

const (
	SlotWeapon  Slot = "weapon"
	SlotArmor   Slot = "armor"
	SlotUtility Slot = "utility"
)

type Rarity string

const (
	RarityCommon    Rarity = "common"
	RarityRare      Rarity = "rare"
	RarityEpic      Rarity = "epic"
	RarityLegendary Rarity = "legendary"
)

// Definition is one craftable or drop-only item. ResourceBonusPct is
// the passive resource-generation bonus (e.g. 3.0 = +3%) it grants
// while equipped - see the package doc comment above for why this,
// not combat power, is what "stat modifier" means for gear in this
// build. Craftable gear (Common/Rare) has a real CraftCost/CraftMinutes;
// drop-only gear (Epic/Legendary) has both left at their zero values
// and is never offered in the Craft sub-panel.
type Definition struct {
	Key              string
	Name             string
	Icon             string
	Slot             Slot
	Rarity           Rarity
	ResourceBonusPct float64
	Craftable        bool
	CraftCost        map[string]float64 // resource column -> amount
	CraftMinutes     float64
}

// Catalog is the full launch gear set - one Common, one Rare (both
// craftable), one Epic, and one Legendary (both drop-only, see
// GrantRandomDrop) per slot. A future phase can grow this catalog as
// pure data with no code changes, the same property
// progression.Achievements/Quests were built around.
var Catalog = []Definition{
	// --- Weapon ---
	{Key: "scrap_blade", Name: "Scrap Blade", Icon: "🔪", Slot: SlotWeapon, Rarity: RarityCommon, ResourceBonusPct: 2, Craftable: true, CraftCost: map[string]float64{"scrap": 500}, CraftMinutes: 15},
	{Key: "salvaged_rifle", Name: "Salvaged Rifle", Icon: "🔫", Slot: SlotWeapon, Rarity: RarityRare, ResourceBonusPct: 5, Craftable: true, CraftCost: map[string]float64{"scrap": 1500, "metal": 500}, CraftMinutes: 60},
	{Key: "arc_cannon", Name: "Arc Cannon", Icon: "⚡", Slot: SlotWeapon, Rarity: RarityEpic, ResourceBonusPct: 9},
	{Key: "void_reaver", Name: "Void Reaver", Icon: "🌌", Slot: SlotWeapon, Rarity: RarityLegendary, ResourceBonusPct: 15},

	// --- Armor ---
	{Key: "patched_vest", Name: "Patched Vest", Icon: "🦺", Slot: SlotArmor, Rarity: RarityCommon, ResourceBonusPct: 2, Craftable: true, CraftCost: map[string]float64{"scrap": 500}, CraftMinutes: 15},
	{Key: "plated_harness", Name: "Plated Harness", Icon: "🛡️", Slot: SlotArmor, Rarity: RarityRare, ResourceBonusPct: 5, Craftable: true, CraftCost: map[string]float64{"scrap": 1500, "metal": 500}, CraftMinutes: 60},
	{Key: "crystal_weave", Name: "Crystal-Weave Plating", Icon: "💠", Slot: SlotArmor, Rarity: RarityEpic, ResourceBonusPct: 9},
	{Key: "aegis_core", Name: "Aegis Core", Icon: "🌟", Slot: SlotArmor, Rarity: RarityLegendary, ResourceBonusPct: 15},

	// --- Utility ---
	{Key: "scavenger_pack", Name: "Scavenger's Pack", Icon: "🎒", Slot: SlotUtility, Rarity: RarityCommon, ResourceBonusPct: 2, Craftable: true, CraftCost: map[string]float64{"scrap": 500}, CraftMinutes: 15},
	{Key: "signal_booster", Name: "Signal Booster", Icon: "📡", Slot: SlotUtility, Rarity: RarityRare, ResourceBonusPct: 5, Craftable: true, CraftCost: map[string]float64{"scrap": 1500, "crystal": 200}, CraftMinutes: 60},
	{Key: "ether_conduit", Name: "Ether Conduit", Icon: "🔮", Slot: SlotUtility, Rarity: RarityEpic, ResourceBonusPct: 9},
	{Key: "singularity_core", Name: "Singularity Core", Icon: "🕳️", Slot: SlotUtility, Rarity: RarityLegendary, ResourceBonusPct: 15},
}

// ByKey looks up a catalog Definition, or (zero-value, false) if
// gearKey doesn't match anything - a deliberately safe result, since
// gearKey most often arrives as caller-controlled text (a callback
// arg, a player_gear row that could in theory outlive a catalog
// change).
func ByKey(gearKey string) (Definition, bool) {
	for _, d := range Catalog {
		if d.Key == gearKey {
			return d, true
		}
	}
	return Definition{}, false
}

// ByRarity returns every catalog Definition of the given rarity - used
// by GrantRandomDrop's own random pick.
func ByRarity(r Rarity) []Definition {
	var out []Definition
	for _, d := range Catalog {
		if d.Rarity == r {
			out = append(out, d)
		}
	}
	return out
}

// StartCraft begins crafting one unit of gearKey for campID: validates
// it's a real, Craftable definition, deducts CraftCost (failing
// cleanly on insufficient resources, same "check then deduct in one
// transaction" pattern doPostListing/doBuyListingQty already
// establish), and inserts a crafting_queue row due
// CraftMinutes from now. ResolveCraftingQueue (the tick phase) credits
// the finished item later - crafting is never instant, matching the
// plan doc's explicit "spend resources up front, wait, tick phase
// resolves completion" design.
func StartCraft(ctx context.Context, q Querier, campID, gearKey string) (string, error) {
	def, ok := ByKey(gearKey)
	if !ok || !def.Craftable {
		return "❌ That item can't be crafted.", fmt.Errorf("gear %s is not craftable", gearKey)
	}

	for resource, amount := range def.CraftCost {
		var current float64
		if err := q.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1", resource), campID).Scan(&current); err != nil {
			return "⚠️ Error reading resources.", err
		}
		if current < amount {
			return fmt.Sprintf("❌ Insufficient %s! Need %.0f.", resource, amount), fmt.Errorf("insufficient %s", resource)
		}
	}
	for resource, amount := range def.CraftCost {
		if _, err := q.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = %s - $1 WHERE encampment_id = $2", resource, resource), amount, campID); err != nil {
			return "⚠️ Error deducting resources.", err
		}
	}

	if _, err := q.ExecContext(ctx, `
		INSERT INTO crafting_queue (encampment_id, gear_key, completes_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP + ($3 * INTERVAL '1 minute'))`,
		campID, gearKey, def.CraftMinutes); err != nil {
		return "⚠️ Error queuing craft.", err
	}

	return fmt.Sprintf("🔨 %s %s queued - ready in %.0f minutes.", def.Icon, def.Name, def.CraftMinutes), nil
}

// ResolveCraftingQueue is the "crafting_resolve" tick phase - credits
// every crafting_queue row whose completes_at has passed and isn't yet
// marked complete, upserting the finished item into player_gear and
// notifying the owner.
func ResolveCraftingQueue(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT cq.id, cq.encampment_id, cq.gear_key, e.user_id
		FROM crafting_queue cq
		JOIN encampments e ON e.id = cq.encampment_id
		WHERE cq.is_completed = FALSE AND cq.completes_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("querying completed crafting queue: %w", err)
	}
	type done struct {
		id, campID, gearKey string
		userID              int64
	}
	var toResolve []done
	for rows.Next() {
		var d done
		if scanErr := rows.Scan(&d.id, &d.campID, &d.gearKey, &d.userID); scanErr == nil {
			toResolve = append(toResolve, d)
		}
	}
	rows.Close()

	for _, d := range toResolve {
		if _, err := tx.ExecContext(ctx, "UPDATE crafting_queue SET is_completed = TRUE WHERE id = $1", d.id); err != nil {
			return fmt.Errorf("marking craft %s complete: %w", d.id, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO player_gear (encampment_id, gear_key, quantity)
			VALUES ($1, $2, 1)
			ON CONFLICT (encampment_id, gear_key) DO UPDATE SET quantity = player_gear.quantity + 1`,
			d.campID, d.gearKey); err != nil {
			return fmt.Errorf("crediting crafted gear %s: %w", d.gearKey, err)
		}
		def, _ := ByKey(d.gearKey)
		if d.userID > 0 {
			msg := fmt.Sprintf("🔨✅ CRAFTING COMPLETE: %s %s is ready! Equip it from /gear.", def.Icon, def.Name)
			_ = notifications.Queue(ctx, tx, d.userID, msg, "general")
		}
	}
	return nil
}

// Equip sets gearKey as the equipped item in its slot for campID,
// unequipping whatever else was in that slot - a player owns at most
// one equipped item per slot at a time (Weapon/Armor/Utility), same
// three-slot model the plan doc's own design describes. Fails if the
// player doesn't own at least one unit of gearKey.
func Equip(ctx context.Context, q Querier, campID, gearKey string) error {
	def, ok := ByKey(gearKey)
	if !ok {
		return fmt.Errorf("unknown gear %s", gearKey)
	}
	var quantity int
	if err := q.QueryRowContext(ctx, "SELECT quantity FROM player_gear WHERE encampment_id = $1 AND gear_key = $2", campID, gearKey).Scan(&quantity); err != nil || quantity <= 0 {
		return fmt.Errorf("you don't own %s", def.Name)
	}

	// Unequip anything else currently in this slot - one query using
	// the catalog's own slot membership rather than a slot column on
	// player_gear, so the catalog stays the single source of truth for
	// which items belong to which slot.
	var sameSlotKeys []string
	for _, d := range Catalog {
		if d.Slot == def.Slot {
			sameSlotKeys = append(sameSlotKeys, d.Key)
		}
	}
	if _, err := q.ExecContext(ctx, `
		UPDATE player_gear SET equipped = FALSE
		WHERE encampment_id = $1 AND gear_key = ANY($2::text[])`, campID, pqStringArray(sameSlotKeys)); err != nil {
		return err
	}

	_, err := q.ExecContext(ctx, "UPDATE player_gear SET equipped = TRUE WHERE encampment_id = $1 AND gear_key = $2", campID, gearKey)
	return err
}

// GrantRandomDrop credits one random Definition of the given rarity to
// campID (used for Epic/Legendary World Boss and Relic Convoy drops -
// see engine.go's payoutWorldBossLoot and relicconvoy.go's
// doClaimRelicConvoy) and notifies the owner. Returns the granted
// Definition so the caller can fold its name into its own reward
// message if it wants to.
func GrantRandomDrop(ctx context.Context, q Querier, campID string, rarity Rarity) (Definition, error) {
	pool := ByRarity(rarity)
	if len(pool) == 0 {
		return Definition{}, fmt.Errorf("no gear defined for rarity %s", rarity)
	}
	def := pool[rand.Intn(len(pool))]

	if _, err := q.ExecContext(ctx, `
		INSERT INTO player_gear (encampment_id, gear_key, quantity)
		VALUES ($1, $2, 1)
		ON CONFLICT (encampment_id, gear_key) DO UPDATE SET quantity = player_gear.quantity + 1`,
		campID, def.Key); err != nil {
		return def, err
	}

	var userID int64
	_ = q.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", campID).Scan(&userID)
	if userID > 0 {
		msg := fmt.Sprintf("✨🎁 RARE DROP: %s %s (%s)! Equip it from /gear.", def.Icon, def.Name, string(def.Rarity))
		_ = notifications.Queue(ctx, q, userID, msg, "general")
	}
	return def, nil
}

// ResourceBonusPctFor sums the ResourceBonusPct of every currently
// equipped item for campID - the value
// internal/engine/resource/resource.go reads at the same call site
// Phase 2's Bloom multiplier already uses.
func ResourceBonusPctFor(ctx context.Context, q Querier, campID string) float64 {
	rows, err := q.QueryContext(ctx, "SELECT gear_key FROM player_gear WHERE encampment_id = $1 AND equipped = TRUE", campID)
	if err != nil {
		return 0
	}
	defer rows.Close()
	var total float64
	for rows.Next() {
		var key string
		if scanErr := rows.Scan(&key); scanErr == nil {
			if def, ok := ByKey(key); ok {
				total += def.ResourceBonusPct
			}
		}
	}
	return total
}

// AffordCraft reports whether campID currently has enough resources to
// start crafting def - used by the /gear panel to show affordability
// inline, matching the exchange panel's own established
// affordability-preview pattern, without actually spending anything.
func AffordCraft(ctx context.Context, q Querier, campID string, def Definition) bool {
	for resource, amount := range def.CraftCost {
		var current float64
		if err := q.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1", resource), campID).Scan(&current); err != nil || current < amount {
			return false
		}
	}
	return true
}

// pqStringArray formats a Go string slice as a Postgres text[] array
// literal for use with = ANY($n) - a small local helper rather than
// pulling in the lib/pq Array helper type just for this one call site.
func pqStringArray(ss []string) string {
	out := "{"
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += `"` + s + `"`
	}
	return out + "}"
}
