package herodepth

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
)

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type Branch string

const (
	BranchFortification Branch = "fortification"
	BranchOffense       Branch = "offense"
	BranchCommand       Branch = "command"
	BranchScavenging    Branch = "scavenging"
	BranchInfiltration  Branch = "infiltration"
	BranchSurvival      Branch = "survival"
)

type EffectType string

const (
	EffectTrainingCostPct EffectType = "training_cost_pct"
	EffectHealCostPct     EffectType = "heal_cost_pct"
	EffectResourceGenPct  EffectType = "resource_gen_pct"
)

type SkillNode struct {
	Key         string
	Name        string
	Description string
	Faction     string
	Branch      Branch
	Effect      EffectType
	Value       float64
}

var SkillTree = []SkillNode{
	{Key: "sv_fort_1", Name: "Reinforced Bunkers", Description: "-10% Hero training cost.", Faction: "steel_vanguard", Branch: BranchFortification, Effect: EffectTrainingCostPct, Value: -10},
	{Key: "sv_fort_2", Name: "Hardened Perimeter", Description: "-15% Hero healing cost.", Faction: "steel_vanguard", Branch: BranchFortification, Effect: EffectHealCostPct, Value: -15},
	{Key: "sv_fort_3", Name: "Bastion Doctrine", Description: "+3% passive resource generation.", Faction: "steel_vanguard", Branch: BranchFortification, Effect: EffectResourceGenPct, Value: 3},
	{Key: "sv_off_1", Name: "Forward Momentum", Description: "-10% Hero training cost.", Faction: "steel_vanguard", Branch: BranchOffense, Effect: EffectTrainingCostPct, Value: -10},
	{Key: "sv_off_2", Name: "Battlefield Medicine", Description: "-15% Hero healing cost.", Faction: "steel_vanguard", Branch: BranchOffense, Effect: EffectHealCostPct, Value: -15},
	{Key: "sv_off_3", Name: "Spoils of War", Description: "+3% passive resource generation.", Faction: "steel_vanguard", Branch: BranchOffense, Effect: EffectResourceGenPct, Value: 3},
	{Key: "sv_cmd_1", Name: "Officer Corps", Description: "-10% Hero training cost.", Faction: "steel_vanguard", Branch: BranchCommand, Effect: EffectTrainingCostPct, Value: -10},
	{Key: "sv_cmd_2", Name: "Field Hospitals", Description: "-15% Hero healing cost.", Faction: "steel_vanguard", Branch: BranchCommand, Effect: EffectHealCostPct, Value: -15},
	{Key: "sv_cmd_3", Name: "Logistics Mastery", Description: "+3% passive resource generation.", Faction: "steel_vanguard", Branch: BranchCommand, Effect: EffectResourceGenPct, Value: 3},

	{Key: "rn_scv_1", Name: "Efficient Salvage", Description: "-10% Hero training cost.", Faction: "rust_nomads", Branch: BranchScavenging, Effect: EffectTrainingCostPct, Value: -10},
	{Key: "rn_scv_2", Name: "Junkyard Remedies", Description: "-15% Hero healing cost.", Faction: "rust_nomads", Branch: BranchScavenging, Effect: EffectHealCostPct, Value: -15},
	{Key: "rn_scv_3", Name: "Waste Not", Description: "+3% passive resource generation.", Faction: "rust_nomads", Branch: BranchScavenging, Effect: EffectResourceGenPct, Value: 3},
	{Key: "rn_inf_1", Name: "Silent Approach", Description: "-10% Hero training cost.", Faction: "rust_nomads", Branch: BranchInfiltration, Effect: EffectTrainingCostPct, Value: -10},
	{Key: "rn_inf_2", Name: "Shadow Medic", Description: "-15% Hero healing cost.", Faction: "rust_nomads", Branch: BranchInfiltration, Effect: EffectHealCostPct, Value: -15},
	{Key: "rn_inf_3", Name: "Ghost Economy", Description: "+3% passive resource generation.", Faction: "rust_nomads", Branch: BranchInfiltration, Effect: EffectResourceGenPct, Value: 3},
	{Key: "rn_sur_1", Name: "Hardened Nomad", Description: "-10% Hero training cost.", Faction: "rust_nomads", Branch: BranchSurvival, Effect: EffectTrainingCostPct, Value: -10},
	{Key: "rn_sur_2", Name: "Wasteland Remedies", Description: "-15% Hero healing cost.", Faction: "rust_nomads", Branch: BranchSurvival, Effect: EffectHealCostPct, Value: -15},
	{Key: "rn_sur_3", Name: "Nomad Resourcefulness", Description: "+3% passive resource generation.", Faction: "rust_nomads", Branch: BranchSurvival, Effect: EffectResourceGenPct, Value: 3},
}

