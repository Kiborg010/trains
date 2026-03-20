package main

import (
	"encoding/json"
	"fmt"
	"time"

	"trains/backend/normalized"
)

func (s *InMemoryStore) CreateNormalizedScheme(userID int, scheme normalized.Scheme) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	schemeID := s.nextSchemeID
	s.nextSchemeID++
	scheme.SchemeID = schemeID
	assignSchemeID(&scheme)
	s.schemesByID[schemeID] = cloneNormalizedScheme(scheme)
	return schemeID, nil
}

func (s *InMemoryStore) GetNormalizedScheme(schemeID int, userID int) (*normalized.Scheme, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scheme, ok := s.schemesByID[schemeID]
	if !ok {
		return nil, fmt.Errorf("схема не найдена")
	}
	copy := cloneNormalizedScheme(scheme)
	return &copy, nil
}

func (s *InMemoryStore) ListNormalizedSchemes(userID int) ([]normalized.Scheme, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]normalized.Scheme, 0, len(s.schemesByID))
	for _, scheme := range s.schemesByID {
		result = append(result, cloneNormalizedScheme(scheme))
	}
	return result, nil
}

func (s *InMemoryStore) UpdateNormalizedScheme(userID int, scheme normalized.Scheme) error {
	return s.updateSchemePart(scheme.SchemeID, func(current *normalized.Scheme) {
		current.Name = scheme.Name
		assignSchemeID(&scheme)
		current.Tracks = cloneTracks(scheme.Tracks)
		current.TrackConnections = cloneTrackConnections(scheme.TrackConnections)
		current.Wagons = cloneWagons(scheme.Wagons)
		current.Locomotives = cloneLocomotives(scheme.Locomotives)
		current.Couplings = cloneNormalizedCouplings(scheme.Couplings)
	})
}

func (s *InMemoryStore) DeleteNormalizedScheme(userID int, schemeID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schemesByID[schemeID]; !ok {
		return fmt.Errorf("схема не найдена")
	}
	delete(s.schemesByID, schemeID)

	for scenarioID, scenario := range s.normalizedScenariosByID {
		if scenario.SchemeID == schemeID {
			delete(s.normalizedScenariosByID, scenarioID)
		}
	}
	return nil
}

func (s *InMemoryStore) CreateTracks(userID int, schemeID int, tracks []normalized.Track) error {
	return s.updateSchemePart(schemeID, func(scheme *normalized.Scheme) {
		scheme.Tracks = cloneTracks(withSchemeIDForTracks(schemeID, tracks))
	})
}

func (s *InMemoryStore) GetTracksByScheme(userID int, schemeID int) ([]normalized.Track, error) {
	return s.ListTracksByScheme(userID, schemeID)
}

func (s *InMemoryStore) ListTracksByScheme(userID int, schemeID int) ([]normalized.Track, error) {
	scheme, err := s.GetNormalizedScheme(schemeID, userID)
	if err != nil {
		return nil, err
	}
	return cloneTracks(scheme.Tracks), nil
}

func (s *InMemoryStore) CreateTrackConnections(userID int, schemeID int, connections []normalized.TrackConnection) error {
	return s.updateSchemePart(schemeID, func(scheme *normalized.Scheme) {
		scheme.TrackConnections = cloneTrackConnections(withSchemeIDForConnections(schemeID, connections))
	})
}

func (s *InMemoryStore) GetTrackConnectionsByScheme(userID int, schemeID int) ([]normalized.TrackConnection, error) {
	return s.ListTrackConnectionsByScheme(userID, schemeID)
}

func (s *InMemoryStore) ListTrackConnectionsByScheme(userID int, schemeID int) ([]normalized.TrackConnection, error) {
	scheme, err := s.GetNormalizedScheme(schemeID, userID)
	if err != nil {
		return nil, err
	}
	return cloneTrackConnections(scheme.TrackConnections), nil
}

func (s *InMemoryStore) CreateWagons(userID int, schemeID int, wagons []normalized.Wagon) error {
	return s.updateSchemePart(schemeID, func(scheme *normalized.Scheme) {
		scheme.Wagons = cloneWagons(withSchemeIDForWagons(schemeID, wagons))
	})
}

func (s *InMemoryStore) GetWagonsByScheme(userID int, schemeID int) ([]normalized.Wagon, error) {
	return s.ListWagonsByScheme(userID, schemeID)
}

func (s *InMemoryStore) ListWagonsByScheme(userID int, schemeID int) ([]normalized.Wagon, error) {
	scheme, err := s.GetNormalizedScheme(schemeID, userID)
	if err != nil {
		return nil, err
	}
	return cloneWagons(scheme.Wagons), nil
}

