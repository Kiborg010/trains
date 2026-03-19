package main

import (
	"testing"

	"trains/backend/normalized"
)

func TestInMemoryStoreDeleteNormalizedScenarioDeletesHeuristicSource(t *testing.T) {
	store := NewInMemoryStore()

	heuristicID, err := store.CreateHeuristicScenario(1, normalized.HeuristicScenario{
		SchemeID:            1,
		Name:                "heuristic",
		TargetColor:         "blue",
		RequiredTargetCount: 2,
		FormationTrackID:    "f1",
		BufferTrackID:       "b1",
		MainTrackID:         "m1",
	})
	if err != nil {
		t.Fatalf("CreateHeuristicScenario() error = %v", err)
	}

	scenarioID, err := store.CreateNormalizedScenario(1, normalized.Scenario{
		SchemeID:                  1,
		Name:                      "scenario",
		SourceHeuristicScenarioID: &heuristicID,
	})
	if err != nil {
		t.Fatalf("CreateNormalizedScenario() error = %v", err)
	}

	if err := store.DeleteNormalizedScenario(1, scenarioID); err != nil {
		t.Fatalf("DeleteNormalizedScenario() error = %v", err)
	}

	if _, err := store.GetHeuristicScenario(heuristicID, 1); err == nil {
		t.Fatalf("expected heuristic scenario %q to be deleted together with derived scenario", heuristicID)
	}
}
