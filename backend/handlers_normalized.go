package main

import (
	"net/http"
	"strconv"
	"strings"
)

func normalizedSchemesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	schemes, err := appStore.ListNormalizedSchemes(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ListNormalizedSchemesResponse{
			OK:      false,
			Message: "failed to list normalized schemes",
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
	if r.Method != http.MethodGet {
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

	if len(parts) == 1 {
		scheme, err := appStore.GetNormalizedScheme(schemeID, userID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, GetNormalizedSchemeResponse{
				OK:      false,
				Message: "normalized scheme not found",
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

	if len(parts) == 2 && parts[1] == "details" {
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
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	scenarios, err := appStore.ListNormalizedScenarios(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ListNormalizedScenariosResponse{
			OK:      false,
			Message: "failed to list normalized scenarios",
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
	if r.Method != http.MethodGet {
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

	if len(parts) == 1 {
		scenario, err := appStore.GetNormalizedScenario(scenarioID, userID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, GetNormalizedScenarioResponse{
				OK:      false,
				Message: "normalized scenario not found",
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

	if len(parts) == 2 && parts[1] == "steps" {
		steps, err := appStore.ListScenarioStepsByScenario(userID, scenarioID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, ListScenarioStepsResponse{
				OK:      false,
				Message: "normalized scenario steps not found",
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
		writeNormalizedScenarioDetails(w, userID, scenarioID)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func writeNormalizedSchemeDetails(w http.ResponseWriter, userID int, schemeID int) {
	scheme, err := appStore.GetNormalizedScheme(schemeID, userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, SchemeDetailsResponse{
			OK:      false,
			Message: "normalized scheme not found",
		})
		return
	}

	tracks, err := appStore.ListTracksByScheme(userID, schemeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{
			OK:      false,
			Message: "failed to load tracks",
		})
		return
	}
	connections, err := appStore.ListTrackConnectionsByScheme(userID, schemeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{
			OK:      false,
			Message: "failed to load track connections",
		})
		return
	}
	wagons, err := appStore.ListWagonsByScheme(userID, schemeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{
			OK:      false,
			Message: "failed to load wagons",
		})
		return
	}
	locomotives, err := appStore.ListLocomotivesByScheme(userID, schemeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{
			OK:      false,
			Message: "failed to load locomotives",
		})
		return
	}
	couplings, err := appStore.ListNormalizedCouplingsByScheme(userID, schemeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SchemeDetailsResponse{
			OK:      false,
			Message: "failed to load couplings",
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
			Message: "normalized scenario not found",
		})
		return
	}
	steps, err := appStore.ListScenarioStepsByScenario(userID, scenarioID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ScenarioDetailsResponse{
			OK:      false,
			Message: "failed to load scenario steps",
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
