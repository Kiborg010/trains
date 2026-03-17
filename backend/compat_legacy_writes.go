package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"trains/backend/normalized"
)

func createLegacyLayoutNormalizedFirst(userID int, name string, state LayoutState) (*Layout, error) {
	normalizedResult, err := BuildNormalizedSchemeFromLegacyLayout(name, state)
	if err != nil {
		return nil, err
	}

	switch store := appStore.(type) {
	case *InMemoryStore:
		return createLegacyLayoutNormalizedFirstInMemory(store, userID, name, state, normalizedResult.Scheme)
	case *PostgresStore:
		return createLegacyLayoutNormalizedFirstPostgres(store, userID, name, state, normalizedResult.Scheme)
	default:
		return nil, fmt.Errorf("normalized-first layout writes are not supported for this store")
	}
}

func updateLegacyLayoutNormalizedFirst(userID int, layoutID int, name string, state LayoutState) error {
	normalizedResult, err := BuildNormalizedSchemeFromLegacyLayout(name, state)
	if err != nil {
		return err
	}

	switch store := appStore.(type) {
	case *InMemoryStore:
		return updateLegacyLayoutNormalizedFirstInMemory(store, userID, layoutID, name, state, normalizedResult.Scheme)
	case *PostgresStore:
		return updateLegacyLayoutNormalizedFirstPostgres(store, userID, layoutID, name, state, normalizedResult.Scheme)
	default:
		return fmt.Errorf("normalized-first layout writes are not supported for this store")
	}
}

func deleteLegacyLayoutNormalizedFirst(userID int, layoutID int) error {
	switch store := appStore.(type) {
	case *InMemoryStore:
		return deleteLegacyLayoutNormalizedFirstInMemory(store, userID, layoutID)
	case *PostgresStore:
		return deleteLegacyLayoutNormalizedFirstPostgres(store, userID, layoutID)
	default:
		return fmt.Errorf("normalized-first layout deletes are not supported for this store")
	}
}

func createLegacyScenarioNormalizedFirst(userID int, layoutID int, name string) (*Scenario, error) {
	switch store := appStore.(type) {
	case *InMemoryStore:
		return createLegacyScenarioNormalizedFirstInMemory(store, userID, layoutID, name)
	case *PostgresStore:
		return createLegacyScenarioNormalizedFirstPostgres(store, userID, layoutID, name)
	default:
		return nil, fmt.Errorf("normalized-first scenario writes are not supported for this store")
	}
}

func deleteLegacyScenarioNormalizedFirst(userID int, scenarioID string) error {
	switch store := appStore.(type) {
	case *InMemoryStore:
		return deleteLegacyScenarioNormalizedFirstInMemory(store, userID, scenarioID)
	case *PostgresStore:
		return deleteLegacyScenarioNormalizedFirstPostgres(store, userID, scenarioID)
	default:
		return fmt.Errorf("normalized-first scenario deletes are not supported for this store")
	}
}

func appendLegacyScenarioCommandNormalizedFirst(userID int, scenarioID string, command CommandSpec) error {
	scenario, err := getLegacyScenarioFromNormalized(userID, scenarioID)
	if err != nil {
		return err
	}

	nextCommands := append([]CommandSpec{}, scenario.Commands...)
	nextCommands = append(nextCommands, command)
	normalizedCommands, err := BuildNormalizedScenarioStepsFromLegacyCommands(nextCommands)
	if err != nil {
		return err
	}

	switch store := appStore.(type) {
	case *InMemoryStore:
		return appendLegacyScenarioCommandNormalizedFirstInMemory(store, userID, scenarioID, nextCommands, normalizedCommands.Steps)
	case *PostgresStore:
		return appendLegacyScenarioCommandNormalizedFirstPostgres(store, userID, scenarioID, nextCommands, normalizedCommands.Steps)
	default:
		return fmt.Errorf("normalized-first scenario command writes are not supported for this store")
	}
}

