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

type Store interface {
	CreateUser(email string, passwordHash string) (*User, error)
	GetUserByEmail(email string) (*User, error)
	GetUserByID(id int) (*User, error)

	SaveExecution(userID int, scenarioID string) (string, error)
	GetExecution(id string, userID int) (*Execution, error)
	UpdateExecution(id string, userID int, execution Execution) error

	CreateNormalizedScheme(userID int, scheme normalized.Scheme) (int, error)
	GetNormalizedScheme(schemeID int, userID int) (*normalized.Scheme, error)
	ListNormalizedSchemes(userID int) ([]normalized.Scheme, error)
	UpdateNormalizedScheme(userID int, scheme normalized.Scheme) error
	DeleteNormalizedScheme(userID int, schemeID int) error

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
	UpdateNormalizedScenario(userID int, scenario normalized.Scenario) error
	DeleteNormalizedScenario(userID int, scenarioID string) error

	CreateScenarioSteps(userID int, scenarioID string, steps []normalized.ScenarioStep) error
	GetScenarioStepsByScenario(userID int, scenarioID string) ([]normalized.ScenarioStep, error)
	ListScenarioStepsByScenario(userID int, scenarioID string) ([]normalized.ScenarioStep, error)

	CreateHeuristicScenario(userID int, scenario normalized.HeuristicScenario) (string, error)
	GetHeuristicScenario(id string, userID int) (*normalized.HeuristicScenario, error)
	ListHeuristicScenarios(userID int) ([]normalized.HeuristicScenario, error)
	DeleteHeuristicScenario(userID int, heuristicScenarioID string) error
	CreateHeuristicScenarioSteps(userID int, heuristicScenarioID string, steps []normalized.HeuristicScenarioStep) error
	ListHeuristicScenarioStepsByScenario(userID int, heuristicScenarioID string) ([]normalized.HeuristicScenarioStep, error)
}

type InMemoryStore struct {
	mu                      sync.Mutex
	nextUserID              int
	nextSchemeID            int
	usersByID               map[int]User
	userIDsByEmail          map[string]int
	executionsByID          map[string]Execution
	schemesByID             map[int]normalized.Scheme
	normalizedScenariosByID map[string]normalized.Scenario
	heuristicScenariosByID  map[string]normalized.HeuristicScenario
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		nextUserID:              1,
		nextSchemeID:            1,
		usersByID:               map[int]User{},
		userIDsByEmail:          map[string]int{},
		executionsByID:          map[string]Execution{},
		schemesByID:             map[int]normalized.Scheme{},
		normalizedScenariosByID: map[string]normalized.Scenario{},
		heuristicScenariosByID:  map[string]normalized.HeuristicScenario{},
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

func (s *InMemoryStore) SaveExecution(userID int, scenarioID string) (string, error) {
	runtime, err := buildExecutionRuntimeFromNormalized(s, userID, scenarioID)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("ex-%d", time.Now().UnixNano())
	execution := Execution{
		ID:          id,
		UserID:      userID,
		ScenarioID:  scenarioID,
		Status:      "running",
		CurrentStep: 0,
		State:       runtime.State,
		Log:         []string{"execution created"},
	}
	s.executionsByID[id] = execution
	return id, nil
}

func (s *InMemoryStore) GetExecution(id string, userID int) (*Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	execution, ok := s.executionsByID[id]
	if !ok || execution.UserID != userID {
		return nil, fmt.Errorf("выполнение не найдено")
	}
	return &execution, nil
}

func (s *InMemoryStore) UpdateExecution(id string, userID int, execution Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.executionsByID[id]
	if !ok || existing.UserID != userID {
		return fmt.Errorf("выполнение не найдено")
	}
	execution.ID = id
	execution.UserID = userID
	s.executionsByID[id] = execution
	return nil
}

type PostgresStore struct {
	db *sql.DB
}

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

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

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

func (s *PostgresStore) SaveExecution(userID int, scenarioID string) (string, error) {
	runtime, err := buildExecutionRuntimeFromNormalized(s, userID, scenarioID)
	if err != nil {
		return "", err
	}

	stateJSON, err := json.Marshal(runtime.State)
	if err != nil {
		return "", err
	}
	logJSON, err := json.Marshal([]string{"execution created"})
	if err != nil {
		return "", err
	}

	var executionID int
	err = s.db.QueryRow(
		"INSERT INTO executions (user_id, scenario_id, status, current_step, state, log, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $7) RETURNING id",
		userID, scenarioID, "running", 0, stateJSON, logJSON, time.Now(),
	).Scan(&executionID)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(executionID), nil
}

func (s *PostgresStore) GetExecution(id string, userID int) (*Execution, error) {
	executionID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("некорректный идентификатор выполнения")
	}

	var execution Execution
	var stateJSON, logJSON []byte

	err = s.db.QueryRow(
		"SELECT id, user_id, scenario_id, status, current_step, state, log FROM executions WHERE id = $1 AND user_id = $2",
		executionID, userID,
	).Scan(&execution.ID, &execution.UserID, &execution.ScenarioID, &execution.Status, &execution.CurrentStep, &stateJSON, &logJSON)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("выполнение не найдено")
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(stateJSON, &execution.State); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(logJSON, &execution.Log); err != nil {
		return nil, err
	}
	return &execution, nil
}

func (s *PostgresStore) UpdateExecution(id string, userID int, execution Execution) error {
	executionID, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("некорректный идентификатор выполнения")
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
		"UPDATE executions SET status = $1, current_step = $2, state = $3, log = $4, updated_at = $5 WHERE id = $6 AND user_id = $7",
		execution.Status, execution.CurrentStep, stateJSON, logJSON, time.Now(), executionID, userID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("выполнение не найдено")
	}
	return nil
}