func NodeByKey(key string) (SkillNode, bool) {
	for _, n := range SkillTree {
		if n.Key == key {
			return n, true
		}
	}
	return SkillNode{}, false
}

func NodesForFaction(faction string) []SkillNode {
	var out []SkillNode
	for _, n := range SkillTree {
		if n.Faction == faction {
			out = append(out, n)
		}
	}
	return out
}

type StoryBeat struct {
	Level   int
	Faction string
	Text    string
}

var StoryArc = []StoryBeat{
	{Level: 5, Faction: "steel_vanguard", Text: "📖 Your commander speaks less now. The scars from the early days have taught them that survivors who talk too much don't survive long."},
	{Level: 10, Faction: "steel_vanguard", Text: "📖 A defector from a rival Clan asked to serve under your commander today. They said the name carries weight in the wasteland now."},
	{Level: 15, Faction: "steel_vanguard", Text: "📖 Your commander found an old service record - their own, from before the world ended. They don't recognize the person in it anymore."},
	{Level: 20, Faction: "steel_vanguard", Text: "📖 Younger soldiers have started calling your commander 'the Warden' behind their back. It isn't meant unkindly."},
	{Level: 25, Faction: "steel_vanguard", Text: "📖 Your commander has stopped counting the battles. What matters now, they say, is that the outpost is still standing."},

	{Level: 5, Faction: "rust_nomads", Text: "📖 Your commander salvaged something they won't talk about from the last raid. It sits wrapped in a corner of the camp."},
	{Level: 10, Faction: "rust_nomads", Text: "📖 Word has spread that your commander can find anything in the ruins. Other camps have started sending scouts to watch how they work."},
	{Level: 15, Faction: "rust_nomads", Text: "📖 Your commander finally opened the wrapped salvage from months ago. Whatever it was, they buried it quietly and never mentioned it again."},
	{Level: 20, Faction: "rust_nomads", Text: "📖 A rival scavenger crew offered your commander a place among them. They turned it down without a second thought."},
	{Level: 25, Faction: "rust_nomads", Text: "📖 Your commander doesn't call it survival anymore. They call it home."},
}

func DeliverStoryBeats(ctx context.Context, q Querier, userID int64, heroID string, faction string, newLevel int) error {
	var stage int
	if err := q.QueryRowContext(ctx, "SELECT story_arc_stage FROM heroes WHERE id = $1", heroID).Scan(&stage); err != nil {
		return fmt.Errorf("reading story_arc_stage: %w", err)
	}

	delivered := stage
	for _, beat := range StoryArc {
		if beat.Faction != faction || beat.Level > newLevel {
			continue
		}
		if beat.Level <= stage {
			continue
		}
		if userID > 0 {
			_ = notifications.Queue(ctx, q, userID, beat.Text, "general")
		}
		if beat.Level > delivered {
			delivered = beat.Level
		}
	}
	if delivered != stage {
		if _, err := q.ExecContext(ctx, "UPDATE heroes SET story_arc_stage = $1 WHERE id = $2", delivered, heroID); err != nil {
			return fmt.Errorf("updating story_arc_stage: %w", err)
		}
	}
	return nil
}

