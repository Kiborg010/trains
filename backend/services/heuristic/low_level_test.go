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

	if len(steps) != 5 {
		t.Fatalf("expected 5 low-level steps for two-wagon transfer, got %d", len(steps))
	}
	if steps[0].StepType != "move_loco" {
		t.Fatalf("expected first step to be move_loco, got %s", steps[0].StepType)
	}
	if steps[1].StepType != "couple" {
		t.Fatalf("expected second step to be couple, got %s", steps[1].StepType)
	}
	if steps[2].StepType != "couple" {
		t.Fatalf("expected third step to be couple for internal wagon chain, got %s", steps[2].StepType)
	}
	if steps[3].StepType != "move_loco" {
		t.Fatalf("expected fourth step to be move_loco, got %s", steps[3].StepType)
	}
	if steps[4].StepType != "decouple" {
		t.Fatalf("expected fifth step to be decouple, got %s", steps[4].StepType)
	}

	if steps[0].FromTrackID == nil || *steps[0].FromTrackID != "lead-1" {
		t.Fatalf("unexpected first move source: %+v", steps[0].FromTrackID)
	}
	if steps[0].ToTrackID == nil || *steps[0].ToTrackID != "sorting-1" {
		t.Fatalf("unexpected first move destination: %+v", steps[0].ToTrackID)
	}
	if steps[0].ToIndex == nil || *steps[0].ToIndex != 3 {
		t.Fatalf("expected approach to free index 3 after group ending at index 2, got %+v", steps[0].ToIndex)
	}
	if steps[1].Object2ID == nil || *steps[1].Object2ID != "w3" {
		t.Fatalf("expected couple with end-side wagon w3, got %+v", steps[1].Object2ID)
	}
	if steps[2].Object1ID == nil || *steps[2].Object1ID != "w3" || steps[2].Object2ID == nil || *steps[2].Object2ID != "w2" {
		t.Fatalf("expected internal group coupling w3-w2 before transfer, got %+v %+v", steps[2].Object1ID, steps[2].Object2ID)
	}
	if steps[3].ToTrackID == nil || *steps[3].ToTrackID != "lead-2" {
		t.Fatalf("unexpected transfer destination: %+v", steps[3].ToTrackID)
	}
	if steps[3].ToIndex == nil || *steps[3].ToIndex != 3 {
		t.Fatalf("expected locomotive to remain after the transferred group and the junction clearance slot, got %+v", steps[2].ToIndex)
	}
}

