package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVehicleJSONIncludesZeroPathIndex(t *testing.T) {
	item := Vehicle{
		ID:        "l1",
		Type:      "locomotive",
		PathID:    "scheme-17-track-2",
		PathIndex: 0,
		X:         0,
		Y:         0,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	if !strings.Contains(string(data), `"pathIndex":0`) {
		t.Fatalf("expected marshaled vehicle to include pathIndex=0, got %s", string(data))
	}
}
