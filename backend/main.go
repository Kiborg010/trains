package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Segment struct {
	ID   string `json:"id"`
	From Point  `json:"from"`
	To   Point  `json:"to"`
}

type Vehicle struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Code      string  `json:"code,omitempty"`
	PathID    string  `json:"pathId,omitempty"`
	PathIndex int     `json:"pathIndex,omitempty"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
}

type Coupling struct {
	ID string `json:"id"`
	A  string `json:"a"`
	B  string `json:"b"`
}

type Slot struct {
	ID string
	X  float64
	Y  float64
}

type ValidateCouplingRequest struct {
	GridSize           float64    `json:"gridSize"`
	Segments           []Segment  `json:"segments"`
	Vehicles           []Vehicle  `json:"vehicles"`
	Couplings          []Coupling `json:"couplings"`
	SelectedVehicleIDs []string   `json:"selectedVehicleIds"`
}

type ValidateCouplingResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type PlanMovementRequest struct {
	GridSize             float64    `json:"gridSize"`
	Segments             []Segment  `json:"segments"`
	Vehicles             []Vehicle  `json:"vehicles"`
	Couplings            []Coupling `json:"couplings"`
	SelectedLocomotiveID string     `json:"selectedLocomotiveId"`
	TargetPathID         string     `json:"targetPathId"`
	TargetIndex          int        `json:"targetIndex"`
}

type Position struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

type PathSlot struct {
	PathID string
	Index  int
	X      float64
	Y      float64
}

type PlanMovementResponse struct {
	OK          bool         `json:"ok"`
	Message     string       `json:"message,omitempty"`
	Timeline    [][]Position `json:"timeline,omitempty"`
	CellsPassed int          `json:"cellsPassed,omitempty"`
}

type PlaceVehicleRequest struct {
	GridSize     float64   `json:"gridSize"`
	Segments     []Segment `json:"segments"`
	Vehicles     []Vehicle `json:"vehicles"`
	VehicleType  string    `json:"vehicleType"`
	TargetPathID string    `json:"targetPathId"`
	TargetIndex  int       `json:"targetIndex"`
}

type PlaceVehicleResponse struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message,omitempty"`
	Vehicle *Vehicle `json:"vehicle,omitempty"`
}

type ResolveVehiclesRequest struct {
	GridSize        float64    `json:"gridSize"`
	Segments        []Segment  `json:"segments"`
	Vehicles        []Vehicle  `json:"vehicles"`
	Couplings       []Coupling `json:"couplings"`
	MovedVehicleIDs []string   `json:"movedVehicleIds"`
	StrictCouplings bool       `json:"strictCouplings"`
}

type ResolveVehiclesResponse struct {
	OK       bool      `json:"ok"`
	Message  string    `json:"message,omitempty"`
	Vehicles []Vehicle `json:"vehicles,omitempty"`
}

type LayoutState struct {
	Segments  []Segment   `json:"segments"`
	Vehicles  []Vehicle   `json:"vehicles"`
	Couplings []Coupling  `json:"couplings"`
	Paths     []PathState `json:"paths,omitempty"`
}

type PathState struct {
	ID         string   `json:"id"`
	Capacity   int      `json:"capacity"`
	VehicleIDs []string `json:"vehicleIds,omitempty"`
	Neighbors  []string `json:"neighbors,omitempty"`
}

type LayoutOperationRequest struct {
	GridSize float64     `json:"gridSize"`
	State    LayoutState `json:"state"`
	Action   string      `json:"action"`

	From               *Point   `json:"from,omitempty"`
	To                 *Point   `json:"to,omitempty"`
	IDs                []string `json:"ids,omitempty"`
	SelectedVehicleIDs []string `json:"selectedVehicleIds,omitempty"`
	VehicleType        string   `json:"vehicleType,omitempty"`
	TargetPathID       string   `json:"targetPathId,omitempty"`
	TargetIndex        int      `json:"targetIndex,omitempty"`
	MovedVehicleIDs    []string `json:"movedVehicleIds,omitempty"`
	StrictCouplings    bool     `json:"strictCouplings,omitempty"`
}

type LayoutOperationResponse struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	State   LayoutState `json:"state"`
}

type Scenario struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	InitialState LayoutState   `json:"initialState"`
	Commands     []CommandSpec `json:"commands"`
}

type CommandSpec struct {
	ID      string         `json:"id"`
	Order   int            `json:"order"`
	Type    string         `json:"type"`
	Payload CommandPayload `json:"payload"`
}

type CommandPayload struct {
	LocoID       string `json:"locoId,omitempty"`
	TargetPathID string `json:"targetPathId,omitempty"`
	TargetIndex  int    `json:"targetIndex,omitempty"`
	AID          string `json:"aId,omitempty"`
	BID          string `json:"bId,omitempty"`
}

type Execution struct {
	ID             string      `json:"id"`
	ScenarioID     string      `json:"scenarioId"`
	Status         string      `json:"status"`
	CurrentCommand int         `json:"currentCommand"`
	State          LayoutState `json:"state"`
	Log            []string    `json:"log"`
}

type CreateScenarioRequest struct {
	Name         string      `json:"name"`
	InitialState LayoutState `json:"initialState"`
}

type CreateScenarioResponse struct {
	OK       bool     `json:"ok"`
	Message  string   `json:"message,omitempty"`
	Scenario Scenario `json:"scenario"`
}

type AddCommandRequest struct {
	Type    string         `json:"type"`
	Payload CommandPayload `json:"payload"`
}

type AddCommandResponse struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	Command CommandSpec `json:"command"`
}

type RunScenarioResponse struct {
	OK        bool      `json:"ok"`
	Message   string    `json:"message,omitempty"`
	Execution Execution `json:"execution"`
}

type StepExecutionResponse struct {
	OK        bool      `json:"ok"`
	Message   string    `json:"message,omitempty"`
	Execution Execution `json:"execution"`
}

