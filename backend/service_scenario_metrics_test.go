package main

import (
	"testing"

	"trains/backend/normalized"
)

func TestComputeScenarioMetrics(t *testing.T) {
	oldStore := appStore
	store := NewInMemoryStore()
	appStore = store
	t.Cleanup(func() {
		appStore = oldStore
	})

	user, err := store.CreateUser("metrics@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	schemeID, err := store.CreateNormalizedScheme(user.ID, normalized.Scheme{
		Name: "Схема метрик",
		Tracks: []normalized.Track{
			{
				TrackID:        "track-1",
				Name:           "1",
				Type:           "lead",
				StartX:         0,
				StartY:         0,
				EndX:           80,
				EndY:           0,
				Capacity:       3,
				StorageAllowed: true,
			},
			{
				TrackID:        "track-2",
				Name:           "2",
				Type:           "sorting",
				StartX:         80,
				StartY:         0,
				EndX:           160,
				EndY:           0,
				Capacity:       3,
				StorageAllowed: true,
			},
		},
		TrackConnections: []normalized.TrackConnection{
			{
				ConnectionID:   "conn-1",
				Track1ID:       "track-1",
				Track2ID:       "track-2",
				Track1Side:     "end",
				Track2Side:     "start",
				ConnectionType: "switch",
			},
		},
		Wagons: []normalized.Wagon{
			{
				WagonID:    "wagon-1",
				Name:       "в1",
				Color:      "#0ea5e9",
				TrackID:    "track-2",
				TrackIndex: 2,
			},
		},
		Locomotives: []normalized.Locomotive{
			{
				LocoID:     "loco-1",
				Name:       "л1",
				Color:      "#f59e0b",
				TrackID:    "track-1",
				TrackIndex: 0,
			},
		},
	})
	if err != nil {
		t.Fatalf("create scheme: %v", err)
	}

	scenarioID, err := store.CreateNormalizedScenario(user.ID, normalized.Scenario{
		SchemeID: schemeID,
		Name:     "Сценарий метрик",
		Steps: []normalized.ScenarioStep{
			{
				StepID:      "step-1",
				StepOrder:   1,
				StepType:    "move_loco",
				Object1ID:   stringPtr("loco-1"),
				FromTrackID: stringPtr("track-1"),
				FromIndex:   intPtr(0),
				ToTrackID:   stringPtr("track-2"),
				ToIndex:     intPtr(1),
			},
			{
				StepID:    "step-2",
				StepOrder: 2,
				StepType:  "couple",
				Object1ID: stringPtr("loco-1"),
				Object2ID: stringPtr("wagon-1"),
			},
			{
				StepID:    "step-3",
				StepOrder: 3,
				StepType:  "decouple",
				Object1ID: stringPtr("loco-1"),
				Object2ID: stringPtr("wagon-1"),
			},
		},
	})
	if err != nil {
		t.Fatalf("create scenario: %v", err)
	}

	metrics, err := ComputeScenarioMetrics(user.ID, scenarioID)
	if err != nil {
		t.Fatalf("compute metrics: %v", err)
	}

	if metrics.TotalLocoDistance <= 0 {
		t.Fatalf("expected positive loco distance, got %d", metrics.TotalLocoDistance)
	}
	if metrics.TotalCouples != 1 {
		t.Fatalf("expected 1 couple, got %d", metrics.TotalCouples)
	}
	if metrics.TotalDecouples != 1 {
		t.Fatalf("expected 1 decouple, got %d", metrics.TotalDecouples)
	}
	if metrics.TotalSwitchCrossings != 1 {
		t.Fatalf("expected 1 switch crossing, got %d", metrics.TotalSwitchCrossings)
	}

	savedMetrics, err := store.GetScenarioMetrics(user.ID, scenarioID)
	if err != nil {
		t.Fatalf("load saved metrics: %v", err)
	}
	if savedMetrics.TotalSwitchCrossings != 1 {
		t.Fatalf("expected saved metrics, got %#v", savedMetrics)
	}
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}