func createLegacyLayoutNormalizedFirstInMemory(store *InMemoryStore, userID int, name string, state LayoutState, scheme normalized.Scheme) (*Layout, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	schemeID := store.nextSchemeID
	store.nextSchemeID++
	if store.nextLayoutID <= schemeID {
		store.nextLayoutID = schemeID + 1
	}

	scheme.SchemeID = schemeID
	assignSchemeID(&scheme)
	store.schemesByID[schemeID] = cloneNormalizedScheme(scheme)

	now := time.Now()
	layout := Layout{
		ID:        schemeID,
		UserID:    userID,
		Name:      name,
		State:     cloneLayoutState(state),
		CreatedAt: now,
		UpdatedAt: now,
	}
	store.layoutsByID[schemeID] = layout
	return &layout, nil
}

func updateLegacyLayoutNormalizedFirstInMemory(store *InMemoryStore, userID int, layoutID int, name string, state LayoutState, scheme normalized.Scheme) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	layout, ok := store.layoutsByID[layoutID]
	if !ok || layout.UserID != userID {
		return fmt.Errorf("layout not found")
	}
	if _, ok := store.schemesByID[layoutID]; !ok {
		return fmt.Errorf("normalized scheme not found for layout %d", layoutID)
	}

	scheme.SchemeID = layoutID
	assignSchemeID(&scheme)
	store.schemesByID[layoutID] = cloneNormalizedScheme(scheme)

	layout.Name = name
	layout.State = cloneLayoutState(state)
	layout.UpdatedAt = time.Now()
	store.layoutsByID[layoutID] = layout
	return nil
}

func deleteLegacyLayoutNormalizedFirstInMemory(store *InMemoryStore, userID int, layoutID int) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	layout, ok := store.layoutsByID[layoutID]
	if !ok || layout.UserID != userID {
		return fmt.Errorf("layout not found")
	}
	if _, ok := store.schemesByID[layoutID]; !ok {
		return fmt.Errorf("normalized scheme not found for layout %d", layoutID)
	}

	delete(store.layoutsByID, layoutID)
	delete(store.schemesByID, layoutID)

	for scenarioID, scenario := range store.scenariosByID {
		if scenario.LayoutID != layoutID || scenario.UserID != userID {
			continue
		}
		delete(store.scenariosByID, scenarioID)
		for executionID, execution := range store.executionsByID {
			if execution.ScenarioID == scenarioID && execution.UserID == userID {
				delete(store.executionsByID, executionID)
			}
		}
	}
	return nil
}

func createLegacyScenarioNormalizedFirstInMemory(store *InMemoryStore, userID int, layoutID int, name string) (*Scenario, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	layout, ok := store.layoutsByID[layoutID]
	if !ok || layout.UserID != userID {
		return nil, fmt.Errorf("layout not found")
	}
	if _, ok := store.schemesByID[layoutID]; !ok {
		return nil, fmt.Errorf("normalized scheme not found for layout %d", layoutID)
	}

	id := strconv.Itoa(store.nextScenarioID)
	store.nextScenarioID++
	scenario := Scenario{
		ID:          id,
		UserID:      userID,
		LayoutID:    layoutID,
		Name:        name,
		Commands:    []CommandSpec{},
		CommandsMap: map[string]CommandSpec{},
	}
	store.scenariosByID[id] = scenario
	return &scenario, nil
}

func deleteLegacyScenarioNormalizedFirstInMemory(store *InMemoryStore, userID int, scenarioID string) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	scenario, ok := store.scenariosByID[scenarioID]
	if !ok || scenario.UserID != userID {
		return fmt.Errorf("scenario not found")
	}
	delete(store.scenariosByID, scenarioID)
	for executionID, execution := range store.executionsByID {
		if execution.ScenarioID == scenarioID && execution.UserID == userID {
			delete(store.executionsByID, executionID)
		}
	}
	return nil
}