var scenarioStore = struct {
	mu         sync.Mutex
	scenarios  map[string]Scenario
	executions map[string]Execution
}{
	scenarios:  map[string]Scenario{},
	executions: map[string]Execution{},
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", healthHandler)
	mux.HandleFunc("/api/couplings/validate", validateCouplingHandler)
	mux.HandleFunc("/api/movement/plan", planMovementHandler)
	mux.HandleFunc("/api/vehicles/place", placeVehicleHandler)
	mux.HandleFunc("/api/vehicles/resolve", resolveVehiclesHandler)
	mux.HandleFunc("/api/layout/apply", layoutApplyHandler)
	mux.HandleFunc("/api/scenarios", scenariosHandler)
	mux.HandleFunc("/api/scenarios/", scenarioByIDHandler)
	mux.HandleFunc("/api/executions/", executionByIDHandler)

	handler := withCORS(mux)
	log.Println("backend started on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func validateCouplingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateCouplingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ValidateCouplingResponse{
			OK:      false,
			Message: "invalid json",
		})
		return
	}

	if req.GridSize <= 0 {
		req.GridSize = 32
	}

	resp, err := validateCouplingInternal(req)
	if err != nil {
		writeJSON(w, http.StatusOK, ValidateCouplingResponse{
			OK:      false,
			Message: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func placeVehicleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PlaceVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, PlaceVehicleResponse{
			OK:      false,
			Message: "invalid json",
		})
		return
	}
	if req.GridSize <= 0 {
		req.GridSize = 32
	}

	resp, err := placeVehicleInternal(req)
	if err != nil {
		writeJSON(w, http.StatusOK, PlaceVehicleResponse{
			OK:      false,
			Message: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func validateCouplingInternal(req ValidateCouplingRequest) (ValidateCouplingResponse, error) {
	if len(req.SelectedVehicleIDs) < 2 {
		return ValidateCouplingResponse{OK: false, Message: "Select two vehicles."}, nil
	}

	pathSlots := collectPathSlots(req.Segments, req.GridSize)
	vehicles := make([]Vehicle, 0, len(req.Vehicles))
	for _, v := range req.Vehicles {
		vehicles = append(vehicles, normalizeVehicleToPath(v, pathSlots))
	}

	a := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-2]
	b := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-1]
	if a == b {
		return ValidateCouplingResponse{OK: false, Message: "Cannot couple a vehicle with itself."}, nil
	}

	vehicleByID := make(map[string]Vehicle, len(req.Vehicles))
	for _, v := range vehicles {
		vehicleByID[v.ID] = v
	}

	va, okA := vehicleByID[a]
	vb, okB := vehicleByID[b]
	if !okA || !okB {
		return ValidateCouplingResponse{OK: false, Message: "Selected vehicles were not found."}, nil
	}

	existing := make(map[string]struct{}, len(req.Couplings))
	for _, c := range req.Couplings {
		existing[pairKey(c.A, c.B)] = struct{}{}
	}
	if _, exists := existing[pairKey(a, b)]; exists {
		return ValidateCouplingResponse{OK: false, Message: "These vehicles are already coupled."}, nil
	}

	adjacentPairs := buildAdjacentSlotPairs(req.Segments, req.GridSize)
	slotA := slotID(va.X, va.Y)
	slotB := slotID(vb.X, vb.Y)
	if _, ok := adjacentPairs[pairKey(slotA, slotB)]; !ok {
		return ValidateCouplingResponse{OK: false, Message: "Coupling is allowed only for adjacent slots."}, nil
	}

	return ValidateCouplingResponse{OK: true}, nil
}

func placeVehicleInternal(req PlaceVehicleRequest) (PlaceVehicleResponse, error) {
	if req.VehicleType != "wagon" && req.VehicleType != "locomotive" {
		return PlaceVehicleResponse{}, errors.New("Vehicle type must be wagon or locomotive.")
	}

	pathSlots := collectPathSlots(req.Segments, req.GridSize)
	target, ok := findPathSlot(pathSlots, req.TargetPathID, req.TargetIndex)
	if !ok {
		return PlaceVehicleResponse{}, errors.New("Target slot is not on rail.")
	}

	occupied := map[string]struct{}{}
	for _, v := range req.Vehicles {
		if v.PathID != "" {
			occupied[pathSlotKey(v.PathID, v.PathIndex)] = struct{}{}
			continue
		}
		occupied[slotID(v.X, v.Y)] = struct{}{}
	}
	if _, exists := occupied[pathSlotKey(target.PathID, target.Index)]; exists {
		return PlaceVehicleResponse{}, errors.New("Target slot is occupied.")
	}

	vehicle := Vehicle{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:      req.VehicleType,
		Code:      nextVehicleCode(req.Vehicles, req.VehicleType),
		PathID:    target.PathID,
		PathIndex: target.Index,
		X:         target.X,
		Y:         target.Y,
	}
	return PlaceVehicleResponse{
		OK:      true,
		Vehicle: &vehicle,
	}, nil
}

func resolveVehiclesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ResolveVehiclesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ResolveVehiclesResponse{
			OK:      false,
			Message: "invalid json",
		})
		return
	}
	if req.GridSize <= 0 {
		req.GridSize = 32
	}

	vehicles, err := resolveVehicles(req)
	if err != nil {
		writeJSON(w, http.StatusOK, ResolveVehiclesResponse{
			OK:      false,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, ResolveVehiclesResponse{
		OK:       true,
		Vehicles: vehicles,
	})
}

func layoutApplyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LayoutOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, LayoutOperationResponse{
			OK:      false,
			Message: "invalid json",
		})
		return
	}
	if req.GridSize <= 0 {
		req.GridSize = 32
	}

	nextState, message, err := applyLayoutOperation(req)
	if err != nil {
		writeJSON(w, http.StatusOK, LayoutOperationResponse{
			OK:      false,
			Message: err.Error(),
			State:   req.State,
		})
		return
	}
	nextState = finalizeLayoutState(nextState, req.GridSize)

	writeJSON(w, http.StatusOK, LayoutOperationResponse{
		OK:      true,
		Message: message,
		State:   nextState,
	})
}

func scenariosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, CreateScenarioResponse{
			OK:      false,
			Message: "invalid json",
		})
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		req.Name = "Scenario"
	}

	scenario := Scenario{
		ID:           fmt.Sprintf("sc-%d", time.Now().UnixNano()),
		Name:         req.Name,
		InitialState: req.InitialState,
		Commands:     []CommandSpec{},
	}

	scenarioStore.mu.Lock()
	scenarioStore.scenarios[scenario.ID] = scenario
	scenarioStore.mu.Unlock()

	writeJSON(w, http.StatusOK, CreateScenarioResponse{
		OK:       true,
		Scenario: scenario,
	})
}

func scenarioByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/scenarios/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.Error(w, "scenario id required", http.StatusBadRequest)
		return
	}
	parts := strings.Split(path, "/")
	scenarioID := parts[0]

	if len(parts) == 1 && r.Method == http.MethodGet {
		scenarioStore.mu.Lock()
		scenario, ok := scenarioStore.scenarios[scenarioID]
		scenarioStore.mu.Unlock()
		if !ok {
			http.Error(w, "scenario not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, scenario)
		return
	}

	if len(parts) == 2 && parts[1] == "commands" && r.Method == http.MethodPost {
		addCommandHandler(w, r, scenarioID)
		return
	}

	if len(parts) == 2 && parts[1] == "run" && r.Method == http.MethodPost {
		runScenarioHandler(w, r, scenarioID)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func addCommandHandler(w http.ResponseWriter, r *http.Request, scenarioID string) {
	var req AddCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, AddCommandResponse{
			OK:      false,
			Message: "invalid json",
		})
		return
	}

	req.Type = strings.ToUpper(strings.TrimSpace(req.Type))
	if req.Type == "" {
		writeJSON(w, http.StatusOK, AddCommandResponse{
			OK:      false,
			Message: "command type is required",
		})
		return
	}

	scenarioStore.mu.Lock()
	defer scenarioStore.mu.Unlock()

	scenario, ok := scenarioStore.scenarios[scenarioID]
	if !ok {
		http.Error(w, "scenario not found", http.StatusNotFound)
		return
	}

	command := CommandSpec{
		ID:      fmt.Sprintf("cmd-%d", time.Now().UnixNano()),
		Order:   len(scenario.Commands),
		Type:    req.Type,
		Payload: req.Payload,
	}
	scenario.Commands = append(scenario.Commands, command)
	scenarioStore.scenarios[scenarioID] = scenario

	writeJSON(w, http.StatusOK, AddCommandResponse{
		OK:      true,
		Command: command,
	})
}

func runScenarioHandler(w http.ResponseWriter, r *http.Request, scenarioID string) {
	scenarioStore.mu.Lock()
	defer scenarioStore.mu.Unlock()

	scenario, ok := scenarioStore.scenarios[scenarioID]
	if !ok {
		http.Error(w, "scenario not found", http.StatusNotFound)
		return
	}

	execution := Execution{
		ID:             fmt.Sprintf("ex-%d", time.Now().UnixNano()),
		ScenarioID:     scenarioID,
		Status:         "running",
		CurrentCommand: 0,
		State:          cloneLayoutState(scenario.InitialState),
		Log:            []string{"execution created"},
	}
	scenarioStore.executions[execution.ID] = execution

	writeJSON(w, http.StatusOK, RunScenarioResponse{
		OK:        true,
		Execution: execution,
	})
}

func executionByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/executions/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.Error(w, "execution id required", http.StatusBadRequest)
		return
	}
	parts := strings.Split(path, "/")
	executionID := parts[0]

	if len(parts) == 1 && r.Method == http.MethodGet {
		scenarioStore.mu.Lock()
		execution, ok := scenarioStore.executions[executionID]
		scenarioStore.mu.Unlock()
		if !ok {
			http.Error(w, "execution not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, execution)
		return
	}

	if len(parts) == 2 && parts[1] == "step" && r.Method == http.MethodPost {
		stepExecutionHandler(w, r, executionID)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func stepExecutionHandler(w http.ResponseWriter, r *http.Request, executionID string) {
	scenarioStore.mu.Lock()
	defer scenarioStore.mu.Unlock()

	execution, ok := scenarioStore.executions[executionID]
	if !ok {
		http.Error(w, "execution not found", http.StatusNotFound)
		return
	}
	if execution.Status != "running" {
		writeJSON(w, http.StatusOK, StepExecutionResponse{
			OK:        false,
			Message:   "execution is not running",
			Execution: execution,
		})
		return
	}

	scenario, ok := scenarioStore.scenarios[execution.ScenarioID]
	if !ok {
		http.Error(w, "scenario not found", http.StatusNotFound)
		return
	}
	if execution.CurrentCommand >= len(scenario.Commands) {
		execution.Status = "completed"
		execution.Log = append(execution.Log, "completed")
		scenarioStore.executions[executionID] = execution
		writeJSON(w, http.StatusOK, StepExecutionResponse{
			OK:        true,
			Message:   "execution completed",
			Execution: execution,
		})
		return
	}

	command := scenario.Commands[execution.CurrentCommand]
	nextState, msg, err := applyCommand(execution.State, command)
	if err != nil {
		execution.Status = "failed"
		execution.Log = append(execution.Log, fmt.Sprintf("command %s failed: %s", command.Type, err.Error()))
		scenarioStore.executions[executionID] = execution
		writeJSON(w, http.StatusOK, StepExecutionResponse{
			OK:        false,
			Message:   err.Error(),
			Execution: execution,
		})
		return
	}

	execution.State = nextState
	execution.CurrentCommand++
	execution.Log = append(execution.Log, fmt.Sprintf("command %s ok %s", command.Type, msg))
	if execution.CurrentCommand >= len(scenario.Commands) {
		execution.Status = "completed"
		execution.Log = append(execution.Log, "completed")
	}

	scenarioStore.executions[executionID] = execution
	writeJSON(w, http.StatusOK, StepExecutionResponse{
		OK:        true,
		Message:   "step applied",
		Execution: execution,
	})
}

func applyCommand(state LayoutState, command CommandSpec) (LayoutState, string, error) {
	switch command.Type {
	case "MOVE_LOCO":
		if command.Payload.LocoID == "" || command.Payload.TargetPathID == "" {
			return state, "", errors.New("MOVE_LOCO requires locoId and targetPathId")
		}
		plan, err := buildMovementPlan(PlanMovementRequest{
			GridSize:             32,
			Segments:             state.Segments,
			Vehicles:             state.Vehicles,
			Couplings:            state.Couplings,
			SelectedLocomotiveID: command.Payload.LocoID,
			TargetPathID:         command.Payload.TargetPathID,
			TargetIndex:          command.Payload.TargetIndex,
		})
		if err != nil {
			return state, "", err
		}
		if len(plan.Timeline) == 0 {
			return state, "", errors.New("movement timeline is empty")
		}
		lastStep := plan.Timeline[len(plan.Timeline)-1]
		posByID := map[string]Position{}
		for _, p := range lastStep {
			posByID[p.ID] = p
		}
		nextVehicles := make([]Vehicle, 0, len(state.Vehicles))
		for _, v := range state.Vehicles {
			pos, ok := posByID[v.ID]
			if !ok {
				nextVehicles = append(nextVehicles, v)
				continue
			}
			nextVehicles = append(nextVehicles, Vehicle{
				ID:   v.ID,
				Type: v.Type,
				Code: v.Code,
				X:    pos.X,
				Y:    pos.Y,
			})
		}
		state.Vehicles = finalizeLayoutState(LayoutState{
			Segments:  state.Segments,
			Vehicles:  nextVehicles,
			Couplings: state.Couplings,
		}, 32).Vehicles
		return state, "move applied", nil

	case "COUPLE":
		if command.Payload.AID == "" || command.Payload.BID == "" {
			return state, "", errors.New("COUPLE requires aId and bId")
		}
		next, _, err := applyLayoutOperation(LayoutOperationRequest{
			GridSize: 32,
			State:    state,
			Action:   "couple",
			SelectedVehicleIDs: []string{
				command.Payload.AID,
				command.Payload.BID,
			},
		})
		if err != nil {
			return state, "", err
		}
		return next, "couple applied", nil

	case "DECOUPLE":
		if command.Payload.AID == "" || command.Payload.BID == "" {
			return state, "", errors.New("DECOUPLE requires aId and bId")
		}
		next, _, err := applyLayoutOperation(LayoutOperationRequest{
			GridSize: 32,
			State:    state,
			Action:   "decouple",
			SelectedVehicleIDs: []string{
				command.Payload.AID,
				command.Payload.BID,
			},
		})
		if err != nil {
			return state, "", err
		}
		return next, "decouple applied", nil

	default:
		return state, "", errors.New("unsupported command type")
	}
}

func cloneLayoutState(state LayoutState) LayoutState {
	next := LayoutState{
		Segments:  make([]Segment, len(state.Segments)),
		Vehicles:  make([]Vehicle, len(state.Vehicles)),
		Couplings: make([]Coupling, len(state.Couplings)),
	}
	copy(next.Segments, state.Segments)
	copy(next.Vehicles, state.Vehicles)
	copy(next.Couplings, state.Couplings)
	return next
}

func applyLayoutOperation(req LayoutOperationRequest) (LayoutState, string, error) {
	state := req.State

	switch req.Action {
	case "add_segment":
		if req.From == nil || req.To == nil {
			return state, "", errors.New("from/to are required")
		}
		if req.From.X == req.To.X && req.From.Y == req.To.Y {
			return state, "", errors.New("segment length must be non-zero")
		}
		state.Segments = append(state.Segments, Segment{
			ID:   nextPathID(state.Segments),
			From: *req.From,
			To:   *req.To,
		})
		return state, "", nil

	case "delete_segments":
		toDelete := map[string]struct{}{}
		for _, id := range req.IDs {
			toDelete[id] = struct{}{}
		}
		if len(toDelete) == 0 {
			return state, "", nil
		}
		filtered := make([]Segment, 0, len(state.Segments))
		for _, segment := range state.Segments {
			if _, drop := toDelete[segment.ID]; drop {
				continue
			}
			filtered = append(filtered, segment)
		}
		state.Segments = filtered
		if len(state.Segments) == 0 {
			state.Vehicles = nil
			state.Couplings = nil
			return state, "", nil
		}
		resolved, err := resolveVehicles(ResolveVehiclesRequest{
			GridSize:        req.GridSize,
			Segments:        state.Segments,
			Vehicles:        state.Vehicles,
			Couplings:       state.Couplings,
			StrictCouplings: false,
		})
		if err != nil {
			state.Vehicles = nil
			state.Couplings = nil
			return state, "vehicles reset: rails changed", nil
		}
		state.Vehicles = resolved
		return state, "", nil

	case "delete_vehicles":
		toDelete := map[string]struct{}{}
		for _, id := range req.IDs {
			toDelete[id] = struct{}{}
		}
		if len(toDelete) == 0 {
			return state, "", nil
		}
		filteredVehicles := make([]Vehicle, 0, len(state.Vehicles))
		for _, v := range state.Vehicles {
			if _, drop := toDelete[v.ID]; drop {
				continue
			}
			filteredVehicles = append(filteredVehicles, v)
		}
		filteredCouplings := make([]Coupling, 0, len(state.Couplings))
		for _, c := range state.Couplings {
			if _, drop := toDelete[c.A]; drop {
				continue
			}
			if _, drop := toDelete[c.B]; drop {
				continue
			}
			filteredCouplings = append(filteredCouplings, c)
		}
		state.Vehicles = filteredVehicles
		state.Couplings = filteredCouplings
		return state, "", nil

	case "clear":
		return LayoutState{}, "", nil

	case "place_vehicle":
		resp, err := placeVehicleInternal(PlaceVehicleRequest{
			GridSize:     req.GridSize,
			Segments:     state.Segments,
			Vehicles:     state.Vehicles,
			VehicleType:  req.VehicleType,
			TargetPathID: req.TargetPathID,
			TargetIndex:  req.TargetIndex,
		})
		if err != nil {
			return state, "", err
		}
		state.Vehicles = append(state.Vehicles, *resp.Vehicle)
		return state, "", nil

	case "resolve_vehicles":
		resolved, err := resolveVehicles(ResolveVehiclesRequest{
			GridSize:        req.GridSize,
			Segments:        state.Segments,
			Vehicles:        state.Vehicles,
			Couplings:       state.Couplings,
			MovedVehicleIDs: req.MovedVehicleIDs,
			StrictCouplings: req.StrictCouplings,
		})
		if err != nil {
			return state, "", err
		}
		state.Vehicles = resolved
		return state, "", nil

	case "couple":
		if len(req.SelectedVehicleIDs) < 2 {
			return state, "", errors.New("select two vehicles")
		}
		validateResp, err := validateCouplingInternal(ValidateCouplingRequest{
			GridSize:           req.GridSize,
			Segments:           state.Segments,
			Vehicles:           state.Vehicles,
			Couplings:          state.Couplings,
			SelectedVehicleIDs: req.SelectedVehicleIDs,
		})
		if err != nil {
			return state, "", err
		}
		if !validateResp.OK {
			return state, "", errors.New(validateResp.Message)
		}
		a := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-2]
		b := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-1]
		state.Couplings = append(state.Couplings, Coupling{
			ID: fmt.Sprintf("%d", time.Now().UnixNano()),
			A:  a,
			B:  b,
		})
		return state, "", nil

	case "decouple":
		if len(req.SelectedVehicleIDs) < 2 {
			return state, "", errors.New("select two vehicles")
		}
		a := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-2]
		b := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-1]
		key := pairKey(a, b)
		filtered := make([]Coupling, 0, len(state.Couplings))
		for _, coupling := range state.Couplings {
			if pairKey(coupling.A, coupling.B) == key {
				continue
			}
			filtered = append(filtered, coupling)
		}
		state.Couplings = filtered
		return state, "", nil

	default:
		return state, "", errors.New("unknown action")
	}
}

