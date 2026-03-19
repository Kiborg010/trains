ALTER TABLE scenarios
    ADD COLUMN IF NOT EXISTS source_heuristic_scenario_id VARCHAR(255);

ALTER TABLE scenarios
    DROP CONSTRAINT IF EXISTS scenarios_source_heuristic_scenario_id_fkey;

ALTER TABLE scenarios
    ADD CONSTRAINT scenarios_source_heuristic_scenario_id_fkey
    FOREIGN KEY (source_heuristic_scenario_id)
    REFERENCES heuristic_scenarios(heuristic_scenario_id)
    ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_scenarios_source_heuristic_id
    ON scenarios(source_heuristic_scenario_id);
