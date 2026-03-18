package heuristic

import (
	"testing"

	"trains/backend/normalized"
)

// Этот файл содержит тесты для STEP 5 — преобразования high-level actions
// в промежуточные доменные операции.
//
// Здесь проверяется только доменное преобразование action -> operation.
// Никаких scenario_steps, low-level movement-команд и path finding здесь нет.

// TestBuildHeuristicOperations проверяет базовый положительный сценарий:
// high-level actions должны быть преобразованы в доменные операции
// в том же порядке, без потери ключевых параметров.
func TestBuildHeuristicOperations(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 6},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "blue", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "red", TrackID: "sorting-1", TrackIndex: 1},
			{WagonID: "w3", Color: "red", TrackID: "sorting-1", TrackIndex: 2},
			{WagonID: "w4", Color: "red", TrackID: "sorting-2", TrackIndex: 0},
			{WagonID: "w5", Color: "blue", TrackID: "sorting-2", TrackIndex: 1},
			{WagonID: "w6", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected problem build error: %v", err)
	}

	actions := []HeuristicAction{
		{
			ActionType:       HeuristicActionExtractTargetGroupToFormation,
			SourceTrackID:    "sorting-1",
			SourceSide:       "end",
			BufferTrackID:    "lead-1",
			FormationTrackID: "lead-2",
			MainTrackID:      "main-1",
			BlockingCount:    0,
			TakeCount:        2,
			TargetGroupSize:  2,
		},
		{
			ActionType:       HeuristicActionExtractTargetGroupToFormation,
			SourceTrackID:    "sorting-2",
			SourceSide:       "start",
			BufferTrackID:    "lead-1",
			FormationTrackID: "lead-2",
			MainTrackID:      "main-1",
			BlockingCount:    0,
			TakeCount:        1,
			TargetGroupSize:  1,
		},
		{
			ActionType:       HeuristicActionFinalTransferFormationToMain,
			SourceTrackID:    "lead-2",
			SourceSide:       "",
			BufferTrackID:    "lead-1",
			FormationTrackID: "lead-2",
			MainTrackID:      "main-1",
			BlockingCount:    0,
			TakeCount:        3,
			TargetGroupSize:  3,
		},
	}

	operations := BuildHeuristicOperations(problem, actions)
	if len(operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(operations))
	}
	if operations[0].OperationType != HeuristicOperationTransferTargetsToFormation {
		t.Fatalf("expected first operation to be transfer_targets_to_formation, got %q", operations[0].OperationType)
	}
	if operations[0].SourceTrackID != "sorting-1" || operations[0].DestinationTrackID != "lead-2" {
		t.Fatalf("unexpected first operation routing: %#v", operations[0])
	}
	if operations[1].OperationType != HeuristicOperationTransferTargetsToFormation {
		t.Fatalf("expected second operation to be transfer_targets_to_formation, got %q", operations[1].OperationType)
	}
	if operations[2].OperationType != HeuristicOperationTransferFormationToMain {
		t.Fatalf("expected final operation to be transfer_formation_to_main, got %q", operations[2].OperationType)
	}
	if operations[2].SourceTrackID != "lead-2" || operations[2].DestinationTrackID != "main-1" {
		t.Fatalf("unexpected final operation routing: %#v", operations[2])
	}
}

// TestBuildHeuristicOperationsWithBlockers проверяет, что action переноса
// блокировок преобразуется в buffer_blockers с корректным направлением
// и количеством вагонов.
func TestBuildHeuristicOperationsWithBlockers(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 6},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected problem build error: %v", err)
	}

	actions := []HeuristicAction{
		{
			ActionType:       HeuristicActionMoveBlockersToBuffer,
			SourceTrackID:    "sorting-1",
			SourceSide:       "start",
			BufferTrackID:    "lead-1",
			FormationTrackID: "lead-2",
			MainTrackID:      "main-1",
			BlockingCount:    2,
			TakeCount:        0,
			TargetGroupSize:  1,
		},
		{
			ActionType:       HeuristicActionExtractTargetGroupToFormation,
			SourceTrackID:    "sorting-1",
			SourceSide:       "start",
			BufferTrackID:    "lead-1",
			FormationTrackID: "lead-2",
			MainTrackID:      "main-1",
			BlockingCount:    2,
			TakeCount:        1,
			TargetGroupSize:  1,
		},
	}

	operations := BuildHeuristicOperations(problem, actions)
	if len(operations) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(operations))
	}
	if operations[0].OperationType != HeuristicOperationBufferBlockers {
		t.Fatalf("expected first operation to be buffer_blockers, got %q", operations[0].OperationType)
	}
	if operations[0].DestinationTrackID != "lead-1" || operations[0].WagonCount != 2 {
		t.Fatalf("unexpected blockers operation: %#v", operations[0])
	}
	if operations[1].OperationType != HeuristicOperationTransferTargetsToFormation {
		t.Fatalf("expected second operation to be transfer_targets_to_formation, got %q", operations[1].OperationType)
	}
	if operations[1].DestinationTrackID != "lead-2" || operations[1].WagonCount != 1 {
		t.Fatalf("unexpected target transfer operation: %#v", operations[1])
	}
}