func resolveVehicles(req ResolveVehiclesRequest) ([]Vehicle, error) {
	if len(req.Vehicles) == 0 {
		return req.Vehicles, nil
	}
	pathSlots := collectPathSlots(req.Segments, req.GridSize)
	if len(pathSlots) == 0 {
		return nil, errors.New("No rail slots available.")
	}

	movedSet := map[string]struct{}{}
	for _, id := range req.MovedVehicleIDs {
		movedSet[id] = struct{}{}
	}
	if len(movedSet) == 0 {
		for _, v := range req.Vehicles {
			movedSet[v.ID] = struct{}{}
		}
	}

	blocked := map[string]struct{}{}
	for _, v := range req.Vehicles {
		if _, moved := movedSet[v.ID]; moved {
			continue
		}
		if v.PathID != "" {
			blocked[pathSlotKey(v.PathID, v.PathIndex)] = struct{}{}
			continue
		}
		blocked[slotID(v.X, v.Y)] = struct{}{}
	}

	next := make([]Vehicle, 0, len(req.Vehicles))
	nextByID := map[string]Vehicle{}

	for _, v := range req.Vehicles {
		if _, moved := movedSet[v.ID]; !moved {
			normalized := normalizeVehicleToPath(v, pathSlots)
			next = append(next, normalized)
			nextByID[v.ID] = normalized
			continue
		}

		nearest := findNearestPathSlot(Point{X: v.X, Y: v.Y}, pathSlots, blocked)
		if nearest == nil {
			return nil, errors.New("Cannot place moved vehicles on free rail slots.")
		}

		resolved := Vehicle{
			ID:        v.ID,
			Type:      v.Type,
			Code:      v.Code,
			PathID:    nearest.PathID,
			PathIndex: nearest.Index,
			X:         nearest.X,
			Y:         nearest.Y,
		}
		next = append(next, resolved)
		nextByID[v.ID] = resolved
		blocked[pathSlotKey(nearest.PathID, nearest.Index)] = struct{}{}
	}

	if req.StrictCouplings {
		pathAdjPairs := buildAdjacentPathSlotPairs(req.Segments, req.GridSize)
		for _, c := range req.Couplings {
			va, okA := nextByID[c.A]
			vb, okB := nextByID[c.B]
			if !okA || !okB {
				continue
			}
			if _, ok := pathAdjPairs[pathSlotPairKey(va.PathID, va.PathIndex, vb.PathID, vb.PathIndex)]; !ok {
				return nil, errors.New("Coupled vehicles must stay on adjacent slots.")
			}
		}
	}

	return next, nil
}

