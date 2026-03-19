package heuristic

import (
	"fmt"
	"strings"
	"testing"

	"trains/backend/normalized"
)

// Этот файл содержит целевые (локальные) тесты для текущего слоя эвристики
// для фиксированного минимального класса схем.
//
// Здесь покрываются:
//   - ШАГ 1: построение задачи и строгая проверка минимального класса
//   - ШАГ 2: проверка реализуемости и выбор путей formation/buffer
//   - ШАГ 3: генерация кандидатов извлечения и построение порядка извлечения
//
// Тесты намеренно сделаны простыми и явными. Их задача — не симулировать
// полный процесс маневров на станции, а зафиксировать контракты эвристики:
//   - какие формы схем допускаются
//   - как разделяются target/non-target вагоны
//   - как выбираются пути formation и buffer
//   - как ранжируются кандидаты извлечения
//
// Пока не покрыто:
//   - генерация scenario_steps
//   - движение локомотива
//   - сцепка/расцепка
//   - интеграция с execution/runtime слоями

// TestBuildFixedClassProblem проверяет, что корректная схема минимального класса
// правильно преобразуется в внутреннее представление задачи с ролями путей.
//
// Проверяется:
//   - корректная классификация путей (main / bypass / lead)
//   - явное назначение formation и buffer путей
//   - корректное разделение вагонов на target и non-target
//   - корректная индексация вагонов по путям
func TestBuildFixedClassProblem(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "sorting-2", TrackIndex: 1},
			{WagonID: "w3", Color: "red", TrackID: "lead-2", TrackIndex: 2},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if problem.MainTrack.TrackID != "main-1" {
		t.Fatalf("expected main track main-1, got %s", problem.MainTrack.TrackID)
	}
	if problem.BypassTrack.TrackID != "bypass-1" {
		t.Fatalf("expected bypass track bypass-1, got %s", problem.BypassTrack.TrackID)
	}
	if problem.FormationTrack.TrackID != "lead-2" {
		t.Fatalf("expected formation track lead-2, got %s", problem.FormationTrack.TrackID)
	}
	if problem.BufferTrack.TrackID != "lead-1" {
		t.Fatalf("expected buffer track lead-1, got %s", problem.BufferTrack.TrackID)
	}
	if len(problem.TargetWagons) != 2 {
		t.Fatalf("expected 2 target wagons, got %d", len(problem.TargetWagons))
	}
	if len(problem.NonTargetWagons) != 1 {
		t.Fatalf("expected 1 non-target wagon, got %d", len(problem.NonTargetWagons))
	}
	if len(problem.WagonsByTrack["sorting-1"]) != 1 {
		t.Fatalf("expected wagon index on sorting-1")
	}
}

func TestBuildFixedClassProblemAllowsMultipleNonTargetColors(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "sorting-2", TrackIndex: 1},
			{WagonID: "w3", Color: "green", TrackID: "lead-2", TrackIndex: 2},
			{WagonID: "w4", Color: "yellow", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(problem.TargetWagons) != 1 {
		t.Fatalf("expected 1 target wagon, got %d", len(problem.TargetWagons))
	}
	if len(problem.NonTargetWagons) != 3 {
		t.Fatalf("expected 3 non-target wagons, got %d", len(problem.NonTargetWagons))
	}
}

// TestBuildFixedClassProblemRejectsWrongTrackCounts проверяет, что эвристика
// не принимает схемы, которые не соответствуют минимальному классу.
//
// В данном тесте отсутствуют необходимые типы/количество путей,
// поэтому построение задачи должно завершиться ошибкой.
func TestBuildFixedClassProblemRejectsWrongTrackCounts(t *testing.T) {
	scheme := normalized.Scheme{
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "lead-1", TrackIndex: 1},
		},
	}

	if _, err := BuildFixedClassProblem(scheme, "red", "lead-1"); err == nil {
		t.Fatal("expected error for invalid fixed-class scheme")
	}
}

