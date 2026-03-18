package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"trains/backend/normalized"
)

func (s *PostgresStore) CreateHeuristicScenario(userID int, scenario normalized.HeuristicScenario) (string, error) {
	if scenario.HeuristicScenarioID == "" {
		scenario.HeuristicScenarioID = fmt.Sprintf("nhs-%d", time.Now().UnixNano())
	}

	if _, err := s.db.Exec(`
		INSERT INTO heuristic_scenarios (
			heuristic_scenario_id, user_id, scheme_id, name, target_color,
			required_target_count, formation_track_id, buffer_track_id, main_track_id,
			feasible, reasons_json, metrics_json
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`,
		scenario.HeuristicScenarioID,
		userID,
		scenario.SchemeID,
		scenario.Name,
		scenario.TargetColor,
		scenario.RequiredTargetCount,
		scenario.FormationTrackID,
		scenario.BufferTrackID,
		scenario.MainTrackID,
		scenario.Feasible,
		nullJSON(mustMarshalJSON(scenario.Reasons)),
		nullJSON(scenario.MetricsJSON),
	); err != nil {
		return "", err
	}

	if err := s.CreateHeuristicScenarioSteps(userID, scenario.HeuristicScenarioID, scenario.Steps); err != nil {
		return "", err
	}

	return scenario.HeuristicScenarioID, nil
}

func (s *PostgresStore) GetHeuristicScenario(id string, userID int) (*normalized.HeuristicScenario, error) {
	var scenario normalized.HeuristicScenario
	var reasonsRaw []byte
	var metricsRaw []byte

	err := s.db.QueryRow(`
		SELECT heuristic_scenario_id, scheme_id, name, target_color, required_target_count,
		       formation_track_id, buffer_track_id, main_track_id, feasible, reasons_json, metrics_json
		FROM heuristic_scenarios
		WHERE heuristic_scenario_id = $1 AND user_id = $2
	`, id, userID).Scan(
		&scenario.HeuristicScenarioID,
		&scenario.SchemeID,
		&scenario.Name,
		&scenario.TargetColor,
		&scenario.RequiredTargetCount,
		&scenario.FormationTrackID,
		&scenario.BufferTrackID,
		&scenario.MainTrackID,
		&scenario.Feasible,
		&reasonsRaw,
		&metricsRaw,
	)
	if err != nil {
		return nil, err
	}

	if len(reasonsRaw) > 0 {
		if err := json.Unmarshal(reasonsRaw, &scenario.Reasons); err != nil {
			return nil, err
		}
	}
	scenario.MetricsJSON = append(json.RawMessage{}, metricsRaw...)

	steps, err := s.ListHeuristicScenarioStepsByScenario(userID, id)
	if err != nil {
		return nil, err
	}
	scenario.Steps = steps
	return &scenario, nil
}

func (s *PostgresStore) ListHeuristicScenarios(userID int) ([]normalized.HeuristicScenario, error) {
	rows, err := s.db.Query(`
		SELECT heuristic_scenario_id, scheme_id, name, target_color, required_target_count,
		       formation_track_id, buffer_track_id, main_track_id, feasible, reasons_json, metrics_json
		FROM heuristic_scenarios
		WHERE user_id = $1
		ORDER BY heuristic_scenario_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]normalized.HeuristicScenario, 0)
	for rows.Next() {
		var scenario normalized.HeuristicScenario
		var reasonsRaw []byte
		var metricsRaw []byte
		if err := rows.Scan(
			&scenario.HeuristicScenarioID,
			&scenario.SchemeID,
			&scenario.Name,
			&scenario.TargetColor,
			&scenario.RequiredTargetCount,
			&scenario.FormationTrackID,
			&scenario.BufferTrackID,
			&scenario.MainTrackID,
			&scenario.Feasible,
			&reasonsRaw,
			&metricsRaw,
		); err != nil {
			return nil, err
		}
		if len(reasonsRaw) > 0 {
			if err := json.Unmarshal(reasonsRaw, &scenario.Reasons); err != nil {
				return nil, err
			}
		}
		scenario.MetricsJSON = append(json.RawMessage{}, metricsRaw...)
		result = append(result, scenario)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateHeuristicScenarioSteps(userID int, heuristicScenarioID string, steps []normalized.HeuristicScenarioStep) error {
	var ownerID int
	if err := s.db.QueryRow(`SELECT user_id FROM heuristic_scenarios WHERE heuristic_scenario_id = $1`, heuristicScenarioID).Scan(&ownerID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("heuristic scenario not found")
		}
		return err
	}
	if ownerID != userID {
		return fmt.Errorf("heuristic scenario not found")
	}

	if _, err := s.db.Exec(`DELETE FROM heuristic_scenario_steps WHERE heuristic_scenario_id = $1`, heuristicScenarioID); err != nil {
		return err
	}

	for _, step := range withHeuristicScenarioIDForSteps(heuristicScenarioID, steps) {
		if _, err := s.db.Exec(`
			INSERT INTO heuristic_scenario_steps (
				step_id, heuristic_scenario_id, step_order, step_type, source_track_id,
				destination_track_id, source_side, wagon_count, target_color,
				formation_track_id, buffer_track_id, main_track_id
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		`,
			step.StepID,
			step.HeuristicScenarioID,
			step.StepOrder,
			step.StepType,
			step.SourceTrackID,
			step.DestinationTrackID,
			step.SourceSide,
			step.WagonCount,
			step.TargetColor,
			step.FormationTrackID,
			step.BufferTrackID,
			step.MainTrackID,
		); err != nil {
			return err
		}
	}

	return nil
}

func (s *PostgresStore) ListHeuristicScenarioStepsByScenario(userID int, heuristicScenarioID string) ([]normalized.HeuristicScenarioStep, error) {
	rows, err := s.db.Query(`
		SELECT step_id, heuristic_scenario_id, step_order, step_type, source_track_id,
		       destination_track_id, source_side, wagon_count, target_color,
		       formation_track_id, buffer_track_id, main_track_id
		FROM heuristic_scenario_steps
		WHERE heuristic_scenario_id = $1
		  AND EXISTS (
		      SELECT 1
		      FROM heuristic_scenarios
		      WHERE heuristic_scenario_id = $1 AND user_id = $2
		  )
		ORDER BY step_order, step_id
	`, heuristicScenarioID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]normalized.HeuristicScenarioStep, 0)
	for rows.Next() {
		var step normalized.HeuristicScenarioStep
		if err := rows.Scan(
			&step.StepID,
			&step.HeuristicScenarioID,
			&step.StepOrder,
			&step.StepType,
			&step.SourceTrackID,
			&step.DestinationTrackID,
			&step.SourceSide,
			&step.WagonCount,
			&step.TargetColor,
			&step.FormationTrackID,
			&step.BufferTrackID,
			&step.MainTrackID,
		); err != nil {
			return nil, err
		}
		result = append(result, step)
	}
	return result, rows.Err()
}

func mustMarshalJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}