func normalizeVehicleToPath(vehicle Vehicle, pathSlots []PathSlot) Vehicle {
	if vehicle.PathID != "" && vehicle.PathIndex >= 0 {
		if slot, ok := findPathSlot(pathSlots, vehicle.PathID, vehicle.PathIndex); ok {
			vehicle.X = slot.X
			vehicle.Y = slot.Y
			return vehicle
		}
	}
	nearest := findNearestPathSlot(Point{X: vehicle.X, Y: vehicle.Y}, pathSlots, nil)
	if nearest == nil {
		return vehicle
	}
	vehicle.PathID = nearest.PathID
	vehicle.PathIndex = nearest.Index
	vehicle.X = nearest.X
	vehicle.Y = nearest.Y
	return vehicle
}

func planMovementHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PlanMovementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, PlanMovementResponse{
			OK:      false,
			Message: "invalid json",
		})
		return
	}

	if req.GridSize <= 0 {
		req.GridSize = 32
	}

	resp, err := buildMovementPlan(req)
	if err != nil {
		writeJSON(w, http.StatusOK, PlanMovementResponse{
			OK:      false,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func buildMovementPlan(req PlanMovementRequest) (PlanMovementResponse, error) {
	if req.SelectedLocomotiveID == "" {
		return PlanMovementResponse{}, errors.New("Select locomotive.")
	}
	if strings.TrimSpace(req.TargetPathID) == "" {
		return PlanMovementResponse{}, errors.New("Select target path.")
	}

	pathSlots := collectPathSlots(req.Segments, req.GridSize)
	if len(pathSlots) == 0 {
		return PlanMovementResponse{}, errors.New("No rail slots available.")
	}
	targetSlot, ok := findPathSlot(pathSlots, req.TargetPathID, req.TargetIndex)
	if !ok {
		return PlanMovementResponse{}, errors.New("Target slot is unavailable.")
	}
	targetSlotID := slotID(targetSlot.X, targetSlot.Y)

	normalizedVehicles := make([]Vehicle, 0, len(req.Vehicles))
	vehicleByID := make(map[string]Vehicle, len(req.Vehicles))
	for _, v := range req.Vehicles {
		nv := normalizeVehicleToPath(v, pathSlots)
		normalizedVehicles = append(normalizedVehicles, nv)
		vehicleByID[nv.ID] = nv
	}

	locomotive, exists := vehicleByID[req.SelectedLocomotiveID]
	if !exists || locomotive.Type != "locomotive" {
		return PlanMovementResponse{}, errors.New("Selected unit is not a locomotive.")
	}

	slots := collectRailSlots(req.Segments, req.GridSize)
	slotByID := make(map[string]Slot, len(slots))
	for _, s := range slots {
		slotByID[s.ID] = s
	}

	slotAdj := buildSlotAdjacency(req.Segments, req.GridSize)
	trainOrder, err := buildTrainOrder(req.SelectedLocomotiveID, normalizedVehicles, req.Couplings)
	if err != nil {
		return PlanMovementResponse{}, err
	}

	currentSlotByVehicleID := make(map[string]string, len(trainOrder))
	for _, id := range trainOrder {
		v, ok := vehicleByID[id]
		if !ok {
			return PlanMovementResponse{}, errors.New("Train contains unknown vehicle.")
		}
		nearest := findNearestSlot(Point{X: v.X, Y: v.Y}, slots)
		if nearest == nil {
			return PlanMovementResponse{}, errors.New("No rail slots available.")
		}
		currentSlotByVehicleID[id] = nearest.ID
	}

	trail := reverseStrings(trainOrder)
	initialSlots := make([]string, 0, len(trail))
	for _, id := range trail {
		initialSlots = append(initialSlots, currentSlotByVehicleID[id])
	}
	for i := 0; i < len(initialSlots)-1; i++ {
		a := initialSlots[i]
		b := initialSlots[i+1]
		if _, ok := slotAdj[a][b]; !ok {
			return PlanMovementResponse{}, errors.New("Coupled train must stand on adjacent slots.")
		}
	}

	locoStart := currentSlotByVehicleID[req.SelectedLocomotiveID]
	path := dijkstraPath(slotAdj, locoStart, targetSlotID)
	if len(path) < 2 {
		return PlanMovementResponse{}, errors.New("Path was not found.")
	}

	currentLocoToTail := make([]string, 0, len(trainOrder))
	for _, id := range trainOrder {
		currentLocoToTail = append(currentLocoToTail, currentSlotByVehicleID[id])
	}

	isBackwardPush := len(trainOrder) > 1 && len(path) > 1 && path[1] == currentLocoToTail[1]
	drivingPath := path
	if isBackwardPush && len(trainOrder) > 1 {
		extended, extErr := extendPathForBackwardPush(path, slotAdj, slotByID, len(trainOrder)-1)
		if extErr != nil {
			return PlanMovementResponse{}, extErr
		}
		drivingPath = extended
	}

	staticOccupied := make(map[string]struct{})
	trainSet := make(map[string]struct{}, len(trainOrder))
	for _, id := range trainOrder {
		trainSet[id] = struct{}{}
	}
	for _, v := range normalizedVehicles {
		if _, ok := trainSet[v.ID]; ok {
			continue
		}
		staticOccupied[slotID(v.X, v.Y)] = struct{}{}
	}

	maxSteps := len(path) - 1
	if maxSteps < 1 {
		return PlanMovementResponse{}, errors.New("Not enough path length.")
	}

	timeline := make([][]Position, 0, maxSteps)
	for step := 1; step <= maxSteps; step++ {
		stepPositions := make([]Position, 0, len(trainOrder))
		used := make(map[string]struct{})
		valid := true

		for i := 0; i < len(trainOrder); i++ {
			var slotKey string
			if isBackwardPush {
				idx := step + i
				if idx >= len(drivingPath) {
					valid = false
					break
				}
				slotKey = drivingPath[idx]
			} else {
				historyIndex := step - i
				if historyIndex > 0 {
					if historyIndex >= len(path) {
						valid = false
						break
					}
					slotKey = path[historyIndex]
				} else {
					idx := -historyIndex
					if idx >= len(currentLocoToTail) {
						valid = false
						break
					}
					slotKey = currentLocoToTail[idx]
				}
			}

			slot, ok := slotByID[slotKey]
			if !ok {
				valid = false
				break
			}
			if _, blocked := staticOccupied[slotKey]; blocked {
				valid = false
				break
			}
			if _, duplicated := used[slotKey]; duplicated {
				valid = false
				break
			}

			used[slotKey] = struct{}{}
			stepPositions = append(stepPositions, Position{
				ID: trainOrder[i],
				X:  slot.X,
				Y:  slot.Y,
			})
		}

		if !valid {
			return PlanMovementResponse{}, errors.New("Movement is blocked: not enough free slots.")
		}

		timeline = append(timeline, stepPositions)
	}

	return PlanMovementResponse{
		OK:          true,
		Message:     "Movement started.",
		Timeline:    timeline,
		CellsPassed: len(timeline),
	}, nil
}

func buildTrainOrder(locomotiveID string, vehicles []Vehicle, couplings []Coupling) ([]string, error) {
	graph := make(map[string]map[string]struct{}, len(vehicles))
	for _, v := range vehicles {
		graph[v.ID] = map[string]struct{}{}
	}
	for _, c := range couplings {
		if _, ok := graph[c.A]; !ok {
			continue
		}
		if _, ok := graph[c.B]; !ok {
			continue
		}
		graph[c.A][c.B] = struct{}{}
		graph[c.B][c.A] = struct{}{}
	}

	connected := map[string]struct{}{locomotiveID: {}}
	queue := []string{locomotiveID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for next := range graph[cur] {
			if _, seen := connected[next]; seen {
				continue
			}
			connected[next] = struct{}{}
			queue = append(queue, next)
		}
	}

	if len(connected) == 1 {
		return []string{locomotiveID}, nil
	}

	for id := range connected {
		degree := 0
		for next := range graph[id] {
			if _, ok := connected[next]; ok {
				degree++
			}
		}
		if degree > 2 {
			return nil, errors.New("Only linear train order is supported.")
		}
	}

	locoDegree := 0
	for next := range graph[locomotiveID] {
		if _, ok := connected[next]; ok {
			locoDegree++
		}
	}
	if locoDegree > 1 {
		return nil, errors.New("Locomotive must be at train head.")
	}

	endpoints := make([]string, 0, 2)
	for id := range connected {
		degree := 0
		for next := range graph[id] {
			if _, ok := connected[next]; ok {
				degree++
			}
		}
		if degree <= 1 {
			endpoints = append(endpoints, id)
		}
	}

	var tail string
	for _, id := range endpoints {
		if id != locomotiveID {
			tail = id
			break
		}
	}
	if tail == "" {
		return nil, errors.New("Locomotive must be at train head.")
	}

	orderTailToLoco := []string{}
	prev := ""
	cur := tail
	for cur != "" {
		orderTailToLoco = append(orderTailToLoco, cur)
		if cur == locomotiveID {
			break
		}
		next := ""
		for n := range graph[cur] {
			if n != prev {
				if _, ok := connected[n]; ok {
					next = n
					break
				}
			}
		}
		prev = cur
		cur = next
	}

	if len(orderTailToLoco) == 0 || orderTailToLoco[len(orderTailToLoco)-1] != locomotiveID {
		return nil, errors.New("Locomotive must be at train head.")
	}

	return reverseStrings(orderTailToLoco), nil
}

func dijkstraPath(adjacency map[string]map[string]struct{}, startID, goalID string) []string {
	if startID == goalID {
		return []string{startID}
	}
	if _, ok := adjacency[startID]; !ok {
		return nil
	}
	if _, ok := adjacency[goalID]; !ok {
		return nil
	}

	dist := map[string]int{startID: 0}
	prev := map[string]string{}
	visited := map[string]struct{}{}
	queue := []string{startID}

	for len(queue) > 0 {
		sort.Slice(queue, func(i, j int) bool {
			return dist[queue[i]] < dist[queue[j]]
		})
		cur := queue[0]
		queue = queue[1:]
		if _, seen := visited[cur]; seen {
			continue
		}
		visited[cur] = struct{}{}
		if cur == goalID {
			break
		}
		for next := range adjacency[cur] {
			if _, seen := visited[next]; seen {
				continue
			}
			nd := dist[cur] + 1
			old, ok := dist[next]
			if !ok || nd < old {
				dist[next] = nd
				prev[next] = cur
				queue = append(queue, next)
			}
		}
	}

	if _, ok := prev[goalID]; !ok {
		return nil
	}

	path := []string{goalID}
	node := goalID
	for {
		p, ok := prev[node]
		if !ok {
			break
		}
		path = append(path, p)
		node = p
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func extendPathForBackwardPush(path []string, adjacency map[string]map[string]struct{}, slotByID map[string]Slot, neededTailSlots int) ([]string, error) {
	extended := append([]string{}, path...)
	var prev string
	if len(path) >= 2 {
		prev = path[len(path)-2]
	}
	cur := path[len(path)-1]

	for i := 0; i < neededTailSlots; i++ {
		candidates := []string{}
		for id := range adjacency[cur] {
			if id != prev {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) == 0 {
			return nil, errors.New("Not enough space after target for backward push.")
		}

		next := candidates[0]
		if len(candidates) > 1 && prev != "" {
			pPrev, okPrev := slotByID[prev]
			pCur, okCur := slotByID[cur]
			if okPrev && okCur {
				inX := pCur.X - pPrev.X
				inY := pCur.Y - pPrev.Y
				bestScore := math.Inf(-1)
				for _, candidateID := range candidates {
					pNext, okNext := slotByID[candidateID]
					if !okNext {
						continue
					}
					outX := pNext.X - pCur.X
					outY := pNext.Y - pCur.Y
					score := inX*outX + inY*outY
					if score > bestScore {
						bestScore = score
						next = candidateID
					}
				}
			}
		}

		extended = append(extended, next)
		prev = cur
		cur = next
	}

	return extended, nil
}

func collectRailSlots(segments []Segment, gridSize float64) []Slot {
	uniq := map[string]Slot{}
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
		for _, p := range points {
			id := slotID(p.X, p.Y)
			uniq[id] = Slot{ID: id, X: p.X, Y: p.Y}
		}
	}
	result := make([]Slot, 0, len(uniq))
	for _, s := range uniq {
		result = append(result, s)
	}
	return result
}

func collectPathSlots(segments []Segment, gridSize float64) []PathSlot {
	slots := make([]PathSlot, 0)
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
		for i, p := range points {
			slots = append(slots, PathSlot{
				PathID: segment.ID,
				Index:  i,
				X:      p.X,
				Y:      p.Y,
			})
		}
	}
	return slots
}

func findPathSlot(slots []PathSlot, pathID string, index int) (PathSlot, bool) {
	for _, slot := range slots {
		if slot.PathID == pathID && slot.Index == index {
			return slot, true
		}
	}
	return PathSlot{}, false
}

func findNearestPathSlot(point Point, slots []PathSlot, blocked map[string]struct{}) *PathSlot {
	var best *PathSlot
	bestDist := math.Inf(1)
	for i := range slots {
		if blocked != nil {
			if _, used := blocked[pathSlotKey(slots[i].PathID, slots[i].Index)]; used {
				continue
			}
		}
		dx := point.X - slots[i].X
		dy := point.Y - slots[i].Y
		dist := dx*dx + dy*dy
		if dist < bestDist {
			bestDist = dist
			best = &slots[i]
		}
	}
	return best
}

func buildSlotAdjacency(segments []Segment, gridSize float64) map[string]map[string]struct{} {
	adj := map[string]map[string]struct{}{}
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
		for i := 0; i < len(points)-1; i++ {
			a := slotID(points[i].X, points[i].Y)
			b := slotID(points[i+1].X, points[i+1].Y)
			if _, ok := adj[a]; !ok {
				adj[a] = map[string]struct{}{}
			}
			if _, ok := adj[b]; !ok {
				adj[b] = map[string]struct{}{}
			}
			adj[a][b] = struct{}{}
			adj[b][a] = struct{}{}
		}
	}
	return adj
}

func buildAdjacentSlotPairs(segments []Segment, gridSize float64) map[string]struct{} {
	pairs := map[string]struct{}{}
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
		for i := 0; i < len(points)-1; i++ {
			a := slotID(points[i].X, points[i].Y)
			b := slotID(points[i+1].X, points[i+1].Y)
			pairs[pairKey(a, b)] = struct{}{}
		}
	}
	return pairs
}

func buildAdjacentPathSlotPairs(segments []Segment, gridSize float64) map[string]struct{} {
	pairs := map[string]struct{}{}
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
		for i := 0; i < len(points)-1; i++ {
			pairs[pathSlotPairKey(segment.ID, i, segment.ID, i+1)] = struct{}{}
		}
	}
	return pairs
}

func finalizeLayoutState(state LayoutState, gridSize float64) LayoutState {
	state = normalizeSegmentIDs(state)
	pathSlots := collectPathSlots(state.Segments, gridSize)
	normalized := make([]Vehicle, 0, len(state.Vehicles))
	for _, v := range state.Vehicles {
		normalized = append(normalized, normalizeVehicleToPath(v, pathSlots))
	}
	state.Vehicles = normalized
	state.Paths = buildPathStates(state.Segments, state.Vehicles, gridSize)
	return state
}

func buildPathStates(segments []Segment, vehicles []Vehicle, gridSize float64) []PathState {
	vehicleByPath := map[string][]string{}
	for _, v := range vehicles {
		if v.PathID == "" {
			continue
		}
		vehicleByPath[v.PathID] = append(vehicleByPath[v.PathID], v.ID)
	}

	neighbors := buildPathAdjacency(segments)
	states := make([]PathState, 0, len(segments))
	for _, segment := range segments {
		capacity := len(getSegmentSlots(segment, gridSize))
		states = append(states, PathState{
			ID:         segment.ID,
			Capacity:   capacity,
			VehicleIDs: vehicleByPath[segment.ID],
			Neighbors:  neighbors[segment.ID],
		})
	}
	return states
}

func buildPathAdjacency(segments []Segment) map[string][]string {
	endpointMap := map[string][]string{}
	for _, segment := range segments {
		fromKey := slotID(segment.From.X, segment.From.Y)
		toKey := slotID(segment.To.X, segment.To.Y)
		endpointMap[fromKey] = append(endpointMap[fromKey], segment.ID)
		endpointMap[toKey] = append(endpointMap[toKey], segment.ID)
	}

	adj := map[string]map[string]struct{}{}
	for _, ids := range endpointMap {
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				a := ids[i]
				b := ids[j]
				if _, ok := adj[a]; !ok {
					adj[a] = map[string]struct{}{}
				}
				if _, ok := adj[b]; !ok {
					adj[b] = map[string]struct{}{}
				}
				adj[a][b] = struct{}{}
				adj[b][a] = struct{}{}
			}
		}
	}

	result := map[string][]string{}
	for id, neighbors := range adj {
		for neighbor := range neighbors {
			result[id] = append(result[id], neighbor)
		}
	}
	return result
}

