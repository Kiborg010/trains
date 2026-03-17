package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"trains/backend/normalized"
)

// Store interface for data persistence
type Store interface {
	// User operations
	CreateUser(email string, passwordHash string) (*User, error)
	GetUserByEmail(email string) (*User, error)
	GetUserByID(id int) (*User, error)

	// Layout operations
	SaveLayout(userID int, name string, state LayoutState) (int, error)
	GetLayout(id int, userID int) (*Layout, error)
	ListLayouts(userID int) ([]Layout, error)
	UpdateLayout(id int, userID int, name string, state LayoutState) error
	DeleteLayout(id int, userID int) error

	// Scenario operations
	SaveScenario(userID int, layoutID int, name string, commands []CommandSpec) (string, error)
	GetScenario(id string) (*Scenario, error)
	ListScenarios(userID int) ([]Scenario, error)
	UpdateScenarioCommands(id string, userID int, commands []CommandSpec) error
	DeleteScenario(id string, userID int) error

	// Execution operations
	SaveExecution(userID int, scenarioID string) (string, error)
	GetExecution(id string, userID int) (*Execution, error)
	UpdateExecution(id string, userID int, execution Execution) error

	// Normalized model operations
	CreateNormalizedScheme(userID int, scheme normalized.Scheme) (int, error)
	GetNormalizedScheme(schemeID int, userID int) (*normalized.Scheme, error)
	ListNormalizedSchemes(userID int) ([]normalized.Scheme, error)

	CreateTracks(userID int, schemeID int, tracks []normalized.Track) error
	GetTracksByScheme(userID int, schemeID int) ([]normalized.Track, error)
	ListTracksByScheme(userID int, schemeID int) ([]normalized.Track, error)

	CreateTrackConnections(userID int, schemeID int, connections []normalized.TrackConnection) error
	GetTrackConnectionsByScheme(userID int, schemeID int) ([]normalized.TrackConnection, error)
	ListTrackConnectionsByScheme(userID int, schemeID int) ([]normalized.TrackConnection, error)

	CreateWagons(userID int, schemeID int, wagons []normalized.Wagon) error
	GetWagonsByScheme(userID int, schemeID int) ([]normalized.Wagon, error)
	ListWagonsByScheme(userID int, schemeID int) ([]normalized.Wagon, error)

	CreateLocomotives(userID int, schemeID int, locomotives []normalized.Locomotive) error
	GetLocomotivesByScheme(userID int, schemeID int) ([]normalized.Locomotive, error)
	ListLocomotivesByScheme(userID int, schemeID int) ([]normalized.Locomotive, error)

	CreateNormalizedCouplings(userID int, schemeID int, couplings []normalized.Coupling) error
	GetNormalizedCouplingsByScheme(userID int, schemeID int) ([]normalized.Coupling, error)
	ListNormalizedCouplingsByScheme(userID int, schemeID int) ([]normalized.Coupling, error)

	CreateNormalizedScenario(userID int, scenario normalized.Scenario) (string, error)
	GetNormalizedScenario(id string, userID int) (*normalized.Scenario, error)
	ListNormalizedScenarios(userID int) ([]normalized.Scenario, error)

	CreateScenarioSteps(userID int, scenarioID string, steps []normalized.ScenarioStep) error
	GetScenarioStepsByScenario(userID int, scenarioID string) ([]normalized.ScenarioStep, error)
	ListScenarioStepsByScenario(userID int, scenarioID string) ([]normalized.ScenarioStep, error)
}

// InMemoryStore implements Store interface for local development/testing fallback.
type InMemoryStore struct {
	mu             sync.Mutex
	nextUserID     int
	nextLayoutID   int
	nextSchemeID   int
	usersByID      map[int]User
	userIDsByEmail map[string]int
	layoutsByID    map[int]Layout
	scenariosByID  map[string]Scenario
	executionsByID map[string]Execution
	schemesByID    map[int]normalized.Scheme
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		nextUserID:     1,
		nextLayoutID:   1,
		nextSchemeID:   1,
		usersByID:      map[int]User{},
		userIDsByEmail: map[string]int{},
		layoutsByID:    map[int]Layout{},
		scenariosByID:  map[string]Scenario{},
		executionsByID: map[string]Execution{},
		schemesByID:    map[int]normalized.Scheme{},
	}
}