// TestCheckFixedClassFeasibilityFeasibleAutoSelection проверяет успешный сценарий,
// когда путь formation не задан явно.
//
// Ожидается:
//   - задача выполнима для K=3
//   - lead-2 выбран как formation (вмещает K и лучше по текущим правилам)
//   - lead-1 становится buffer
func TestCheckFixedClassFeasibilityFeasibleAutoSelection(t *testing.T) {
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
			{WagonID: "w2", Color: "red", TrackID: "sorting-2", TrackIndex: 1},
			{WagonID: "w3", Color: "red", TrackID: "lead-2", TrackIndex: 2},
			{WagonID: "w4", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	result := CheckFixedClassFeasibility(scheme, "red", 3, "")
	if !result.Feasible {
		t.Fatalf("expected feasible result, got reasons: %v", result.Reasons)
	}
	if result.ChosenFormationTrackID != "lead-2" {
		t.Fatalf("expected lead-2 as formation track, got %s", result.ChosenFormationTrackID)
	}
	if result.ChosenBufferTrackID != "lead-1" {
		t.Fatalf("expected lead-1 as buffer track, got %s", result.ChosenBufferTrackID)
	}
	if result.TargetCount != 3 {
		t.Fatalf("expected target count 3, got %d", result.TargetCount)
	}
	if result.RequiredTargetCount != 3 {
		t.Fatalf("expected required target count 3, got %d", result.RequiredTargetCount)
	}
}

func TestCheckFixedClassFeasibilityAllowsMultipleNonTargetColors(t *testing.T) {
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
			{WagonID: "w2", Color: "red", TrackID: "sorting-2", TrackIndex: 1},
			{WagonID: "w3", Color: "blue", TrackID: "lead-2", TrackIndex: 2},
			{WagonID: "w4", Color: "green", TrackID: "lead-1", TrackIndex: 0},
			{WagonID: "w5", Color: "yellow", TrackID: "sorting-1", TrackIndex: 2},
		},
	}

	result := CheckFixedClassFeasibility(scheme, "red", 2, "")
	if !result.Feasible {
		t.Fatalf("expected feasible result with multiple non-target colors, got reasons: %v", result.Reasons)
	}
}

// TestCheckFixedClassFeasibilityInfeasible проверяет отрицательный сценарий,
// когда задача не может быть выполнена.
//
// Здесь оба lead пути слишком короткие для K=3 и недостаточно target вагонов.
func TestCheckFixedClassFeasibilityInfeasible(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 2},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 2},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "sorting-2", TrackIndex: 1},
		},
	}

	result := CheckFixedClassFeasibility(scheme, "red", 3, "")
	if result.Feasible {
		t.Fatal("expected infeasible result")
	}
	if len(result.Reasons) == 0 {
		t.Fatal("expected infeasible reasons")
	}
}

// TestEnumerateTargetExtractionCandidatesAndPlan проверяет заготовку ШАГА 3:
// генерацию кандидатов, их ранжирование и построение порядка извлечения.
//
// Конфигурация выбрана так, что:
//   - с sorting-1 со стороны "end" доступны 2 target без блокировок
//   - sorting-2 со стороны "start" даёт последний target
//   - итоговый план состоит из этих двух действий
func TestEnumerateTargetExtractionCandidatesAndPlan(t *testing.T) {
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
	candidates := EnumerateTargetExtractionCandidates(state)
	if len(candidates) != 4 {
		t.Fatalf("expected 4 candidates, got %d", len(candidates))
	}

	best, ok := ChooseNextTargetExtractionCandidate(candidates)
	if !ok {
		t.Fatal("expected feasible candidate")
	}
	if best.SourceSortingTrackID != "sorting-1" || best.SourceSide != "end" {
		t.Fatalf("unexpected best candidate: %#v", best)
	}
	if best.ChosenBufferTrackID != "lead-1" {
		t.Fatalf("expected single remaining lead to stay as buffer, got %q", best.ChosenBufferTrackID)
	}

	plan := BuildOrderedExtractionPlan(state)
	if len(plan) != 2 {
		t.Fatalf("expected 2 extraction decisions, got %d", len(plan))
	}
	if plan[0].SourceSortingTrackID != "sorting-1" || plan[0].SourceSide != "end" {
		t.Fatalf("unexpected first plan step: %#v", plan[0])
	}
	if plan[1].SourceSortingTrackID != "sorting-2" || plan[1].SourceSide != "start" {
		t.Fatalf("unexpected second plan step: %#v", plan[1])
	}
}

// TestEnumerateTargetExtractionCandidatesMarksBufferInfeasible проверяет,
// что кандидат считается непригодным, если буферный путь не вмещает блокирующие вагоны.
//
// В этом случае:
//   - кандидаты генерируются
//   - но ни один из них не считается допустимым
func TestEnumerateTargetExtractionCandidatesMarksBufferInfeasible(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 1},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "blue", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "sorting-1", TrackIndex: 1},
			{WagonID: "w3", Color: "red", TrackID: "sorting-1", TrackIndex: 2},
			{WagonID: "w4", Color: "blue", TrackID: "sorting-1", TrackIndex: 3},
			{WagonID: "w5", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected problem build error: %v", err)
	}
	state := BuildFixedClassPlanningState(problem, 1)
	candidates := EnumerateTargetExtractionCandidates(state)
	best, ok := ChooseNextTargetExtractionCandidate(candidates)
	if ok {
		t.Fatalf("expected no feasible candidate, got %#v", best)
	}
}

