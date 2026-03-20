CREATE TABLE IF NOT EXISTS scenario_metrics (
    scenario_id VARCHAR(255) PRIMARY KEY REFERENCES scenarios(scenario_id) ON DELETE CASCADE,
    total_loco_distance INT NOT NULL DEFAULT 0,
    total_couples INT NOT NULL DEFAULT 0,
    total_decouples INT NOT NULL DEFAULT 0,
    total_switch_crossings INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