func (s *InMemoryStore) CreateUser(email string, passwordHash string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.userIDsByEmail[email]; exists {
		return nil, ErrUserExists
	}

	now := time.Now()
	user := User{
		ID:           s.nextUserID,
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.nextUserID++
	s.usersByID[user.ID] = user
	s.userIDsByEmail[email] = user.ID
	return &user, nil
}

func (s *InMemoryStore) GetUserByEmail(email string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.userIDsByEmail[email]
	if !ok {
		return nil, ErrUserNotFound
	}
	user := s.usersByID[id]
	return &user, nil
}

func (s *InMemoryStore) GetUserByID(id int) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.usersByID[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

func (s *InMemoryStore) SaveLayout(userID int, name string, state LayoutState) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	layout := Layout{
		ID:        s.nextLayoutID,
		UserID:    userID,
		Name:      name,
		State:     state,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.nextLayoutID++
	s.layoutsByID[layout.ID] = layout
	return layout.ID, nil
}

func (s *InMemoryStore) GetLayout(id int, userID int) (*Layout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	layout, ok := s.layoutsByID[id]
	if !ok || layout.UserID != userID {
		return nil, fmt.Errorf("layout not found")
	}
	return &layout, nil
}

func (s *InMemoryStore) ListLayouts(userID int) ([]Layout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Layout, 0)
	for _, layout := range s.layoutsByID {
		if layout.UserID == userID {
			result = append(result, layout)
		}
	}
	return result, nil
}

func (s *InMemoryStore) UpdateLayout(id int, userID int, name string, state LayoutState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	layout, ok := s.layoutsByID[id]
	if !ok || layout.UserID != userID {
		return fmt.Errorf("layout not found")
	}
	layout.Name = name
	layout.State = state
	layout.UpdatedAt = time.Now()
	s.layoutsByID[id] = layout
	return nil
}

func (s *InMemoryStore) DeleteLayout(id int, userID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	layout, ok := s.layoutsByID[id]
	if !ok || layout.UserID != userID {
		return fmt.Errorf("layout not found")
	}
	delete(s.layoutsByID, id)
	return nil
}

func (s *InMemoryStore) SaveScenario(userID int, layoutID int, name string, commands []CommandSpec) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("sc-%d", time.Now().UnixNano())
	scenario := Scenario{
		ID:       id,
		UserID:   userID,
		LayoutID: layoutID,
		Name:     name,
		Commands: append([]CommandSpec{}, commands...),
	}
	s.scenariosByID[id] = scenario
	return id, nil
}

func (s *InMemoryStore) GetScenario(id string) (*Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scenario, ok := s.scenariosByID[id]
	if !ok {
		return nil, fmt.Errorf("scenario not found")
	}
	return &scenario, nil
}

func (s *InMemoryStore) ListScenarios(userID int) ([]Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Scenario, 0)
	for _, scenario := range s.scenariosByID {
		if scenario.UserID == userID {
			result = append(result, scenario)
		}
	}
	return result, nil
}

func (s *InMemoryStore) UpdateScenarioCommands(id string, userID int, commands []CommandSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scenario, ok := s.scenariosByID[id]
	if !ok || scenario.UserID != userID {
		return fmt.Errorf("scenario not found")
	}
	scenario.Commands = append([]CommandSpec{}, commands...)
	s.scenariosByID[id] = scenario
	return nil
}

func (s *InMemoryStore) DeleteScenario(id string, userID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scenario, ok := s.scenariosByID[id]
	if !ok || scenario.UserID != userID {
		return fmt.Errorf("scenario not found")
	}
	delete(s.scenariosByID, id)
	return nil
}