func (s *InMemoryStore) CreateLocomotives(userID int, schemeID int, locomotives []normalized.Locomotive) error {
	return s.updateSchemePart(schemeID, func(scheme *normalized.Scheme) {
		scheme.Locomotives = cloneLocomotives(withSchemeIDForLocomotives(schemeID, locomotives))
	})
}

func (s *InMemoryStore) GetLocomotivesByScheme(userID int, schemeID int) ([]normalized.Locomotive, error) {
	return s.ListLocomotivesByScheme(userID, schemeID)
}

func (s *InMemoryStore) ListLocomotivesByScheme(userID int, schemeID int) ([]normalized.Locomotive, error) {
	scheme, err := s.GetNormalizedScheme(schemeID, userID)
	if err != nil {
		return nil, err
	}
	return cloneLocomotives(scheme.Locomotives), nil
}

func (s *InMemoryStore) CreateNormalizedCouplings(userID int, schemeID int, couplings []normalized.Coupling) error {
	return s.updateSchemePart(schemeID, func(scheme *normalized.Scheme) {
		scheme.Couplings = cloneNormalizedCouplings(withSchemeIDForCouplings(schemeID, couplings))
	})
}

func (s *InMemoryStore) GetNormalizedCouplingsByScheme(userID int, schemeID int) ([]normalized.Coupling, error) {
	return s.ListNormalizedCouplingsByScheme(userID, schemeID)
}

func (s *InMemoryStore) ListNormalizedCouplingsByScheme(userID int, schemeID int) ([]normalized.Coupling, error) {
	scheme, err := s.GetNormalizedScheme(schemeID, userID)
	if err != nil {
		return nil, err
	}
	return cloneNormalizedCouplings(scheme.Couplings), nil
}

func (s *InMemoryStore) CreateNormalizedScenario(userID int, scenario normalized.Scenario) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if scenario.ScenarioID == "" {
		scenario.ScenarioID = fmt.Sprintf("nsc-%d", time.Now().UnixNano())
	}
	s.normalizedScenariosByID[scenario.ScenarioID] = normalized.Scenario{
		ScenarioID:                scenario.ScenarioID,
		SchemeID:                  scenario.SchemeID,
		Name:                      scenario.Name,
		SourceHeuristicScenarioID: cloneOptionalString(scenario.SourceHeuristicScenarioID),
		Steps:                     cloneScenarioSteps(scenario.Steps),
	}
	return scenario.ScenarioID, nil
}

func (s *InMemoryStore) GetNormalizedScenario(id string, userID int) (*normalized.Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scenario, ok := s.normalizedScenariosByID[id]
	if !ok {
		return nil, fmt.Errorf("сценарий не найден")
	}
	copy := normalized.Scenario{
		ScenarioID:                scenario.ScenarioID,
		SchemeID:                  scenario.SchemeID,
		Name:                      scenario.Name,
		SourceHeuristicScenarioID: cloneOptionalString(scenario.SourceHeuristicScenarioID),
		Steps:                     cloneScenarioSteps(scenario.Steps),
	}
	return &copy, nil
}

func (s *InMemoryStore) ListNormalizedScenarios(userID int) ([]normalized.Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]normalized.Scenario, 0, len(s.normalizedScenariosByID))
	for _, scenario := range s.normalizedScenariosByID {
		result = append(result, normalized.Scenario{
			ScenarioID:                scenario.ScenarioID,
			SchemeID:                  scenario.SchemeID,
			Name:                      scenario.Name,
			SourceHeuristicScenarioID: cloneOptionalString(scenario.SourceHeuristicScenarioID),
			Steps:                     cloneScenarioSteps(scenario.Steps),
		})
	}
	return result, nil
}

func (s *InMemoryStore) UpdateNormalizedScenario(userID int, scenario normalized.Scenario) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.normalizedScenariosByID[scenario.ScenarioID]; !ok {
		return fmt.Errorf("сценарий не найден")
	}
	s.normalizedScenariosByID[scenario.ScenarioID] = normalized.Scenario{
		ScenarioID:                scenario.ScenarioID,
		SchemeID:                  scenario.SchemeID,
		Name:                      scenario.Name,
		SourceHeuristicScenarioID: cloneOptionalString(scenario.SourceHeuristicScenarioID),
		Steps:                     cloneScenarioSteps(withScenarioIDForSteps(scenario.ScenarioID, scenario.Steps)),
	}
	return nil
}

