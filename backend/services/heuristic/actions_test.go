package heuristic

import (
	"testing"

	"trains/backend/normalized"
)

// Этот файл содержит тесты для STEP 4 — построения high-level action plan.
//
// Здесь мы проверяем только преобразование уже выбранных extraction decisions
// в декларативный план действий. Никаких low-level movement команд в тестах
// не ожидается и не проверяется.

// TestBuildHighLevelHeuristicPlan проверяет базовый положительный сценарий:
// два заранее выбранных extraction decision должны быть преобразованы
// в два extraction action и один финальный action перевода состава на main.
func TestBuildHighLevelHeuristicPlan(t *testing.T) {
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
	state := BuildFixedClassPlanningState(problem, 3)
	orderedPlan := []TargetExtractionCandidate{
		{
			SourceSortingTrackID: "sorting-1",
			SourceSide:           "end",
			BlockingCount:        0,
			TargetGroupSize:      2,
			TakeCount:            2,
			EstimatedCost:        1,
			Feasible:             true,
		},
		{
			SourceSortingTrackID: "sorting-2",
			SourceSide:           "start",
			BlockingCount:        0,
			TargetGroupSize:      1,
			TakeCount:            1,
			EstimatedCost:        2,
			Feasible:             true,
		},
	}
	actions := BuildHighLevelHeuristicPlan(problem, state, orderedPlan)

	if len(actions) != 3 {
		t.Fatalf("expected 3 high-level actions, got %d", len(actions))
	}
	if actions[0].ActionType != HeuristicActionExtractTargetGroupToFormation {
		t.Fatalf("expected first action to be extract_target_group_to_formation, got %q", actions[0].ActionType)
	}
	if actions[1].ActionType != HeuristicActionExtractTargetGroupToFormation {
		t.Fatalf("expected second action to be extract_target_group_to_formation, got %q", actions[1].ActionType)
	}
	if actions[2].ActionType != HeuristicActionFinalTransferFormationToMain {
		t.Fatalf("expected final action to be final_transfer_formation_to_main, got %q", actions[2].ActionType)
	}
	if actions[2].FormationTrackID != "lead-2" || actions[2].MainTrackID != "main-1" {
		t.Fatalf("unexpected final transfer tracks: %#v", actions[2])
	}
}

// TestBuildHighLevelHeuristicPlanAddsBlockerAction проверяет, что для extraction
// decision с blocking вагонами builder сначала добавляет action переноса блокировок
// в буфер, а затем action извлечения target-группы.
func TestBuildHighLevelHeuristicPlanAddsBlockerAction(t *testing.T) {
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
	state := BuildFixedClassPlanningState(problem, 1)
	orderedPlan := []TargetExtractionCandidate{
		{
			SourceSortingTrackID: "sorting-1",
			SourceSide:           "start",
			BlockingCount:        2,
			TargetGroupSize:      1,
			TakeCount:            1,
			EstimatedCost:        20,
			Feasible:             true,
		},
	}

	actions := BuildHighLevelHeuristicPlan(problem, state, orderedPlan)
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(actions))
	}
	if actions[0].ActionType != HeuristicActionMoveBlockersToBuffer {
		t.Fatalf("expected first action to be move_blockers_to_buffer, got %q", actions[0].ActionType)
	}
	if actions[1].ActionType != HeuristicActionExtractTargetGroupToFormation {
		t.Fatalf("expected second action to be extract_target_group_to_formation, got %q", actions[1].ActionType)
	}
	if actions[2].ActionType != HeuristicActionFinalTransferFormationToMain {
		t.Fatalf("expected third action to be final_transfer_formation_to_main, got %q", actions[2].ActionType)
	}
	if actions[0].BlockingCount != 2 {
		t.Fatalf("expected blocking_count=2, got %d", actions[0].BlockingCount)
	}
	if actions[1].TakeCount != 1 {
		t.Fatalf("expected take_count=1, got %d", actions[1].TakeCount)
	}
}
