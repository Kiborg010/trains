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

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodGet {
		scenarios, err := appStore.ListScenarios(userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ListScenariosResponse{
				OK:      false,
				Message: "failed to list scenarios",
			})
			return
		}
		writeJSON(w, http.StatusOK, ListScenariosResponse{
			OK:        true,
			Scenarios: scenarios,
		})
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

	id, err := appStore.SaveScenario(userID, nil, req.Name, req.InitialState, []CommandSpec{})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, CreateScenarioResponse{
			OK:      false,
			Message: "failed to create scenario",
		})
		return
	}

	scenario, err := appStore.GetScenario(id)
	if err != nil || scenario.UserID != userID {
		writeJSON(w, http.StatusInternalServerError, CreateScenarioResponse{
			OK:      false,
			Message: "failed to load scenario",
		})
		return
	}

	writeJSON(w, http.StatusOK, CreateScenarioResponse{
		OK:       true,
		Scenario: *scenario,
	})
}

func scenarioByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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
		scenario, err := appStore.GetScenario(scenarioID)
		if err != nil || scenario.UserID != userID {
			http.Error(w, "scenario not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, scenario)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := appStore.DeleteScenario(scenarioID, userID); err != nil {
			http.Error(w, "scenario not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"message": "scenario deleted",
		})
		return
	}

	if len(parts) == 2 && parts[1] == "commands" && r.Method == http.MethodPost {
		addCommandHandler(w, r, userID, scenarioID)
		return
	}

	if len(parts) == 2 && parts[1] == "run" && r.Method == http.MethodPost {
		runScenarioHandler(w, r, userID, scenarioID)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func addCommandHandler(w http.ResponseWriter, r *http.Request, userID int, scenarioID string) {
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

	scenario, err := appStore.GetScenario(scenarioID)
	if err != nil || scenario.UserID != userID {
		http.Error(w, "scenario not found", http.StatusNotFound)
		return
	}

	command := CommandSpec{
		ID:      fmt.Sprintf("cmd-%d", time.Now().UnixNano()),
		Order:   len(scenario.Commands),
		Type:    req.Type,
		Payload: req.Payload,
	}

	nextCommands := append([]CommandSpec{}, scenario.Commands...)
	nextCommands = append(nextCommands, command)
	if err := appStore.UpdateScenarioCommands(scenarioID, userID, nextCommands); err != nil {
		writeJSON(w, http.StatusInternalServerError, AddCommandResponse{
			OK:      false,
			Message: "failed to add command",
		})
		return
	}

	writeJSON(w, http.StatusOK, AddCommandResponse{
		OK:      true,
		Command: command,
	})
}

func runScenarioHandler(w http.ResponseWriter, r *http.Request, userID int, scenarioID string) {
	scenario, err := appStore.GetScenario(scenarioID)
	if err != nil || scenario.UserID != userID {
		http.Error(w, "scenario not found", http.StatusNotFound)
		return
	}

	executionID, err := appStore.SaveExecution(userID, scenarioID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, RunScenarioResponse{
			OK:      false,
			Message: "failed to start execution",
		})
		return
	}

	execution, err := appStore.GetExecution(executionID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, RunScenarioResponse{
			OK:      false,
			Message: "failed to load execution",
		})
		return
	}

	writeJSON(w, http.StatusOK, RunScenarioResponse{
		OK:        true,
		Execution: *execution,
	})
}

func executionByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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
		execution, err := appStore.GetExecution(executionID, userID)
		if err != nil {
			http.Error(w, "execution not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, execution)
		return
	}

	if len(parts) == 2 && parts[1] == "step" && r.Method == http.MethodPost {
		stepExecutionHandler(w, r, userID, executionID)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func stepExecutionHandler(w http.ResponseWriter, r *http.Request, userID int, executionID string) {
	execution, err := appStore.GetExecution(executionID, userID)
	if err != nil {
		http.Error(w, "execution not found", http.StatusNotFound)
		return
	}
	if execution.Status != "running" {
		writeJSON(w, http.StatusOK, StepExecutionResponse{
			OK:        false,
			Message:   "execution is not running",
			Execution: *execution,
		})
		return
	}

	scenario, err := appStore.GetScenario(execution.ScenarioID)
	if err != nil || scenario.UserID != userID {
		http.Error(w, "scenario not found", http.StatusNotFound)
		return
	}
	if execution.CurrentCommand >= len(scenario.Commands) {
		execution.Status = "completed"
		execution.Log = append(execution.Log, "completed")
		_ = appStore.UpdateExecution(executionID, userID, *execution)
		writeJSON(w, http.StatusOK, StepExecutionResponse{
			OK:        true,
			Message:   "execution completed",
			Execution: *execution,
		})
		return
	}

	command := scenario.Commands[execution.CurrentCommand]
	nextState, msg, err := applyCommand(execution.State, command)
	if err != nil {
		execution.Status = "failed"
		execution.Log = append(execution.Log, fmt.Sprintf("command %s failed: %s", command.Type, err.Error()))
		_ = appStore.UpdateExecution(executionID, userID, *execution)
		writeJSON(w, http.StatusOK, StepExecutionResponse{
			OK:        false,
			Message:   err.Error(),
			Execution: *execution,
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

	if err := appStore.UpdateExecution(executionID, userID, *execution); err != nil {
		writeJSON(w, http.StatusInternalServerError, StepExecutionResponse{
			OK:        false,
			Message:   "failed to persist execution state",
			Execution: *execution,
		})
		return
	}

	writeJSON(w, http.StatusOK, StepExecutionResponse{
		OK:        true,
		Message:   "step applied",
		Execution: *execution,
	})
}