func (s *InMemoryStore) DeleteNormalizedScenario(userID int, scenarioID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.normalizedScenariosByID[scenarioID]; !ok {
		return fmt.Errorf("сценарий не найден")
	}
	if heuristicScenarioID := s.normalizedScenariosByID[scenarioID].SourceHeuristicScenarioID; heuristicScenarioID != nil && *heuristicScenarioID != "" {
		delete(s.heuristicScenariosByID, *heuristicScenarioID)
	}
	delete(s.normalizedScenariosByID, scenarioID)
	return nil
}

func (s *InMemoryStore) CreateScenarioSteps(userID int, scenarioID string, steps []normalized.ScenarioStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scenario, ok := s.normalizedScenariosByID[scenarioID]
	if !ok {
		return fmt.Errorf("сценарий не найден")
	}
	scenario.Steps = cloneScenarioSteps(withScenarioIDForSteps(scenarioID, steps))
	s.normalizedScenariosByID[scenarioID] = scenario
	return nil
}

func (s *InMemoryStore) GetScenarioStepsByScenario(userID int, scenarioID string) ([]normalized.ScenarioStep, error) {
	return s.ListScenarioStepsByScenario(userID, scenarioID)
}

func (s *InMemoryStore) ListScenarioStepsByScenario(userID int, scenarioID string) ([]normalized.ScenarioStep, error) {
	scenario, err := s.GetNormalizedScenario(scenarioID, userID)
	if err != nil {
		return nil, err
	}
	return cloneScenarioSteps(scenario.Steps), nil
}

func (s *InMemoryStore) CreateHeuristicScenario(userID int, scenario normalized.HeuristicScenario) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if scenario.HeuristicScenarioID == "" {
		scenario.HeuristicScenarioID = fmt.Sprintf("nhs-%d", time.Now().UnixNano())
	}
	scenario.Steps = withHeuristicScenarioIDForSteps(scenario.HeuristicScenarioID, scenario.Steps)
	s.heuristicScenariosByID[scenario.HeuristicScenarioID] = cloneHeuristicScenario(scenario)
	return scenario.HeuristicScenarioID, nil
}

func (s *InMemoryStore) GetHeuristicScenario(id string, userID int) (*normalized.HeuristicScenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scenario, ok := s.heuristicScenariosByID[id]
	if !ok {
		return nil, fmt.Errorf("эвристический сценарий не найден")
	}
	copyScenario := cloneHeuristicScenario(scenario)
	return &copyScenario, nil
}

func (s *InMemoryStore) ListHeuristicScenarios(userID int) ([]normalized.HeuristicScenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]normalized.HeuristicScenario, 0, len(s.heuristicScenariosByID))
	for _, scenario := range s.heuristicScenariosByID {
		copyScenario := cloneHeuristicScenario(scenario)
		copyScenario.Steps = nil
		result = append(result, copyScenario)
	}
	return result, nil
}

func (s *InMemoryStore) DeleteHeuristicScenario(userID int, heuristicScenarioID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.heuristicScenariosByID[heuristicScenarioID]; !ok {
		return fmt.Errorf("эвристический сценарий не найден")
	}
	delete(s.heuristicScenariosByID, heuristicScenarioID)
	return nil
}

func (s *InMemoryStore) CreateHeuristicScenarioSteps(userID int, heuristicScenarioID string, steps []normalized.HeuristicScenarioStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scenario, ok := s.heuristicScenariosByID[heuristicScenarioID]
	if !ok {
		return fmt.Errorf("эвристический сценарий не найден")
	}
	scenario.Steps = cloneHeuristicScenarioSteps(withHeuristicScenarioIDForSteps(heuristicScenarioID, steps))
	s.heuristicScenariosByID[heuristicScenarioID] = cloneHeuristicScenario(scenario)
	return nil
}

func (s *InMemoryStore) ListHeuristicScenarioStepsByScenario(userID int, heuristicScenarioID string) ([]normalized.HeuristicScenarioStep, error) {
	scenario, err := s.GetHeuristicScenario(heuristicScenarioID, userID)
	if err != nil {
		return nil, err
	}
	return cloneHeuristicScenarioSteps(scenario.Steps), nil
}

func (s *InMemoryStore) updateSchemePart(schemeID int, updater func(*normalized.Scheme)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scheme, ok := s.schemesByID[schemeID]
	if !ok {
		return fmt.Errorf("схема не найдена")
	}
	updater(&scheme)
	s.schemesByID[schemeID] = cloneNormalizedScheme(scheme)
	return nil
}

