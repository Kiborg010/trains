package main

import (
	"encoding/json"

	"trains/backend/normalized"
	heuristicservice "trains/backend/services/heuristic"
)

type SchemeDTO struct {
	SchemeID int    `json:"scheme_id"`
	Name     string `json:"name"`
}

type UpsertNormalizedSchemeRequest struct {
	Name             string               `json:"name"`
	Tracks           []TrackDTO           `json:"tracks"`
	TrackConnections []TrackConnectionDTO `json:"track_connections"`
	Wagons           []WagonDTO           `json:"wagons"`
	Locomotives      []LocomotiveDTO      `json:"locomotives"`
	Couplings        []CouplingDTO        `json:"couplings"`
}

type TrackDTO struct {
	TrackID        string  `json:"track_id"`
	SchemeID       int     `json:"scheme_id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	StartX         float64 `json:"start_x"`
	StartY         float64 `json:"start_y"`
	EndX           float64 `json:"end_x"`
	EndY           float64 `json:"end_y"`
	Capacity       int     `json:"capacity"`
	StorageAllowed bool    `json:"storage_allowed"`
}

type TrackConnectionDTO struct {
	ConnectionID   string `json:"connection_id"`
	SchemeID       int    `json:"scheme_id"`
	Track1ID       string `json:"track1_id"`
	Track2ID       string `json:"track2_id"`
	Track1Side     string `json:"track1_side"`
	Track2Side     string `json:"track2_side"`
	ConnectionType string `json:"connection_type"`
}

type WagonDTO struct {
	WagonID    string `json:"wagon_id"`
	SchemeID   int    `json:"scheme_id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	TrackID    string `json:"track_id"`
	TrackIndex int    `json:"track_index"`
}

