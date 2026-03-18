package heuristic

import (
	"testing"

	"trains/backend/normalized"
)

// Этот файл содержит тесты для STEP 7 — оценки стоимости и допустимости
// draft heuristic scenario.
//
// Здесь проверяется только cost layer:
//   - поиск маршрута по track_connections
//   - подсчёт switch crossings
//   - расчёт общей стоимости
//   - флаг допустимости шага и сценария

func TestEvaluateDraftScenarioStepCostAndMetrics(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8, StartX: 200, StartY: 0, EndX: 300, EndY: 0},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6, StartX: -100, StartY: 100, EndX: 0, EndY: 100},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8, StartX: 0, StartY: 0, EndX: 100, EndY: 0},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8, StartX: 0, StartY: 50, EndX: 100, EndY: 50},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 6, StartX: 100, StartY: 100, EndX: 200, EndY: 100},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8, StartX: 100, StartY: 0, EndX: 200, EndY: 0},
		},
		TrackConnections: []normalized.TrackConnection{
			{ConnectionID: "c1", Track1ID: "sorting-1", Track2ID: "lead-2", ConnectionType: "switch"},
			{ConnectionID: "c2", Track1ID: "sorting-2", Track2ID: "lead-2", ConnectionType: "serial"},
			{ConnectionID: "c3", Track1ID: "lead-2", Track2ID: "main-1", ConnectionType: "switch"},
			{ConnectionID: "c4", Track1ID: "sorting-1", Track2ID: "lead-1", ConnectionType: "serial"},
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

	scenario := DraftScenario{
		SchemeID:            problem.SchemeID,
		TargetColor:         problem.TargetColor,
		RequiredTargetCount: 3,
		FormationTrackID:    problem.FormationTrack.TrackID,
		BufferTrackID:       problem.BufferTrack.TrackID,
		MainTrackID:         problem.MainTrack.TrackID,
		Steps: []DraftScenarioStep{
			{
				StepOrder:          0,
				StepType:           DraftScenarioStepTransferTargetsToFormation,
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
				StepOrder:          1,
				StepType:           DraftScenarioStepTransferFormationToMain,
				SourceTrackID:      "lead-2",
				DestinationTrackID: "main-1",
				SourceSide:         "",
				WagonCount:         3,
				TargetColor:        "red",
				FormationTrackID:   "lead-2",
				BufferTrackID:      "lead-1",
				MainTrackID:        "main-1",
			},
		},
	}

	firstCost := EvaluateDraftScenarioStepCost(problem, scenario.Steps[0])
	if !firstCost.Feasible {
		t.Fatalf("expected first draft step to be feasible, got reasons: %v", firstCost.Reasons)
	}
	if firstCost.SwitchCrossCount != 1 {
		t.Fatalf("expected one switch crossing, got %d", firstCost.SwitchCrossCount)
	}
	if firstCost.CoupleCount != 1 || firstCost.DecoupleCount != 1 {
		t.Fatalf("expected one couple and one decouple, got %+v", firstCost)
	}
	if len(firstCost.Reasons) == 0 {
		t.Fatal("expected a reason describing whole-group switch traversal")
	}

	metrics := EvaluateDraftScenarioMetrics(problem, scenario)
	if !metrics.Success {
		t.Fatalf("expected scenario metrics success, got %+v", metrics)
	}
	if metrics.TotalStepCount != 2 {
		t.Fatalf("expected total_step_count=2, got %d", metrics.TotalStepCount)
	}
	if metrics.TotalCoupleCount != 2 || metrics.TotalDecoupleCount != 2 {
		t.Fatalf("unexpected couple/decouple totals: %+v", metrics)
	}
	if metrics.TotalSwitchCrossCount != 2 {
		t.Fatalf("expected total_switch_cross_count=2, got %d", metrics.TotalSwitchCrossCount)
	}
	if metrics.TotalCost <= 0 {
		t.Fatalf("expected positive total cost, got %f", metrics.TotalCost)
	}
}

func TestEvaluateDraftScenarioStepCostInfeasibleWhenRouteMissing(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 2,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8, StartX: 200, StartY: 0, EndX: 300, EndY: 0},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6, StartX: -100, StartY: 100, EndX: 0, EndY: 100},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8, StartX: 0, StartY: 0, EndX: 100, EndY: 0},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8, StartX: 0, StartY: 50, EndX: 100, EndY: 50},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 6, StartX: 100, StartY: 100, EndX: 200, EndY: 100},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8, StartX: 100, StartY: 0, EndX: 200, EndY: 0},
		},
		TrackConnections: []normalized.TrackConnection{
			{ConnectionID: "c1", Track1ID: "sorting-1", Track2ID: "lead-1", ConnectionType: "serial"},
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

	step := DraftScenarioStep{
		StepOrder:          0,
		StepType:           DraftScenarioStepTransferTargetsToFormation,
		SourceTrackID:      "sorting-1",
		DestinationTrackID: "lead-2",
		SourceSide:         "start",
		WagonCount:         1,
		TargetColor:        "red",
		FormationTrackID:   "lead-2",
		BufferTrackID:      "lead-1",
		MainTrackID:        "main-1",
	}

	cost := EvaluateDraftScenarioStepCost(problem, step)
	if cost.Feasible {
		t.Fatalf("expected infeasible cost evaluation, got %+v", cost)
	}
	if len(cost.Reasons) == 0 {
		t.Fatal("expected infeasibility reasons")
	}
}
