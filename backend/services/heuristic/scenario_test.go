package heuristic

import (
	"testing"

	"trains/backend/normalized"
)

// Этот файл содержит тесты для STEP 6 — построения DraftScenario.
//
// Здесь проверяется только преобразование доменных операций в черновой
// эвристический сценарий. Никаких low-level movement-команд и execution logic
// в этих тестах нет.

// TestBuildDraftScenario проверяет, что ordered operations превращаются
// в упорядоченный DraftScenario с корректными общими параметрами схемы
// и корректными типами шагов.
func TestBuildDraftScenario(t *testing.T) {
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

	operations := []HeuristicOperation{
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
			SourceSide:         "",
			WagonCount:         3,
			TargetColor:        "red",
			FormationTrackID:   "lead-2",
			BufferTrackID:      "lead-1",
			MainTrackID:        "main-1",
		},
	}

	scenario := BuildDraftScenario(problem, operations)
	if scenario.SchemeID != 1 {
		t.Fatalf("expected SchemeID=1, got %d", scenario.SchemeID)
	}
	if scenario.TargetColor != "red" {
		t.Fatalf("expected TargetColor=red, got %q", scenario.TargetColor)
	}
	if scenario.FormationTrackID != "lead-2" || scenario.BufferTrackID != "lead-1" || scenario.MainTrackID != "main-1" {
		t.Fatalf("unexpected scenario track ids: %#v", scenario)
	}
	if len(scenario.Steps) != 3 {
		t.Fatalf("expected 3 draft steps, got %d", len(scenario.Steps))
	}
	if scenario.Steps[0].StepType != DraftScenarioStepTransferTargetsToFormation {
		t.Fatalf("unexpected first draft step type: %q", scenario.Steps[0].StepType)
	}
	if scenario.Steps[1].StepType != DraftScenarioStepTransferTargetsToFormation {
		t.Fatalf("unexpected second draft step type: %q", scenario.Steps[1].StepType)
	}
	if scenario.Steps[2].StepType != DraftScenarioStepTransferFormationToMain {
		t.Fatalf("unexpected third draft step type: %q", scenario.Steps[2].StepType)
	}
}

// TestBuildDraftScenarioWithBufferStep проверяет, что операция buffer_blockers
// преобразуется в соответствующий draft-шаг с корректными полями.
func TestBuildDraftScenarioWithBufferStep(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 2,
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

	operations := []HeuristicOperation{
		{
			OperationType:      HeuristicOperationBufferBlockers,
			SourceTrackID:      "sorting-1",
			DestinationTrackID: "lead-1",
			SourceSide:         "start",
			WagonCount:         2,
			TargetColor:        "red",
			FormationTrackID:   "lead-2",
			BufferTrackID:      "lead-1",
			MainTrackID:        "main-1",
		},
	}

	scenario := BuildDraftScenario(problem, operations)
	if len(scenario.Steps) != 1 {
		t.Fatalf("expected 1 draft step, got %d", len(scenario.Steps))
	}
	if scenario.Steps[0].StepType != DraftScenarioStepBufferBlockers {
		t.Fatalf("unexpected draft step type: %q", scenario.Steps[0].StepType)
	}
	if scenario.Steps[0].DestinationTrackID != "lead-1" || scenario.Steps[0].WagonCount != 2 {
		t.Fatalf("unexpected buffer draft step: %#v", scenario.Steps[0])
	}
}
