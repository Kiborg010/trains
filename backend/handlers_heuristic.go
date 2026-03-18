package main

import (
	"encoding/json"
	"net/http"
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
