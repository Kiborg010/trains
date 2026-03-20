package main

import (
	"fmt"

	"trains/backend/normalized"
)

type NormalizedExecutionRuntime struct {
	State RuntimeState
	Steps []normalized.ScenarioStep
}

func buildExecutionRuntimeFromNormalized(store Store, userID int, scenarioID string) (*NormalizedExecutionRuntime, error) {
	scenario, err := store.GetNormalizedScenario(scenarioID, userID)
	if err != nil {
		return nil, fmt.Errorf("не удалось загрузить нормализованный сценарий: %w", err)
	}
	scheme, err := store.GetNormalizedScheme(scenario.SchemeID, userID)
	if err != nil {
		return nil, fmt.Errorf("не удалось загрузить нормализованную схему: %w", err)
	}

	state := RuntimeState{
		Segments:  make([]Segment, 0, len(scheme.Tracks)),
		Vehicles:  make([]Vehicle, 0, len(scheme.Wagons)+len(scheme.Locomotives)),
		Couplings: make([]Coupling, 0, len(scheme.Couplings)),
	}

	segmentByID := make(map[string]Segment, len(scheme.Tracks))
	for _, track := range scheme.Tracks {
		segment := Segment{
			ID:   track.TrackID,
			Type: track.Type,
			From: Point{X: track.StartX, Y: track.StartY},
			To:   Point{X: track.EndX, Y: track.EndY},
		}
		state.Segments = append(state.Segments, segment)
		segmentByID[segment.ID] = segment
	}

	for _, wagon := range scheme.Wagons {
		state.Vehicles = append(state.Vehicles, normalizedVehicleToRuntimeVehicle(
			wagon.WagonID,
			"wagon",
			wagon.Name,
			wagon.Color,
			wagon.TrackID,
			wagon.TrackIndex,
			segmentByID[wagon.TrackID],
		))
	}
	for _, loco := range scheme.Locomotives {
		state.Vehicles = append(state.Vehicles, normalizedVehicleToRuntimeVehicle(
			loco.LocoID,
			"locomotive",
			loco.Name,
			loco.Color,
			loco.TrackID,
			loco.TrackIndex,
			segmentByID[loco.TrackID],
		))
	}
	for _, coupling := range scheme.Couplings {
		state.Couplings = append(state.Couplings, Coupling{
			ID: coupling.CouplingID,
			A:  coupling.Object1ID,
			B:  coupling.Object2ID,
		})
	}

	state = finalizeRuntimeState(state, 32)
	return &NormalizedExecutionRuntime{
		State: state,
		Steps: append([]normalized.ScenarioStep{}, scenario.Steps...),
	}, nil
}

func normalizedVehicleToRuntimeVehicle(id, vehicleType, code, color, trackID string, trackIndex int, segment Segment) Vehicle {
	slots := getSegmentSlots(segment, 32)
	point := segment.From
	if trackIndex >= 0 && trackIndex < len(slots) {
		point = Point{X: slots[trackIndex].X, Y: slots[trackIndex].Y}
	}
	return Vehicle{
		ID:        id,
		Type:      vehicleType,
		Code:      code,
		Color:     color,
		PathID:    trackID,
		PathIndex: trackIndex,
		X:         point.X,
		Y:         point.Y,
	}
}
