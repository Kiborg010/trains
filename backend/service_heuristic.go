package main

import (
	"encoding/json"
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
	pipeline, err := runDraftHeuristicPipeline(userID, req)
	if err != nil {
		return DraftHeuristicScenarioResponse{}, err
	}
	return pipeline.Response, nil
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

type draftHeuristicPipelineResult struct {
	Response    DraftHeuristicScenarioResponse
	Scenario    heuristicservice.DraftScenario
	Metrics     heuristicservice.DraftScenarioMetrics
	Feasibility heuristicservice.FixedClassFeasibility
}

func runDraftHeuristicPipeline(userID int, req GenerateDraftHeuristicScenarioRequest) (draftHeuristicPipelineResult, error) {
	result := draftHeuristicPipelineResult{
		Response: DraftHeuristicScenarioResponse{
			OK:       true,
			Feasible: false,
			Reasons:  []string{},
		},
	}

	targetColor := strings.TrimSpace(req.TargetColor)
	if req.SchemeID <= 0 {
		return draftHeuristicPipelineResult{}, fmt.Errorf("scheme_id is required")
	}
	if targetColor == "" {
		return draftHeuristicPipelineResult{}, fmt.Errorf("target_color is required")
	}
	if req.RequiredTargetCount <= 0 {
		return draftHeuristicPipelineResult{}, fmt.Errorf("required_target_count must be positive")
	}

	scheme, err := loadNormalizedSchemeForHeuristic(userID, req.SchemeID)
	if err != nil {
		return draftHeuristicPipelineResult{}, err
	}

	feasibility := heuristicservice.CheckFixedClassFeasibility(
		scheme,
		targetColor,
		req.RequiredTargetCount,
		req.FormationTrackID,
	)
	result.Feasibility = feasibility
	result.Response.Feasible = feasibility.Feasible
	result.Response.Reasons = append([]string{}, feasibility.Reasons...)
	result.Response.Feasibility = toDraftHeuristicFeasibilityDTO(feasibility)

	if !feasibility.Feasible {
		return result, nil
	}

	problem, err := heuristicservice.BuildFixedClassProblem(
		scheme,
		targetColor,
		req.FormationTrackID,
	)
	if err != nil {
		return draftHeuristicPipelineResult{}, fmt.Errorf("failed to build fixed-class problem: %w", err)
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
	result.Response.DraftScenario = &draftScenarioDTO
	result.Response.Metrics = &metricsDTO
	result.Scenario = draftScenario
	result.Metrics = metrics
	return result, nil
}

func GenerateAndSaveDraftHeuristicScenario(userID int, req GenerateAndSaveDraftHeuristicScenarioRequest) (SaveDraftHeuristicScenarioResponse, error) {
	pipeline, err := runDraftHeuristicPipeline(userID, GenerateDraftHeuristicScenarioRequest{
		SchemeID:            req.SchemeID,
		TargetColor:         req.TargetColor,
		RequiredTargetCount: req.RequiredTargetCount,
		FormationTrackID:    req.FormationTrackID,
	})
	if err != nil {
		return SaveDraftHeuristicScenarioResponse{}, err
	}

	response := SaveDraftHeuristicScenarioResponse{
		OK:          true,
		Feasible:    pipeline.Response.Feasible,
		Reasons:     append([]string{}, pipeline.Response.Reasons...),
		Feasibility: pipeline.Response.Feasibility,
	}
	if !pipeline.Response.Feasible {
		return response, nil
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf(
			"Heuristic Draft %d (%s, K=%d)",
			req.SchemeID,
			strings.TrimSpace(req.TargetColor),
			req.RequiredTargetCount,
		)
	}

	metricsJSON, err := json.Marshal(toDraftScenarioMetricsDTO(pipeline.Metrics))
	if err != nil {
		return SaveDraftHeuristicScenarioResponse{}, fmt.Errorf("failed to encode heuristic metrics: %w", err)
	}

	stored := normalized.HeuristicScenario{
		SchemeID:            pipeline.Scenario.SchemeID,
		Name:                name,
		TargetColor:         pipeline.Scenario.TargetColor,
		RequiredTargetCount: pipeline.Scenario.RequiredTargetCount,
		FormationTrackID:    pipeline.Scenario.FormationTrackID,
		BufferTrackID:       pipeline.Scenario.BufferTrackID,
		MainTrackID:         pipeline.Scenario.MainTrackID,
		Feasible:            true,
		Reasons:             append([]string{}, pipeline.Response.Reasons...),
		MetricsJSON:         metricsJSON,
		Steps:               draftScenarioToStoredSteps(pipeline.Scenario),
	}

	id, err := appStore.CreateHeuristicScenario(userID, stored)
	if err != nil {
		return SaveDraftHeuristicScenarioResponse{}, fmt.Errorf("failed to save heuristic draft: %w", err)
	}

	saved, err := appStore.GetHeuristicScenario(id, userID)
	if err != nil {
		return SaveDraftHeuristicScenarioResponse{}, fmt.Errorf("failed to load saved heuristic draft: %w", err)
	}

	dto := toHeuristicScenarioDTO(*saved)
	response.SavedHeuristicScenarioID = id
	response.HeuristicScenario = &dto
	return response, nil
}

func ListStoredHeuristicScenarios(userID int) (ListHeuristicScenariosResponse, error) {
	items, err := appStore.ListHeuristicScenarios(userID)
	if err != nil {
		return ListHeuristicScenariosResponse{}, err
	}
	return ListHeuristicScenariosResponse{
		OK:                 true,
		HeuristicScenarios: toHeuristicScenarioDTOs(items),
	}, nil
}

func GetStoredHeuristicScenario(userID int, id string) (GetHeuristicScenarioResponse, error) {
	item, err := appStore.GetHeuristicScenario(id, userID)
	if err != nil {
		return GetHeuristicScenarioResponse{}, err
	}
	dto := toHeuristicScenarioDTO(*item)
	return GetHeuristicScenarioResponse{
		OK:                true,
		HeuristicScenario: &dto,
	}, nil
}

func draftScenarioToStoredSteps(item heuristicservice.DraftScenario) []normalized.HeuristicScenarioStep {
	result := make([]normalized.HeuristicScenarioStep, 0, len(item.Steps))
	for _, step := range item.Steps {
		result = append(result, normalized.HeuristicScenarioStep{
			StepOrder:          step.StepOrder,
			StepType:           string(step.StepType),
			SourceTrackID:      step.SourceTrackID,
			DestinationTrackID: step.DestinationTrackID,
			SourceSide:         step.SourceSide,
			WagonCount:         step.WagonCount,
			TargetColor:        step.TargetColor,
			FormationTrackID:   step.FormationTrackID,
			BufferTrackID:      step.BufferTrackID,
			MainTrackID:        step.MainTrackID,
		})
	}
	return result
}
