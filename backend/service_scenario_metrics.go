package main

import (
	"fmt"
	"math"

	"trains/backend/normalized"
)

const scenarioMetricMetersPerCell = 15.0

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
		return normalized.ScenarioMetrics{}, fmt.Errorf("РЅРµ СѓРґР°Р»РѕСЃСЊ Р·Р°РіСЂСѓР·РёС‚СЊ СЃС†РµРЅР°СЂРёР№: %w", err)
	}
	scheme, err := appStore.GetNormalizedScheme(scenario.SchemeID, userID)
	if err != nil {
		return normalized.ScenarioMetrics{}, fmt.Errorf("РЅРµ СѓРґР°Р»РѕСЃСЊ Р·Р°РіСЂСѓР·РёС‚СЊ СЃС…РµРјСѓ СЃС†РµРЅР°СЂРёСЏ: %w", err)
	}

	metrics := normalized.ScenarioMetrics{
		ScenarioID: scenarioID,
	}
	state := runtime.State

	for _, step := range runtime.Steps {
		switch step.StepType {
		case "move_loco":
			if step.Object1ID == nil || step.ToTrackID == nil || step.ToIndex == nil {
				return normalized.ScenarioMetrics{}, fmt.Errorf("РІ С€Р°РіРµ move_loco РЅРµ С…РІР°С‚Р°РµС‚ object1_id, to_track_id РёР»Рё to_index")
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
				return normalized.ScenarioMetrics{}, fmt.Errorf("РЅРµ СѓРґР°Р»РѕСЃСЊ РїРѕСЃС‡РёС‚Р°С‚СЊ РјР°СЂС€СЂСѓС‚ РґР»СЏ С€Р°РіР° %s: %w", step.StepID, err)
			}

			wagonCount, err := currentTrainWagonCount(state, *step.Object1ID)
			if err != nil {
				return normalized.ScenarioMetrics{}, err
			}

			metrics.TotalLocoDistance += plan.CellsPassed
			metrics.TotalSwitchCrossings += countScenarioSwitchCrossings(
				scheme.Tracks,
				scheme.TrackConnections,
				sourceTrackID,
				*step.ToTrackID,
			)
			if wagonCount > 0 {
				metrics.LoadedLocoDistance += plan.CellsPassed
				metrics.TotalWagonsMoved += wagonCount
				metrics.TotalWagonDistance += wagonCount * plan.CellsPassed
			} else {
				metrics.EmptyLocoDistance += plan.CellsPassed
			}

		case "couple":
			metrics.TotalCouples++
		case "decouple":
			metrics.TotalDecouples++
		}

		nextState, _, err := applyScenarioStep(state, step)
		if err != nil {
			return normalized.ScenarioMetrics{}, fmt.Errorf("РЅРµ СѓРґР°Р»РѕСЃСЊ РїСЂРёРјРµРЅРёС‚СЊ С€Р°Рі %s: %w", step.StepID, err)
		}
		state = nextState
	}

	metrics.TotalLocoDistanceMeters = scenarioCellsToMeters(metrics.TotalLocoDistance)
	metrics.EmptyLocoDistanceMeters = scenarioCellsToMeters(metrics.EmptyLocoDistance)
	metrics.LoadedLocoDistanceMeters = scenarioCellsToMeters(metrics.LoadedLocoDistance)
	metrics.TotalWagonDistanceMeters = scenarioCellsToMeters(metrics.TotalWagonDistance)

	if err := appStore.SaveScenarioMetrics(scenarioID, metrics); err != nil {
		return normalized.ScenarioMetrics{}, fmt.Errorf("РЅРµ СѓРґР°Р»РѕСЃСЊ СЃРѕС…СЂР°РЅРёС‚СЊ РјРµС‚СЂРёРєРё СЃС†РµРЅР°СЂРёСЏ: %w", err)
	}
	return metrics, nil
}

func currentVehicleTrackID(state RuntimeState, vehicleID string) (string, error) {
	for _, vehicle := range state.Vehicles {
		if vehicle.ID == vehicleID {
			return vehicle.PathID, nil
		}
	}
	return "", fmt.Errorf("РЅРµ РЅР°Р№РґРµРЅ РѕР±СЉРµРєС‚ %s РґР»СЏ СЂР°СЃС‡С‘С‚Р° РјРµС‚СЂРёРє", vehicleID)
}

func currentTrainWagonCount(state RuntimeState, locomotiveID string) (int, error) {
	trainOrder, err := buildTrainOrder(locomotiveID, state.Vehicles, state.Couplings)
	if err != nil {
		return 0, fmt.Errorf("Не удалось определить состав для расчета метрик: %w", err)
	}
	if len(trainOrder) <= 1 {
		return 0, nil
	}

	vehicleByID := make(map[string]Vehicle, len(state.Vehicles))
	for _, vehicle := range state.Vehicles {
		vehicleByID[vehicle.ID] = vehicle
	}

	wagonCount := 0
	for _, id := range trainOrder[1:] {
		vehicle, ok := vehicleByID[id]
		if ok && vehicle.Type == "wagon" {
			wagonCount++
		}
	}
	return wagonCount, nil
}

func scenarioCellsToMeters(cells int) float64 {
	return math.Round(float64(cells)*scenarioMetricMetersPerCell*10) / 10
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