func (s *InMemoryStore) SaveExecution(userID int, scenarioID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scenario, ok := s.scenariosByID[scenarioID]
	if !ok || scenario.UserID != userID {
		return "", fmt.Errorf("scenario not found")
	}
	layout, ok := s.layoutsByID[scenario.LayoutID]
	if !ok || layout.UserID != userID {
		return "", fmt.Errorf("layout not found")
	}

	id := fmt.Sprintf("ex-%d", time.Now().UnixNano())
	execution := Execution{
		ID:             id,
		UserID:         userID,
		ScenarioID:     scenarioID,
		Status:         "running",
		CurrentCommand: 0,
		State:          layout.State,
		Log:            []string{"execution created"},
	}
	s.executionsByID[id] = execution
	return id, nil
}

func (s *InMemoryStore) GetExecution(id string, userID int) (*Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	execution, ok := s.executionsByID[id]
	if !ok || execution.UserID != userID {
		return nil, fmt.Errorf("execution not found")
	}
	return &execution, nil
}

func (s *InMemoryStore) UpdateExecution(id string, userID int, execution Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.executionsByID[id]
	if !ok || existing.UserID != userID {
		return fmt.Errorf("execution not found")
	}
	execution.ID = id
	execution.UserID = userID
	s.executionsByID[id] = execution
	return nil
}

