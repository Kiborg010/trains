package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func cloneLayoutState(state LayoutState) LayoutState {
	next := LayoutState{
		Segments:  make([]Segment, len(state.Segments)),
		Vehicles:  make([]Vehicle, len(state.Vehicles)),
		Couplings: make([]Coupling, len(state.Couplings)),
	}
	copy(next.Segments, state.Segments)
	copy(next.Vehicles, state.Vehicles)
	copy(next.Couplings, state.Couplings)
	return next
}

func reverseStrings(items []string) []string {
	result := append([]string{}, items...)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func nextVehicleCode(vehicles []Vehicle, vehicleType string) string {
	prefix := "\u0432"
	if vehicleType == "locomotive" {
		prefix = "\u043b"
	}

	maxNumber := 0
	for _, vehicle := range vehicles {
		if vehicle.Type != vehicleType {
			continue
		}
		if !strings.HasPrefix(vehicle.Code, prefix) {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(vehicle.Code, prefix+"%d", &n); err == nil && n > maxNumber {
			maxNumber = n
		}
	}

	return fmt.Sprintf("%s%d", prefix, maxNumber+1)
}

func slotID(x, y float64) string {
	return fmt.Sprintf("%.2f:%.2f", x, y)
}

func pathSlotKey(pathID string, index int) string {
	return fmt.Sprintf("%s:%d", pathID, index)
}

func pathSlotPairKey(pathA string, indexA int, pathB string, indexB int) string {
	keyA := pathSlotKey(pathA, indexA)
	keyB := pathSlotKey(pathB, indexB)
	if keyA < keyB {
		return keyA + "|" + keyB
	}
	return keyB + "|" + keyA
}

func pairKey(a, b string) string {
	if a < b {
		return a + "|" + b
	}
	return b + "|" + a
}

func normalizeSegmentIDs(state LayoutState) LayoutState {
	if len(state.Segments) == 0 {
		return state
	}

	needsNormalization := false
	for _, segment := range state.Segments {
		if !isSimpleNumericID(segment.ID) {
			needsNormalization = true
			break
		}
	}
	if !needsNormalization {
		return state
	}

	idMap := map[string]string{}
	for i, segment := range state.Segments {
		newID := fmt.Sprintf("%d", i+1)
		idMap[segment.ID] = newID
		state.Segments[i].ID = newID
	}

	for i, vehicle := range state.Vehicles {
		if vehicle.PathID == "" {
			continue
		}
		if newID, ok := idMap[vehicle.PathID]; ok {
			state.Vehicles[i].PathID = newID
		}
	}

	return state
}

func isSimpleNumericID(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func nextPathID(segments []Segment) string {
	maxID := 0
	for _, segment := range segments {
		if !isSimpleNumericID(segment.ID) {
			continue
		}
		var value int
		if _, err := fmt.Sscanf(segment.ID, "%d", &value); err == nil && value > maxID {
			maxID = value
		}
	}
	return fmt.Sprintf("%d", maxID+1)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func userIDFromContext(r *http.Request) (int, error) {
	userID, ok := r.Context().Value("userID").(int)
	if !ok || userID <= 0 {
		return 0, errors.New("unauthorized")
	}
	return userID, nil
}
