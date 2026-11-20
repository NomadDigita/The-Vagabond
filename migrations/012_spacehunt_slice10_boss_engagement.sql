
ALTER TABLE world_bosses ADD COLUMN IF NOT EXISTS retaliation_rating DOUBLE PRECISION DEFAULT 8.0;
UPDATE world_bosses SET retaliation_rating = 6.0 WHERE name = 'The Rustlord' AND retaliation_rating = 8.0;
UPDATE world_bosses SET retaliation_rating = 12.0 WHERE name = 'Scrap Titan' AND retaliation_rating = 8.0;
UPDATE world_bosses SET retaliation_rating = 22.0 WHERE name = 'Apex Wraith' AND retaliation_rating = 8.0;

CREATE TABLE IF NOT EXISTS world_boss_attacks (
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
);

