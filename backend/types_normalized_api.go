package main

import (
	"encoding/json"

	"trains/backend/normalized"
)

type SchemeDTO struct {
	SchemeID int    `json:"scheme_id"`
	Name     string `json:"name"`
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
	ScenarioID string `json:"scenario_id"`
	SchemeID   int    `json:"scheme_id"`
	Name       string `json:"name"`
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
		ScenarioID: item.ScenarioID,
		SchemeID:   item.SchemeID,
		Name:       item.Name,
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
