package main

import (
	"encoding/json"
	"net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
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
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateCouplingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ValidateCouplingResponse{
			OK:      false,
			Message: "некорректный JSON",
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
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	var req PlaceVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, PlaceVehicleResponse{
			OK:      false,
			Message: "некорректный JSON",
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

func resolveVehiclesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	var req ResolveVehiclesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ResolveVehiclesResponse{
			OK:      false,
			Message: "некорректный JSON",
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
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	var req LayoutOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, LayoutOperationResponse{
			OK:      false,
			Message: "некорректный JSON",
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
	nextState = finalizeRuntimeState(nextState, req.GridSize)

	writeJSON(w, http.StatusOK, LayoutOperationResponse{
		OK:      true,
		Message: message,
		State:   nextState,
	})
}
