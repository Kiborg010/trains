package heuristic

import (
	"testing"

	"trains/backend/normalized"
)

// TestBuildLowLevelScenarioStepsFromHeuristicOperationsForTargetTransfer
// проверяет самый базовый положительный кейс:
// одна операция transfer_targets_to_formation должна развернуться
// в четыре обычных шага move_loco/couple/move_loco/decouple.
func TestBuildLowLevelScenarioStepsFromHeuristicOperationsForTargetTransfer(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "sorting-1", Type: "sorting", Capacity: 6, StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "main-1", Type: "main", Capacity: 8, StorageAllowed: false},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "blue", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "red", TrackID: "sorting-1", TrackIndex: 1},
			{WagonID: "w3", Color: "red", TrackID: "sorting-1", TrackIndex: 2},
		},
		Locomotives: []normalized.Locomotive{
			{LocoID: "l1", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	steps, err := BuildLowLevelScenarioStepsFromHeuristicOperations(
		"nsc-test",
		scheme,
		[]HeuristicOperation{
			{
				OperationType:      HeuristicOperationTransferTargetsToFormation,
				SourceTrackID:      "sorting-1",
				DestinationTrackID: "lead-2",
				SourceSide:         "end",
				WagonCount:         2,
				TargetColor:        "red",
				FormationTrackID:   "lead-2",
				BufferTrackID:      "lead-1",
				MainTrackID:        "main-1",
			},
		},
		scheme.Locomotives[0],
		scheme.Wagons,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(steps) != 4 {
		t.Fatalf("expected 4 low-level steps, got %d", len(steps))
	}
	if steps[0].StepType != "move_loco" {
		t.Fatalf("expected first step to be move_loco, got %s", steps[0].StepType)
	}
	if steps[1].StepType != "couple" {
		t.Fatalf("expected second step to be couple, got %s", steps[1].StepType)
	}
	if steps[2].StepType != "move_loco" {
		t.Fatalf("expected third step to be move_loco, got %s", steps[2].StepType)
	}
	if steps[3].StepType != "decouple" {
		t.Fatalf("expected fourth step to be decouple, got %s", steps[3].StepType)
	}

	if steps[0].FromTrackID == nil || *steps[0].FromTrackID != "lead-1" {
		t.Fatalf("unexpected first move source: %+v", steps[0].FromTrackID)
	}
	if steps[0].ToTrackID == nil || *steps[0].ToTrackID != "sorting-1" {
		t.Fatalf("unexpected first move destination: %+v", steps[0].ToTrackID)
	}
	if steps[0].ToIndex == nil || *steps[0].ToIndex != 2 {
		t.Fatalf("expected approach to end index 2, got %+v", steps[0].ToIndex)
	}
	if steps[1].Object2ID == nil || *steps[1].Object2ID != "w3" {
		t.Fatalf("expected couple with end-side wagon w3, got %+v", steps[1].Object2ID)
	}
	if steps[2].ToTrackID == nil || *steps[2].ToTrackID != "lead-2" {
		t.Fatalf("unexpected transfer destination: %+v", steps[2].ToTrackID)
	}
	if steps[2].ToIndex == nil || *steps[2].ToIndex != 1 {
		t.Fatalf("expected transfer boundary index 1 on destination, got %+v", steps[2].ToIndex)
	}
}

// TestBuildLowLevelScenarioStepsFromHeuristicOperationsForWholeScenario
// проверяет, что последовательность из нескольких heuristic operations
// полностью разворачивается в обычные шаги и больше не содержит move_group.
func TestBuildLowLevelScenarioStepsFromHeuristicOperationsForWholeScenario(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 2,
		Tracks: []normalized.Track{
			{TrackID: "sorting-1", Type: "sorting", Capacity: 6, StorageAllowed: true},
			{TrackID: "sorting-2", Type: "sorting", Capacity: 6, StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "main-1", Type: "main", Capacity: 8, StorageAllowed: false},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "blue", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "red", TrackID: "sorting-1", TrackIndex: 1},
			{WagonID: "w3", Color: "red", TrackID: "sorting-1", TrackIndex: 2},
			{WagonID: "w4", Color: "red", TrackID: "sorting-2", TrackIndex: 0},
			{WagonID: "w5", Color: "blue", TrackID: "sorting-2", TrackIndex: 1},
		},
		Locomotives: []normalized.Locomotive{
			{LocoID: "l1", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	steps, err := BuildLowLevelScenarioStepsFromHeuristicOperations(
		"nsc-test-whole",
		scheme,
		[]HeuristicOperation{
			{
				OperationType:      HeuristicOperationTransferTargetsToFormation,
				SourceTrackID:      "sorting-1",
				DestinationTrackID: "lead-2",
				SourceSide:         "end",
				WagonCount:         2,
				TargetColor:        "red",
				FormationTrackID:   "lead-2",
				BufferTrackID:      "lead-1",
				MainTrackID:        "main-1",
			},
			{
				OperationType:      HeuristicOperationTransferTargetsToFormation,
				SourceTrackID:      "sorting-2",
				DestinationTrackID: "lead-2",
				SourceSide:         "start",
				WagonCount:         1,
				TargetColor:        "red",
				FormationTrackID:   "lead-2",
				BufferTrackID:      "lead-1",
				MainTrackID:        "main-1",
			},
			{
				OperationType:      HeuristicOperationTransferFormationToMain,
				SourceTrackID:      "lead-2",
				DestinationTrackID: "main-1",
				WagonCount:         3,
				TargetColor:        "red",
				FormationTrackID:   "lead-2",
				BufferTrackID:      "lead-1",
				MainTrackID:        "main-1",
			},
		},
		scheme.Locomotives[0],
		scheme.Wagons,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(steps) != 12 {
		t.Fatalf("expected 12 low-level steps for three operations, got %d", len(steps))
	}
	for _, step := range steps {
		if step.StepType == "move_group" {
			t.Fatalf("move_group should not be used by low-level builder")
		}
	}

	// Проверяем финальную четвёрку шагов для transfer_formation_to_main.
	last := steps[8:]
	if last[0].StepType != "move_loco" || last[1].StepType != "couple" || last[2].StepType != "move_loco" || last[3].StepType != "decouple" {
		t.Fatalf("unexpected final operation shape: %+v", last)
	}
	if last[0].FromTrackID == nil || *last[0].FromTrackID != "lead-2" {
		t.Fatalf("expected final approach to start from formation track, got %+v", last[0].FromTrackID)
	}
	if last[2].ToTrackID == nil || *last[2].ToTrackID != "main-1" {
		t.Fatalf("expected final transfer to main track, got %+v", last[2].ToTrackID)
	}
}