func normalizeSegmentIDs(state LayoutState) LayoutState {
	if len(state.Segments) == 0 {
		return state
	}

	needsNormalization := false
	for _, segment := range state.Segments {
		if !isSimpleNumericID(segment.ID) {
			needsNormalization = true
			break
		}
	}
	if !needsNormalization {
		return state
	}

	idMap := map[string]string{}
	for i, segment := range state.Segments {
		newID := fmt.Sprintf("%d", i+1)
		idMap[segment.ID] = newID
		state.Segments[i].ID = newID
	}

	for i, vehicle := range state.Vehicles {
		if vehicle.PathID == "" {
			continue
		}
		if newID, ok := idMap[vehicle.PathID]; ok {
			state.Vehicles[i].PathID = newID
		}
	}

	return state
}

func isSimpleNumericID(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func nextPathID(segments []Segment) string {
	maxID := 0
	for _, segment := range segments {
		if !isSimpleNumericID(segment.ID) {
			continue
		}
		var value int
		if _, err := fmt.Sscanf(segment.ID, "%d", &value); err == nil && value > maxID {
			maxID = value
		}
	}
	return fmt.Sprintf("%d", maxID+1)
}

func getSegmentSlots(segment Segment, step float64) []Point {
	dx := segment.To.X - segment.From.X
	dy := segment.To.Y - segment.From.Y
	length := math.Hypot(dx, dy)

	if length == 0 {
		return []Point{{X: segment.From.X, Y: segment.From.Y}}
	}

	count := int(math.Floor(length / step))
	ux := dx / length
	uy := dy / length
	slots := make([]Point, 0, count+2)
	for i := 0; i <= count; i++ {
		slots = append(slots, Point{
			X: segment.From.X + ux*step*float64(i),
			Y: segment.From.Y + uy*step*float64(i),
		})
	}

	last := slots[len(slots)-1]
	if math.Hypot(last.X-segment.To.X, last.Y-segment.To.Y) >= step*0.25 {
		slots = append(slots, Point{X: segment.To.X, Y: segment.To.Y})
	}

	return slots
}

func findNearestSlot(point Point, slots []Slot) *Slot {
	var best *Slot
	bestDist := math.Inf(1)
	for i := range slots {
		dx := point.X - slots[i].X
		dy := point.Y - slots[i].Y
		dist := dx*dx + dy*dy
		if dist < bestDist {
			bestDist = dist
			best = &slots[i]
		}
	}
	return best
}

func findNearestFreeSlot(point Point, slots []Slot, blocked map[string]struct{}) *Slot {
	var best *Slot
	bestDist := math.Inf(1)
	for i := range slots {
		if _, used := blocked[slots[i].ID]; used {
			continue
		}
		dx := point.X - slots[i].X
		dy := point.Y - slots[i].Y
		dist := dx*dx + dy*dy
		if dist < bestDist {
			bestDist = dist
			best = &slots[i]
		}
	}
	return best
}

func reverseStrings(items []string) []string {
	result := append([]string{}, items...)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func nextVehicleCode(vehicles []Vehicle, vehicleType string) string {
	prefix := "в"
	if vehicleType == "locomotive" {
		prefix = "л"
	}

	maxNumber := 0
	for _, vehicle := range vehicles {
		if vehicle.Type != vehicleType {
			continue
		}
		if !strings.HasPrefix(vehicle.Code, prefix) {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(vehicle.Code, prefix+"%d", &n); err == nil && n > maxNumber {
			maxNumber = n
		}
	}

	return fmt.Sprintf("%s%d", prefix, maxNumber+1)
}

func slotID(x, y float64) string {
	return fmt.Sprintf("%.2f:%.2f", x, y)
}

func pathSlotKey(pathID string, index int) string {
	return fmt.Sprintf("%s:%d", pathID, index)
}

func pathSlotPairKey(pathA string, indexA int, pathB string, indexB int) string {
	keyA := pathSlotKey(pathA, indexA)
	keyB := pathSlotKey(pathB, indexB)
	if keyA < keyB {
		return keyA + "|" + keyB
	}
	return keyB + "|" + keyA
}

func pairKey(a, b string) string {
	if a < b {
		return a + "|" + b
	}
	return b + "|" + a
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
