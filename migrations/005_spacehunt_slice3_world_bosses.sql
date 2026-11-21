
CREATE TABLE IF NOT EXISTS world_bosses (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name VARCHAR(255) UNIQUE NOT NULL,
	emoji VARCHAR(10) DEFAULT '👹',
	max_hp DOUBLE PRECISION NOT NULL,
	current_hp DOUBLE PRECISION NOT NULL,
	loot_pool_dollars DOUBLE PRECISION DEFAULT 0,
	last_defeated_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS world_boss_contributions (
	boss_id UUID NOT NULL REFERENCES world_bosses(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
	encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
	damage_dealt DOUBLE PRECISION DEFAULT 0,
	PRIMARY KEY (boss_id, user_id)
);

INSERT INTO world_bosses (name, emoji, max_hp, current_hp, loot_pool_dollars) VALUES
	('The Rustlord', '🤖👹', 500000, 500000, 5000),
	('Scrap Titan', '⚙️👹', 1200000, 1200000, 12000),
	('Apex Wraith', '☠️👹', 3000000, 3000000, 30000)
	ON CONFLICT (name) DO NOTHING;
