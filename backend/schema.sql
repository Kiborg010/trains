-- PostgreSQL schema for trains application

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS layouts (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    -- Legacy compatibility shadow only. Normalized tables are the active source of truth.
    state JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scenarios (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    layout_id INT NOT NULL REFERENCES layouts(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    -- Legacy compatibility shadow only. Normalized tables are the active source of truth.
    commands JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS executions (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scenario_id INT NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
    status VARCHAR(50) DEFAULT 'running',
    current_command INT DEFAULT 0,
    state JSONB NOT NULL,
    log JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indices for better query performance
CREATE INDEX IF NOT EXISTS idx_layouts_user_id ON layouts(user_id);
CREATE INDEX IF NOT EXISTS idx_scenarios_user_id ON scenarios(user_id);
CREATE INDEX IF NOT EXISTS idx_executions_user_id ON executions(user_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Parallel normalized schema (additive, legacy JSON flow remains in place)
CREATE TABLE IF NOT EXISTS schemes (
    scheme_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS tracks (
    track_id VARCHAR(255) PRIMARY KEY,
    scheme_id INT NOT NULL REFERENCES schemes(scheme_id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('normal', 'bypass', 'sorting', 'lead', 'main')),
    start_x DOUBLE PRECISION NOT NULL,
    start_y DOUBLE PRECISION NOT NULL,
    end_x DOUBLE PRECISION NOT NULL,
    end_y DOUBLE PRECISION NOT NULL,
    capacity INT NOT NULL DEFAULT 0 CHECK (capacity >= 0),
    storage_allowed BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS track_connections (
    connection_id VARCHAR(255) PRIMARY KEY,
    scheme_id INT NOT NULL REFERENCES schemes(scheme_id) ON DELETE CASCADE,
    track1_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    track2_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    track1_side VARCHAR(16) NOT NULL CHECK (track1_side IN ('start', 'end')),
    track2_side VARCHAR(16) NOT NULL CHECK (track2_side IN ('start', 'end')),
    connection_type VARCHAR(16) NOT NULL CHECK (connection_type IN ('serial', 'switch'))
);

CREATE TABLE IF NOT EXISTS wagons (
    wagon_id VARCHAR(255) PRIMARY KEY,
    scheme_id INT NOT NULL REFERENCES schemes(scheme_id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    color VARCHAR(64) NOT NULL,
    track_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    track_index INT NOT NULL
);

CREATE TABLE IF NOT EXISTS locomotives (
    loco_id VARCHAR(255) PRIMARY KEY,
    scheme_id INT NOT NULL REFERENCES schemes(scheme_id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    color VARCHAR(64) NOT NULL,
    track_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    track_index INT NOT NULL
);

CREATE TABLE IF NOT EXISTS couplings (
    coupling_id VARCHAR(255) PRIMARY KEY,
    scheme_id INT NOT NULL REFERENCES schemes(scheme_id) ON DELETE CASCADE,
    object1_id VARCHAR(255) NOT NULL,
    object2_id VARCHAR(255) NOT NULL
);

ALTER TABLE scenarios
    ADD COLUMN IF NOT EXISTS scenario_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS scheme_id INT REFERENCES schemes(scheme_id) ON DELETE CASCADE;

ALTER TABLE scenarios
    ALTER COLUMN user_id DROP NOT NULL,
    ALTER COLUMN layout_id DROP NOT NULL;

CREATE TABLE IF NOT EXISTS scenario_steps (
    step_id VARCHAR(255) PRIMARY KEY,
    scenario_id VARCHAR(255) NOT NULL REFERENCES scenarios(scenario_id) ON DELETE CASCADE,
    step_order INT NOT NULL,
    step_type VARCHAR(32) NOT NULL CHECK (step_type IN ('move_loco', 'couple', 'decouple', 'move_group')),
    from_track_id VARCHAR(255) REFERENCES tracks(track_id) ON DELETE SET NULL,
    from_index INT,
    to_track_id VARCHAR(255) REFERENCES tracks(track_id) ON DELETE SET NULL,
    to_index INT,
    object1_id VARCHAR(255),
    object2_id VARCHAR(255),
    payload_json JSONB
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_scenarios_scenario_id_unique ON scenarios(scenario_id) WHERE scenario_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_scenarios_scheme_id ON scenarios(scheme_id);
CREATE INDEX IF NOT EXISTS idx_tracks_scheme_id ON tracks(scheme_id);
CREATE INDEX IF NOT EXISTS idx_track_connections_scheme_id ON track_connections(scheme_id);
CREATE INDEX IF NOT EXISTS idx_wagons_scheme_track_index ON wagons(scheme_id, track_id, track_index);
CREATE INDEX IF NOT EXISTS idx_locomotives_scheme_track_index ON locomotives(scheme_id, track_id, track_index);
CREATE INDEX IF NOT EXISTS idx_scenario_steps_scenario_order ON scenario_steps(scenario_id, step_order);