func TestEnumerateTargetExtractionCandidatesChoosesBestDynamicBufferTrack(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 4},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-3", Type: "lead", StorageAllowed: true, Capacity: 10},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "f1", Color: "blue", TrackID: "lead-2", TrackIndex: 0},
			{WagonID: "b1", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
			{WagonID: "b2", Color: "blue", TrackID: "lead-1", TrackIndex: 1},
			{WagonID: "t1", Color: "blue", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "r1", Color: "red", TrackID: "sorting-1", TrackIndex: 1},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected problem build error: %v", err)
	}
	state := BuildFixedClassPlanningState(problem, 1)
	candidates := EnumerateTargetExtractionCandidates(state)
	best, ok := ChooseNextTargetExtractionCandidate(candidates)
	if !ok {
		t.Fatal("expected feasible candidate")
	}
	if best.ChosenBufferTrackID != "lead-3" {
		t.Fatalf("expected lead-3 as best dynamic buffer, got %q", best.ChosenBufferTrackID)
	}
}

func TestEnumerateTargetExtractionCandidatesFeasibleWhenAnyBufferFits(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 2},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-3", Type: "lead", StorageAllowed: true, Capacity: 5},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "f1", Color: "blue", TrackID: "lead-2", TrackIndex: 0},
			{WagonID: "x1", Color: "blue", TrackID: "lead-3", TrackIndex: 0},
			{WagonID: "b1", Color: "blue", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "b2", Color: "blue", TrackID: "sorting-1", TrackIndex: 1},
			{WagonID: "r1", Color: "red", TrackID: "sorting-1", TrackIndex: 2},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected problem build error: %v", err)
	}
	state := BuildFixedClassPlanningState(problem, 1)
	best, ok := ChooseNextTargetExtractionCandidate(EnumerateTargetExtractionCandidates(state))
	if !ok {
		t.Fatal("expected feasible candidate")
	}
	if best.ChosenBufferTrackID != "lead-3" {
		t.Fatalf("expected lead-3 to make candidate feasible, got %q", best.ChosenBufferTrackID)
	}
}

func TestEnumerateTargetExtractionCandidatesInfeasibleWhenNoDynamicBufferFits(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 2},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-3", Type: "lead", StorageAllowed: true, Capacity: 2},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "f1", Color: "blue", TrackID: "lead-2", TrackIndex: 0},
			{WagonID: "b1", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
			{WagonID: "b1b", Color: "blue", TrackID: "lead-1", TrackIndex: 1},
			{WagonID: "c1", Color: "blue", TrackID: "lead-3", TrackIndex: 0},
			{WagonID: "c1b", Color: "blue", TrackID: "lead-3", TrackIndex: 1},
			{WagonID: "b2", Color: "blue", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "r1", Color: "red", TrackID: "sorting-1", TrackIndex: 1},
			{WagonID: "b3", Color: "blue", TrackID: "sorting-1", TrackIndex: 2},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected problem build error: %v", err)
	}
	state := BuildFixedClassPlanningState(problem, 1)
	best, ok := ChooseNextTargetExtractionCandidate(EnumerateTargetExtractionCandidates(state))
	if ok {
		t.Fatalf("expected no feasible candidate, got %#v", best)
	}
}

func TestBuildOrderedExtractionPlanUsesDynamicBufferSelection(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 4},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-3", Type: "lead", StorageAllowed: true, Capacity: 10},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "f1", Color: "blue", TrackID: "lead-2", TrackIndex: 0},
			{WagonID: "b1", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
			{WagonID: "b2", Color: "blue", TrackID: "lead-1", TrackIndex: 1},
			{WagonID: "t1", Color: "blue", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "r1", Color: "red", TrackID: "sorting-1", TrackIndex: 1},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected problem build error: %v", err)
	}
	plan := BuildOrderedExtractionPlan(BuildFixedClassPlanningState(problem, 1))
	if len(plan) != 1 {
		t.Fatalf("expected one extraction decision, got %d", len(plan))
	}
	if plan[0].ChosenBufferTrackID != "lead-3" {
		t.Fatalf("expected ordered plan to keep chosen dynamic buffer lead-3, got %q", plan[0].ChosenBufferTrackID)
	}
}