func (s *PostgresStore) CreateNormalizedScheme(userID int, scheme normalized.Scheme) (int, error) {
	var schemeID int
	if err := s.db.QueryRow(`INSERT INTO schemes (user_id, name) VALUES ($1, $2) RETURNING scheme_id`, userID, scheme.Name).Scan(&schemeID); err != nil {
		return 0, err
	}
	assignSchemeID(&scheme)
	scheme.SchemeID = schemeID
	if err := s.replaceNormalizedSchemeData(schemeID, scheme); err != nil {
		return 0, err
	}
	return schemeID, nil
}

func (s *PostgresStore) GetNormalizedScheme(schemeID int, userID int) (*normalized.Scheme, error) {
	var scheme normalized.Scheme
	if err := s.db.QueryRow(`SELECT scheme_id, name FROM schemes WHERE scheme_id = $1 AND user_id = $2`, schemeID, userID).Scan(&scheme.SchemeID, &scheme.Name); err != nil {
		return nil, err
	}

	var err error
	if scheme.Tracks, err = s.ListTracksByScheme(userID, schemeID); err != nil {
		return nil, err
	}
	if scheme.TrackConnections, err = s.ListTrackConnectionsByScheme(userID, schemeID); err != nil {
		return nil, err
	}
	if scheme.Wagons, err = s.ListWagonsByScheme(userID, schemeID); err != nil {
		return nil, err
	}
	if scheme.Locomotives, err = s.ListLocomotivesByScheme(userID, schemeID); err != nil {
		return nil, err
	}
	if scheme.Couplings, err = s.ListNormalizedCouplingsByScheme(userID, schemeID); err != nil {
		return nil, err
	}
	return &scheme, nil
}

