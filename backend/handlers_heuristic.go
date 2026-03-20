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
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "требуется авторизация", http.StatusUnauthorized)
		return
	}

	var req GenerateDraftHeuristicScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, DraftHeuristicScenarioResponse{
			OK:      false,
			Message: "некорректный запрос на генерацию эвристического сценария",
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
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "требуется авторизация", http.StatusUnauthorized)
		return
	}

	var req GenerateAndSaveDraftHeuristicScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, SaveDraftHeuristicScenarioResponse{
			OK:      false,
			Message: "некорректный запрос на сохранение эвристического сценария",
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
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "требуется авторизация", http.StatusUnauthorized)
		return
	}

	var req GenerateFullHeuristicScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, GenerateFullHeuristicScenarioResponse{
			OK:      false,
			Message: "некорректный запрос на полный эвристический сценарий",
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
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "требуется авторизация", http.StatusUnauthorized)
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
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "требуется авторизация", http.StatusUnauthorized)
		return
	}

	prefix := "/api/normalized/heuristic/scenarios/"
	id := strings.TrimPrefix(r.URL.Path, prefix)
	if id == "" || id == r.URL.Path {
		writeJSON(w, http.StatusBadRequest, GetHeuristicScenarioResponse{
			OK:      false,
			Message: "нужно указать идентификатор эвристического сценария",
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
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID == 0 {
		http.Error(w, "требуется авторизация", http.StatusUnauthorized)
		return
	}

	var req SaveHeuristicAsScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, SaveHeuristicAsScenarioResponse{
			OK:      false,
			Message: "некорректный запрос на сохранение как обычного сценария",
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
