package main

import (
	"encoding/json"
	"net/http"
)

func planMovementHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	var req PlanMovementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, PlanMovementResponse{
			OK:      false,
			Message: "некорректный JSON",
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
