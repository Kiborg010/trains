package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"trains/backend/normalized"
)

func normalizedSchemesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodPost {
		handleCreateNormalizedScheme(w, r, userID)
		return
	}

	schemes, err := appStore.ListNormalizedSchemes(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ListNormalizedSchemesResponse{
			OK:      false,
			Message: fmt.Sprintf("failed to list normalized schemes: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, ListNormalizedSchemesResponse{
		OK:      true,
		Schemes: toSchemeDTOs(schemes),
	})
}

func normalizedSchemeByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/normalized/schemes/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.Error(w, "scheme id required", http.StatusBadRequest)
		return
	}

	parts := strings.Split(path, "/")
	schemeID, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "invalid scheme id", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		scheme, err := appStore.GetNormalizedScheme(schemeID, userID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, GetNormalizedSchemeResponse{
				OK:      false,
				Message: fmt.Sprintf("normalized scheme not found: %v", err),
			})
			return
		}
		dto := toSchemeDTO(*scheme)
		writeJSON(w, http.StatusOK, GetNormalizedSchemeResponse{
			OK:     true,
			Scheme: &dto,
		})
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPut {
		handleUpdateNormalizedScheme(w, r, userID, schemeID)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := appStore.DeleteNormalizedScheme(userID, schemeID); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"ok":      false,
				"message": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"message": "normalized scheme deleted",
		})
		return
	}

	if len(parts) == 2 && parts[1] == "details" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeNormalizedSchemeDetails(w, userID, schemeID)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func normalizedScenariosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodPost {
		handleCreateNormalizedScenario(w, r, userID)
		return
	}

	scenarios, err := appStore.ListNormalizedScenarios(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ListNormalizedScenariosResponse{
			OK:      false,
			Message: fmt.Sprintf("failed to list normalized scenarios: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, ListNormalizedScenariosResponse{
		OK:        true,
		Scenarios: toScenarioDTOs(scenarios),
	})
}

func normalizedScenarioByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/normalized/scenarios/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.Error(w, "scenario id required", http.StatusBadRequest)
		return
	}

	parts := strings.Split(path, "/")
	scenarioID := parts[0]

	if len(parts) == 1 && r.Method == http.MethodGet {
		scenario, err := appStore.GetNormalizedScenario(scenarioID, userID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, GetNormalizedScenarioResponse{
				OK:      false,
				Message: fmt.Sprintf("normalized scenario not found: %v", err),
			})
			return
		}
		dto := toScenarioDTO(*scenario)
		writeJSON(w, http.StatusOK, GetNormalizedScenarioResponse{
			OK:       true,
			Scenario: &dto,
		})
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPut {
		handleUpdateNormalizedScenario(w, r, userID, scenarioID)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := appStore.DeleteNormalizedScenario(userID, scenarioID); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"ok":      false,
				"message": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"message": "normalized scenario deleted",
		})
		return
	}

	if len(parts) == 2 && parts[1] == "steps" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		steps, err := appStore.ListScenarioStepsByScenario(userID, scenarioID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, ListScenarioStepsResponse{
				OK:      false,
				Message: fmt.Sprintf("normalized scenario steps not found: %v", err),
			})
			return
		}
		writeJSON(w, http.StatusOK, ListScenarioStepsResponse{
			OK:            true,
			ScenarioSteps: toScenarioStepDTOs(steps),
		})
		return
	}

	if len(parts) == 2 && parts[1] == "details" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeNormalizedScenarioDetails(w, userID, scenarioID)
		return
	}

	if len(parts) == 2 && parts[1] == "run" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		runNormalizedScenarioHandler(w, r, userID, scenarioID)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func writeNormalizedSchemeDetails(w http.ResponseWriter, userID int, schemeID int) {
	scheme, err := appStore.GetNormalizedScheme(schemeID, userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, SchemeDetailsResponse{
			OK:      false,
			Message: fmt.Sprintf("normalized scheme not found: %v", err),
		})
		return
	}

	tracks, err := appStore.ListTracksByScheme(userID, schemeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{
			OK:      false,
			Message: fmt.Sprintf("failed to load tracks: %v", err),
		})
		return
	}
	connections, err := appStore.ListTrackConnectionsByScheme(userID, schemeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{
			OK:      false,
			Message: fmt.Sprintf("failed to load track connections: %v", err),
		})
		return
	}
	wagons, err := appStore.ListWagonsByScheme(userID, schemeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{
			OK:      false,
			Message: fmt.Sprintf("failed to load wagons: %v", err),
		})
		return
	}
	locomotives, err := appStore.ListLocomotivesByScheme(userID, schemeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{
			OK:      false,
			Message: fmt.Sprintf("failed to load locomotives: %v", err),
		})
		return
	}
	couplings, err := appStore.ListNormalizedCouplingsByScheme(userID, schemeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{
			OK:      false,
			Message: fmt.Sprintf("failed to load couplings: %v", err),
		})
		return
	}

	schemeDTO := toSchemeDTO(*scheme)
	writeJSON(w, http.StatusOK, SchemeDetailsResponse{
		OK:               true,
		Scheme:           &schemeDTO,
		Tracks:           toTrackDTOs(tracks),
		TrackConnections: toTrackConnectionDTOs(connections),
		Wagons:           toWagonDTOs(wagons),
		Locomotives:      toLocomotiveDTOs(locomotives),
		Couplings:        toCouplingDTOs(couplings),
	})
}

