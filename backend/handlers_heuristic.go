package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func normalizedHeuristicGenerateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req GenerateDraftHeuristicScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, DraftHeuristicScenarioResponse{
			OK:      false,
			Message: "invalid heuristic request payload",
		})
		return
	}

	resp, err := GenerateDraftHeuristicScenario(userID, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, DraftHeuristicScenarioResponse{
			OK:      false,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func normalizedHeuristicGenerateAndSaveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req GenerateAndSaveDraftHeuristicScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, SaveDraftHeuristicScenarioResponse{
			OK:      false,
			Message: "invalid heuristic save request payload",
		})
		return
	}

	resp, err := GenerateAndSaveDraftHeuristicScenario(userID, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, SaveDraftHeuristicScenarioResponse{
			OK:      false,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func normalizedHeuristicGenerateFullScenarioHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req GenerateFullHeuristicScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, GenerateFullHeuristicScenarioResponse{
			OK:      false,
			Message: "invalid full heuristic scenario request payload",
		})
		return
	}

	resp, err := GenerateFullHeuristicScenario(userID, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, GenerateFullHeuristicScenarioResponse{
			OK:      false,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func normalizedHeuristicScenariosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	resp, err := ListStoredHeuristicScenarios(userID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ListHeuristicScenariosResponse{
			OK:      false,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func normalizedHeuristicScenarioByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	prefix := "/api/normalized/heuristic/scenarios/"
	id := strings.TrimPrefix(r.URL.Path, prefix)
	if id == "" || id == r.URL.Path {
		writeJSON(w, http.StatusBadRequest, GetHeuristicScenarioResponse{
			OK:      false,
			Message: "heuristic scenario id is required",
		})
		return
	}

	resp, err := GetStoredHeuristicScenario(userID, id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, GetHeuristicScenarioResponse{
			OK:      false,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func normalizedHeuristicSaveAsScenarioHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req SaveHeuristicAsScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, SaveHeuristicAsScenarioResponse{
			OK:      false,
			Message: "invalid save-as-scenario request payload",
		})
		return
	}

	resp, err := SaveHeuristicDraftAsScenario(userID, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, SaveHeuristicAsScenarioResponse{
			OK:      false,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