func (s *PostgresStore) ListNormalizedSchemes(userID int) ([]normalized.Scheme, error) {
	rows, err := s.db.Query(`SELECT scheme_id, name FROM schemes WHERE user_id = $1 ORDER BY scheme_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]normalized.Scheme, 0)
	for rows.Next() {
		var scheme normalized.Scheme
		if err := rows.Scan(&scheme.SchemeID, &scheme.Name); err != nil {
			return nil, err
		}
		result = append(result, scheme)
	}
	return result, rows.Err()
}

func (s *PostgresStore) UpdateNormalizedScheme(userID int, scheme normalized.Scheme) error {
	result, err := s.db.Exec(`UPDATE schemes SET name = $1 WHERE scheme_id = $2 AND user_id = $3`, scheme.Name, scheme.SchemeID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("схема не найдена")
	}
	assignSchemeID(&scheme)
	return s.replaceNormalizedSchemeData(scheme.SchemeID, scheme)
}

func (s *PostgresStore) DeleteNormalizedScheme(userID int, schemeID int) error {
	result, err := s.db.Exec(`DELETE FROM schemes WHERE scheme_id = $1 AND user_id = $2`, schemeID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("схема не найдена")
	}
	return nil
}

func (s *PostgresStore) CreateTracks(userID int, schemeID int, tracks []normalized.Track) error {
	return s.replaceTracks(schemeID, tracks)
}

func (s *PostgresStore) GetTracksByScheme(userID int, schemeID int) ([]normalized.Track, error) {
	return s.ListTracksByScheme(userID, schemeID)
}

func (s *PostgresStore) ListTracksByScheme(userID int, schemeID int) ([]normalized.Track, error) {
	rows, err := s.db.Query(`
		SELECT track_id, scheme_id, name, type, start_x, start_y, end_x, end_y, capacity, storage_allowed
		FROM tracks
		WHERE scheme_id = $1
		ORDER BY track_id
	`, schemeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]normalized.Track, 0)
	for rows.Next() {
		var track normalized.Track
		if err := rows.Scan(
			&track.TrackID,
			&track.SchemeID,
			&track.Name,
			&track.Type,
			&track.StartX,
			&track.StartY,
			&track.EndX,
			&track.EndY,
			&track.Capacity,
			&track.StorageAllowed,
		); err != nil {
			return nil, err
		}
		result = append(result, track)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateTrackConnections(userID int, schemeID int, connections []normalized.TrackConnection) error {
	return s.replaceTrackConnections(schemeID, connections)
}

func (s *PostgresStore) GetTrackConnectionsByScheme(userID int, schemeID int) ([]normalized.TrackConnection, error) {
	return s.ListTrackConnectionsByScheme(userID, schemeID)
}

func (s *PostgresStore) ListTrackConnectionsByScheme(userID int, schemeID int) ([]normalized.TrackConnection, error) {
	rows, err := s.db.Query(`
		SELECT connection_id, scheme_id, track1_id, track2_id, track1_side, track2_side, connection_type
		FROM track_connections
		WHERE scheme_id = $1
		ORDER BY connection_id
	`, schemeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]normalized.TrackConnection, 0)
	for rows.Next() {
		var item normalized.TrackConnection
		if err := rows.Scan(
			&item.ConnectionID,
			&item.SchemeID,
			&item.Track1ID,
			&item.Track2ID,
			&item.Track1Side,
			&item.Track2Side,
			&item.ConnectionType,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateWagons(userID int, schemeID int, wagons []normalized.Wagon) error {
	return s.replaceWagons(schemeID, wagons)
}

func (s *PostgresStore) GetWagonsByScheme(userID int, schemeID int) ([]normalized.Wagon, error) {
	return s.ListWagonsByScheme(userID, schemeID)
}

func (s *PostgresStore) ListWagonsByScheme(userID int, schemeID int) ([]normalized.Wagon, error) {
	rows, err := s.db.Query(`
		SELECT wagon_id, scheme_id, name, color, track_id, track_index
		FROM wagons
		WHERE scheme_id = $1
		ORDER BY track_id, track_index, wagon_id
	`, schemeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]normalized.Wagon, 0)
	for rows.Next() {
		var item normalized.Wagon
		if err := rows.Scan(&item.WagonID, &item.SchemeID, &item.Name, &item.Color, &item.TrackID, &item.TrackIndex); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateLocomotives(userID int, schemeID int, locomotives []normalized.Locomotive) error {
	return s.replaceLocomotives(schemeID, locomotives)
}

func (s *PostgresStore) GetLocomotivesByScheme(userID int, schemeID int) ([]normalized.Locomotive, error) {
	return s.ListLocomotivesByScheme(userID, schemeID)
}

func (s *PostgresStore) ListLocomotivesByScheme(userID int, schemeID int) ([]normalized.Locomotive, error) {
	rows, err := s.db.Query(`
		SELECT loco_id, scheme_id, name, color, track_id, track_index
		FROM locomotives
		WHERE scheme_id = $1
		ORDER BY track_id, track_index, loco_id
	`, schemeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]normalized.Locomotive, 0)
	for rows.Next() {
		var item normalized.Locomotive
		if err := rows.Scan(&item.LocoID, &item.SchemeID, &item.Name, &item.Color, &item.TrackID, &item.TrackIndex); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateNormalizedCouplings(userID int, schemeID int, couplings []normalized.Coupling) error {
	return s.replaceNormalizedCouplings(schemeID, couplings)
}

func (s *PostgresStore) GetNormalizedCouplingsByScheme(userID int, schemeID int) ([]normalized.Coupling, error) {
	return s.ListNormalizedCouplingsByScheme(userID, schemeID)
}

func (s *PostgresStore) ListNormalizedCouplingsByScheme(userID int, schemeID int) ([]normalized.Coupling, error) {
	rows, err := s.db.Query(`
		SELECT coupling_id, scheme_id, object1_id, object2_id
		FROM couplings
		WHERE scheme_id = $1
		ORDER BY coupling_id
	`, schemeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]normalized.Coupling, 0)
	for rows.Next() {
		var item normalized.Coupling
		if err := rows.Scan(&item.CouplingID, &item.SchemeID, &item.Object1ID, &item.Object2ID); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateNormalizedScenario(userID int, scenario normalized.Scenario) (string, error) {
	if scenario.ScenarioID == "" {
		scenario.ScenarioID = fmt.Sprintf("nsc-%d", time.Now().UnixNano())
	}
	if _, err := s.db.Exec(
		`INSERT INTO scenarios (scenario_id, user_id, scheme_id, name, source_heuristic_scenario_id) VALUES ($1, $2, $3, $4, $5)`,
		scenario.ScenarioID,
		userID,
		scenario.SchemeID,
		scenario.Name,
		scenario.SourceHeuristicScenarioID,
	); err != nil {
		return "", err
	}
	if err := s.CreateScenarioSteps(userID, scenario.ScenarioID, scenario.Steps); err != nil {
		return "", err
	}
	return scenario.ScenarioID, nil
}

func (s *PostgresStore) GetNormalizedScenario(id string, userID int) (*normalized.Scenario, error) {
	var scenario normalized.Scenario
	if err := s.db.QueryRow(
		`SELECT scenario_id, scheme_id, name, source_heuristic_scenario_id FROM scenarios WHERE scenario_id = $1 AND user_id = $2`,
		id,
		userID,
	).Scan(&scenario.ScenarioID, &scenario.SchemeID, &scenario.Name, &scenario.SourceHeuristicScenarioID); err != nil {
		return nil, err
	}
	steps, err := s.ListScenarioStepsByScenario(userID, id)
	if err != nil {
		return nil, err
	}
	scenario.Steps = steps
	return &scenario, nil
}

func (s *PostgresStore) ListNormalizedScenarios(userID int) ([]normalized.Scenario, error) {
	rows, err := s.db.Query(
		`SELECT scenario_id, scheme_id, name, source_heuristic_scenario_id FROM scenarios WHERE user_id = $1 ORDER BY scenario_id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]normalized.Scenario, 0)
	for rows.Next() {
		var scenario normalized.Scenario
		if err := rows.Scan(&scenario.ScenarioID, &scenario.SchemeID, &scenario.Name, &scenario.SourceHeuristicScenarioID); err != nil {
			return nil, err
		}
		result = append(result, scenario)
	}
	return result, rows.Err()
}

