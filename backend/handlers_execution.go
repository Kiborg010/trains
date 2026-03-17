package main

import (
	"fmt"
	"net/http"
	"strings"
)

func runNormalizedScenarioHandler(w http.ResponseWriter, r *http.Request, userID int, scenarioID string) {
	if _, err := appStore.GetNormalizedScenario(scenarioID, userID); err != nil {
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

	runtime, err := buildExecutionRuntimeFromNormalized(appStore, userID, execution.ScenarioID)
	if err != nil {
		http.Error(w, "scenario not found", http.StatusNotFound)
		return
	}
	if execution.CurrentStep >= len(runtime.Steps) {
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

	step := runtime.Steps[execution.CurrentStep]
	nextState, msg, err := applyScenarioStep(execution.State, step)
	if err != nil {
		execution.Status = "failed"
		execution.Log = append(execution.Log, fmt.Sprintf("step %s failed: %s", step.StepType, err.Error()))
		_ = appStore.UpdateExecution(executionID, userID, *execution)
		writeJSON(w, http.StatusOK, StepExecutionResponse{
			OK:        false,
			Message:   err.Error(),
			Execution: *execution,
		})
		return
	}

	execution.State = nextState
	execution.CurrentStep++
	execution.Log = append(execution.Log, fmt.Sprintf("step %s ok %s", step.StepType, msg))
	if execution.CurrentStep >= len(runtime.Steps) {
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