// TestBuildLowLevelScenarioStepsFromHeuristicOperationsApproachFromStart
// проверяет, что при подходе со стороны start локомотив встаёт
// на соседний свободный индекс перед вагоном группы, а не в сам вагон.
func TestBuildLowLevelScenarioStepsFromHeuristicOperationsApproachFromStart(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 10,
		Tracks: []normalized.Track{
			{TrackID: "sorting-1", Type: "sorting", Capacity: 8, StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "main-1", Type: "main", Capacity: 8, StorageAllowed: false},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 3},
		},
		Locomotives: []normalized.Locomotive{
			{LocoID: "l1", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	steps, err := BuildLowLevelScenarioStepsFromHeuristicOperations(
		"nsc-approach-start",
		scheme,
		[]HeuristicOperation{
			{
				OperationType:      HeuristicOperationTransferTargetsToFormation,
				SourceTrackID:      "sorting-1",
				DestinationTrackID: "lead-2",
				SourceSide:         "start",
				WagonCount:         1,
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

	if steps[0].ToIndex == nil || *steps[0].ToIndex != 2 {
		t.Fatalf("expected approach index 2 before wagon at index 3, got %+v", steps[0].ToIndex)
	}
	if steps[0].ToIndex != nil && *steps[0].ToIndex == 3 {
		t.Fatalf("locomotive must not be placed on occupied wagon index")
	}
}

// TestBuildLowLevelScenarioStepsFromHeuristicOperationsApproachFromEnd
// проверяет, что при подходе со стороны end локомотив встаёт
// на соседний свободный индекс после вагона группы, а не в сам вагон.
func TestBuildLowLevelScenarioStepsFromHeuristicOperationsApproachFromEnd(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 11,
		Tracks: []normalized.Track{
			{TrackID: "sorting-1", Type: "sorting", Capacity: 8, StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "main-1", Type: "main", Capacity: 8, StorageAllowed: false},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 3},
		},
		Locomotives: []normalized.Locomotive{
			{LocoID: "l1", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	steps, err := BuildLowLevelScenarioStepsFromHeuristicOperations(
		"nsc-approach-end",
		scheme,
		[]HeuristicOperation{
			{
				OperationType:      HeuristicOperationTransferTargetsToFormation,
				SourceTrackID:      "sorting-1",
				DestinationTrackID: "lead-2",
				SourceSide:         "end",
				WagonCount:         1,
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

	if steps[0].ToIndex == nil || *steps[0].ToIndex != 4 {
		t.Fatalf("expected approach index 4 after wagon at index 3, got %+v", steps[0].ToIndex)
	}
	if steps[0].ToIndex != nil && *steps[0].ToIndex == 3 {
		t.Fatalf("locomotive must not be placed on occupied wagon index")
	}
}

// TestBuildLowLevelScenarioStepsFromHeuristicOperationsForBufferBlockers
// проверяет разворачивание buffer_blockers в тот же базовый skeleton
// и убеждается, что builder выбирает крайний вагон со стороны start.
func TestBuildLowLevelScenarioStepsFromHeuristicOperationsForBufferBlockers(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 3,
		Tracks: []normalized.Track{
			{TrackID: "sorting-1", Type: "sorting", Capacity: 6, StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "main-1", Type: "main", Capacity: 8, StorageAllowed: false},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "blue", TrackID: "sorting-1", TrackIndex: 2},
			{WagonID: "w2", Color: "blue", TrackID: "sorting-1", TrackIndex: 3},
			{WagonID: "w3", Color: "red", TrackID: "sorting-1", TrackIndex: 4},
		},
		Locomotives: []normalized.Locomotive{
			{LocoID: "l1", TrackID: "lead-2", TrackIndex: 0},
		},
	}

	steps, err := BuildLowLevelScenarioStepsFromHeuristicOperations(
		"nsc-buffer",
		scheme,
		[]HeuristicOperation{
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
		},
		scheme.Locomotives[0],
		scheme.Wagons,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(steps) != 5 {
		t.Fatalf("expected 5 low-level steps, got %d", len(steps))
	}
	if steps[0].ToIndex == nil || *steps[0].ToIndex != 1 {
		t.Fatalf("expected approach index 1 before start-side blockers, got %+v", steps[0].ToIndex)
	}
	if steps[1].Object2ID == nil || *steps[1].Object2ID != "w1" {
		t.Fatalf("expected couple with start-side blocking wagon w1, got %+v", steps[1].Object2ID)
	}
	if steps[3].ToTrackID == nil || *steps[3].ToTrackID != "lead-1" {
		t.Fatalf("expected blockers to be transferred to buffer track, got %+v", steps[3].ToTrackID)
	}
	if steps[3].ToIndex == nil || *steps[3].ToIndex != 0 {
		t.Fatalf("expected blockers to be placed from buffer start, got %+v", steps[3].ToIndex)
	}
}

// TestBuildLowLevelScenarioStepsFromHeuristicOperationsForTargetTransferFromStart
// проверяет важный случай prepend на путь формирования:
// если source_side == start и на formation уже есть вагоны,
// новая группа должна ложиться в начало, а не просто дописываться в хвост.
func TestBuildLowLevelScenarioStepsFromHeuristicOperationsForTargetTransferFromStart(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 4,
		Tracks: []normalized.Track{
			{TrackID: "sorting-1", Type: "sorting", Capacity: 6, StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", Capacity: 6, StorageAllowed: true},
			{TrackID: "main-1", Type: "main", Capacity: 8, StorageAllowed: false},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 3},
			{WagonID: "w2", Color: "blue", TrackID: "sorting-1", TrackIndex: 4},
			{WagonID: "f1", Color: "red", TrackID: "lead-2", TrackIndex: 0},
		},
		Locomotives: []normalized.Locomotive{
			{LocoID: "l1", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	steps, err := BuildLowLevelScenarioStepsFromHeuristicOperations(
		"nsc-start",
		scheme,
		[]HeuristicOperation{
			{
				OperationType:      HeuristicOperationTransferTargetsToFormation,
				SourceTrackID:      "sorting-1",
				DestinationTrackID: "lead-2",
				SourceSide:         "start",
				WagonCount:         1,
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

	if steps[1].Object2ID == nil || *steps[1].Object2ID != "w1" {
		t.Fatalf("expected couple with start-side target wagon w1, got %+v", steps[1].Object2ID)
	}
	if steps[0].ToIndex == nil || *steps[0].ToIndex != 2 {
		t.Fatalf("expected approach index 2 before start-side target wagon at 3, got %+v", steps[0].ToIndex)
	}
	if steps[2].ToIndex == nil || *steps[2].ToIndex != 0 {
		t.Fatalf("expected prepend placement on formation start, got %+v", steps[2].ToIndex)
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
			{WagonID: "w4", Color: "red", TrackID: "sorting-2", TrackIndex: 3},
			{WagonID: "w5", Color: "blue", TrackID: "sorting-2", TrackIndex: 4},
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

	if len(steps) != 13 {
		t.Fatalf("expected 13 low-level steps for three operations with final transfer left coupled on main track, got %d", len(steps))
	}
	for _, step := range steps {
		if step.StepType == "move_group" {
			t.Fatalf("move_group should not be used by low-level builder")
		}
	}

	// Проверяем финальную пятёрку шагов для transfer_formation_to_main:
	// подход, сцепка локомотива, достройка цепочки состава, перенос, расцепка.
	last := steps[9:]
	if len(last) != 4 {
		t.Fatalf("expected 4 steps in final operation without final decouple, got %d", len(last))
	}
	if last[0].StepType != "move_loco" || last[1].StepType != "couple" || last[2].StepType != "couple" || last[3].StepType != "move_loco" {
		t.Fatalf("unexpected final operation shape: %+v", last)
	}
	if last[0].FromTrackID == nil || *last[0].FromTrackID != "lead-2" {
		t.Fatalf("expected final approach to start from formation track, got %+v", last[0].FromTrackID)
	}
	if last[0].ToIndex == nil || *last[0].ToIndex != 0 {
		t.Fatalf("expected final approach to start from the locomotive-side edge of the formation, got %+v", last[0].ToIndex)
	}
	if last[1].Object2ID == nil || *last[1].Object2ID != "w4" {
		t.Fatalf("expected final couple to use the boundary wagon of the whole formation, got %+v", last[1].Object2ID)
	}
	if last[2].Object1ID == nil || last[2].Object2ID == nil {
		t.Fatalf("expected final operation to add an internal coupling before transfer, got %+v %+v", last[2].Object1ID, last[2].Object2ID)
	}
	if *last[2].Object1ID == "l1" || *last[2].Object2ID == "l1" {
		t.Fatalf("expected internal wagon coupling before final transfer, got locomotive coupling %+v %+v", last[2].Object1ID, last[2].Object2ID)
	}
	if last[3].ToTrackID == nil || *last[3].ToTrackID != "main-1" {
		t.Fatalf("expected final transfer to main track, got %+v", last[3].ToTrackID)
	}
	if last[3].ToIndex == nil || *last[3].ToIndex != 1 {
		t.Fatalf("expected final transfer to leave locomotive on first inner main-track node, got %+v", last[3].ToIndex)
	}
}

func TestBuildLowLevelScenarioStepsAddsInternalCouplingsForMultiWagonGroup(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 21,
		Tracks: []normalized.Track{
			{TrackID: "sorting-1", Type: "sorting", Capacity: 8, StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", Capacity: 8, StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", Capacity: 8, StorageAllowed: true},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 1},
			{WagonID: "w2", Color: "red", TrackID: "sorting-1", TrackIndex: 2},
			{WagonID: "w3", Color: "red", TrackID: "sorting-1", TrackIndex: 3},
		},
		Locomotives: []normalized.Locomotive{
			{LocoID: "l1", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	steps, err := BuildLowLevelScenarioStepsFromHeuristicOperations(
		"nsc-group-couplings",
		scheme,
		[]HeuristicOperation{
			{
				OperationType:      HeuristicOperationTransferTargetsToFormation,
				SourceTrackID:      "sorting-1",
				DestinationTrackID: "lead-2",
				SourceSide:         "end",
				WagonCount:         3,
				TargetColor:        "red",
				FormationTrackID:   "lead-2",
				BufferTrackID:      "lead-1",
			},
		},
		scheme.Locomotives[0],
		scheme.Wagons,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(steps) != 6 {
		t.Fatalf("expected 6 steps for three-wagon transfer, got %d", len(steps))
	}
	if steps[1].StepType != "couple" || steps[2].StepType != "couple" || steps[3].StepType != "couple" {
		t.Fatalf("expected loco couple plus two internal wagon couplings, got step types %s, %s, %s", steps[1].StepType, steps[2].StepType, steps[3].StepType)
	}
	if steps[2].Object1ID == nil || *steps[2].Object1ID != "w3" || steps[2].Object2ID == nil || *steps[2].Object2ID != "w2" {
		t.Fatalf("expected first internal coupling w3-w2, got %+v %+v", steps[2].Object1ID, steps[2].Object2ID)
	}
	if steps[3].Object1ID == nil || *steps[3].Object1ID != "w2" || steps[3].Object2ID == nil || *steps[3].Object2ID != "w1" {
		t.Fatalf("expected second internal coupling w2-w1, got %+v %+v", steps[3].Object1ID, steps[3].Object2ID)
	}
}

func TestBuildLowLevelScenarioStepsCutsSourceCouplingBeforeMovingSelectedGroup(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 22,
		Tracks: []normalized.Track{
			{TrackID: "sorting-1", Type: "sorting", Capacity: 8, StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", Capacity: 8, StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", Capacity: 8, StorageAllowed: true},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "blue", TrackID: "sorting-1", TrackIndex: 1},
			{WagonID: "w2", Color: "red", TrackID: "sorting-1", TrackIndex: 2},
			{WagonID: "w3", Color: "red", TrackID: "sorting-1", TrackIndex: 3},
		},
		Locomotives: []normalized.Locomotive{
			{LocoID: "l1", TrackID: "lead-1", TrackIndex: 0},
		},
		Couplings: []normalized.Coupling{
			{CouplingID: "c1", Object1ID: "w1", Object2ID: "w2"},
			{CouplingID: "c2", Object1ID: "w2", Object2ID: "w3"},
		},
	}

	steps, err := BuildLowLevelScenarioStepsFromHeuristicOperations(
		"nsc-cut-source",
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
			},
		},
		scheme.Locomotives[0],
		scheme.Wagons,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(steps) != 5 {
		t.Fatalf("expected source split plus approach/couple/transfer/decouple, got %d steps", len(steps))
	}
	if steps[0].StepType != "decouple" {
		t.Fatalf("expected first step to decouple source boundary, got %s", steps[0].StepType)
	}
	if steps[0].Object1ID == nil || *steps[0].Object1ID != "w1" || steps[0].Object2ID == nil || *steps[0].Object2ID != "w2" {
		t.Fatalf("expected source split on w1-w2, got %+v %+v", steps[0].Object1ID, steps[0].Object2ID)
	}
}

func TestBuildLowLevelScenarioStepsPreservesRealSourceIndicesBetweenOperations(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 20,
		Tracks: []normalized.Track{
			{TrackID: "sorting-a", Type: "sorting", Capacity: 8, StorageAllowed: true},
			{TrackID: "sorting-b", Type: "sorting", Capacity: 8, StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", Capacity: 8, StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", Capacity: 8, StorageAllowed: true},
			{TrackID: "main-1", Type: "main", Capacity: 8, StorageAllowed: false},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "blue", TrackID: "sorting-a", TrackIndex: 0},
			{WagonID: "w3", Color: "blue", TrackID: "sorting-a", TrackIndex: 2},
			{WagonID: "w4", Color: "blue", TrackID: "sorting-a", TrackIndex: 5},
		},
		Locomotives: []normalized.Locomotive{
			{LocoID: "l1", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	steps, err := BuildLowLevelScenarioStepsFromHeuristicOperations(
		"nsc-gap-state",
		scheme,
		[]HeuristicOperation{
			{
				OperationType:      HeuristicOperationTransferTargetsToFormation,
				SourceTrackID:      "sorting-a",
				DestinationTrackID: "sorting-b",
				SourceSide:         "end",
				WagonCount:         1,
				TargetColor:        "blue",
				FormationTrackID:   "lead-2",
				BufferTrackID:      "lead-1",
				MainTrackID:        "main-1",
			},
			{
				OperationType:      HeuristicOperationTransferTargetsToFormation,
				SourceTrackID:      "sorting-a",
				DestinationTrackID: "lead-2",
				SourceSide:         "end",
				WagonCount:         1,
				TargetColor:        "blue",
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

	if len(steps) != 8 {
		t.Fatalf("expected 8 low-level steps, got %d", len(steps))
	}

	secondApproach := steps[4]
	if secondApproach.StepType != "move_loco" {
		t.Fatalf("expected second operation to start with move_loco, got %s", secondApproach.StepType)
	}
	if secondApproach.ToIndex == nil || *secondApproach.ToIndex != 3 {
		t.Fatalf("expected second approach to use real free slot next to remaining wagon at index 2, got %+v", secondApproach.ToIndex)
	}
}