type LocomotiveDTO struct {
	LocoID     string `json:"loco_id"`
	SchemeID   int    `json:"scheme_id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	TrackID    string `json:"track_id"`
	TrackIndex int    `json:"track_index"`
}

type CouplingDTO struct {
	CouplingID string `json:"coupling_id"`
	SchemeID   int    `json:"scheme_id"`
	Object1ID  string `json:"object1_id"`
	Object2ID  string `json:"object2_id"`
}

type ScenarioDTO struct {
	ScenarioID                string  `json:"scenario_id"`
	SchemeID                  int     `json:"scheme_id"`
	Name                      string  `json:"name"`
	SourceHeuristicScenarioID *string `json:"source_heuristic_scenario_id,omitempty"`
}

type ScenarioMetricsDTO struct {
	ScenarioID           string `json:"scenario_id"`
	TotalLocoDistance    int    `json:"total_loco_distance"`
	TotalCouples         int    `json:"total_couples"`
	TotalDecouples       int    `json:"total_decouples"`
	TotalSwitchCrossings int    `json:"total_switch_crossings"`
}

type UpsertNormalizedScenarioRequest struct {
	SchemeID      int               `json:"scheme_id"`
	Name          string            `json:"name"`
	ScenarioSteps []ScenarioStepDTO `json:"scenario_steps"`
}

type ScenarioStepDTO struct {
	StepID      string          `json:"step_id"`
	ScenarioID  string          `json:"scenario_id"`
	StepOrder   int             `json:"step_order"`
	StepType    string          `json:"step_type"`
	FromTrackID *string         `json:"from_track_id,omitempty"`
	FromIndex   *int            `json:"from_index,omitempty"`
	ToTrackID   *string         `json:"to_track_id,omitempty"`
	ToIndex     *int            `json:"to_index,omitempty"`
	Object1ID   *string         `json:"object1_id,omitempty"`
	Object2ID   *string         `json:"object2_id,omitempty"`
	PayloadJSON json.RawMessage `json:"payload_json,omitempty"`
}

type ListNormalizedSchemesResponse struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	Schemes []SchemeDTO `json:"schemes,omitempty"`
}

type GetNormalizedSchemeResponse struct {
	OK      bool       `json:"ok"`
	Message string     `json:"message,omitempty"`
	Scheme  *SchemeDTO `json:"scheme,omitempty"`
}

type SchemeDetailsResponse struct {
	OK               bool                 `json:"ok"`
	Message          string               `json:"message,omitempty"`
	Scheme           *SchemeDTO           `json:"scheme,omitempty"`
	Tracks           []TrackDTO           `json:"tracks,omitempty"`
	TrackConnections []TrackConnectionDTO `json:"track_connections,omitempty"`
	Wagons           []WagonDTO           `json:"wagons,omitempty"`
	Locomotives      []LocomotiveDTO      `json:"locomotives,omitempty"`
	Couplings        []CouplingDTO        `json:"couplings,omitempty"`
}

type ListNormalizedScenariosResponse struct {
	OK        bool          `json:"ok"`
	Message   string        `json:"message,omitempty"`
	Scenarios []ScenarioDTO `json:"scenarios,omitempty"`
}

type GetNormalizedScenarioResponse struct {
	OK       bool         `json:"ok"`
	Message  string       `json:"message,omitempty"`
	Scenario *ScenarioDTO `json:"scenario,omitempty"`
}

type ListScenarioStepsResponse struct {
	OK            bool              `json:"ok"`
	Message       string            `json:"message,omitempty"`
	ScenarioSteps []ScenarioStepDTO `json:"scenario_steps,omitempty"`
}

type ScenarioDetailsResponse struct {
	OK            bool              `json:"ok"`
	Message       string            `json:"message,omitempty"`
	Scenario      *ScenarioDTO      `json:"scenario,omitempty"`
	ScenarioSteps []ScenarioStepDTO `json:"scenario_steps,omitempty"`
}

type ScenarioMetricsResponse struct {
	OK      bool                `json:"ok"`
	Message string              `json:"message,omitempty"`
	Metrics *ScenarioMetricsDTO `json:"metrics,omitempty"`
}

func toSchemeDTO(item normalized.Scheme) SchemeDTO {
	return SchemeDTO{
		SchemeID: item.SchemeID,
		Name:     item.Name,
	}
}

func toTrackDTO(item normalized.Track) TrackDTO {
	return TrackDTO{
		TrackID:        item.TrackID,
		SchemeID:       item.SchemeID,
		Name:           item.Name,
		Type:           item.Type,
		StartX:         item.StartX,
		StartY:         item.StartY,
		EndX:           item.EndX,
		EndY:           item.EndY,
		Capacity:       item.Capacity,
		StorageAllowed: item.StorageAllowed,
	}
}

func toTrackConnectionDTO(item normalized.TrackConnection) TrackConnectionDTO {
	return TrackConnectionDTO{
		ConnectionID:   item.ConnectionID,
		SchemeID:       item.SchemeID,
		Track1ID:       item.Track1ID,
		Track2ID:       item.Track2ID,
		Track1Side:     item.Track1Side,
		Track2Side:     item.Track2Side,
		ConnectionType: item.ConnectionType,
	}
}

func toWagonDTO(item normalized.Wagon) WagonDTO {
	return WagonDTO{
		WagonID:    item.WagonID,
		SchemeID:   item.SchemeID,
		Name:       item.Name,
		Color:      item.Color,
		TrackID:    item.TrackID,
		TrackIndex: item.TrackIndex,
	}
}

func toLocomotiveDTO(item normalized.Locomotive) LocomotiveDTO {
	return LocomotiveDTO{
		LocoID:     item.LocoID,
		SchemeID:   item.SchemeID,
		Name:       item.Name,
		Color:      item.Color,
		TrackID:    item.TrackID,
		TrackIndex: item.TrackIndex,
	}
}

func toCouplingDTO(item normalized.Coupling) CouplingDTO {
	return CouplingDTO{
		CouplingID: item.CouplingID,
		SchemeID:   item.SchemeID,
		Object1ID:  item.Object1ID,
		Object2ID:  item.Object2ID,
	}
}

func toScenarioDTO(item normalized.Scenario) ScenarioDTO {
	return ScenarioDTO{
		ScenarioID:                item.ScenarioID,
		SchemeID:                  item.SchemeID,
		Name:                      item.Name,
		SourceHeuristicScenarioID: item.SourceHeuristicScenarioID,
	}
}

func toScenarioStepDTO(item normalized.ScenarioStep) ScenarioStepDTO {
	payload := make([]byte, len(item.PayloadJSON))
	copy(payload, item.PayloadJSON)
	return ScenarioStepDTO{
		StepID:      item.StepID,
		ScenarioID:  item.ScenarioID,
		StepOrder:   item.StepOrder,
		StepType:    item.StepType,
		FromTrackID: item.FromTrackID,
		FromIndex:   item.FromIndex,
		ToTrackID:   item.ToTrackID,
		ToIndex:     item.ToIndex,
		Object1ID:   item.Object1ID,
		Object2ID:   item.Object2ID,
		PayloadJSON: json.RawMessage(payload),
	}
}

func toTrackDTOs(items []normalized.Track) []TrackDTO {
	result := make([]TrackDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toTrackDTO(item))
	}
	return result
}

func toTrackConnectionDTOs(items []normalized.TrackConnection) []TrackConnectionDTO {
	result := make([]TrackConnectionDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toTrackConnectionDTO(item))
	}
	return result
}

func toWagonDTOs(items []normalized.Wagon) []WagonDTO {
	result := make([]WagonDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toWagonDTO(item))
	}
	return result
}

func toLocomotiveDTOs(items []normalized.Locomotive) []LocomotiveDTO {
	result := make([]LocomotiveDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toLocomotiveDTO(item))
	}
	return result
}

func toCouplingDTOs(items []normalized.Coupling) []CouplingDTO {
	result := make([]CouplingDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toCouplingDTO(item))
	}
	return result
}

func toSchemeDTOs(items []normalized.Scheme) []SchemeDTO {
	result := make([]SchemeDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toSchemeDTO(item))
	}
	return result
}

func toScenarioDTOs(items []normalized.Scenario) []ScenarioDTO {
	result := make([]ScenarioDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toScenarioDTO(item))
	}
	return result
}

func toScenarioStepDTOs(items []normalized.ScenarioStep) []ScenarioStepDTO {
	result := make([]ScenarioStepDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toScenarioStepDTO(item))
	}
	return result
}

func toScenarioMetricsDTO(item normalized.ScenarioMetrics) ScenarioMetricsDTO {
	return ScenarioMetricsDTO{
		ScenarioID:           item.ScenarioID,
		TotalLocoDistance:    item.TotalLocoDistance,
		TotalCouples:         item.TotalCouples,
		TotalDecouples:       item.TotalDecouples,
		TotalSwitchCrossings: item.TotalSwitchCrossings,
	}
}

func dtoToTrack(item TrackDTO) normalized.Track {
	return normalized.Track{
		TrackID:        item.TrackID,
		SchemeID:       item.SchemeID,
		Name:           item.Name,
		Type:           item.Type,
		StartX:         item.StartX,
		StartY:         item.StartY,
		EndX:           item.EndX,
		EndY:           item.EndY,
		Capacity:       item.Capacity,
		StorageAllowed: item.StorageAllowed,
	}
}

func dtoToTrackConnection(item TrackConnectionDTO) normalized.TrackConnection {
	return normalized.TrackConnection{
		ConnectionID:   item.ConnectionID,
		SchemeID:       item.SchemeID,
		Track1ID:       item.Track1ID,
		Track2ID:       item.Track2ID,
		Track1Side:     item.Track1Side,
		Track2Side:     item.Track2Side,
		ConnectionType: item.ConnectionType,
	}
}

func dtoToWagon(item WagonDTO) normalized.Wagon {
	return normalized.Wagon{
		WagonID:    item.WagonID,
		SchemeID:   item.SchemeID,
		Name:       item.Name,
		Color:      item.Color,
		TrackID:    item.TrackID,
		TrackIndex: item.TrackIndex,
	}
}

func dtoToLocomotive(item LocomotiveDTO) normalized.Locomotive {
	return normalized.Locomotive{
		LocoID:     item.LocoID,
		SchemeID:   item.SchemeID,
		Name:       item.Name,
		Color:      item.Color,
		TrackID:    item.TrackID,
		TrackIndex: item.TrackIndex,
	}
}

func dtoToCoupling(item CouplingDTO) normalized.Coupling {
	return normalized.Coupling{
		CouplingID: item.CouplingID,
		SchemeID:   item.SchemeID,
		Object1ID:  item.Object1ID,
		Object2ID:  item.Object2ID,
	}
}

func dtoToScenarioStep(item ScenarioStepDTO) normalized.ScenarioStep {
	payload := make([]byte, len(item.PayloadJSON))
	copy(payload, item.PayloadJSON)
	return normalized.ScenarioStep{
		StepID:      item.StepID,
		ScenarioID:  item.ScenarioID,
		StepOrder:   item.StepOrder,
		StepType:    item.StepType,
		FromTrackID: item.FromTrackID,
		FromIndex:   item.FromIndex,
		ToTrackID:   item.ToTrackID,
		ToIndex:     item.ToIndex,
		Object1ID:   item.Object1ID,
		Object2ID:   item.Object2ID,
		PayloadJSON: json.RawMessage(payload),
	}
}

func dtoToTracks(items []TrackDTO) []normalized.Track {
	result := make([]normalized.Track, 0, len(items))
	for _, item := range items {
		result = append(result, dtoToTrack(item))
	}
	return result
}

func dtoToTrackConnections(items []TrackConnectionDTO) []normalized.TrackConnection {
	result := make([]normalized.TrackConnection, 0, len(items))
	for _, item := range items {
		result = append(result, dtoToTrackConnection(item))
	}
	return result
}

func dtoToWagons(items []WagonDTO) []normalized.Wagon {
	result := make([]normalized.Wagon, 0, len(items))
	for _, item := range items {
		result = append(result, dtoToWagon(item))
	}
	return result
}

func dtoToLocomotives(items []LocomotiveDTO) []normalized.Locomotive {
	result := make([]normalized.Locomotive, 0, len(items))
	for _, item := range items {
		result = append(result, dtoToLocomotive(item))
	}
	return result
}

func dtoToCouplings(items []CouplingDTO) []normalized.Coupling {
	result := make([]normalized.Coupling, 0, len(items))
	for _, item := range items {
		result = append(result, dtoToCoupling(item))
	}
	return result
}

func dtoToScenarioSteps(items []ScenarioStepDTO) []normalized.ScenarioStep {
	result := make([]normalized.ScenarioStep, 0, len(items))
	for _, item := range items {
		result = append(result, dtoToScenarioStep(item))
	}
	return result
}

type GenerateDraftHeuristicScenarioRequest struct {
	SchemeID            int    `json:"scheme_id"`
	TargetColor         string `json:"target_color"`
	RequiredTargetCount int    `json:"required_target_count"`
	FormationTrackID    string `json:"formation_track_id,omitempty"`
}

type GenerateAndSaveDraftHeuristicScenarioRequest struct {
	SchemeID            int    `json:"scheme_id"`
	TargetColor         string `json:"target_color"`
	RequiredTargetCount int    `json:"required_target_count"`
	FormationTrackID    string `json:"formation_track_id,omitempty"`
	Name                string `json:"name,omitempty"`
}

type SaveHeuristicAsScenarioRequest struct {
	HeuristicScenarioID string `json:"heuristic_scenario_id"`
	Name                string `json:"name,omitempty"`
}

type DraftHeuristicFeasibilityDTO struct {
	ChosenFormationTrackID  string   `json:"chosen_formation_track_id"`
	ChosenBufferTrackID     string   `json:"chosen_buffer_track_id"`
	TargetCount             int      `json:"target_count"`
	RequiredTargetCount     int      `json:"required_target_count"`
	AvailableBufferCapacity int      `json:"available_buffer_capacity"`
	Reasons                 []string `json:"reasons,omitempty"`
}

type DraftScenarioStepDTO struct {
	StepOrder          int    `json:"step_order"`
	StepType           string `json:"step_type"`
	SourceTrackID      string `json:"source_track_id"`
	DestinationTrackID string `json:"destination_track_id"`
	SourceSide         string `json:"source_side,omitempty"`
	WagonCount         int    `json:"wagon_count"`
	TargetColor        string `json:"target_color"`
	FormationTrackID   string `json:"formation_track_id"`
	BufferTrackID      string `json:"buffer_track_id"`
	MainTrackID        string `json:"main_track_id"`
}

type DraftScenarioDTO struct {
	SchemeID            int                    `json:"scheme_id"`
	TargetColor         string                 `json:"target_color"`
	RequiredTargetCount int                    `json:"required_target_count"`
	FormationTrackID    string                 `json:"formation_track_id"`
	BufferTrackID       string                 `json:"buffer_track_id"`
	MainTrackID         string                 `json:"main_track_id"`
	Steps               []DraftScenarioStepDTO `json:"steps,omitempty"`
}

type DraftStepCostDTO struct {
	CoupleCount      int      `json:"couple_count"`
	DecoupleCount    int      `json:"decouple_count"`
	LocoDistance     float64  `json:"loco_distance"`
	SwitchCrossCount int      `json:"switch_cross_count"`
	TotalCost        float64  `json:"total_cost"`
	Feasible         bool     `json:"feasible"`
	Reasons          []string `json:"reasons,omitempty"`
}

type DraftScenarioMetricsDTO struct {
	TotalStepCount        int     `json:"total_step_count"`
	TotalCoupleCount      int     `json:"total_couple_count"`
	TotalDecoupleCount    int     `json:"total_decouple_count"`
	TotalLocoDistance     float64 `json:"total_loco_distance"`
	TotalSwitchCrossCount int     `json:"total_switch_cross_count"`
	TotalCost             float64 `json:"total_cost"`
	Success               bool    `json:"success"`
}

type DraftHeuristicScenarioResponse struct {
	OK            bool                          `json:"ok"`
	Message       string                        `json:"message,omitempty"`
	Feasible      bool                          `json:"feasible"`
	Reasons       []string                      `json:"reasons,omitempty"`
	Feasibility   *DraftHeuristicFeasibilityDTO `json:"feasibility,omitempty"`
	DraftScenario *DraftScenarioDTO             `json:"draft_scenario,omitempty"`
	Metrics       *DraftScenarioMetricsDTO      `json:"metrics,omitempty"`
}

type HeuristicScenarioDTO struct {
	HeuristicScenarioID string                   `json:"heuristic_scenario_id"`
	SchemeID            int                      `json:"scheme_id"`
	Name                string                   `json:"name"`
	TargetColor         string                   `json:"target_color"`
	RequiredTargetCount int                      `json:"required_target_count"`
	FormationTrackID    string                   `json:"formation_track_id"`
	BufferTrackID       string                   `json:"buffer_track_id"`
	MainTrackID         string                   `json:"main_track_id"`
	Feasible            bool                     `json:"feasible"`
	Reasons             []string                 `json:"reasons,omitempty"`
	Steps               []DraftScenarioStepDTO   `json:"steps,omitempty"`
	Metrics             *DraftScenarioMetricsDTO `json:"metrics,omitempty"`
}

type SaveDraftHeuristicScenarioResponse struct {
	OK                       bool                          `json:"ok"`
	Message                  string                        `json:"message,omitempty"`
	Feasible                 bool                          `json:"feasible"`
	Reasons                  []string                      `json:"reasons,omitempty"`
	Feasibility              *DraftHeuristicFeasibilityDTO `json:"feasibility,omitempty"`
	SavedHeuristicScenarioID string                        `json:"saved_heuristic_scenario_id,omitempty"`
	HeuristicScenario        *HeuristicScenarioDTO         `json:"heuristic_scenario,omitempty"`
}

type ListHeuristicScenariosResponse struct {
	OK                 bool                   `json:"ok"`
	Message            string                 `json:"message,omitempty"`
	HeuristicScenarios []HeuristicScenarioDTO `json:"heuristic_scenarios,omitempty"`
}

type GetHeuristicScenarioResponse struct {
	OK                bool                  `json:"ok"`
	Message           string                `json:"message,omitempty"`
	HeuristicScenario *HeuristicScenarioDTO `json:"heuristic_scenario,omitempty"`
}

type SaveHeuristicAsScenarioResponse struct {
	OK                bool              `json:"ok"`
	Message           string            `json:"message,omitempty"`
	CreatedScenarioID string            `json:"created_scenario_id,omitempty"`
	Scenario          *ScenarioDTO      `json:"scenario,omitempty"`
	ScenarioSteps     []ScenarioStepDTO `json:"scenario_steps,omitempty"`
}

type GenerateFullHeuristicScenarioRequest struct {
	SchemeID            int    `json:"scheme_id"`
	TargetColor         string `json:"target_color"`
	RequiredTargetCount int    `json:"required_target_count"`
	FormationTrackID    string `json:"formation_track_id,omitempty"`
	Name                string `json:"name,omitempty"`
}

type GenerateFullHeuristicScenarioResponse struct {
	OK                  bool                          `json:"ok"`
	Message             string                        `json:"message,omitempty"`
	Feasible            bool                          `json:"feasible"`
	Reasons             []string                      `json:"reasons,omitempty"`
	Feasibility         *DraftHeuristicFeasibilityDTO `json:"feasibility,omitempty"`
	HeuristicScenarioID string                        `json:"heuristic_scenario_id,omitempty"`
	CreatedScenarioID   string                        `json:"created_scenario_id,omitempty"`
	Scenario            *ScenarioDTO                  `json:"scenario,omitempty"`
	ScenarioSteps       []ScenarioStepDTO             `json:"scenario_steps,omitempty"`
}

func toDraftHeuristicFeasibilityDTO(item heuristicservice.FixedClassFeasibility) *DraftHeuristicFeasibilityDTO {
	return &DraftHeuristicFeasibilityDTO{
		ChosenFormationTrackID:  item.ChosenFormationTrackID,
		ChosenBufferTrackID:     item.ChosenBufferTrackID,
		TargetCount:             item.TargetCount,
		RequiredTargetCount:     item.RequiredTargetCount,
		AvailableBufferCapacity: item.AvailableBufferCapacity,
		Reasons:                 append([]string{}, item.Reasons...),
	}
}

func toDraftScenarioDTO(item heuristicservice.DraftScenario) DraftScenarioDTO {
	return DraftScenarioDTO{
		SchemeID:            item.SchemeID,
		TargetColor:         item.TargetColor,
		RequiredTargetCount: item.RequiredTargetCount,
		FormationTrackID:    item.FormationTrackID,
		BufferTrackID:       item.BufferTrackID,
		MainTrackID:         item.MainTrackID,
		Steps:               toDraftScenarioStepDTOs(item.Steps),
	}
}

func toDraftScenarioStepDTO(item heuristicservice.DraftScenarioStep) DraftScenarioStepDTO {
	return DraftScenarioStepDTO{
		StepOrder:          item.StepOrder,
		StepType:           string(item.StepType),
		SourceTrackID:      item.SourceTrackID,
		DestinationTrackID: item.DestinationTrackID,
		SourceSide:         item.SourceSide,
		WagonCount:         item.WagonCount,
		TargetColor:        item.TargetColor,
		FormationTrackID:   item.FormationTrackID,
		BufferTrackID:      item.BufferTrackID,
		MainTrackID:        item.MainTrackID,
	}
}

func toDraftScenarioStepDTOs(items []heuristicservice.DraftScenarioStep) []DraftScenarioStepDTO {
	result := make([]DraftScenarioStepDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toDraftScenarioStepDTO(item))
	}
	return result
}

func toDraftStepCostDTO(item heuristicservice.DraftStepCost) DraftStepCostDTO {
	return DraftStepCostDTO{
		CoupleCount:      item.CoupleCount,
		DecoupleCount:    item.DecoupleCount,
		LocoDistance:     item.LocoDistance,
		SwitchCrossCount: item.SwitchCrossCount,
		TotalCost:        item.TotalCost,
		Feasible:         item.Feasible,
		Reasons:          append([]string{}, item.Reasons...),
	}
}

func toDraftScenarioMetricsDTO(item heuristicservice.DraftScenarioMetrics) DraftScenarioMetricsDTO {
	return DraftScenarioMetricsDTO{
		TotalStepCount:        item.TotalStepCount,
		TotalCoupleCount:      item.TotalCoupleCount,
		TotalDecoupleCount:    item.TotalDecoupleCount,
		TotalLocoDistance:     item.TotalLocoDistance,
		TotalSwitchCrossCount: item.TotalSwitchCrossCount,
		TotalCost:             item.TotalCost,
		Success:               item.Success,
	}
}

func toHeuristicScenarioDTO(item normalized.HeuristicScenario) HeuristicScenarioDTO {
	var metrics *DraftScenarioMetricsDTO
	if len(item.MetricsJSON) > 0 {
		var parsed DraftScenarioMetricsDTO
		if err := json.Unmarshal(item.MetricsJSON, &parsed); err == nil {
			metrics = &parsed
		}
	}

	return HeuristicScenarioDTO{
		HeuristicScenarioID: item.HeuristicScenarioID,
		SchemeID:            item.SchemeID,
		Name:                item.Name,
		TargetColor:         item.TargetColor,
		RequiredTargetCount: item.RequiredTargetCount,
		FormationTrackID:    item.FormationTrackID,
		BufferTrackID:       item.BufferTrackID,
		MainTrackID:         item.MainTrackID,
		Feasible:            item.Feasible,
		Reasons:             append([]string{}, item.Reasons...),
		Steps:               toStoredHeuristicStepDTOs(item.Steps),
		Metrics:             metrics,
	}
}

func toStoredHeuristicStepDTO(item normalized.HeuristicScenarioStep) DraftScenarioStepDTO {
	return DraftScenarioStepDTO{
		StepOrder:          item.StepOrder,
		StepType:           item.StepType,
		SourceTrackID:      item.SourceTrackID,
		DestinationTrackID: item.DestinationTrackID,
		SourceSide:         item.SourceSide,
		WagonCount:         item.WagonCount,
		TargetColor:        item.TargetColor,
		FormationTrackID:   item.FormationTrackID,
		BufferTrackID:      item.BufferTrackID,
		MainTrackID:        item.MainTrackID,
	}
}

func toStoredHeuristicStepDTOs(items []normalized.HeuristicScenarioStep) []DraftScenarioStepDTO {
	result := make([]DraftScenarioStepDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toStoredHeuristicStepDTO(item))
	}
	return result
}

func toHeuristicScenarioDTOs(items []normalized.HeuristicScenario) []HeuristicScenarioDTO {
	result := make([]HeuristicScenarioDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toHeuristicScenarioDTO(item))
	}
	return result
}
