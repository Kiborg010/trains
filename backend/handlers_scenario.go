package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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
