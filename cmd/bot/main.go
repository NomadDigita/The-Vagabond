package main

import (
	"context"
	"database/sql"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/anthropic"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/gemini"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/mock"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/ollama"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/openaicompat"
	"github.com/NomadDigita/The-Vagabond/internal/bot/handlers"
	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications" // Added missing package import
	"github.com/NomadDigita/The-Vagabond/internal/engine/realtime"
	"github.com/NomadDigita/The-Vagabond/internal/engine/tick"
	"github.com/NomadDigita/The-Vagabond/internal/game/econadvisor"
	"github.com/NomadDigita/The-Vagabond/internal/game/fleetcommander"
	"github.com/NomadDigita/The-Vagabond/internal/game/governor"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gopkg.in/telebot.v3"
)

func executeStartupMigrations(db *sql.DB) {
	log.Println("Executing database initialization check...")

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			telegram_id BIGINT PRIMARY KEY,
			username VARCHAR(255) DEFAULT '',
			first_name VARCHAR(255) DEFAULT '',
			state VARCHAR(50) DEFAULT 'onboarding',
			faction VARCHAR(50) DEFAULT '',
			premium_until TIMESTAMP WITH TIME ZONE,
			registered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			last_active TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`ALTER TABLE users ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS notify_on_raid BOOLEAN DEFAULT TRUE;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS notify_on_storage_full BOOLEAN DEFAULT TRUE;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS referred_by BIGINT;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code VARCHAR(20);`,

		`CREATE TABLE IF NOT EXISTS user_mutes (
			muter_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			muted_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (muter_id, muted_id)
		);`,

		`CREATE TABLE IF NOT EXISTS event_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			message TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS feedback_submissions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			message TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`ALTER TABLE users ADD COLUMN IF NOT EXISTS idle_miner_notifications BOOLEAN DEFAULT FALSE;`,

		`CREATE TABLE IF NOT EXISTS coordinates (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			x INT NOT NULL,
			y INT NOT NULL,
			biome VARCHAR(50) NOT NULL,
			danger_level INT DEFAULT 1,
			region VARCHAR(50) NOT NULL,
			terrain VARCHAR(50) NOT NULL,
			CONSTRAINT unique_coordinates UNIQUE (x, y)
		);`,

		`CREATE INDEX IF NOT EXISTS idx_coordinates_xy ON coordinates(x, y);`,

		`CREATE TABLE IF NOT EXISTS encampments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id BIGINT UNIQUE NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			coordinate_id UUID NOT NULL REFERENCES coordinates(id),
			level INT DEFAULT 1,
			established_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`ALTER TABLE encampments ADD COLUMN IF NOT EXISTS auto_scan_enabled BOOLEAN DEFAULT FALSE;`,
		`ALTER TABLE encampments ADD COLUMN IF NOT EXISTS extension_lvl INT DEFAULT 0;`,
		`ALTER TABLE encampments ADD COLUMN IF NOT EXISTS orbital_buff_until TIMESTAMP WITH TIME ZONE;`,
		`ALTER TABLE encampments ADD COLUMN IF NOT EXISTS last_teleport_at TIMESTAMP WITH TIME ZONE;`,
		`ALTER TABLE encampments ADD COLUMN IF NOT EXISTS last_sunlight_at TIMESTAMP WITH TIME ZONE;`,

		`CREATE TABLE IF NOT EXISTS resources (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			scrap DOUBLE PRECISION DEFAULT 0.00,
			rations DOUBLE PRECISION DEFAULT 0.00,
			electricity DOUBLE PRECISION DEFAULT 0.00,
			neuro_cores DOUBLE PRECISION DEFAULT 0.00,
			metal DOUBLE PRECISION DEFAULT 0.00,
			crystal DOUBLE PRECISION DEFAULT 0.00,
			hydrogen DOUBLE PRECISION DEFAULT 0.00,
			dollars DOUBLE PRECISION DEFAULT 0.00,
			ether DOUBLE PRECISION DEFAULT 0.00,
			last_mined_at TIMESTAMP WITH TIME ZONE,
			last_ticked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		// --- SpaceHunt resource revival migration ---
		// Renames energy/steel/uranium -> electricity/metal/crystal on any
		// pre-existing database (no-op on a fresh install, since the CREATE
		// TABLE above already uses the new names directly).
		`DO $$
		BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='energy')
			   AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='electricity') THEN
				ALTER TABLE resources RENAME COLUMN energy TO electricity;
			END IF;
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='steel')
			   AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='metal') THEN
				ALTER TABLE resources RENAME COLUMN steel TO metal;
			END IF;
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='uranium')
			   AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='crystal') THEN
				ALTER TABLE resources RENAME COLUMN uranium TO crystal;
			END IF;
		END $$;`,

		// Folds iron+oil into metal, and diamond+gold+silver into crystal,
		// then drops the now-redundant columns. Guarded so it only runs
		// once (columns won't exist on subsequent boots).
		`DO $$
		BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='iron') THEN
				UPDATE resources SET metal = metal + COALESCE(iron, 0) + COALESCE(oil, 0);
				ALTER TABLE resources DROP COLUMN IF EXISTS iron;
				ALTER TABLE resources DROP COLUMN IF EXISTS oil;
			END IF;
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='diamond') THEN
				UPDATE resources SET crystal = crystal + COALESCE(diamond, 0) + COALESCE(gold, 0) + COALESCE(silver, 0);
				ALTER TABLE resources DROP COLUMN IF EXISTS diamond;
				ALTER TABLE resources DROP COLUMN IF EXISTS gold;
				ALTER TABLE resources DROP COLUMN IF EXISTS silver;
			END IF;
		END $$;`,

		// Bugfix (found live, 2026-07-15): CREATE TABLE IF NOT EXISTS
		// resources is a no-op on any database where the table already
		// existed before `ether`/`neuro_cores` were added to its column
		// list above, so those two columns silently never got created on
		// already-deployed databases. This broke internal/game/econadvisor
		// ("column ether does not exist") and would have caused the same
		// failure in internal/game/governor. Idempotent regardless of
		// whether the table is brand new or years old.
		`ALTER TABLE resources ADD COLUMN IF NOT EXISTS ether DOUBLE PRECISION DEFAULT 0.00;`,
		`ALTER TABLE resources ADD COLUMN IF NOT EXISTS neuro_cores DOUBLE PRECISION DEFAULT 0.00;`,

		`CREATE TABLE IF NOT EXISTS modules (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			type VARCHAR(50) NOT NULL,
			level INT DEFAULT 1,
			is_upgrading BOOLEAN DEFAULT FALSE,
			upgrade_ready_at TIMESTAMP WITH TIME ZONE,
			CONSTRAINT unique_camp_module UNIQUE (encampment_id, type)
		);`,

		`CREATE TABLE IF NOT EXISTS workshop_inventory (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			fusion_tanks INT DEFAULT 0,
			nuclear_shields INT DEFAULT 0,
			soldiers INT DEFAULT 0,
			drones INT DEFAULT 0,
			jets INT DEFAULT 0,
			mechs INT DEFAULT 0,
			nukes INT DEFAULT 0,
			buggies INT DEFAULT 0,
			ships INT DEFAULT 0,
			haulers INT DEFAULT 0,
			tankers INT DEFAULT 0,
			rigs INT DEFAULT 0
		);`,

		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS miners INT DEFAULT 1;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS buggies INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS ships INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS jets INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS haulers INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS tankers INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS rigs INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS destroyers INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS bombers INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS scouts INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS battlecruisers INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS deathstars INT DEFAULT 0;`,

		`CREATE TABLE IF NOT EXISTS active_mining_queues (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			resource_type VARCHAR(50) NOT NULL,
			miners_assigned INT DEFAULT 1,
			started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			ready_at TIMESTAMP WITH TIME ZONE NOT NULL,
			is_completed BOOLEAN DEFAULT FALSE
		);`,

		`CREATE TABLE IF NOT EXISTS raids (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			attacker_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			defender_id UUID REFERENCES encampments(id) ON DELETE CASCADE,
			state VARCHAR(50) NOT NULL,
			resolve_time TIMESTAMP WITH TIME ZONE NOT NULL,
			round_number INT DEFAULT 0,
			attacker_rations DOUBLE PRECISION DEFAULT 100.0,
			attacker_ammo DOUBLE PRECISION DEFAULT 100.0,
			attacker_losses INT DEFAULT 0,
			defender_losses INT DEFAULT 0
		);`,

		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_scrap DOUBLE PRECISION DEFAULT 0.00;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_metal DOUBLE PRECISION DEFAULT 0.00;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_crystal DOUBLE PRECISION DEFAULT 0.00;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS base_march_minutes DOUBLE PRECISION DEFAULT 15.0;`,

		`CREATE TABLE IF NOT EXISTS raid_forces (
			raid_id UUID PRIMARY KEY REFERENCES raids(id) ON DELETE CASCADE,
			hero_id UUID,
			soldiers_mobilized INT DEFAULT 0,
			mechs_mobilized INT DEFAULT 0,
			buggies_mobilized INT DEFAULT 0,
			route_type VARCHAR(50) DEFAULT 'direct'
		);`,

		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS destroyers_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS bombers_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS battlecruisers_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS deathstars_mobilized INT DEFAULT 0;`,

		`CREATE TABLE IF NOT EXISTS tax_law (
			id INT PRIMARY KEY DEFAULT 1,
			tax_rate_percent INT DEFAULT 5,
			last_collected_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT single_row CHECK (id = 1)
		);`,
		`INSERT INTO tax_law (id, tax_rate_percent) VALUES (1, 5) ON CONFLICT (id) DO NOTHING;`,

		`CREATE TABLE IF NOT EXISTS world_bosses (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) UNIQUE NOT NULL,
			emoji VARCHAR(10) DEFAULT '👹',
			max_hp DOUBLE PRECISION NOT NULL,
			current_hp DOUBLE PRECISION NOT NULL,
			loot_pool_dollars DOUBLE PRECISION DEFAULT 0,
			last_defeated_at TIMESTAMP WITH TIME ZONE
		);`,
		`CREATE TABLE IF NOT EXISTS world_boss_contributions (
			boss_id UUID NOT NULL REFERENCES world_bosses(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			damage_dealt DOUBLE PRECISION DEFAULT 0,
			PRIMARY KEY (boss_id, user_id)
		);`,
		`INSERT INTO world_bosses (name, emoji, max_hp, current_hp, loot_pool_dollars) VALUES
			('The Rustlord', '🤖👹', 500000, 500000, 5000),
			('Scrap Titan', '⚙️👹', 1200000, 1200000, 12000),
			('Apex Wraith', '☠️👹', 3000000, 3000000, 30000)
			ON CONFLICT (name) DO NOTHING;`,

		`ALTER TABLE world_bosses ADD COLUMN IF NOT EXISTS retaliation_rating DOUBLE PRECISION DEFAULT 8.0;`,
		`UPDATE world_bosses SET retaliation_rating = 6.0 WHERE name = 'The Rustlord' AND retaliation_rating = 8.0;`,
		`UPDATE world_bosses SET retaliation_rating = 12.0 WHERE name = 'Scrap Titan' AND retaliation_rating = 8.0;`,
		`UPDATE world_bosses SET retaliation_rating = 22.0 WHERE name = 'Apex Wraith' AND retaliation_rating = 8.0;`,

		`CREATE TABLE IF NOT EXISTS world_boss_attacks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			boss_id UUID NOT NULL REFERENCES world_bosses(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			soldiers_committed INT DEFAULT 0,
			mechs_committed INT DEFAULT 0,
			state VARCHAR(50) DEFAULT 'marching',
			resolve_time TIMESTAMP WITH TIME ZONE NOT NULL,
			march_minutes DOUBLE PRECISION DEFAULT 8.0,
			damage_dealt DOUBLE PRECISION DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS rebellion_support (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			total_contributed DOUBLE PRECISION DEFAULT 0
		);`,

		`CREATE TABLE IF NOT EXISTS raid_coop_members (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			soldiers_contributed INT DEFAULT 0,
			mechs_contributed INT DEFAULT 0,
			CONSTRAINT unique_raid_coop_member UNIQUE (raid_id, encampment_id)
		);`,

		`ALTER TABLE raid_coop_members ADD COLUMN IF NOT EXISTS state VARCHAR(50) DEFAULT 'marching_to_ally';`,
		`ALTER TABLE raid_coop_members ADD COLUMN IF NOT EXISTS arrival_time TIMESTAMP WITH TIME ZONE;`,

		`CREATE TABLE IF NOT EXISTS agent_tasks (
			user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
			mode VARCHAR(50) DEFAULT 'collector',
			is_active BOOLEAN DEFAULT FALSE
		);`,

		`CREATE TABLE IF NOT EXISTS mutation_states (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			synaptic_lvl INT DEFAULT 1,
			salvage_lvl INT DEFAULT 1,
			bio_lvl INT DEFAULT 1
		);`,

		`CREATE TABLE IF NOT EXISTS research_states (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			econ_tech_lvl INT DEFAULT 1,
			defense_tech_lvl INT DEFAULT 1,
			military_tech_lvl INT DEFAULT 1
		);`,

		`ALTER TABLE research_states ADD COLUMN IF NOT EXISTS production_tech_lvl INT DEFAULT 1;`,
		`ALTER TABLE research_states ADD COLUMN IF NOT EXISTS integrity_tech_lvl INT DEFAULT 1;`,
		`ALTER TABLE research_states ADD COLUMN IF NOT EXISTS intel_tech_lvl INT DEFAULT 1;`,
		`ALTER TABLE research_states ADD COLUMN IF NOT EXISTS speed_tech_lvl INT DEFAULT 1;`,

		`CREATE TABLE IF NOT EXISTS bank_accounts (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			balance DOUBLE PRECISION DEFAULT 0.00,
			balance_cash DOUBLE PRECISION DEFAULT 0.00,
			loan_amount DOUBLE PRECISION DEFAULT 0.00,
			loan_cash DOUBLE PRECISION DEFAULT 0.00
		);`,

		`ALTER TABLE bank_accounts ADD COLUMN IF NOT EXISTS balance_cash DOUBLE PRECISION DEFAULT 0.00;`,
		`ALTER TABLE bank_accounts ADD COLUMN IF NOT EXISTS loan_cash DOUBLE PRECISION DEFAULT 0.00;`,

		`CREATE TABLE IF NOT EXISTS clans (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) UNIQUE NOT NULL,
			leader_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE
		);`,
		`ALTER TABLE clans ADD COLUMN IF NOT EXISTS icon VARCHAR(10) DEFAULT '🏴';`,
		`ALTER TABLE clans ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';`,
		`ALTER TABLE clans ADD COLUMN IF NOT EXISTS recruiting BOOLEAN DEFAULT TRUE;`,

		`CREATE TABLE IF NOT EXISTS federations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) UNIQUE NOT NULL,
			icon VARCHAR(10) DEFAULT '🌐',
			description TEXT DEFAULT '',
			founder_clan_id UUID REFERENCES clans(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,
		`ALTER TABLE clans ADD COLUMN IF NOT EXISTS federation_id UUID REFERENCES federations(id) ON DELETE SET NULL;`,

		`CREATE TABLE IF NOT EXISTS user_clans (
			user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
			clan_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
			role VARCHAR(50) NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS clan_applications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			clan_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(clan_id, user_id)
		);`,

		`CREATE TABLE IF NOT EXISTS clan_wars (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			clan_a_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
			clan_b_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
			score_a DOUBLE PRECISION DEFAULT 0,
			score_b DOUBLE PRECISION DEFAULT 0,
			status VARCHAR(50) DEFAULT 'active',
			started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			ends_at TIMESTAMP WITH TIME ZONE NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS market_exchange (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			seller_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			item_type VARCHAR(50) NOT NULL,
			quantity INT NOT NULL,
			price_dollars DOUBLE PRECISION NOT NULL,
			is_sold BOOLEAN DEFAULT FALSE
		);`,

		`CREATE TABLE IF NOT EXISTS spy_missions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			spy_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			target_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			is_intercepted BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`ALTER TABLE spy_missions ADD COLUMN IF NOT EXISTS resolved BOOLEAN DEFAULT FALSE;`,
		`ALTER TABLE spy_missions ADD COLUMN IF NOT EXISTS resolve_time TIMESTAMP WITH TIME ZONE;`,

		`CREATE TABLE IF NOT EXISTS arena_queue (
			user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
			bracket VARCHAR(50) NOT NULL,
			entered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`ALTER TABLE arena_queue ADD COLUMN IF NOT EXISTS entered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;`,

		`CREATE TABLE IF NOT EXISTS arena_battles (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			bracket VARCHAR(50) NOT NULL,
			winner_username VARCHAR(255) NOT NULL,
			loser_username VARCHAR(255) NOT NULL,
			winner_loot DOUBLE PRECISION DEFAULT 0.00,
			battle_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS heroes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			encampment_id UUID UNIQUE NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			trait VARCHAR(255) NOT NULL,
			injuries VARCHAR(255) NOT NULL,
			battles_survived INT DEFAULT 0,
			superpower VARCHAR(255) NOT NULL,
			level INT DEFAULT 1,
			xp INT DEFAULT 0
		);`,

		`ALTER TABLE heroes ADD COLUMN IF NOT EXISTS level INT DEFAULT 1;`,
		`ALTER TABLE heroes ADD COLUMN IF NOT EXISTS xp INT DEFAULT 0;`,

		`CREATE TABLE IF NOT EXISTS notifications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			message TEXT NOT NULL,
			is_sent BOOLEAN DEFAULT FALSE,
			queued_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS world_state (
			id INT PRIMARY KEY,
			active_weather VARCHAR(50) DEFAULT 'nominal',
			last_changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`INSERT INTO world_state (id, active_weather, last_changed_at)
		VALUES (1, 'nominal', CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO NOTHING;`,

		`CREATE TABLE IF NOT EXISTS world_news (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			headline TEXT NOT NULL,
			logged_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS world_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			event_type VARCHAR(50) NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL
		);`,

		`CREATE INDEX IF NOT EXISTS idx_world_events_expires ON world_events(expires_at);`,

		`CREATE TABLE IF NOT EXISTS campaign_drafts (
			user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
			target_id VARCHAR(50) NOT NULL,
			soldiers INT DEFAULT 0,
			mechs INT DEFAULT 0,
			buggies INT DEFAULT 0,
			ships INT DEFAULT 0,
			jets INT DEFAULT 0,
			nukes INT DEFAULT 0
		);`,

		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS destroyers INT DEFAULT 0;`,
		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS bombers INT DEFAULT 0;`,
		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS battlecruisers INT DEFAULT 0;`,
		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS deathstars INT DEFAULT 0;`,

		`CREATE OR REPLACE FUNCTION notify_realtime_event() 
		RETURNS trigger AS $$
		BEGIN
			PERFORM pg_notify('realtime_notification_event', NEW.id::text);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;`,

		`DROP TRIGGER IF EXISTS trg_after_notification_insert ON notifications;`,
		`CREATE TRIGGER trg_after_notification_insert
		AFTER INSERT ON notifications
		FOR EACH ROW
		EXECUTE FUNCTION notify_realtime_event();`,

		// --- AI Foundation (Phase A, independent AI roadmap branch) ---
		// See migrations/020_vagabond_ai_foundation.sql for the annotated
		// standalone copy of this schema and internal/ai for the Go layer
		// that reads/writes it.
		`CREATE TABLE IF NOT EXISTS ai_feature_flags (
			feature     VARCHAR(50) PRIMARY KEY,
			enabled     BOOLEAN NOT NULL DEFAULT TRUE,
			updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS ai_permissions (
			user_id     BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			feature     VARCHAR(50) NOT NULL,
			enabled     BOOLEAN NOT NULL DEFAULT TRUE,
			updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, feature)
		);`,

		`CREATE TABLE IF NOT EXISTS ai_memory (
			id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id      BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			scope        VARCHAR(100) NOT NULL,
			role         VARCHAR(20) NOT NULL,
			content      TEXT NOT NULL,
			tool_call_id VARCHAR(100),
			created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE INDEX IF NOT EXISTS idx_ai_memory_user_scope_time ON ai_memory (user_id, scope, created_at);`,

		`CREATE TABLE IF NOT EXISTS ai_cost_log (
			id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id        BIGINT REFERENCES users(telegram_id) ON DELETE SET NULL,
			feature        VARCHAR(50) NOT NULL,
			provider       VARCHAR(50) NOT NULL,
			model          VARCHAR(100) NOT NULL,
			input_tokens   INT NOT NULL DEFAULT 0,
			output_tokens  INT NOT NULL DEFAULT 0,
			cost_usd       DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE INDEX IF NOT EXISTS idx_ai_cost_log_user_time ON ai_cost_log (user_id, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_ai_cost_log_time ON ai_cost_log (created_at);`,

		`INSERT INTO ai_feature_flags (feature, enabled) VALUES
			('ai_planet_governor', TRUE),
			('ai_fleet_commander', TRUE),
			('ai_economy_advisor', TRUE),
			('ai_research_planner', TRUE),
			('ai_battle_analyst', TRUE),
			('ai_guild_assistant', TRUE),
			('ai_dynamic_galaxy', TRUE),
			('ai_npc_intelligence', TRUE),
			('ai_developer_console', TRUE)
		ON CONFLICT (feature) DO NOTHING;`,

		// --- AI Planet Governor (Phase B, independent AI roadmap branch) ---
		// See migrations/021_vagabond_ai_governor.sql for the annotated
		// standalone copy and internal/game/governor for the Go layer.
		`CREATE TABLE IF NOT EXISTS governor_settings (
			encampment_id      UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			autopilot_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
			updated_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		// --- Phase 6: Engage-weapon turret differentiation + remaining
		// new units (Liberator, Observer, Wraith, Piercing Missile,
		// Guardian, Cargo Ship Mk I/II/III). See
		// migrations/022_spacehunt_phase6_weapons_and_units.sql for the
		// annotated standalone copy.
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS liberators INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS observers INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS wraiths INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS piercing_missiles INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS guardians INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS cargo_mk1 INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS cargo_mk2 INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS cargo_mk3 INT DEFAULT 0;`,

		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS liberators INT DEFAULT 0;`,
		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS wraiths INT DEFAULT 0;`,

		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS liberators_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS wraiths_mobilized INT DEFAULT 0;`,

		// --- Phase 7: Hero Commander / manual defense garrison. Lets a
		// player lock a portion of their Soldiers/Mechs as a permanent
		// home garrison that campaign drafts can never pull from, and
		// withdraw them back to the general pool at any time. See
		// migrations/023_spacehunt_phase7_garrison.sql for the annotated
		// standalone copy.
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS garrisoned_soldiers INT DEFAULT 0;`,
		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS garrisoned_mechs INT DEFAULT 0;`,
	}

	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("Fatal: Failed to execute startup database initialization script: %v", err)
		}
	}
	log.Println("All schema initialization verifications complete.")
}

func relocateZeroCoordinates(db *sql.DB) {
	log.Println("Geographical Spawning Self-Healing relocator pass active...")
	ctx := context.Background()

	queryZeroAndDuplicated := `
		SELECT DISTINCT c.id, c.region 
		FROM coordinates c
		JOIN encampments e ON e.coordinate_id = c.id
		WHERE (c.x = 0 AND c.y = 0) 
		   OR (c.x = 913 AND c.y = -843)
		   OR c.id IN (
		       SELECT coordinate_id 
		       FROM encampments 
		       GROUP BY coordinate_id 
		       HAVING COUNT(*) > 1
		   )`

	rows, err := db.QueryContext(ctx, queryZeroAndDuplicated)
	if err != nil {
		log.Printf("Spawning relocator sweep skipped: %v", err)
		return
	}
	defer rows.Close()

	type zeroCoord struct {
		id     string
		region string
	}
	var coords []zeroCoord
	for rows.Next() {
		var z zeroCoord
		if err := rows.Scan(&z.id, &z.region); err == nil {
			coords = append(coords, z)
		}
	}
	rows.Close()

	rSource := rand.NewSource(time.Now().UnixNano())
	rGen := rand.New(rSource)

	for _, c := range coords {
		success := false
		var x, y int
		for attempt := 0; attempt < 100; attempt++ {
			switch c.region {
			case "Africa":
				x = rGen.Intn(991) + 10
				y = rGen.Intn(991) + 10
			case "Europe":
				x = -(rGen.Intn(991) + 10)
				y = rGen.Intn(991) + 10
			case "Asia":
				x = rGen.Intn(991) + 10
				y = -(rGen.Intn(991) + 10)
			default:
				x = -(rGen.Intn(991) + 10)
				y = -(rGen.Intn(991) + 10)
			}

			_, err := db.ExecContext(ctx, "UPDATE coordinates SET x = $1, y = $2 WHERE id = $3 AND NOT EXISTS(SELECT 1 FROM coordinates WHERE x = $1 AND y = $2)", x, y, c.id)
			if err == nil {
				success = true
				break
			}
		}
		if success {
			log.Printf("Database Healing: Stuck coordinate [%s] redistributed to [%s quadrant: %d, %d]", c.id, c.region, x, y)
		}
	}
}

func main() {
	log.Println("Starting The Vagabond server initialization sequence...")

	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not detected. Loading configuration from system environment variables.")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("Fatal: DATABASE_URL environment parameter not set.")
	}

	botToken := os.Getenv("TELEGRAM_TOKEN")
	if botToken == "" {
		log.Fatal("Fatal: TELEGRAM_TOKEN environment parameter not set.")
	}

	adminIDs := os.Getenv("ADMIN_IDS")
	if adminIDs == "" {
		log.Println("Warning: ADMIN_IDS is empty. Admin overrides will be inaccessible.")
	}

	tickSeconds := 60
	if intervalStr := os.Getenv("GAME_TICK_SECONDS"); intervalStr != "" {
		if val, err := strconv.Atoi(intervalStr); err == nil {
			tickSeconds = val
		}
	}

	log.Println("Connecting to Supabase Database...")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Fatal: Database driver initialization failure: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("Fatal: Database network connection check failed: %v", err)
	}
	log.Println("Database connection pool established successfully.")

	executeStartupMigrations(db)
	relocateZeroCoordinates(db)

	pref := telebot.Settings{
		Token:  botToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatalf("Fatal: Telegram API initialization failure: %v", err)
	}
	log.Printf("Telegram credentials accepted. Bot logged in as: @%s", bot.Me.Username)

	tickEngine := tick.NewEngine(db, time.Duration(tickSeconds)*time.Second)
	tickEngine.Start()

	realtimeListener := realtime.NewListener(dbURL, db, bot)
	realtimeListener.Start()

	// INSTANTIATE AND START THE BACKUP NOTIFICATION DISPATCHER
	notificationDispatcher := notifications.NewDispatcher(db, bot)
	notificationDispatcher.Start()

	onboarding := handlers.NewOnboardingHandler(db)
	camp := handlers.NewCampHandler(db, adminIDs)
	combat := handlers.NewCombatHandler(db, adminIDs)
	agentH := handlers.NewAgentHandler(db, adminIDs)
	admin := handlers.NewAdminHandler(db, tickEngine, adminIDs)
	hero := handlers.NewHeroHandler(db)
	world := handlers.NewWorldHandler(db)
	exchange := handlers.NewExchangeHandler(db)
	econ := handlers.NewEconomyHandler(db, exchange)
	clan := handlers.NewClanHandler(db)
	factory := handlers.NewFactoryHandler(db)
	arena := handlers.NewArenaHandler(db)
	silo := handlers.NewSiloHandler(db)
	research := handlers.NewResearchHandler(db)
	deconstruct := handlers.NewDeconstructHandler(db)
	ranking := handlers.NewRankingHandler(db)
	boss := handlers.NewBossHandler(db)
	rebellion := handlers.NewRebellionHandler(db)
	federation := handlers.NewFederationHandler(db)
	profile := handlers.NewProfileHandler(db)
	ether := handlers.NewEtherHandler(db)
	jobs := handlers.NewJobsHandler(db)
	nlp := handlers.NewNLPHandler(onboarding, camp, combat, econ, clan, hero, agentH, factory, silo, research, exchange, world)

	// --- AI Foundation wiring (Phase A, independent AI roadmap branch) ---
	// Provider-agnostic by design: register additional providers here
	// (OpenAI, Gemini, Qwen, Grok, DeepSeek, Ollama, ...) without
	// touching internal/ai itself. Mock is always registered last so
	// the bot degrades gracefully instead of failing when no real
	// provider key is configured.
	aiConfig := ai.LoadConfig()
	aiRegistry := ai.NewRegistry()
	aiRegistry.Register(anthropic.New(aiConfig.AnthropicAPIKey, aiConfig.AnthropicModel))
	aiRegistry.Register(openaicompat.New("openai", "https://api.openai.com/v1", aiConfig.OpenAIAPIKey, aiConfig.OpenAIModel, true))
	aiRegistry.Register(openaicompat.New("deepseek", "https://api.deepseek.com/v1", aiConfig.DeepSeekAPIKey, aiConfig.DeepSeekModel, true))
	aiRegistry.Register(openaicompat.New("qwen", aiConfig.QwenBaseURL, aiConfig.QwenAPIKey, aiConfig.QwenModel, true))
	aiRegistry.Register(openaicompat.New("grok", "https://api.x.ai/v1", aiConfig.GrokAPIKey, aiConfig.GrokModel, true))
	aiRegistry.Register(gemini.New(aiConfig.GeminiAPIKey, aiConfig.GeminiModel))
	aiRegistry.Register(ollama.New(aiConfig.OllamaBaseURL, aiConfig.OllamaModel))
	aiRegistry.Register(mock.New())
	aiCostTracker := ai.NewPostgresCostTracker(db)
	aiPermissions := ai.NewPermissionManager(db)
	aiMemory := ai.NewPostgresMemoryStore(db)
	aiService := ai.NewService(aiConfig, aiRegistry, aiCostTracker, aiPermissions, aiMemory)
	aiStatus := handlers.NewAIStatusHandler(aiService, adminIDs)
	log.Printf("AI Foundation initialized. Default provider: %s | Fallback order: %v | Enabled: %v",
		aiConfig.DefaultProvider, aiConfig.FallbackOrder, aiConfig.Enabled)

	// --- AI Planet Governor wiring (Phase B, independent AI roadmap branch) ---
	aiGovernor := governor.New(db, aiService)
	governorHandler := handlers.NewGovernorHandler(aiGovernor)

	// --- AI Fleet Commander wiring (Phase C, independent AI roadmap branch) ---
	aiFleetCommander := fleetcommander.New(db, aiService)
	fleetCommanderHandler := handlers.NewFleetCommanderHandler(aiFleetCommander)

	// --- AI Economy Advisor wiring (Phase D, independent AI roadmap branch) ---
	aiEconAdvisor := econadvisor.New(db, aiService)
	econAdvisorHandler := handlers.NewEconomyAdvisorHandler(aiEconAdvisor)

	bot.Handle("/start", onboarding.HandleStart)
	bot.Handle("/name", onboarding.HandleRenameOutpost)
	bot.Handle("/camp", camp.HandleCamp)
	bot.Handle("/raid", combat.HandleRaidBoard)
	bot.Handle("/agent", agentH.HandleAgent)
	bot.Handle("/hero", hero.HandleHeroPanel)
	bot.Handle("/world", world.HandleWorldFeed)
	bot.Handle("/econ", econ.HandleEconPanel)
	bot.Handle("/clan", clan.HandleClanPanel)
	bot.Handle("/clans", clan.HandleBrowseClans)
	bot.Handle("/clan_create", clan.HandleCreateClanCommand)
	bot.Handle("/clan_rename", clan.HandleRenameClanCommand)
	bot.Handle("/guild_missions", clan.HandleGuildMissions)
	bot.Handle("/guildmsg", clan.HandleGuildMsg)
	bot.Handle("/guild_icon", clan.HandleGuildIcon)
	bot.Handle("/guild_description", clan.HandleGuildDescription)
	bot.Handle("/board", clan.HandleBoard)
	bot.Handle("/scout", combat.HandleScout)
	bot.Handle("/factory", factory.HandleFactoryPanel)
	bot.Handle("/map", world.HandleSectorMap)
	bot.Handle("/help", onboarding.HandleHelp)
	bot.Handle("/inventory", econ.HandleWarehouseReserves)
	bot.Handle("/admin", admin.HandleAdminPanel)
	bot.Handle("/arena", arena.HandleArenaPanel)
	bot.Handle("/broadcast", world.HandleSectorBroadcast)
	bot.Handle("/mutations", camp.HandleMutationsPanel)
	bot.Handle("/silo", silo.HandleSiloPanel)
	bot.Handle("/mine", camp.HandleActiveMining)
	bot.Handle("/research", research.HandleResearchPanel)
	bot.Handle("/deconstruct", deconstruct.HandleDeconstructPanel)
	bot.Handle("/defense", camp.HandleDefenseGridPanel)
	bot.Handle("/infrastructure", camp.HandleInfrastructureGridPanel)
	bot.Handle("/ranking", ranking.HandleRankingPanel)
	bot.Handle("/bosses", boss.HandleBossPanel)
	bot.Handle("/autoscan", combat.HandleAutoScanToggle)
	bot.Handle("/rebellion", rebellion.HandleRebellionPanel)
	bot.Handle("/federations", federation.HandleFederationsPanel)
	bot.Handle("/federation", federation.HandleMyFederationPanel)
	bot.Handle("/fed_found", federation.HandleFoundFederation)
	bot.Handle("/fed_join", federation.HandleJoinFederation)
	bot.Handle("/fed_leave", federation.HandleLeaveFederation)
	bot.Handle("/description", profile.HandleDescription)
	bot.Handle("/settings", profile.HandleSettings)
	bot.Handle("/refer", profile.HandleRefer)
	bot.Handle("/feedback", profile.HandleFeedback)
	bot.Handle("/msg", profile.HandleMsg)
	bot.Handle("/mute", profile.HandleMute)
	bot.Handle("/unmute", profile.HandleUnmute)
	bot.Handle("/mutes", profile.HandleMutesList)
	bot.Handle("/log", profile.HandleLog)
	bot.Handle("/stats", profile.HandleStats)
	bot.Handle("/units", profile.HandleUnits)
	bot.Handle("/ether", ether.HandleEtherShop)
	bot.Handle("/missions", profile.HandleMissions)
	bot.Handle("/destinations", profile.HandleDestinations)
	bot.Handle("/newjobhyperspeed", jobs.HandleHyperSpeed)
	bot.Handle("/newjobextendplanet", jobs.HandleExtendPlanet)
	bot.Handle("/newjobteleport", jobs.HandleTeleport)
	bot.Handle("/newjoborbitalmaneuver", jobs.HandleOrbitalManeuver)
	bot.Handle("/newjobrepairunits", jobs.HandleRepairUnits)
	bot.Handle("/newjobrepairbuildings", jobs.HandleRepairBuildings)
	bot.Handle("/newjobgathersunlight", jobs.HandleGatherSunlight)
	bot.Handle("/newjobmanualscan", jobs.HandleManualScanAlias)
	bot.Handle("/newjobautoscan", jobs.HandleAutoScanAlias)
	bot.Handle("/newjobadvancedscan", jobs.HandleAdvancedScanAlias)
	bot.Handle("/newjobpublishtrade", jobs.HandlePublishTradeAlias)
	bot.Handle("/ai_status", aiStatus.HandleAIStatus)
	bot.Handle("/ai_status_toggle", aiStatus.HandleAIStatusToggle)
	bot.Handle("/ai_settings", aiStatus.HandleAISettings)
	bot.Handle("/governor", governorHandler.HandleGovernor)
	bot.Handle("/governor_autopilot", governorHandler.HandleGovernorAutopilot)
	bot.Handle("\fgov_refresh", governorHandler.HandleGovernorRefreshCallback)
	bot.Handle("\fgov_toggle_autopilot", governorHandler.HandleGovernorToggleCallback)
	bot.Handle("/fleet_commander", fleetCommanderHandler.HandleFleetCommander)
	bot.Handle("\ffleet_refresh", fleetCommanderHandler.HandleFleetCommanderRefreshCallback)
	bot.Handle("/economy_advisor", econAdvisorHandler.HandleEconomyAdvisor)
	bot.Handle("\fecon_refresh", econAdvisorHandler.HandleEconomyAdvisorRefreshCallback)
	bot.Handle("👹 World Bosses", boss.HandleBossPanel)
	bot.Handle("✊ The Rebellion", rebellion.HandleRebellionPanel)
	bot.Handle("/settaxrate", admin.HandleAdminSetTaxRate)
	bot.Handle("🏆 Global Ranking", ranking.HandleRankingPanel)

	bot.Handle("/admin_tick", admin.HandleAdminTick)
	bot.Handle("/admin_db_reset", admin.HandleAdminDBReset)
	bot.Handle("/admin_broadcast", admin.HandleAdminBroadcast)
	bot.Handle("/admin_metrics", admin.HandleAdminMetrics)
	bot.Handle("/admin_give", admin.HandleAdminGive)
	bot.Handle("/admin_faction", admin.HandleAdminFaction)
	bot.Handle("/admin_gift_premium", admin.HandleAdminGiftPremium)
	bot.Handle("/admin_gift_resources", admin.HandleAdminGiftResources)

	bot.Handle("📡 Terminal HQ", onboarding.HandleStart)
	bot.Handle("⛺ Outpost Camp", camp.HandleCamp)
	bot.Handle("⚔️ Tactical Combat", combat.HandleRaidBoard)
	bot.Handle("🏦 System Economy", econ.HandleEconPanel)
	bot.Handle("🏭 Heavy Workshop", factory.HandleFactoryPanel)

	bot.Handle("🏛️ Admin Terminal", admin.HandleAdminPanel)
	bot.Handle("⚡ Force Master Tick", admin.HandleAdminTick)
	bot.Handle("🪙 Inject Resources", admin.HandleAdminGive)
	bot.Handle("🛰️ Server Metrics", admin.HandleAdminMetrics)

	bot.Handle("🔨 Structural Upgrades", camp.HandleStructuralUpgrades)
	bot.Handle("👥 Hero Commander", hero.HandleHeroPanel)
	bot.Handle("🧠 Automation Agent", agentH.HandleAgent)
	bot.Handle("🧪 Research Lab", research.HandleResearchPanel)
	bot.Handle("🧬 Mutation Core", camp.HandleMutationsPanel)
	bot.Handle("⛏️ Active Mining", camp.HandleActiveMining)
	bot.Handle("🛰️ Scan Targets", combat.HandleTargetMatrix)
	bot.Handle("🛸 Expedition Radar", combat.HandleExpeditionRadar)
	bot.Handle("📻 Wasteland Radio", world.HandleWorldFeed)
	bot.Handle("📦 Warehouse Reserves", econ.HandleWarehouseReserves)
	bot.Handle("🪙 Financial Vault", econ.HandleFinancialVault)
	bot.Handle("🛡️ Clan Alliances", clan.HandleClanPanel)
	bot.Handle("🏟️ Combat Arena", arena.HandleArenaPanel)
	bot.Handle("☢️ Strategic Silo", silo.HandleSiloPanel)
	bot.Handle("💱 Market Exchange", exchange.HandleExchangePanel)
	bot.Handle("🪖 Recruit Troops", factory.HandleRecruitPanel)
	bot.Handle("🚗 Logistics Vehicles", factory.HandleVehiclesPanel)
	bot.Handle("♻️ Deconstruct Units", deconstruct.HandleDeconstructPanel)
	bot.Handle("🛡️ Defense Grid", camp.HandleDefenseGridPanel)
	bot.Handle("🏗️ Infrastructure Grid", camp.HandleInfrastructureGridPanel)
	bot.Handle("⬅️ Back to HQ", onboarding.HandleStart)

	bot.Handle(telebot.OnText, nlp.HandleTextMessage)

	bot.Handle("\fupgrade_mod", camp.HandleUpgradeCallback)
	bot.Handle("\flaunch_raid", combat.HandleLaunchRaidCallback)
	bot.Handle("\ftoggle_agent", agentH.HandleToggleAgentCallback)
	bot.Handle("\fset_agent_mode", agentH.HandleSetModeCallback)
	bot.Handle("\fjoin_faction", onboarding.HandleFactionCallback)
	bot.Handle("\fbank_action", econ.HandleBankCallback)
	bot.Handle("\fmarket_buy", econ.HandleMarketCallback)
	bot.Handle("\fbrowse_clans", clan.HandleBrowseClans)
	bot.Handle("\fclan_apply", clan.HandleApplyToClanCallback)
	bot.Handle("\fclan_apps", clan.HandleApplicationsInboxCallback)
	bot.Handle("\fclan_app_accept", clan.HandleAcceptApplicationCallback)
	bot.Handle("\fclan_app_reject", clan.HandleRejectApplicationCallback)
	bot.Handle("\fleave_clan", clan.HandleLeaveClanCallback)
	bot.Handle("\fdeclare_clan_war", clan.HandleDeclareClanWarCallback)
	bot.Handle("\fexp_action", combat.HandleExpeditionActions)
	bot.Handle("\fcraft_item", factory.HandleCraftCallback)
	bot.Handle("\fdeconstruct_item", deconstruct.HandleDeconstructCallback)
	bot.Handle("\fattack_boss", boss.HandleAttackBossCallback)
	bot.Handle("\frebellion_donate", rebellion.HandleRebellionDonateCallback)
	bot.Handle("\ftrade_hub_nav", econ.HandleTradeHubNavCallback)
	bot.Handle("\frecon_ai", combat.HandleReconAICallback)
	bot.Handle("\fsettings_toggle", profile.HandleSettingsToggleCallback)
	bot.Handle("\fether_convert", ether.HandleEtherConvertCallback)
	bot.Handle("\fspy_action", combat.HandleSpyCallback)
	bot.Handle("\fupgrade_tech", research.HandleUpgradeTechCallback)
	bot.Handle("\fpost_listing", exchange.HandlePostListingCallback)
	bot.Handle("\fbuy_listing", exchange.HandleBuyListingCallback)
	bot.Handle("\fmutate_mod", camp.HandleMutationCallback)
	bot.Handle("\fjoin_queue", arena.HandleJoinQueueCallback)
	bot.Handle("\flaunch_icbm", silo.HandleLaunchICBMCallback)
	bot.Handle("\flaunch_piercing", silo.HandleLaunchPiercingMissileCallback)
	bot.Handle("\fmine_action", camp.HandleMineCallback)
	bot.Handle("\fhero_action", hero.HandleHeroCallback)
	bot.Handle("\fgarrison_adjust", hero.HandleGarrisonAdjustCallback)
	bot.Handle("\flaunch_interceptor", combat.HandleLaunchInterceptor)
	bot.Handle("\fadmin_action", admin.HandleAdminActionCallback)
	bot.Handle("\fstage_coop", combat.HandleStageCoopCallback)
	bot.Handle("\fjoin_coop", combat.HandleJoinCoopCallback)

	bot.Handle("\fclan_manage", clan.HandleManageMembersCallback)
	bot.Handle("\fclan_stats", clan.HandleAllianceStatsCallback)
	bot.Handle("\fclan_kick", clan.HandleKickMemberCallback)
	bot.Handle("\fclan_promote", clan.HandlePromoteMemberCallback)

	bot.Handle("\fconfirm_launch", combat.HandleConfirmHangarLaunchCallback)
	bot.Handle("\fadjust_draft", combat.HandleAdjustDraftCallback)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("SYSTEM OPERATIONAL"))
	})

	go func() {
		log.Printf("Inbound HTTP listener bound to port :%s for health telemetry checks.", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("Warning: HTTP Server closed: %v", err)
		}
	}()

	selfPingURL := os.Getenv("SELF_PING_URL")
	if selfPingURL != "" {
		go func() {
			log.Printf("Autonomous Keep-Alive Pinger active. target: %s", selfPingURL)
			ticker := time.NewTicker(10 * time.Minute)
			for range ticker.C {
				resp, err := http.Get(selfPingURL)
				if err != nil {
					log.Printf("Keep-Alive Pinger connection warning: %v", err)
					continue
				}
				_ = resp.Body.Close()
				log.Println("⚡ Keep-Alive Pinger succeeded. Instance held awake.")
			}
		}()
	} else {
		log.Println("Note: SELF_PING_URL parameters not set. Keep-Alive pinger is idle.")
	}

	go func() {
		log.Println("Active long-polling loop engaged. System operational.")
		bot.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit

	log.Println("Termination request received. Initiating graceful shutdown protocol...")

	tickEngine.Stop()
	realtimeListener.Stop()
	notificationDispatcher.Stop() // Terminate dispatcher cleanly on shutdown
	db.Close()

	log.Println("System components cleanly dismantled. Server offline.")
}