func (s *PostgresStore) UpdateNormalizedScenario(userID int, scenario normalized.Scenario) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`UPDATE scenarios SET scheme_id = $1, name = $2, source_heuristic_scenario_id = $3, updated_at = $4 WHERE scenario_id = $5 AND user_id = $6`,
		scenario.SchemeID,
		scenario.Name,
		scenario.SourceHeuristicScenarioID,
		time.Now(),
		scenario.ScenarioID,
		userID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("сценарий не найден")
	}

	if _, err := tx.Exec(`DELETE FROM scenario_steps WHERE scenario_id = $1`, scenario.ScenarioID); err != nil {
		return err
	}
	for _, step := range withScenarioIDForSteps(scenario.ScenarioID, scenario.Steps) {
		if _, err := tx.Exec(`
			INSERT INTO scenario_steps (
				step_id, scenario_id, step_order, step_type, from_track_id, from_index,
				to_track_id, to_index, object1_id, object2_id, payload_json
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, step.StepID, step.ScenarioID, step.StepOrder, step.StepType, step.FromTrackID, step.FromIndex, step.ToTrackID, step.ToIndex, step.Object1ID, step.Object2ID, nullJSON(step.PayloadJSON)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) DeleteNormalizedScenario(userID int, scenarioID string) error {
	var sourceHeuristicScenarioID *string
	if err := s.db.QueryRow(
		`SELECT source_heuristic_scenario_id FROM scenarios WHERE scenario_id = $1 AND user_id = $2`,
		scenarioID,
		userID,
	).Scan(&sourceHeuristicScenarioID); err != nil {
		return fmt.Errorf("сценарий не найден")
	}

	result, err := s.db.Exec(`DELETE FROM scenarios WHERE scenario_id = $1 AND user_id = $2`, scenarioID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("сценарий не найден")
	}
	if sourceHeuristicScenarioID != nil && *sourceHeuristicScenarioID != "" {
		if err := s.DeleteHeuristicScenario(userID, *sourceHeuristicScenarioID); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) CreateScenarioSteps(userID int, scenarioID string, steps []normalized.ScenarioStep) error {
	if _, err := s.db.Exec(`DELETE FROM scenario_steps WHERE scenario_id = $1`, scenarioID); err != nil {
		return err
	}
	for _, step := range withScenarioIDForSteps(scenarioID, steps) {
		if step.StepID == "" {
			step.StepID = fmt.Sprintf("nst-%d", time.Now().UnixNano())
		}
		if _, err := s.db.Exec(`
			INSERT INTO scenario_steps (
				step_id, scenario_id, step_order, step_type, from_track_id, from_index,
				to_track_id, to_index, object1_id, object2_id, payload_json
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, step.StepID, step.ScenarioID, step.StepOrder, step.StepType, step.FromTrackID, step.FromIndex, step.ToTrackID, step.ToIndex, step.Object1ID, step.Object2ID, nullJSON(step.PayloadJSON)); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) GetScenarioStepsByScenario(userID int, scenarioID string) ([]normalized.ScenarioStep, error) {
	return s.ListScenarioStepsByScenario(userID, scenarioID)
}

func (s *PostgresStore) ListScenarioStepsByScenario(userID int, scenarioID string) ([]normalized.ScenarioStep, error) {
	rows, err := s.db.Query(`
		SELECT step_id, scenario_id, step_order, step_type, from_track_id, from_index, to_track_id, to_index, object1_id, object2_id, payload_json
		FROM scenario_steps
		WHERE scenario_id = $1
		ORDER BY step_order, step_id
	`, scenarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]normalized.ScenarioStep, 0)
	for rows.Next() {
		var step normalized.ScenarioStep
		var payload []byte
		if err := rows.Scan(
			&step.StepID,
			&step.ScenarioID,
			&step.StepOrder,
			&step.StepType,
			&step.FromTrackID,
			&step.FromIndex,
			&step.ToTrackID,
			&step.ToIndex,
			&step.Object1ID,
			&step.Object2ID,
			&payload,
		); err != nil {
			return nil, err
		}
		step.PayloadJSON = payload
		result = append(result, step)
	}
	return result, rows.Err()
}