func UnlockSkill(ctx context.Context, q Querier, heroID, faction, skillKey string) error {
	node, ok := NodeByKey(skillKey)
	if !ok || node.Faction != faction {
		return fmt.Errorf("unknown or mismatched-faction skill %s", skillKey)
	}

	var alreadyUnlocked bool
	_ = q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM hero_skills_unlocked WHERE hero_id = $1 AND skill_key = $2)", heroID, skillKey).Scan(&alreadyUnlocked)
	if alreadyUnlocked {
		return fmt.Errorf("skill %s already unlocked", skillKey)
	}

	var points int
	if err := q.QueryRowContext(ctx, "SELECT skill_points FROM heroes WHERE id = $1", heroID).Scan(&points); err != nil {
		return fmt.Errorf("reading skill points: %w", err)
	}
	if points < 1 {
		return fmt.Errorf("no skill points available")
	}

	if _, err := q.ExecContext(ctx, "UPDATE heroes SET skill_points = skill_points - 1 WHERE id = $1", heroID); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, "INSERT INTO hero_skills_unlocked (hero_id, skill_key) VALUES ($1, $2)", heroID, skillKey)
	return err
}

func TrainingCostMultiplier(ctx context.Context, q Querier, heroID string) float64 {
	return costMultiplier(ctx, q, heroID, EffectTrainingCostPct)
}

func HealCostMultiplier(ctx context.Context, q Querier, heroID string) float64 {
	return costMultiplier(ctx, q, heroID, EffectHealCostPct)
}

func costMultiplier(ctx context.Context, q Querier, heroID string, effect EffectType) float64 {
	total := sumUnlockedEffect(ctx, q, heroID, effect)
	multiplier := 1.0 + total/100.0
	if multiplier < 0.2 {
		multiplier = 0.2
	}
	return multiplier
}

func ResourceBonusPctFor(ctx context.Context, q Querier, campID string) float64 {
	var heroID string
	if err := q.QueryRowContext(ctx, "SELECT id FROM heroes WHERE encampment_id = $1", campID).Scan(&heroID); err != nil {
		return 0
	}
	return sumUnlockedEffect(ctx, q, heroID, EffectResourceGenPct)
}

func sumUnlockedEffect(ctx context.Context, q Querier, heroID string, effect EffectType) float64 {
	rows, err := q.QueryContext(ctx, "SELECT skill_key FROM hero_skills_unlocked WHERE hero_id = $1", heroID)
	if err != nil {
		return 0
	}
	defer rows.Close()
	var total float64
	for rows.Next() {
		var key string
		if scanErr := rows.Scan(&key); scanErr == nil {
			if node, ok := NodeByKey(key); ok && node.Effect == effect {
				total += node.Value
			}
		}
	}
	return total
}

func EquipToHero(ctx context.Context, q Querier, campID, gearKey string) error {
	var quantity int
	if err := q.QueryRowContext(ctx, "SELECT quantity FROM player_gear WHERE encampment_id = $1 AND gear_key = $2", campID, gearKey).Scan(&quantity); err != nil || quantity <= 0 {
		return fmt.Errorf("you don't own that item")
	}
	if _, err := q.ExecContext(ctx, "UPDATE player_gear SET equipped_by_hero = FALSE WHERE encampment_id = $1", campID); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, "UPDATE player_gear SET equipped_by_hero = TRUE WHERE encampment_id = $1 AND gear_key = $2", campID, gearKey)
	return err
}

func HeroXPBonusPct(ctx context.Context, q Querier, campID string, defByKey func(string) (float64, bool)) float64 {
	var gearKey string
	if err := q.QueryRowContext(ctx, "SELECT gear_key FROM player_gear WHERE encampment_id = $1 AND equipped_by_hero = TRUE", campID).Scan(&gearKey); err != nil {
		return 0
	}
	if bonus, ok := defByKey(gearKey); ok {
		return bonus
	}
	return 0
}