func writeNormalizedScenarioDetails(w http.ResponseWriter, userID int, scenarioID string) {
	scenario, err := appStore.GetNormalizedScenario(scenarioID, userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ScenarioDetailsResponse{
			OK:      false,
			Message: fmt.Sprintf("normalized scenario not found: %v", err),
		})
		return
	}
	steps, err := appStore.ListScenarioStepsByScenario(userID, scenarioID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ScenarioDetailsResponse{
			OK:      false,
			Message: fmt.Sprintf("failed to load scenario steps: %v", err),
		})
		return
	}

	scenarioDTO := toScenarioDTO(*scenario)
	writeJSON(w, http.StatusOK, ScenarioDetailsResponse{
		OK:            true,
		Scenario:      &scenarioDTO,
		ScenarioSteps: toScenarioStepDTOs(steps),
	})
}

func handleCreateNormalizedScheme(w http.ResponseWriter, r *http.Request, userID int) {
	var req UpsertNormalizedSchemeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, SchemeDetailsResponse{OK: false, Message: "invalid normalized scheme payload"})
		return
	}

	scheme, err := normalizedSchemeFromRequest(0, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, SchemeDetailsResponse{OK: false, Message: err.Error()})
		return
	}

	schemeID, err := appStore.CreateNormalizedScheme(userID, scheme)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{OK: false, Message: fmt.Sprintf("failed to create normalized scheme: %v", err)})
		return
	}

	writeNormalizedSchemeDetails(w, userID, schemeID)
}

func handleUpdateNormalizedScheme(w http.ResponseWriter, r *http.Request, userID int, schemeID int) {
	var req UpsertNormalizedSchemeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, SchemeDetailsResponse{OK: false, Message: "invalid normalized scheme payload"})
		return
	}

	scheme, err := normalizedSchemeFromRequest(schemeID, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, SchemeDetailsResponse{OK: false, Message: err.Error()})
		return
	}

	if err := appStore.UpdateNormalizedScheme(userID, scheme); err != nil {
		writeJSON(w, http.StatusNotFound, SchemeDetailsResponse{OK: false, Message: fmt.Sprintf("failed to update normalized scheme: %v", err)})
		return
	}

	writeNormalizedSchemeDetails(w, userID, schemeID)
}

func handleCreateNormalizedScenario(w http.ResponseWriter, r *http.Request, userID int) {
	var req UpsertNormalizedScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ScenarioDetailsResponse{OK: false, Message: "invalid normalized scenario payload"})
		return
	}

	scenario, err := normalizedScenarioFromRequest("", req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ScenarioDetailsResponse{OK: false, Message: err.Error()})
		return
	}

	scenarioID, err := appStore.CreateNormalizedScenario(userID, scenario)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ScenarioDetailsResponse{OK: false, Message: fmt.Sprintf("failed to create normalized scenario: %v", err)})
		return
	}

	writeNormalizedScenarioDetails(w, userID, scenarioID)
}

func handleUpdateNormalizedScenario(w http.ResponseWriter, r *http.Request, userID int, scenarioID string) {
	var req UpsertNormalizedScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ScenarioDetailsResponse{OK: false, Message: "invalid normalized scenario payload"})
		return
	}

	scenario, err := normalizedScenarioFromRequest(scenarioID, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ScenarioDetailsResponse{OK: false, Message: err.Error()})
		return
	}

	if err := appStore.UpdateNormalizedScenario(userID, scenario); err != nil {
		writeJSON(w, http.StatusNotFound, ScenarioDetailsResponse{OK: false, Message: fmt.Sprintf("failed to update normalized scenario: %v", err)})
		return
	}

	writeNormalizedScenarioDetails(w, userID, scenarioID)
}

func normalizedSchemeFromRequest(schemeID int, req UpsertNormalizedSchemeRequest) (normalized.Scheme, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return normalized.Scheme{}, fmt.Errorf("scheme name is required")
	}
	if len(req.Tracks) == 0 {
		return normalized.Scheme{}, fmt.Errorf("scheme must contain at least one track")
	}

	return normalized.Scheme{
		SchemeID:         schemeID,
		Name:             name,
		Tracks:           dtoToTracks(req.Tracks),
		TrackConnections: dtoToTrackConnections(req.TrackConnections),
		Wagons:           dtoToWagons(req.Wagons),
		Locomotives:      dtoToLocomotives(req.Locomotives),
		Couplings:        dtoToCouplings(req.Couplings),
	}, nil
}

func normalizedScenarioFromRequest(scenarioID string, req UpsertNormalizedScenarioRequest) (normalized.Scenario, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return normalized.Scenario{}, fmt.Errorf("scenario name is required")
	}
	if req.SchemeID <= 0 {
		return normalized.Scenario{}, fmt.Errorf("scenario scheme_id is required")
	}

	steps := dtoToScenarioSteps(req.ScenarioSteps)
	stepPrefix := scenarioID
	if strings.TrimSpace(stepPrefix) == "" {
		stepPrefix = "pending"
	}
	for i := range steps {
		steps[i].ScenarioID = scenarioID
		if strings.TrimSpace(steps[i].StepID) == "" {
			steps[i].StepID = fmt.Sprintf("%s-step-%d", stepPrefix, i+1)
		}
	}

	return normalized.Scenario{
		ScenarioID: scenarioID,
		SchemeID:   req.SchemeID,
		Name:       name,
		Steps:      steps,
	}, nil
}
