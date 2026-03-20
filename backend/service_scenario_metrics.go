package main

import (
	"fmt"
	"math"

	"trains/backend/normalized"
)

type scenarioTrackRouteNode struct {
	Distance    float64
	TrackID     string
	SwitchCount int
}

type scenarioTrackNeighbor struct {
	TrackID        string
	ConnectionType string
}

func ComputeScenarioMetrics(userID int, scenarioID string) (normalized.ScenarioMetrics, error) {
	runtime, err := buildExecutionRuntimeFromNormalized(appStore, userID, scenarioID)
	if err != nil {
		return normalized.ScenarioMetrics{}, err
	}

	scenario, err := appStore.GetNormalizedScenario(scenarioID, userID)
	if err != nil {
		return normalized.ScenarioMetrics{}, fmt.Errorf("не удалось загрузить сценарий: %w", err)
	}
	scheme, err := appStore.GetNormalizedScheme(scenario.SchemeID, userID)
	if err != nil {
		return normalized.ScenarioMetrics{}, fmt.Errorf("не удалось загрузить схему сценария: %w", err)
	}

	metrics := normalized.ScenarioMetrics{
		ScenarioID: scenarioID,
	}
	state := runtime.State

	for _, step := range runtime.Steps {
		switch step.StepType {
		case "move_loco":
			if step.Object1ID == nil || step.ToTrackID == nil || step.ToIndex == nil {
				return normalized.ScenarioMetrics{}, fmt.Errorf("в шаге move_loco не хватает object1_id, to_track_id или to_index")
			}

			sourceTrackID, err := currentVehicleTrackID(state, *step.Object1ID)
			if err != nil {
				return normalized.ScenarioMetrics{}, err
			}

			plan, err := buildMovementPlan(PlanMovementRequest{
				GridSize:             32,
				Segments:             state.Segments,
				Vehicles:             state.Vehicles,
				Couplings:            state.Couplings,
				SelectedLocomotiveID: *step.Object1ID,
				TargetPathID:         *step.ToTrackID,
				TargetIndex:          *step.ToIndex,
			})
			if err != nil {
				return normalized.ScenarioMetrics{}, fmt.Errorf("не удалось посчитать маршрут для шага %s: %w", step.StepID, err)
			}

			metrics.TotalLocoDistance += plan.CellsPassed
			metrics.TotalSwitchCrossings += countScenarioSwitchCrossings(
				scheme.Tracks,
				scheme.TrackConnections,
				sourceTrackID,
				*step.ToTrackID,
			)

		case "couple":
			metrics.TotalCouples++
		case "decouple":
			metrics.TotalDecouples++
		}

		nextState, _, err := applyScenarioStep(state, step)
		if err != nil {
			return normalized.ScenarioMetrics{}, fmt.Errorf("не удалось применить шаг %s: %w", step.StepID, err)
		}
		state = nextState
	}

	if err := appStore.SaveScenarioMetrics(scenarioID, metrics); err != nil {
		return normalized.ScenarioMetrics{}, fmt.Errorf("не удалось сохранить метрики сценария: %w", err)
	}
	return metrics, nil
}

func currentVehicleTrackID(state RuntimeState, vehicleID string) (string, error) {
	for _, vehicle := range state.Vehicles {
		if vehicle.ID == vehicleID {
			return vehicle.PathID, nil
		}
	}
	return "", fmt.Errorf("не найден объект %s для расчёта метрик", vehicleID)
}

func countScenarioSwitchCrossings(tracks []normalized.Track, connections []normalized.TrackConnection, sourceTrackID string, destinationTrackID string) int {
	if sourceTrackID == "" || destinationTrackID == "" || sourceTrackID == destinationTrackID {
		return 0
	}

	tracksByID := make(map[string]normalized.Track, len(tracks))
	for _, track := range tracks {
		tracksByID[track.TrackID] = track
	}

	adjacency := make(map[string][]scenarioTrackNeighbor)
	for _, connection := range connections {
		adjacency[connection.Track1ID] = append(adjacency[connection.Track1ID], scenarioTrackNeighbor{
			TrackID:        connection.Track2ID,
			ConnectionType: connection.ConnectionType,
		})
		adjacency[connection.Track2ID] = append(adjacency[connection.Track2ID], scenarioTrackNeighbor{
			TrackID:        connection.Track1ID,
			ConnectionType: connection.ConnectionType,
		})
	}

	bestDistance := map[string]float64{
		sourceTrackID: scenarioTrackLength(tracksByID[sourceTrackID]),
	}
	bestSwitches := map[string]int{
		sourceTrackID: 0,
	}
	queue := []scenarioTrackRouteNode{{
		Distance:    bestDistance[sourceTrackID],
		TrackID:     sourceTrackID,
		SwitchCount: 0,
	}}

	for len(queue) > 0 {
		sortScenarioTrackRouteNodes(queue)
		current := queue[0]
		queue = queue[1:]
		if current.TrackID == destinationTrackID {
			return current.SwitchCount
		}

		for _, neighbor := range adjacency[current.TrackID] {
			track, ok := tracksByID[neighbor.TrackID]
			if !ok {
				continue
			}
			nextDistance := current.Distance + scenarioTrackLength(track)
			nextSwitchCount := current.SwitchCount
			if neighbor.ConnectionType == "switch" {
				nextSwitchCount++
			}

			existingDistance, seen := bestDistance[neighbor.TrackID]
			existingSwitchCount := bestSwitches[neighbor.TrackID]
			if seen && (existingDistance < nextDistance || (existingDistance == nextDistance && existingSwitchCount <= nextSwitchCount)) {
				continue
			}

			bestDistance[neighbor.TrackID] = nextDistance
			bestSwitches[neighbor.TrackID] = nextSwitchCount
			queue = append(queue, scenarioTrackRouteNode{
				Distance:    nextDistance,
				TrackID:     neighbor.TrackID,
				SwitchCount: nextSwitchCount,
			})
		}
	}

	return 0
}

func sortScenarioTrackRouteNodes(nodes []scenarioTrackRouteNode) {
	for i := 0; i < len(nodes)-1; i++ {
		best := i
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Distance < nodes[best].Distance ||
				(nodes[j].Distance == nodes[best].Distance && nodes[j].SwitchCount < nodes[best].SwitchCount) ||
				(nodes[j].Distance == nodes[best].Distance && nodes[j].SwitchCount == nodes[best].SwitchCount && nodes[j].TrackID < nodes[best].TrackID) {
				best = j
			}
		}
		nodes[i], nodes[best] = nodes[best], nodes[i]
	}
}

func scenarioTrackLength(track normalized.Track) float64 {
	return math.Hypot(track.EndX-track.StartX, track.EndY-track.StartY)
}