// PostgresStore implements Store interface using PostgreSQL
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(connString string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &PostgresStore{db: db}, nil
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// CreateUser creates a new user
func (s *PostgresStore) CreateUser(email string, passwordHash string) (*User, error) {
	var id int
	var createdAt time.Time

	err := s.db.QueryRow(
		"INSERT INTO users (email, password_hash, created_at, updated_at) VALUES ($1, $2, $3, $3) RETURNING id, created_at",
		email, passwordHash, time.Now(),
	).Scan(&id, &createdAt)

	if err != nil {
		if err.Error() == "pq: duplicate key value violates unique constraint \"users_email_key\"" {
			return nil, ErrUserExists
		}
		return nil, err
	}

	return &User{
		ID:        id,
		Email:     email,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}, nil
}

// GetUserByEmail retrieves a user by email
func (s *PostgresStore) GetUserByEmail(email string) (*User, error) {
	var user User
	var createdAt, updatedAt time.Time

	err := s.db.QueryRow(
		"SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = $1",
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	user.CreatedAt = createdAt
	user.UpdatedAt = updatedAt
	return &user, nil
}

// GetUserByID retrieves a user by ID
func (s *PostgresStore) GetUserByID(id int) (*User, error) {
	var user User
	var createdAt, updatedAt time.Time

	err := s.db.QueryRow(
		"SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1",
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	user.CreatedAt = createdAt
	user.UpdatedAt = updatedAt
	return &user, nil
}

// SaveLayout saves a layout for a user
func (s *PostgresStore) SaveLayout(userID int, name string, state LayoutState) (int, error) {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return 0, err
	}

	var id int
	err = s.db.QueryRow(
		"INSERT INTO layouts (user_id, name, state, created_at, updated_at) VALUES ($1, $2, $3, $4, $4) RETURNING id",
		userID, name, stateJSON, time.Now(),
	).Scan(&id)

	return id, err
}

// GetLayout retrieves a layout by ID
func (s *PostgresStore) GetLayout(id int, userID int) (*Layout, error) {
	var layout Layout
	var stateJSON []byte
	var createdAt, updatedAt time.Time

	err := s.db.QueryRow(
		"SELECT id, user_id, name, state, created_at, updated_at FROM layouts WHERE id = $1 AND user_id = $2",
		id, userID,
	).Scan(&layout.ID, &layout.UserID, &layout.Name, &stateJSON, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("layout not found")
	}
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(stateJSON, &layout.State)
	if err != nil {
		return nil, err
	}

	layout.CreatedAt = createdAt
	layout.UpdatedAt = updatedAt
	return &layout, nil
}

// ListLayouts lists all layouts for a user
func (s *PostgresStore) ListLayouts(userID int) ([]Layout, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, name, state, created_at, updated_at FROM layouts WHERE user_id = $1 ORDER BY updated_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var layouts []Layout
	for rows.Next() {
		var layout Layout
		var stateJSON []byte
		var createdAt, updatedAt time.Time

		err := rows.Scan(&layout.ID, &layout.UserID, &layout.Name, &stateJSON, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(stateJSON, &layout.State)
		if err != nil {
			return nil, err
		}

		layout.CreatedAt = createdAt
		layout.UpdatedAt = updatedAt
		layouts = append(layouts, layout)
	}

	return layouts, rows.Err()
}

// UpdateLayout updates a layout
func (s *PostgresStore) UpdateLayout(id int, userID int, name string, state LayoutState) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return err
	}

	result, err := s.db.Exec(
		"UPDATE layouts SET name = $1, state = $2, updated_at = $3 WHERE id = $4 AND user_id = $5",
		name, stateJSON, time.Now(), id, userID,
	)
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

	return nil
}

// DeleteLayout deletes a layout
func (s *PostgresStore) DeleteLayout(id int, userID int) error {
	result, err := s.db.Exec(
		"DELETE FROM layouts WHERE id = $1 AND user_id = $2",
		id, userID,
	)
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

	return nil
}

// SaveScenario saves a scenario
func (s *PostgresStore) SaveScenario(userID int, layoutID int, name string, commands []CommandSpec) (string, error) {
	commandsJSON, err := json.Marshal(commands)
	if err != nil {
		return "", err
	}

	var scenarioID int
	err = s.db.QueryRow(
		"INSERT INTO scenarios (user_id, layout_id, name, commands, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5) RETURNING id",
		userID, layoutID, name, commandsJSON, time.Now(),
	).Scan(&scenarioID)

	if err != nil {
		// Compatibility with older schema where initial_state is NOT NULL.
		err = s.db.QueryRow(
			"INSERT INTO scenarios (user_id, layout_id, name, initial_state, commands, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6) RETURNING id",
			userID, layoutID, name, []byte(`{}`), commandsJSON, time.Now(),
		).Scan(&scenarioID)
		if err != nil {
			return "", err
		}
	}
	return strconv.Itoa(scenarioID), nil
}

// GetScenario retrieves a scenario by ID.
func (s *PostgresStore) GetScenario(id string) (*Scenario, error) {
	scenarioID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid scenario id")
	}

	var scenario Scenario
	var scenarioIDValue int
	var layoutID sql.NullInt64
	var commandsJSON []byte

	err = s.db.QueryRow(
		"SELECT id, user_id, layout_id, name, commands FROM scenarios WHERE id = $1",
		scenarioID,
	).Scan(&scenarioIDValue, &scenario.UserID, &layoutID, &scenario.Name, &commandsJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("scenario not found")
	}
	if err != nil {
		return nil, err
	}
	scenario.ID = strconv.Itoa(scenarioIDValue)

	if layoutID.Valid {
		scenario.LayoutID = int(layoutID.Int64)
	}

	err = json.Unmarshal(commandsJSON, &scenario.Commands)
	if err != nil {
		return nil, err
	}

	// Keep the old format with map of scenarios for compatibility
	scenario.CommandsMap = map[string]CommandSpec{}
	for _, cmd := range scenario.Commands {
		scenario.CommandsMap[cmd.ID] = cmd
	}

	return &scenario, nil
}

// ListScenarios lists scenarios for a user
func (s *PostgresStore) ListScenarios(userID int) ([]Scenario, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, layout_id, name, commands FROM scenarios WHERE user_id = $1 ORDER BY updated_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scenarios []Scenario
	for rows.Next() {
		var scenario Scenario
		var scenarioIDValue int
		var layoutID sql.NullInt64
		var commandsJSON []byte

		err := rows.Scan(&scenarioIDValue, &scenario.UserID, &layoutID, &scenario.Name, &commandsJSON)
		if err != nil {
			return nil, err
		}
		scenario.ID = strconv.Itoa(scenarioIDValue)

		if layoutID.Valid {
			scenario.LayoutID = int(layoutID.Int64)
		}

		err = json.Unmarshal(commandsJSON, &scenario.Commands)
		if err != nil {
			return nil, err
		}

		scenarios = append(scenarios, scenario)
	}

	return scenarios, rows.Err()
}

