package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

func BuildScenarioStepsFromDraftScenario(scenarioID string, draft normalized.HeuristicScenario) ([]normalized.ScenarioStep, error) {
	steps := make([]normalized.ScenarioStep, 0, len(draft.Steps))
	for _, draftStep := range draft.Steps {
		payload, err := json.Marshal(map[string]any{
			"heuristic_step_type": draftStep.StepType,
			"source_side":         draftStep.SourceSide,
			"wagon_count":         draftStep.WagonCount,
			"target_color":        draftStep.TargetColor,
			"formation_track_id":  draftStep.FormationTrackID,
			"buffer_track_id":     draftStep.BufferTrackID,
			"main_track_id":       draftStep.MainTrackID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to encode heuristic step payload: %w", err)
		}

		fromTrackID := draftStep.SourceTrackID
		toTrackID := draftStep.DestinationTrackID
		steps = append(steps, normalized.ScenarioStep{
			StepID:      fmt.Sprintf("nst-%d-%d", time.Now().UnixNano(), draftStep.StepOrder),
			ScenarioID:  scenarioID,
			StepOrder:   draftStep.StepOrder,
			StepType:    "move_group",
			FromTrackID: &fromTrackID,
			ToTrackID:   &toTrackID,
			PayloadJSON: payload,
		})
	}
	return steps, nil
}

func SaveHeuristicDraftAsScenario(userID int, req SaveHeuristicAsScenarioRequest) (SaveHeuristicAsScenarioResponse, error) {
	heuristicScenarioID := strings.TrimSpace(req.HeuristicScenarioID)
	if heuristicScenarioID == "" {
		return SaveHeuristicAsScenarioResponse{}, fmt.Errorf("heuristic_scenario_id is required")
	}

	draft, err := appStore.GetHeuristicScenario(heuristicScenarioID, userID)
	if err != nil {
		return SaveHeuristicAsScenarioResponse{}, fmt.Errorf("failed to load heuristic draft: %w", err)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = draft.Name
	}
	if name == "" {
		name = fmt.Sprintf("Scenario from %s", heuristicScenarioID)
	}

	scenario := normalized.Scenario{
		SchemeID: draft.SchemeID,
		Name:     name,
	}

	scenarioID, err := appStore.CreateNormalizedScenario(userID, scenario)
	if err != nil {
		return SaveHeuristicAsScenarioResponse{}, fmt.Errorf("failed to create standard scenario: %w", err)
	}

	steps, err := BuildScenarioStepsFromDraftScenario(scenarioID, *draft)
	if err != nil {
		return SaveHeuristicAsScenarioResponse{}, err
	}

	if err := appStore.CreateScenarioSteps(userID, scenarioID, steps); err != nil {
		return SaveHeuristicAsScenarioResponse{}, fmt.Errorf("failed to save standard scenario steps: %w", err)
	}

	saved, err := appStore.GetNormalizedScenario(scenarioID, userID)
	if err != nil {
		return SaveHeuristicAsScenarioResponse{}, fmt.Errorf("failed to load created standard scenario: %w", err)
	}

	return SaveHeuristicAsScenarioResponse{
		OK:                true,
		CreatedScenarioID: scenarioID,
		Scenario:          ptrScenarioDTO(toScenarioDTO(*saved)),
		ScenarioSteps:     toScenarioStepDTOs(saved.Steps),
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

func ptrScenarioDTO(item ScenarioDTO) *ScenarioDTO {
	return &item
}