func TestBuildFixedClassProblemSupportsThreeSortingTracks(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 2,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true},
			{TrackID: "sorting-3", Type: "sorting", StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "sorting-2", TrackIndex: 0},
			{WagonID: "w3", Color: "red", TrackID: "sorting-3", TrackIndex: 0},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(problem.SortingTracks) != 3 {
		t.Fatalf("expected 3 sorting tracks, got %d", len(problem.SortingTracks))
	}
}

func TestCheckFixedClassFeasibilitySupportsThreeLeadTracks(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 3,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 4},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 7},
			{TrackID: "lead-3", Type: "lead", StorageAllowed: true, Capacity: 9},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "red", TrackID: "sorting-2", TrackIndex: 1},
			{WagonID: "w3", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	result := CheckFixedClassFeasibility(scheme, "red", 2, "")
	if !result.Feasible {
		t.Fatalf("expected feasible result, got reasons: %v", result.Reasons)
	}
	if result.ChosenFormationTrackID != "lead-3" {
		t.Fatalf("expected highest-capacity free lead lead-3 as formation, got %s", result.ChosenFormationTrackID)
	}
	if result.ChosenBufferTrackID != "lead-2" {
		t.Fatalf("expected remaining best lead lead-2 as buffer, got %s", result.ChosenBufferTrackID)
	}
}

func TestEnumerateTargetExtractionCandidatesSupportsManySortingTracks(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 4,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-3", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-4", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-5", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 6},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-3", Type: "lead", StorageAllowed: true, Capacity: 7},
			{TrackID: "lead-4", Type: "lead", StorageAllowed: true, Capacity: 9},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "sorting-1", TrackIndex: 1},
			{WagonID: "w3", Color: "red", TrackID: "sorting-2", TrackIndex: 0},
			{WagonID: "w4", Color: "blue", TrackID: "sorting-3", TrackIndex: 0},
			{WagonID: "w5", Color: "red", TrackID: "sorting-4", TrackIndex: 0},
			{WagonID: "w6", Color: "blue", TrackID: "sorting-5", TrackIndex: 0},
			{WagonID: "w7", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-4")
	if err != nil {
		t.Fatalf("unexpected problem build error: %v", err)
	}
	state := BuildFixedClassPlanningState(problem, 3)
	candidates := EnumerateTargetExtractionCandidates(state)
	if len(candidates) != 10 {
		t.Fatalf("expected 10 candidates for 5 sorting tracks, got %d", len(candidates))
	}
}

func TestFixedClassTrackCountValidationRange(t *testing.T) {
	testCases := []struct {
		name          string
		sortingCount  int
		leadCount     int
		expectedError string
	}{
		{name: "sorting below minimum", sortingCount: 1, leadCount: 2, expectedError: "expected at least 2 sorting tracks"},
		{name: "lead below minimum", sortingCount: 2, leadCount: 1, expectedError: "expected at least 2 lead tracks"},
		{name: "sorting above maximum", sortingCount: 11, leadCount: 2, expectedError: "expected at most 10 sorting tracks"},
		{name: "lead above maximum", sortingCount: 2, leadCount: 11, expectedError: "expected at most 10 lead tracks"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tracks := []normalized.Track{
				{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
				{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			}
			for i := 0; i < tc.sortingCount; i++ {
				tracks = append(tracks, normalized.Track{
					TrackID:        fmt.Sprintf("sorting-%d", i+1),
					Type:           "sorting",
					StorageAllowed: true,
					Capacity:       8,
				})
			}
			for i := 0; i < tc.leadCount; i++ {
				tracks = append(tracks, normalized.Track{
					TrackID:        fmt.Sprintf("lead-%d", i+1),
					Type:           "lead",
					StorageAllowed: true,
					Capacity:       8,
				})
			}
			scheme := normalized.Scheme{
				Tracks: tracks,
				Wagons: []normalized.Wagon{
					{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
					{WagonID: "w2", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
				},
			}

			_, err := BuildFixedClassProblem(scheme, "red", "")
			if err == nil || !strings.Contains(err.Error(), tc.expectedError) {
				t.Fatalf("expected error containing %q, got %v", tc.expectedError, err)
			}

			result := CheckFixedClassFeasibility(scheme, "red", 1, "")
			found := false
			for _, reason := range result.Reasons {
				if strings.Contains(reason, tc.expectedError) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected feasibility reasons to contain %q, got %v", tc.expectedError, result.Reasons)
			}
		})
	}
}
