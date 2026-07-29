// Package schema holds the full set of idempotent startup migration
// statements for The Vagabond. This used to live inline inside
// cmd/bot/main.go's executeStartupMigrations, which made it untestable -
// Phase 7 milestone 5 wants tests for "migration compatibility, critical
// idempotency paths", and you can't unit test something that only exists
// inside func main()'s call graph. Extracting it here changes nothing
// about what runs at startup (main.go just calls Statements() now) but
// makes it possible to run every statement twice against a real database
// in a test and confirm CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT
// EXISTS / etc. actually hold up, instead of trusting that by inspection.
package schema

// Statements returns every startup schema statement, in the exact order
// they must run (later ALTERs assume earlier CREATE TABLEs already ran).
// Every statement here must be safe to execute an unbounded number of
// times against a database that already has some or all of the prior
// statements applied - that's what lets this run on every boot.
func Statements() []string {
	return []string{
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

		// Referral system fix (2026-07-26): referral_code was never
		// enforced unique, and the old `telegramID % 1_000_000` scheme
		// could collide, silently misattributing referrals. Codes are
		// now derived deterministically from the player's Telegram ID
		// (base36), which is unique by construction, so this index just
		// protects against any stale/legacy duplicate codes.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_referral_code ON users(referral_code) WHERE referral_code IS NOT NULL;`,
		// Tracks the highest referral-count milestone tier (5/10/25) a
		// player has already claimed, so milestone bonuses are granted
		// exactly once.
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_tier_claimed INT DEFAULT 0;`,

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
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS attacker_electricity DOUBLE PRECISION DEFAULT 100.0;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS attacker_logistics DOUBLE PRECISION DEFAULT 100.0;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_rations DOUBLE PRECISION DEFAULT 0.0;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_electricity DOUBLE PRECISION DEFAULT 0.0;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_hydrogen DOUBLE PRECISION DEFAULT 0.0;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_neuro_cores DOUBLE PRECISION DEFAULT 0.0;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_dollars DOUBLE PRECISION DEFAULT 0.0;`,

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
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS ships_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS jets_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS nukes_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS haulers_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS tankers_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS cargo_mk1_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS cargo_mk2_mobilized INT DEFAULT 0;`,
		`ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS cargo_mk3_mobilized INT DEFAULT 0;`,

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
		// A notification that can never actually be delivered (player
		// blocked the bot, malformed HTML tripping Telegram's "can't
		// parse entities" 400, a message over Telegram's 4096-char cap,
		// a stale/invalid chat) used to retry forever with no limit and
		// no way to tell it apart from a message that's just waiting its
		// turn. Worse: notifications.Dispatcher.drainQueue polls a fixed
		// LIMIT 10 oldest-first batch every 3 seconds - if enough
		// permanently-failing rows accumulate at the front of that
		// queue, they occupy every slot in every batch forever, and
		// genuinely new notifications behind them - for any player, any
		// feature - never get a turn at all. See drainQueue's updated
		// give-up-after-N-attempts logic, which this column supports.
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS failed_attempts INT NOT NULL DEFAULT 0;`,

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

		// Phase 7 (item 12): world events are now scoped per-continent
		// instead of one global front, matching coordinates.region's
		// existing Africa/Europe/Asia/Americas quadrant scheme. The
		// column is nullable-safe (backfilled 'Global' for any legacy
		// rows) so this is a pure additive migration.
		`ALTER TABLE world_events ADD COLUMN IF NOT EXISTS continent VARCHAR(50) NOT NULL DEFAULT 'Global';`,
		`CREATE INDEX IF NOT EXISTS idx_world_events_continent ON world_events(continent, expires_at);`,

		// Phase 7 (item 10): World Exploration. Sites rotate in per
		// continent (same cadence/pattern as world_events above) and
		// are claimed first-come-first-served by whichever outpost
		// dispatches an expedition and survives the timer first.
		`CREATE TABLE IF NOT EXISTS exploration_sites (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			continent VARCHAR(50) NOT NULL,
			site_name VARCHAR(255) NOT NULL,
			site_type VARCHAR(50) NOT NULL,
			reward_type VARCHAR(50) NOT NULL,
			reward_amount DOUBLE PRECISION NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			claimed_by UUID REFERENCES encampments(id) ON DELETE SET NULL,
			claimed_at TIMESTAMP WITH TIME ZONE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_exploration_sites_continent ON exploration_sites(continent, claimed_by, expires_at);`,

		`CREATE TABLE IF NOT EXISTS exploration_dispatches (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			site_id UUID NOT NULL UNIQUE REFERENCES exploration_sites(id) ON DELETE CASCADE,
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			resolve_time TIMESTAMP WITH TIME ZONE NOT NULL
		);`,

		// MMO living-world Phase 2: discoveries are directional and are
		// the authoritative gate for target visibility and raid launch.
		// target_key supports non-encampment world targets while AI
		// civilizations are migrated onto persistent settlements.
		`CREATE TABLE IF NOT EXISTS encampment_discoveries (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			observer_encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			target_encampment_id UUID REFERENCES encampments(id) ON DELETE CASCADE,
			target_key VARCHAR(100),
			discovery_method VARCHAR(50) NOT NULL,
			discovered_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT encampment_discoveries_one_target CHECK (
				(target_encampment_id IS NOT NULL AND target_key IS NULL)
				OR (target_encampment_id IS NULL AND target_key IS NOT NULL)
			)
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_encampment_discoveries_encampment_target
			ON encampment_discoveries(observer_encampment_id, target_encampment_id)
			WHERE target_encampment_id IS NOT NULL;`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_encampment_discoveries_system_target
			ON encampment_discoveries(observer_encampment_id, target_key)
			WHERE target_key IS NOT NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_encampment_discoveries_observer_recent
			ON encampment_discoveries(observer_encampment_id, last_seen_at DESC);`,

		// Route snapshots make proximity-based radar notices deterministic
		// and prepare active campaigns for the later road-event phases.
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS origin_x INT;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS origin_y INT;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS destination_x INT;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS destination_y INT;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS origin_region VARCHAR(50);`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS destination_region VARCHAR(50);`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS radar_alert_sent_at TIMESTAMP WITH TIME ZONE;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS movement_state VARCHAR(30) NOT NULL DEFAULT 'moving';`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS pause_reason TEXT;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS next_route_event_at TIMESTAMP WITH TIME ZONE;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS route_progress DOUBLE PRECISION;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS route_progress_at TIMESTAMP WITH TIME ZONE;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS route_leg_minutes DOUBLE PRECISION;`,
		`UPDATE raids SET route_progress = CASE WHEN state = 'returning' THEN 1.0 ELSE 0.0 END WHERE route_progress IS NULL;`,
		`ALTER TABLE raids ALTER COLUMN route_progress SET DEFAULT 0.0;`,
		`ALTER TABLE raids ALTER COLUMN route_progress SET NOT NULL;`,
		`DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'raids_movement_state_valid') THEN
				ALTER TABLE raids ADD CONSTRAINT raids_movement_state_valid
					CHECK (movement_state IN ('moving', 'encounter_pending', 'encounter_battle', 'battle_recovery', 'weather_paused', 'supply_paused'));
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'raids_route_progress_range') THEN
				ALTER TABLE raids ADD CONSTRAINT raids_route_progress_range
					CHECK (route_progress >= 0.0 AND route_progress <= 1.0);
			END IF;
		END $$;`,
		`CREATE INDEX IF NOT EXISTS idx_raids_marching_radar_pending
			ON raids(resolve_time)
			WHERE state = 'marching' AND defender_id IS NOT NULL AND radar_alert_sent_at IS NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_raids_active_movement
			ON raids(state, movement_state, resolve_time)
			WHERE state IN ('marching', 'returning');`,

		// Phase 7 (item 11): Diplomacy. Mirrors clan_wars's clan_a/clan_b
		// shape, but for peaceful pacts instead of conflicts. A pact only
		// takes effect (blocks raids - see combat.go's launch check) once
		// status = 'active', which requires the receiving Clan King to
		// accept a 'pending' proposal.
		`CREATE TABLE IF NOT EXISTS clan_diplomacy (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			clan_a_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
			clan_b_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
			pact_type VARCHAR(50) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			proposed_by BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			responded_at TIMESTAMP WITH TIME ZONE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_clan_diplomacy_clans ON clan_diplomacy(clan_a_id, clan_b_id, status);`,

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
		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS haulers INT DEFAULT 0;`,
		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS tankers INT DEFAULT 0;`,
		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS cargo_mk1 INT DEFAULT 0;`,
		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS cargo_mk2 INT DEFAULT 0;`,
		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS cargo_mk3 INT DEFAULT 0;`,

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

		// --- Phase 7: Bulk unit selection. A single "step size" stored on
		// the draft itself (cycled via a Step: x1/x10/x100/MAX button row)
		// makes every existing +/- button move that many units at once,
		// instead of forcing 100 taps to draft 100 Soldiers. See
		// migrations/024_spacehunt_phase7_bulk_selection.sql.
		`ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS step_size INT DEFAULT 1;`,

		// --- MMO Living World Phase 3/4: route legs + road encounters.
		// See migrations/030_mmo_route_legs_and_road_encounters.sql for
		// the annotated standalone copy.
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS leg_started_at TIMESTAMP WITH TIME ZONE;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS leg_total_minutes DOUBLE PRECISION;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS movement_state VARCHAR(30) NOT NULL DEFAULT 'moving';`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS paused_remaining_minutes DOUBLE PRECISION;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS active_encounter_id UUID;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS paused_at TIMESTAMP WITH TIME ZONE;`,
		`UPDATE raids SET leg_started_at = COALESCE(leg_started_at, created_at, CURRENT_TIMESTAMP) WHERE leg_started_at IS NULL;`,
		`UPDATE raids SET leg_total_minutes = COALESCE(leg_total_minutes, base_march_minutes, 15.0) WHERE leg_total_minutes IS NULL;`,
		`CREATE TABLE IF NOT EXISTS road_encounters (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			raid_a_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
			raid_b_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
			location_x INT NOT NULL,
			location_y INT NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			decision_a VARCHAR(20),
			decision_b VARCHAR(20),
			outcome VARCHAR(20),
			winner_raid_id UUID REFERENCES raids(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			response_deadline TIMESTAMP WITH TIME ZONE NOT NULL,
			resolved_at TIMESTAMP WITH TIME ZONE,
			CONSTRAINT road_encounters_distinct_parties CHECK (raid_a_id <> raid_b_id),
			CONSTRAINT road_encounters_ordered_pair CHECK (raid_a_id::text < raid_b_id::text)
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_road_encounters_pending_pair ON road_encounters(raid_a_id, raid_b_id) WHERE status = 'pending';`,
		`CREATE INDEX IF NOT EXISTS idx_road_encounters_pending_deadline ON road_encounters(response_deadline) WHERE status = 'pending';`,
		`CREATE INDEX IF NOT EXISTS idx_road_encounters_raid_a_recent ON road_encounters(raid_a_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_road_encounters_raid_b_recent ON road_encounters(raid_b_id, created_at DESC);`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'raids_active_encounter_id_fkey'
			) THEN
				ALTER TABLE raids
					ADD CONSTRAINT raids_active_encounter_id_fkey
					FOREIGN KEY (active_encounter_id) REFERENCES road_encounters(id) ON DELETE SET NULL;
			END IF;
		END $$;`,
		`CREATE INDEX IF NOT EXISTS idx_raids_moving_route_scan ON raids(state) WHERE state IN ('marching', 'returning') AND movement_state = 'moving';`,

		// See migrations/034_mmo_road_base_encounters.sql for the annotated
		// version. Completes Phase 4 milestone 2 (expedition-vs-base road
		// encounters), the "expeditions and bases" half that road_encounters
		// above (expedition-vs-expedition only) didn't cover.
		`CREATE TABLE IF NOT EXISTS road_base_encounters (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			location_x INT NOT NULL,
			location_y INT NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			outcome VARCHAR(20),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			response_deadline TIMESTAMP WITH TIME ZONE NOT NULL,
			resolved_at TIMESTAMP WITH TIME ZONE,
			CONSTRAINT road_base_encounters_status CHECK (status IN ('pending', 'resolved'))
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_road_base_encounters_pending_pair ON road_base_encounters(raid_id, encampment_id) WHERE status = 'pending';`,
		`CREATE INDEX IF NOT EXISTS idx_road_base_encounters_pending_deadline ON road_base_encounters(response_deadline) WHERE status = 'pending';`,
		`CREATE INDEX IF NOT EXISTS idx_road_base_encounters_raid_recent ON road_base_encounters(raid_id, created_at DESC);`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS active_base_encounter_id UUID;`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'raids_active_base_encounter_id_fkey'
			) THEN
				ALTER TABLE raids
					ADD CONSTRAINT raids_active_base_encounter_id_fkey
					FOREIGN KEY (active_base_encounter_id) REFERENCES road_base_encounters(id) ON DELETE SET NULL;
			END IF;
		END $$;`,

		// --- MMO Living World Phase 5: weather route incidents (temporary
		// camps) + reinforcement convoys. See
		// migrations/033_mmo_route_weather_and_reinforcement_convoys.sql
		// for the annotated standalone copy.
		`CREATE TABLE IF NOT EXISTS route_incidents (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
			incident_type VARCHAR(20) NOT NULL,
			severity INT NOT NULL DEFAULT 1,
			location_x INT NOT NULL,
			location_y INT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			cleared_at TIMESTAMP WITH TIME ZONE NOT NULL,
			resolved BOOLEAN NOT NULL DEFAULT FALSE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_route_incidents_pending_clear ON route_incidents(cleared_at) WHERE resolved = FALSE;`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_route_incidents_active_raid ON route_incidents(raid_id) WHERE resolved = FALSE;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS active_incident_id UUID;`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'raids_active_incident_id_fkey'
			) THEN
				ALTER TABLE raids
					ADD CONSTRAINT raids_active_incident_id_fkey
					FOREIGN KEY (active_incident_id) REFERENCES route_incidents(id) ON DELETE SET NULL;
			END IF;
		END $$;`,
		`CREATE TABLE IF NOT EXISTS supply_convoys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			home_encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			target_raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
			state VARCHAR(20) NOT NULL DEFAULT 'marching',
			rations_carried DOUBLE PRECISION NOT NULL DEFAULT 0,
			ammo_carried DOUBLE PRECISION NOT NULL DEFAULT 0,
			electricity_carried DOUBLE PRECISION NOT NULL DEFAULT 0,
			logistics_carried DOUBLE PRECISION NOT NULL DEFAULT 0,
			haulers_committed INT NOT NULL DEFAULT 0,
			tankers_committed INT NOT NULL DEFAULT 0,
			origin_x INT NOT NULL,
			origin_y INT NOT NULL,
			destination_x INT NOT NULL,
			destination_y INT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			resolve_time TIMESTAMP WITH TIME ZONE NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_supply_convoys_pending ON supply_convoys(resolve_time) WHERE state = 'marching';`,
		`CREATE INDEX IF NOT EXISTS idx_supply_convoys_target ON supply_convoys(target_raid_id) WHERE state = 'marching';`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_supply_convoys_active_target ON supply_convoys(target_raid_id) WHERE state = 'marching';`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS high_tech_offline BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS power_outage_ticks INT NOT NULL DEFAULT 0;`,

		// --- MMO Living World Phase 6: persistent AI civilizations.
		// AI factions are REAL encampments (with a synthetic negative-
		// telegram_id "users" row satisfying the existing NOT NULL/UNIQUE
		// FK, seeded once by seedAICivilizations at startup) rather than
		// a special-cased entity, so the entire existing discovery/
		// targeting/raiding/looting pipeline works on them for free -
		// see MMO_WORLD_EVOLUTION_PLAN.md's Phase 6 completed-detail
		// section for why that integration point was chosen.
		`ALTER TABLE encampments ADD COLUMN IF NOT EXISTS is_ai_faction BOOLEAN NOT NULL DEFAULT FALSE;`,
		`ALTER TABLE encampments ADD COLUMN IF NOT EXISTS ai_faction_key VARCHAR(50);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_encampments_ai_faction_key ON encampments(ai_faction_key) WHERE ai_faction_key IS NOT NULL;`,

		// See migrations/035_phase7_notification_preferences.sql for the
		// annotated version. Phase 7 milestone 2: lets a player mute
		// exactly one high-volume, low-stakes category (peaceful road
		// passes, weather clears, convoy arrivals); combat/discovery/
		// supply-loss notifications are never tagged with a mutable
		// category (see notifications/preferences.go's MutableCategories),
		// so this mechanism structurally cannot silence them.
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS category VARCHAR(30) NOT NULL DEFAULT 'general';`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_pending_category ON notifications(category) WHERE is_sent = FALSE;`,
		`CREATE TABLE IF NOT EXISTS notification_preferences (
			user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
			mute_route_status BOOLEAN NOT NULL DEFAULT FALSE,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		// Phase 7 milestone 3's "speed-up expenditure" admin metric needs a
		// structured event to aggregate - nothing else logs this today.
		`CREATE TABLE IF NOT EXISTS speedup_usage_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			scrap_spent DOUBLE PRECISION NOT NULL,
			dollars_spent DOUBLE PRECISION NOT NULL,
			crystal_spent DOUBLE PRECISION NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_speedup_usage_log_created ON speedup_usage_log(created_at);`,

		// See migrations/036_ai_faction_decision_loop.sql for the
		// annotated version. Second half of Phase 6 - see
		// AI_FACTION_DECISION_LOOP_PLAN.md for the full design.
		`ALTER TABLE encampments ADD COLUMN IF NOT EXISTS ai_last_decision_at TIMESTAMP WITH TIME ZONE;`,
		`CREATE TABLE IF NOT EXISTS ai_faction_decisions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			decided_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			intent VARCHAR(20) NOT NULL,
			target_encampment_id UUID REFERENCES encampments(id) ON DELETE SET NULL,
			resulting_raid_id UUID REFERENCES raids(id) ON DELETE SET NULL,
			reason TEXT,
			CONSTRAINT ai_faction_decisions_intent CHECK (intent IN ('scout', 'raid', 'idle'))
		);`,
		`CREATE INDEX IF NOT EXISTS idx_ai_faction_decisions_encampment ON ai_faction_decisions(encampment_id, decided_at DESC);`,

		// AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.4: discovery
		// permanence and Ghost Protocol. known_locations is a NEW concept,
		// distinct from encampment_discoveries (a permanent boolean "have
		// I ever heard of this entity" relationship, which stays
		// untouched) - this table stores an actual coordinate SNAPSHOT,
		// locked at discovery time, so an attacker acts on where they
		// last saw a target rather than re-reading its live position
		// every time. Ghost Protocol (see jobs.go's HandleGhostProtocol)
		// is the only thing that deletes rows here.
		`CREATE TABLE IF NOT EXISTS known_locations (
			observer_encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			target_encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			x INT NOT NULL,
			y INT NOT NULL,
			region VARCHAR(50) NOT NULL,
			locked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (observer_encampment_id, target_encampment_id)
		);`,
		`ALTER TABLE encampments ADD COLUMN IF NOT EXISTS last_ghost_protocol_at TIMESTAMP WITH TIME ZONE;`,
		// AI factions get a new possible decision-loop intent, 'flee'
		// (chosen when a faction's resources have been ground down by
		// repeated raiding - see aidecisions.go's maybeFlee), which needs
		// its own slot in the existing CHECK constraint.
		`ALTER TABLE ai_faction_decisions DROP CONSTRAINT IF EXISTS ai_faction_decisions_intent;`,
		`ALTER TABLE ai_faction_decisions ADD CONSTRAINT ai_faction_decisions_intent CHECK (intent IN ('scout', 'raid', 'idle', 'flee'));`,

		// AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.2: long-range
		// scouting's data model. A wholly separate feature from
		// exploration_sites/exploration_dispatches (guaranteed-resource
		// personal exploration, unrelated purpose) - deliberately its own
		// table, per the plan's explicit instruction not to touch that
		// system for this.
		`CREATE TABLE IF NOT EXISTS scout_missions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			scouts_committed INT NOT NULL,
			phase VARCHAR(20) NOT NULL DEFAULT 'searching',
			started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			found_target_encampment_id UUID REFERENCES encampments(id) ON DELETE SET NULL,
			found_at TIMESTAMP WITH TIME ZONE,
			found_x INT,
			found_y INT,
			found_region VARCHAR(50),
			return_eta TIMESTAMP WITH TIME ZONE,
			return_leg_started_at TIMESTAMP WITH TIME ZONE,
			return_leg_total_minutes DOUBLE PRECISION,
			origin_x INT,
			origin_y INT,
			bonus_discovery_encampment_id UUID REFERENCES encampments(id) ON DELETE SET NULL,
			last_status_notified_at TIMESTAMP WITH TIME ZONE,
			resources_returned_summary TEXT,
			completed_at TIMESTAMP WITH TIME ZONE,
			CONSTRAINT scout_missions_phase CHECK (phase IN ('searching', 'returning', 'complete'))
		);`,
		`CREATE INDEX IF NOT EXISTS idx_scout_missions_searching ON scout_missions(encampment_id) WHERE phase = 'searching';`,
		`CREATE INDEX IF NOT EXISTS idx_scout_missions_returning ON scout_missions(return_eta) WHERE phase = 'returning';`,
	}
}