func appendLegacyScenarioCommandNormalizedFirstInMemory(store *InMemoryStore, userID int, scenarioID string, commands []CommandSpec, steps []normalized.ScenarioStep) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	scenario, ok := store.scenariosByID[scenarioID]
	if !ok || scenario.UserID != userID {
		return fmt.Errorf("scenario not found")
	}
	if _, ok := store.layoutsByID[scenario.LayoutID]; !ok {
		return fmt.Errorf("shadow layout not found for scenario %s", scenarioID)
	}

	scenario.Commands = append([]CommandSpec{}, commands...)
	scenario.CommandsMap = map[string]CommandSpec{}
	for _, command := range scenario.Commands {
		scenario.CommandsMap[command.ID] = command
	}
	store.scenariosByID[scenarioID] = scenario
	_ = steps
	return nil
}

func createLegacyLayoutNormalizedFirstPostgres(store *PostgresStore, userID int, name string, state LayoutState, scheme normalized.Scheme) (*Layout, error) {
	tx, err := store.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var schemeID int
	if err := tx.QueryRow(`INSERT INTO schemes (name) VALUES ($1) RETURNING scheme_id`, name).Scan(&schemeID); err != nil {
		return nil, err
	}

	scheme.SchemeID = schemeID
	assignSchemeID(&scheme)
	if err := replaceNormalizedSchemeDataTx(tx, schemeID, scheme); err != nil {
		return nil, err
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if _, err := tx.Exec(`
		INSERT INTO layouts (id, user_id, name, state, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, schemeID, userID, name, stateJSON, now); err != nil {
		return nil, err
	}
	if err := syncSerialSequenceTx(tx, "layouts", "id"); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Layout{
		ID:        schemeID,
		UserID:    userID,
		Name:      name,
		State:     state,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func updateLegacyLayoutNormalizedFirstPostgres(store *PostgresStore, userID int, layoutID int, name string, state LayoutState, scheme normalized.Scheme) error {
	tx, err := store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := ensureShadowLayoutExistsTx(tx, userID, layoutID); err != nil {
		return err
	}
	if err := ensureNormalizedSchemeExistsTx(tx, layoutID); err != nil {
		return err
	}

	if _, err := tx.Exec(`UPDATE schemes SET name = $1 WHERE scheme_id = $2`, name, layoutID); err != nil {
		return err
	}

	scheme.SchemeID = layoutID
	assignSchemeID(&scheme)
	if err := replaceNormalizedSchemeDataTx(tx, layoutID, scheme); err != nil {
		return err
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return err
	}

	result, err := tx.Exec(`UPDATE layouts SET name = $1, state = $2, updated_at = $3 WHERE id = $4 AND user_id = $5`, name, stateJSON, time.Now(), layoutID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("layout not found")
	}

	return tx.Commit()
}

func deleteLegacyLayoutNormalizedFirstPostgres(store *PostgresStore, userID int, layoutID int) error {
	tx, err := store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := ensureShadowLayoutExistsTx(tx, userID, layoutID); err != nil {
		return err
	}
	if err := ensureNormalizedSchemeExistsTx(tx, layoutID); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM layouts WHERE id = $1 AND user_id = $2`, layoutID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM schemes WHERE scheme_id = $1`, layoutID); err != nil {
		return err
	}
	return tx.Commit()
}

func createLegacyScenarioNormalizedFirstPostgres(store *PostgresStore, userID int, layoutID int, name string) (*Scenario, error) {
	tx, err := store.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := ensureShadowLayoutExistsTx(tx, userID, layoutID); err != nil {
		return nil, err
	}
	if err := ensureNormalizedSchemeExistsTx(tx, layoutID); err != nil {
		return nil, err
	}

	commandsJSON, err := json.Marshal([]CommandSpec{})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var scenarioRowID int
	if err := tx.QueryRow(`
		INSERT INTO scenarios (user_id, layout_id, scheme_id, name, commands, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id
	`, userID, layoutID, layoutID, name, commandsJSON, now).Scan(&scenarioRowID); err != nil {
		return nil, err
	}

	scenarioID := strconv.Itoa(scenarioRowID)
	if _, err := tx.Exec(`UPDATE scenarios SET scenario_id = $1 WHERE id = $2`, scenarioID, scenarioRowID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Scenario{
		ID:          scenarioID,
		UserID:      userID,
		LayoutID:    layoutID,
		Name:        name,
		Commands:    []CommandSpec{},
		CommandsMap: map[string]CommandSpec{},
	}, nil
}

func deleteLegacyScenarioNormalizedFirstPostgres(store *PostgresStore, userID int, scenarioID string) error {
	tx, err := store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := ensureShadowScenarioExistsTx(tx, userID, scenarioID); err != nil {
		return err
	}
	scenarioRowID, err := strconv.Atoi(scenarioID)
	if err != nil {
		return fmt.Errorf("invalid scenario id")
	}
	if _, err := tx.Exec(`DELETE FROM scenarios WHERE id = $1 AND user_id = $2`, scenarioRowID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func appendLegacyScenarioCommandNormalizedFirstPostgres(store *PostgresStore, userID int, scenarioID string, commands []CommandSpec, steps []normalized.ScenarioStep) error {
	tx, err := store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := ensureShadowScenarioExistsTx(tx, userID, scenarioID); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM scenario_steps WHERE scenario_id = $1`, scenarioID); err != nil {
		return err
	}
	for _, step := range withScenarioIDForSteps(scenarioID, steps) {
		if _, err := tx.Exec(`
			INSERT INTO scenario_steps (
				step_id, scenario_id, step_order, step_type, from_track_id, from_index,
				to_track_id, to_index, object1_id, object2_id, payload_json
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, step.StepID, step.ScenarioID, step.StepOrder, step.StepType, step.FromTrackID, step.FromIndex, step.ToTrackID, step.ToIndex, step.Object1ID, step.Object2ID, nullJSON(step.PayloadJSON)); err != nil {
			return err
		}
	}

	commandsJSON, err := json.Marshal(commands)
	if err != nil {
		return err
	}
	scenarioRowID, err := strconv.Atoi(scenarioID)
	if err != nil {
		return fmt.Errorf("invalid scenario id")
	}
	result, err := tx.Exec(`UPDATE scenarios SET commands = $1, updated_at = $2 WHERE id = $3 AND user_id = $4`, commandsJSON, time.Now(), scenarioRowID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("scenario not found")
	}
	return tx.Commit()
}

func ensureShadowLayoutExistsTx(tx *sql.Tx, userID int, layoutID int) error {
	var id int
	if err := tx.QueryRow(`SELECT id FROM layouts WHERE id = $1 AND user_id = $2`, layoutID, userID).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("layout not found")
		}
		return err
	}
	return nil
}

func ensureNormalizedSchemeExistsTx(tx *sql.Tx, schemeID int) error {
	var id int
	if err := tx.QueryRow(`SELECT scheme_id FROM schemes WHERE scheme_id = $1`, schemeID).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("normalized scheme not found")
		}
		return err
	}
	return nil
}

func ensureShadowScenarioExistsTx(tx *sql.Tx, userID int, scenarioID string) error {
	scenarioRowID, err := strconv.Atoi(scenarioID)
	if err != nil {
		return fmt.Errorf("invalid scenario id")
	}
	var id int
	if err := tx.QueryRow(`SELECT id FROM scenarios WHERE id = $1 AND user_id = $2`, scenarioRowID, userID).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("scenario not found")
		}
		return err
	}
	return nil
}

func replaceNormalizedSchemeDataTx(tx *sql.Tx, schemeID int, scheme normalized.Scheme) error {
	if err := replaceTracksTx(tx, schemeID, scheme.Tracks); err != nil {
		return err
	}
	if err := replaceTrackConnectionsTx(tx, schemeID, scheme.TrackConnections); err != nil {
		return err
	}
	if err := replaceWagonsTx(tx, schemeID, scheme.Wagons); err != nil {
		return err
	}
	if err := replaceLocomotivesTx(tx, schemeID, scheme.Locomotives); err != nil {
		return err
	}
	if err := replaceNormalizedCouplingsTx(tx, schemeID, scheme.Couplings); err != nil {
		return err
	}
	return nil
}

func replaceTracksTx(tx *sql.Tx, schemeID int, tracks []normalized.Track) error {
	if _, err := tx.Exec(`DELETE FROM tracks WHERE scheme_id = $1`, schemeID); err != nil {
		return err
	}
	for _, track := range withSchemeIDForTracks(schemeID, tracks) {
		if _, err := tx.Exec(`
			INSERT INTO tracks (track_id, scheme_id, name, type, start_x, start_y, end_x, end_y, capacity, storage_allowed)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, track.TrackID, track.SchemeID, track.Name, track.Type, track.StartX, track.StartY, track.EndX, track.EndY, track.Capacity, track.StorageAllowed); err != nil {
			return err
		}
	}
	return nil
}

func replaceTrackConnectionsTx(tx *sql.Tx, schemeID int, connections []normalized.TrackConnection) error {
	if _, err := tx.Exec(`DELETE FROM track_connections WHERE scheme_id = $1`, schemeID); err != nil {
		return err
	}
	for _, item := range withSchemeIDForConnections(schemeID, connections) {
		if _, err := tx.Exec(`
			INSERT INTO track_connections (connection_id, scheme_id, track1_id, track2_id, track1_side, track2_side, connection_type)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, item.ConnectionID, item.SchemeID, item.Track1ID, item.Track2ID, item.Track1Side, item.Track2Side, item.ConnectionType); err != nil {
			return err
		}
	}
	return nil
}

func replaceWagonsTx(tx *sql.Tx, schemeID int, wagons []normalized.Wagon) error {
	if _, err := tx.Exec(`DELETE FROM wagons WHERE scheme_id = $1`, schemeID); err != nil {
		return err
	}
	for _, item := range withSchemeIDForWagons(schemeID, wagons) {
		if _, err := tx.Exec(`
			INSERT INTO wagons (wagon_id, scheme_id, name, color, track_id, track_index)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, item.WagonID, item.SchemeID, item.Name, item.Color, item.TrackID, item.TrackIndex); err != nil {
			return err
		}
	}
	return nil
}

func replaceLocomotivesTx(tx *sql.Tx, schemeID int, locomotives []normalized.Locomotive) error {
	if _, err := tx.Exec(`DELETE FROM locomotives WHERE scheme_id = $1`, schemeID); err != nil {
		return err
	}
	for _, item := range withSchemeIDForLocomotives(schemeID, locomotives) {
		if _, err := tx.Exec(`
			INSERT INTO locomotives (loco_id, scheme_id, name, color, track_id, track_index)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, item.LocoID, item.SchemeID, item.Name, item.Color, item.TrackID, item.TrackIndex); err != nil {
			return err
		}
	}
	return nil
}

func replaceNormalizedCouplingsTx(tx *sql.Tx, schemeID int, couplings []normalized.Coupling) error {
	if _, err := tx.Exec(`DELETE FROM couplings WHERE scheme_id = $1`, schemeID); err != nil {
		return err
	}
	for _, item := range withSchemeIDForCouplings(schemeID, couplings) {
		if _, err := tx.Exec(`
			INSERT INTO couplings (coupling_id, scheme_id, object1_id, object2_id)
			VALUES ($1,$2,$3,$4)
		`, item.CouplingID, item.SchemeID, item.Object1ID, item.Object2ID); err != nil {
			return err
		}
	}
	return nil
}

func syncSerialSequenceTx(tx *sql.Tx, tableName string, columnName string) error {
	query := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s', '%s'), COALESCE((SELECT MAX(%s) FROM %s), 1), true)`, tableName, columnName, columnName, tableName)
	_, err := tx.Exec(query)
	return err
}
