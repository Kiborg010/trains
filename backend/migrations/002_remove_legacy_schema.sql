-- Final legacy cleanup migration: remove legacy JSON persistence and keep normalized model only.

ALTER TABLE IF EXISTS executions DROP CONSTRAINT IF EXISTS executions_scenario_id_fkey;

ALTER TABLE IF EXISTS executions
    ALTER COLUMN scenario_id TYPE VARCHAR(255) USING scenario_id::VARCHAR;

ALTER TABLE IF EXISTS executions
    RENAME COLUMN current_command TO current_step;

ALTER TABLE IF EXISTS schemes
    ADD COLUMN IF NOT EXISTS user_id INT REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

ALTER TABLE IF EXISTS scenarios
    ADD COLUMN IF NOT EXISTS user_id INT REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE IF EXISTS scenarios
    DROP CONSTRAINT IF EXISTS scenarios_pkey;

ALTER TABLE IF EXISTS scenarios
    DROP COLUMN IF EXISTS commands,
    DROP COLUMN IF EXISTS layout_id,
    DROP COLUMN IF EXISTS id;

ALTER TABLE IF EXISTS scenarios
    ALTER COLUMN scenario_id SET NOT NULL,
    ALTER COLUMN scheme_id SET NOT NULL;

ALTER TABLE IF EXISTS scenarios
    ADD CONSTRAINT scenarios_pkey PRIMARY KEY (scenario_id);

ALTER TABLE IF EXISTS executions
    ADD CONSTRAINT executions_scenario_id_fkey FOREIGN KEY (scenario_id) REFERENCES scenarios(scenario_id) ON DELETE CASCADE;

DROP TABLE IF EXISTS layouts CASCADE;

CREATE INDEX IF NOT EXISTS idx_schemes_user_id ON schemes(user_id);
CREATE INDEX IF NOT EXISTS idx_scenarios_user_id ON scenarios(user_id);
CREATE INDEX IF NOT EXISTS idx_executions_user_id ON executions(user_id);
