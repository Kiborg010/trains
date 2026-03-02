package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func layoutsHandler(w http.ResponseWriter, r *http.Request) {
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
		layouts, err := appStore.ListLayouts(userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ListLayoutsResponse{
				OK:      false,
				Message: "failed to list layouts",
			})
			return
		}
		writeJSON(w, http.StatusOK, ListLayoutsResponse{
			OK:      true,
			Layouts: layouts,
		})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SaveLayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, SaveLayoutResponse{
			OK:      false,
			Message: "invalid json",
		})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = "Layout"
	}

	layoutID, err := appStore.SaveLayout(userID, req.Name, req.State)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SaveLayoutResponse{
			OK:      false,
			Message: "failed to save layout",
		})
		return
	}

	layout, err := appStore.GetLayout(layoutID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, SaveLayoutResponse{
			OK:      false,
			Message: "failed to load saved layout",
		})
		return
	}

	writeJSON(w, http.StatusOK, SaveLayoutResponse{
		OK:     true,
		Layout: layout,
	})
}

func layoutByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/layouts/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.Error(w, "layout id required", http.StatusBadRequest)
		return
	}
	layoutID, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "invalid layout id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		layout, err := appStore.GetLayout(layoutID, userID)
		if err != nil {
			http.Error(w, "layout not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, SaveLayoutResponse{OK: true, Layout: layout})
		return

	case http.MethodPut:
		var req SaveLayoutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, SaveLayoutResponse{
				OK:      false,
				Message: "invalid json",
			})
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			req.Name = "Layout"
		}
		if err := appStore.UpdateLayout(layoutID, userID, req.Name, req.State); err != nil {
			http.Error(w, "layout not found", http.StatusNotFound)
			return
		}
		layout, err := appStore.GetLayout(layoutID, userID)
		if err != nil {
			http.Error(w, "layout not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, SaveLayoutResponse{OK: true, Layout: layout})
		return

	case http.MethodDelete:
		if err := appStore.DeleteLayout(layoutID, userID); err != nil {
			http.Error(w, "layout not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, SaveLayoutResponse{OK: true, Message: "layout deleted"})
		return

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
