CREATE TABLE IF NOT EXISTS heuristic_scenarios (
    heuristic_scenario_id VARCHAR(255) PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scheme_id INT NOT NULL REFERENCES schemes(scheme_id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    target_color VARCHAR(64) NOT NULL,
    required_target_count INT NOT NULL CHECK (required_target_count > 0),
    formation_track_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    buffer_track_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    main_track_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    feasible BOOLEAN NOT NULL DEFAULT TRUE,
    reasons_json JSONB NOT NULL DEFAULT '[]',
    metrics_json JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS heuristic_scenario_steps (
    step_id VARCHAR(255) PRIMARY KEY,
    heuristic_scenario_id VARCHAR(255) NOT NULL REFERENCES heuristic_scenarios(heuristic_scenario_id) ON DELETE CASCADE,
    step_order INT NOT NULL,
    step_type VARCHAR(64) NOT NULL,
    source_track_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    destination_track_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    source_side VARCHAR(16),
    wagon_count INT NOT NULL CHECK (wagon_count >= 0),
    target_color VARCHAR(64) NOT NULL,
    formation_track_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    buffer_track_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    main_track_id VARCHAR(255) NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_heuristic_scenarios_user_id ON heuristic_scenarios(user_id);
CREATE INDEX IF NOT EXISTS idx_heuristic_scenarios_scheme_id ON heuristic_scenarios(scheme_id);
CREATE INDEX IF NOT EXISTS idx_heuristic_scenario_steps_order ON heuristic_scenario_steps(heuristic_scenario_id, step_order);
