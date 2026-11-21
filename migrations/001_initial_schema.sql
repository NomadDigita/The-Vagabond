
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
    telegram_id BIGINT PRIMARY KEY,
    username VARCHAR(100),
    first_name VARCHAR(100) NOT NULL,
    state VARCHAR(50) NOT NULL DEFAULT 'onboarding',
    registered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_active TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_state ON users(state);

CREATE TABLE IF NOT EXISTS coordinates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    x INT NOT NULL,
    y INT NOT NULL,
    biome VARCHAR(50) NOT NULL DEFAULT 'wasteland',
    danger_level INT DEFAULT 1,
    scrap_multiplier NUMERIC(3, 2) DEFAULT 1.00,
    rations_multiplier NUMERIC(3, 2) DEFAULT 1.00,
    energy_multiplier NUMERIC(3, 2) DEFAULT 1.00,
    UNIQUE (x, y)
);

CREATE INDEX IF NOT EXISTS idx_coordinates_grid ON coordinates(x, y);

CREATE TABLE IF NOT EXISTS encampments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(telegram_id) ON DELETE CASCADE,
    name VARCHAR(100) DEFAULT 'Unnamed Camp',
    coordinate_id UUID NOT NULL REFERENCES coordinates(id),
    level INT NOT NULL DEFAULT 1,
    established_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_encampments_user ON encampments(user_id);
CREATE INDEX IF NOT EXISTS idx_encampments_coordinate ON encampments(coordinate_id);

CREATE TABLE IF NOT EXISTS resources (
    encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
    scrap NUMERIC(12, 2) DEFAULT 100.00,
    rations NUMERIC(12, 2) DEFAULT 50.00,
    energy NUMERIC(12, 2) DEFAULT 25.00,
    neuro_cores NUMERIC(12, 2) DEFAULT 0.00,
    last_ticked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS modules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    level INT NOT NULL DEFAULT 1,
    is_upgrading BOOLEAN DEFAULT FALSE,
    upgrade_ready_at TIMESTAMP WITH TIME ZONE,
    UNIQUE (encampment_id, type)
);

CREATE INDEX IF NOT EXISTS idx_modules_lookup ON modules(encampment_id, type);

CREATE TABLE IF NOT EXISTS units (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    quantity INT NOT NULL DEFAULT 0,
    health INT DEFAULT 100,
    morale INT DEFAULT 100,
    status VARCHAR(30) DEFAULT 'idle'
);

CREATE INDEX IF NOT EXISTS idx_units_encampment ON units(encampment_id);

CREATE TABLE IF NOT EXISTS raids (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    attacker_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
    defender_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
    state VARCHAR(30) DEFAULT 'marching',
    launch_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    resolve_time TIMESTAMP WITH TIME ZONE NOT NULL,
    return_time TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_raids_state ON raids(state);
CREATE INDEX IF NOT EXISTS idx_raids_timings ON raids(resolve_time, return_time);

CREATE TABLE IF NOT EXISTS battle_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
    attacker_loss_scrap NUMERIC(12, 2) DEFAULT 0.00,
    defender_loss_scrap NUMERIC(12, 2) DEFAULT 0.00,
    casualties_attacker JSONB DEFAULT '[]'::jsonb,
    casualties_defender JSONB DEFAULT '[]'::jsonb,
    combat_phases_json JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_battle_logs_raid ON battle_logs(raid_id);

CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    is_sent BOOLEAN DEFAULT FALSE,
    queued_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notifications_unsent ON notifications(user_id) WHERE is_sent = FALSE;

CREATE TABLE IF NOT EXISTS agent_tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
    mode VARCHAR(30) NOT NULL DEFAULT 'collector',
    target_module_type VARCHAR(50),
    max_risk_threshold INT DEFAULT 5,
    is_active BOOLEAN DEFAULT TRUE,
    last_run_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_agent_tasks_active ON agent_tasks(user_id) WHERE is_active = TRUE;

CREATE TABLE IF NOT EXISTS leaderboards (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(telegram_id) ON DELETE CASCADE,
    score INT NOT NULL DEFAULT 0,
    category VARCHAR(50) DEFAULT 'scrap_count',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_leaderboard_rank ON leaderboards(score DESC);

CREATE TABLE IF NOT EXISTS world_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(150) NOT NULL,
    description TEXT,
    event_type VARCHAR(50) NOT NULL,
    multiplier NUMERIC(4, 2) DEFAULT 1.00,
    starts_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_world_events_active ON world_events(starts_at, expires_at);

CREATE TABLE IF NOT EXISTS market (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    seller_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
    resource_type VARCHAR(30) NOT NULL,
    quantity NUMERIC(12, 2) NOT NULL,
    price_scrap NUMERIC(12, 2) NOT NULL,
    is_sold BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_market_active ON market(resource_type) WHERE is_sold = FALSE;

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id BIGINT REFERENCES users(telegram_id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    details TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