// UpdateScenarioCommands updates the commands of a scenario
func (s *PostgresStore) UpdateScenarioCommands(id string, userID int, commands []CommandSpec) error {
	commandsJSON, err := json.Marshal(commands)
	if err != nil {
		return err
	}

	scenarioID, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("invalid scenario id")
	}

	result, err := s.db.Exec(
		"UPDATE scenarios SET commands = $1, updated_at = $2 WHERE id = $3 AND user_id = $4",
		commandsJSON, time.Now(), scenarioID, userID,
	)
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

	return nil
}

// DeleteScenario deletes a scenario
func (s *PostgresStore) DeleteScenario(id string, userID int) error {
	scenarioID, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("invalid scenario id")
	}

	result, err := s.db.Exec(
		"DELETE FROM scenarios WHERE id = $1 AND user_id = $2",
		scenarioID, userID,
	)
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

	return nil
}

// SaveExecution saves an execution
func (s *PostgresStore) SaveExecution(userID int, scenarioID string) (string, error) {
	scenario, err := s.GetScenario(scenarioID)
	if err != nil {
		return "", err
	}
	if scenario.UserID != userID {
		return "", fmt.Errorf("scenario not found")
	}
	layout, err := s.GetLayout(scenario.LayoutID, userID)
	if err != nil {
		return "", fmt.Errorf("layout not found")
	}

	stateJSON, err := json.Marshal(layout.State)
	if err != nil {
		return "", err
	}

	logJSON, err := json.Marshal([]string{"execution created"})
	if err != nil {
		return "", err
	}

	scenarioIDInt, err := strconv.Atoi(scenarioID)
	if err != nil {
		return "", fmt.Errorf("invalid scenario id")
	}

	var executionID int
	err = s.db.QueryRow(
		"INSERT INTO executions (user_id, scenario_id, status, current_command, state, log, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $7) RETURNING id",
		userID, scenarioIDInt, "running", 0, stateJSON, logJSON, time.Now(),
	).Scan(&executionID)

	if err != nil {
		return "", err
	}
	return strconv.Itoa(executionID), nil
}

// GetExecution retrieves an execution
func (s *PostgresStore) GetExecution(id string, userID int) (*Execution, error) {
	executionID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid execution id")
	}

	var execution Execution
	var scenarioID int
	var stateJSON, logJSON []byte

	err = s.db.QueryRow(
		"SELECT id, user_id, scenario_id, status, current_command, state, log FROM executions WHERE id = $1 AND user_id = $2",
		executionID, userID,
	).Scan(&execution.ID, &execution.UserID, &scenarioID, &execution.Status, &execution.CurrentCommand, &stateJSON, &logJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("execution not found")
	}
	if err != nil {
		return nil, err
	}
	execution.ScenarioID = strconv.Itoa(scenarioID)

	err = json.Unmarshal(stateJSON, &execution.State)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(logJSON, &execution.Log)
	if err != nil {
		return nil, err
	}

	return &execution, nil
}

// UpdateExecution updates an execution
func (s *PostgresStore) UpdateExecution(id string, userID int, execution Execution) error {
	executionID, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("invalid execution id")
	}

	stateJSON, err := json.Marshal(execution.State)
	if err != nil {
		return err
	}

	logJSON, err := json.Marshal(execution.Log)
	if err != nil {
		return err
	}

	result, err := s.db.Exec(
		"UPDATE executions SET status = $1, current_command = $2, state = $3, log = $4, updated_at = $5 WHERE id = $6 AND user_id = $7",
		execution.Status, execution.CurrentCommand, stateJSON, logJSON, time.Now(), executionID, userID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("execution not found")
	}

	return nil
}
