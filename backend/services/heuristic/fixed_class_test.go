package heuristic

import (
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
