package main

import (
	"fmt"
	"strings"

	"trains/backend/normalized"
	heuristicservice "trains/backend/services/heuristic"
)

// GenerateDraftHeuristicScenario выполняет весь текущий heuristic pipeline
// поверх сохранённой normalized scheme и возвращает только draft scenario
// вместе с feasibility result и метриками.
//
// Функция не создаёт исполнимый сценарий, не сохраняет результат в БД и не
// генерирует low-level scenario steps. Её задача — собрать уже реализованный
// pipeline в один backend service entrypoint.
func GenerateDraftHeuristicScenario(userID int, req GenerateDraftHeuristicScenarioRequest) (DraftHeuristicScenarioResponse, error) {
	result := DraftHeuristicScenarioResponse{
		OK:       true,
		Feasible: false,
		Reasons:  []string{},
	}

	targetColor := strings.TrimSpace(req.TargetColor)
	if req.SchemeID <= 0 {
		return DraftHeuristicScenarioResponse{}, fmt.Errorf("scheme_id is required")
	}
	if targetColor == "" {
		return DraftHeuristicScenarioResponse{}, fmt.Errorf("target_color is required")
	}
	if req.RequiredTargetCount <= 0 {
		return DraftHeuristicScenarioResponse{}, fmt.Errorf("required_target_count must be positive")
	}

	scheme, err := loadNormalizedSchemeForHeuristic(userID, req.SchemeID)
	if err != nil {
		return DraftHeuristicScenarioResponse{}, err
	}

	feasibility := heuristicservice.CheckFixedClassFeasibility(
		scheme,
		targetColor,
		req.RequiredTargetCount,
		req.FormationTrackID,
	)
	result.Feasible = feasibility.Feasible
	result.Reasons = append([]string{}, feasibility.Reasons...)
	result.Feasibility = toDraftHeuristicFeasibilityDTO(feasibility)

	if !feasibility.Feasible {
		return result, nil
	}

	problem, err := heuristicservice.BuildFixedClassProblem(
		scheme,
		targetColor,
		req.FormationTrackID,
	)
	if err != nil {
		return DraftHeuristicScenarioResponse{}, fmt.Errorf("failed to build fixed-class problem: %w", err)
	}

	planningState := heuristicservice.BuildFixedClassPlanningState(problem, req.RequiredTargetCount)
	extractionPlan := heuristicservice.BuildOrderedExtractionPlan(planningState)
	actions := heuristicservice.BuildHighLevelHeuristicPlan(problem, planningState, extractionPlan)
	operations := heuristicservice.BuildHeuristicOperations(problem, actions)
	draftScenario := heuristicservice.BuildDraftScenario(problem, operations)
	draftScenario.RequiredTargetCount = req.RequiredTargetCount
	metrics := heuristicservice.EvaluateDraftScenarioMetrics(problem, draftScenario)

	draftScenarioDTO := toDraftScenarioDTO(draftScenario)
	metricsDTO := toDraftScenarioMetricsDTO(metrics)
	result.DraftScenario = &draftScenarioDTO
	result.Metrics = &metricsDTO
	return result, nil
}

func loadNormalizedSchemeForHeuristic(userID int, schemeID int) (normalizedSchemeDTO, error) {
	scheme, err := appStore.GetNormalizedScheme(schemeID, userID)
	if err != nil {
		return normalizedSchemeDTO{}, fmt.Errorf("failed to load normalized scheme: %w", err)
	}

	tracks, err := appStore.ListTracksByScheme(userID, schemeID)
	if err != nil {
		return normalizedSchemeDTO{}, fmt.Errorf("failed to load tracks: %w", err)
	}
	connections, err := appStore.ListTrackConnectionsByScheme(userID, schemeID)
	if err != nil {
		return normalizedSchemeDTO{}, fmt.Errorf("failed to load track connections: %w", err)
	}
	wagons, err := appStore.ListWagonsByScheme(userID, schemeID)
	if err != nil {
		return normalizedSchemeDTO{}, fmt.Errorf("failed to load wagons: %w", err)
	}
	locomotives, err := appStore.ListLocomotivesByScheme(userID, schemeID)
	if err != nil {
		return normalizedSchemeDTO{}, fmt.Errorf("failed to load locomotives: %w", err)
	}
	couplings, err := appStore.ListNormalizedCouplingsByScheme(userID, schemeID)
	if err != nil {
		return normalizedSchemeDTO{}, fmt.Errorf("failed to load couplings: %w", err)
	}

	return normalizedSchemeDTO{
		SchemeID:         scheme.SchemeID,
		Name:             scheme.Name,
		Tracks:           tracks,
		TrackConnections: connections,
		Wagons:           wagons,
		Locomotives:      locomotives,
		Couplings:        couplings,
	}, nil
}

type normalizedSchemeDTO = normalized.Scheme