func (s *PostgresStore) replaceNormalizedSchemeData(schemeID int, scheme normalized.Scheme) error {
	if err := s.replaceTracks(schemeID, scheme.Tracks); err != nil {
		return err
	}
	if err := s.replaceTrackConnections(schemeID, scheme.TrackConnections); err != nil {
		return err
	}
	if err := s.replaceWagons(schemeID, scheme.Wagons); err != nil {
		return err
	}
	if err := s.replaceLocomotives(schemeID, scheme.Locomotives); err != nil {
		return err
	}
	if err := s.replaceNormalizedCouplings(schemeID, scheme.Couplings); err != nil {
		return err
	}
	return nil
}

func (s *PostgresStore) replaceTracks(schemeID int, tracks []normalized.Track) error {
	if _, err := s.db.Exec(`DELETE FROM tracks WHERE scheme_id = $1`, schemeID); err != nil {
		return err
	}
	for _, track := range withSchemeIDForTracks(schemeID, tracks) {
		if _, err := s.db.Exec(`
			INSERT INTO tracks (track_id, scheme_id, name, type, start_x, start_y, end_x, end_y, capacity, storage_allowed)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, track.TrackID, track.SchemeID, track.Name, track.Type, track.StartX, track.StartY, track.EndX, track.EndY, track.Capacity, track.StorageAllowed); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) replaceTrackConnections(schemeID int, connections []normalized.TrackConnection) error {
	if _, err := s.db.Exec(`DELETE FROM track_connections WHERE scheme_id = $1`, schemeID); err != nil {
		return err
	}
	for _, item := range withSchemeIDForConnections(schemeID, connections) {
		if _, err := s.db.Exec(`
			INSERT INTO track_connections (connection_id, scheme_id, track1_id, track2_id, track1_side, track2_side, connection_type)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, item.ConnectionID, item.SchemeID, item.Track1ID, item.Track2ID, item.Track1Side, item.Track2Side, item.ConnectionType); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) replaceWagons(schemeID int, wagons []normalized.Wagon) error {
	if _, err := s.db.Exec(`DELETE FROM wagons WHERE scheme_id = $1`, schemeID); err != nil {
		return err
	}
	for _, item := range withSchemeIDForWagons(schemeID, wagons) {
		if _, err := s.db.Exec(`
			INSERT INTO wagons (wagon_id, scheme_id, name, color, track_id, track_index)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, item.WagonID, item.SchemeID, item.Name, item.Color, item.TrackID, item.TrackIndex); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) replaceLocomotives(schemeID int, locomotives []normalized.Locomotive) error {
	if _, err := s.db.Exec(`DELETE FROM locomotives WHERE scheme_id = $1`, schemeID); err != nil {
		return err
	}
	for _, item := range withSchemeIDForLocomotives(schemeID, locomotives) {
		if _, err := s.db.Exec(`
			INSERT INTO locomotives (loco_id, scheme_id, name, color, track_id, track_index)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, item.LocoID, item.SchemeID, item.Name, item.Color, item.TrackID, item.TrackIndex); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) replaceNormalizedCouplings(schemeID int, couplings []normalized.Coupling) error {
	if _, err := s.db.Exec(`DELETE FROM couplings WHERE scheme_id = $1`, schemeID); err != nil {
		return err
	}
	for _, item := range withSchemeIDForCouplings(schemeID, couplings) {
		if _, err := s.db.Exec(`
			INSERT INTO couplings (coupling_id, scheme_id, object1_id, object2_id)
			VALUES ($1,$2,$3,$4)
		`, item.CouplingID, item.SchemeID, item.Object1ID, item.Object2ID); err != nil {
			return err
		}
	}
	return nil
}

func assignSchemeID(scheme *normalized.Scheme) {
	scheme.Tracks = withSchemeIDForTracks(scheme.SchemeID, scheme.Tracks)
	scheme.TrackConnections = withSchemeIDForConnections(scheme.SchemeID, scheme.TrackConnections)
	scheme.Wagons = withSchemeIDForWagons(scheme.SchemeID, scheme.Wagons)
	scheme.Locomotives = withSchemeIDForLocomotives(scheme.SchemeID, scheme.Locomotives)
	scheme.Couplings = withSchemeIDForCouplings(scheme.SchemeID, scheme.Couplings)
}

func withSchemeIDForTracks(schemeID int, tracks []normalized.Track) []normalized.Track {
	result := cloneTracks(tracks)
	for i := range result {
		result[i].SchemeID = schemeID
	}
	return result
}

func withSchemeIDForConnections(schemeID int, items []normalized.TrackConnection) []normalized.TrackConnection {
	result := cloneTrackConnections(items)
	for i := range result {
		result[i].SchemeID = schemeID
	}
	return result
}

func withSchemeIDForWagons(schemeID int, items []normalized.Wagon) []normalized.Wagon {
	result := cloneWagons(items)
	for i := range result {
		result[i].SchemeID = schemeID
	}
	return result
}

func withSchemeIDForLocomotives(schemeID int, items []normalized.Locomotive) []normalized.Locomotive {
	result := cloneLocomotives(items)
	for i := range result {
		result[i].SchemeID = schemeID
	}
	return result
}

func withSchemeIDForCouplings(schemeID int, items []normalized.Coupling) []normalized.Coupling {
	result := cloneNormalizedCouplings(items)
	for i := range result {
		result[i].SchemeID = schemeID
	}
	return result
}

func withScenarioIDForSteps(scenarioID string, steps []normalized.ScenarioStep) []normalized.ScenarioStep {
	result := cloneScenarioSteps(steps)
	for i := range result {
		result[i].ScenarioID = scenarioID
	}
	return result
}

func withHeuristicScenarioIDForSteps(heuristicScenarioID string, steps []normalized.HeuristicScenarioStep) []normalized.HeuristicScenarioStep {
	result := cloneHeuristicScenarioSteps(steps)
	for i := range result {
		if result[i].StepID == "" {
			result[i].StepID = fmt.Sprintf("nhss-%d-%d", time.Now().UnixNano(), i)
		}
		result[i].HeuristicScenarioID = heuristicScenarioID
	}
	return result
}

func cloneNormalizedScheme(scheme normalized.Scheme) normalized.Scheme {
	return normalized.Scheme{
		SchemeID:         scheme.SchemeID,
		Name:             scheme.Name,
		Tracks:           cloneTracks(scheme.Tracks),
		TrackConnections: cloneTrackConnections(scheme.TrackConnections),
		Wagons:           cloneWagons(scheme.Wagons),
		Locomotives:      cloneLocomotives(scheme.Locomotives),
		Couplings:        cloneNormalizedCouplings(scheme.Couplings),
	}
}

func cloneTracks(items []normalized.Track) []normalized.Track {
	result := make([]normalized.Track, len(items))
	copy(result, items)
	return result
}

func cloneTrackConnections(items []normalized.TrackConnection) []normalized.TrackConnection {
	result := make([]normalized.TrackConnection, len(items))
	copy(result, items)
	return result
}

func cloneWagons(items []normalized.Wagon) []normalized.Wagon {
	result := make([]normalized.Wagon, len(items))
	copy(result, items)
	return result
}

func cloneLocomotives(items []normalized.Locomotive) []normalized.Locomotive {
	result := make([]normalized.Locomotive, len(items))
	copy(result, items)
	return result
}

func cloneNormalizedCouplings(items []normalized.Coupling) []normalized.Coupling {
	result := make([]normalized.Coupling, len(items))
	copy(result, items)
	return result
}

func cloneScenarioSteps(items []normalized.ScenarioStep) []normalized.ScenarioStep {
	result := make([]normalized.ScenarioStep, len(items))
	copy(result, items)
	return result
}

func cloneOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func cloneHeuristicScenario(scenario normalized.HeuristicScenario) normalized.HeuristicScenario {
	return normalized.HeuristicScenario{
		HeuristicScenarioID: scenario.HeuristicScenarioID,
		SchemeID:            scenario.SchemeID,
		Name:                scenario.Name,
		TargetColor:         scenario.TargetColor,
		RequiredTargetCount: scenario.RequiredTargetCount,
		FormationTrackID:    scenario.FormationTrackID,
		BufferTrackID:       scenario.BufferTrackID,
		MainTrackID:         scenario.MainTrackID,
		Feasible:            scenario.Feasible,
		Reasons:             append([]string{}, scenario.Reasons...),
		MetricsJSON:         append(json.RawMessage{}, scenario.MetricsJSON...),
		Steps:               cloneHeuristicScenarioSteps(scenario.Steps),
	}
}

func cloneHeuristicScenarioSteps(items []normalized.HeuristicScenarioStep) []normalized.HeuristicScenarioStep {
	result := make([]normalized.HeuristicScenarioStep, len(items))
	copy(result, items)
	return result
}

func nullJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
