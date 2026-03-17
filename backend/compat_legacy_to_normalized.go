package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"trains/backend/normalized"
)

type LegacyLayoutToNormalizedResult struct {
	Scheme   normalized.Scheme `json:"scheme"`
	Warnings []string          `json:"warnings,omitempty"`
}

type LegacyCommandsToNormalizedResult struct {
	Steps    []normalized.ScenarioStep `json:"steps"`
	Warnings []string                  `json:"warnings,omitempty"`
}

func BuildNormalizedSchemeFromLegacyLayout(name string, state LayoutState) (LegacyLayoutToNormalizedResult, error) {
	if strings.TrimSpace(name) == "" {
		return LegacyLayoutToNormalizedResult{}, fmt.Errorf("layout name is required")
	}

	pathStateByID := make(map[string]PathState, len(state.Paths))
	for _, path := range state.Paths {
		pathStateByID[path.ID] = path
	}

	seenTrackIDs := map[string]struct{}{}
	tracks := make([]normalized.Track, 0, len(state.Segments))
	warnings := []string{}
	for _, segment := range state.Segments {
		if strings.TrimSpace(segment.ID) == "" {
			return LegacyLayoutToNormalizedResult{}, fmt.Errorf("segment id is required")
		}
		if _, exists := seenTrackIDs[segment.ID]; exists {
			return LegacyLayoutToNormalizedResult{}, fmt.Errorf("duplicate segment id: %s", segment.ID)
		}
		seenTrackIDs[segment.ID] = struct{}{}

		pathState, hasPathState := pathStateByID[segment.ID]
		capacity := deriveNormalizedTrackCapacity(segment.ID, state.Vehicles, pathState, hasPathState)
		if capacity <= 0 {
			return LegacyLayoutToNormalizedResult{}, fmt.Errorf("track %s capacity could not be determined from legacy layout", segment.ID)
		}

		normalizedType, storageAllowed, typeWarnings := normalizedTrackTypeFromLegacySegment(segment.Type)
		warnings = append(warnings, typeWarnings...)
		tracks = append(tracks, normalized.Track{
			TrackID:        segment.ID,
			Name:           normalizedTrackName(segment.ID),
			Type:           normalizedType,
			StartX:         segment.From.X,
			StartY:         segment.From.Y,
			EndX:           segment.To.X,
			EndY:           segment.To.Y,
			Capacity:       capacity,
			StorageAllowed: storageAllowed,
		})
	}

	trackByID := make(map[string]normalized.Track, len(tracks))
	for _, track := range tracks {
		trackByID[track.TrackID] = track
	}

	vehiclesByID := map[string]Vehicle{}
	for _, vehicle := range state.Vehicles {
		if strings.TrimSpace(vehicle.ID) == "" {
			return LegacyLayoutToNormalizedResult{}, fmt.Errorf("vehicle id is required")
		}
		vehiclesByID[vehicle.ID] = vehicle
		if strings.TrimSpace(vehicle.PathID) == "" {
			return LegacyLayoutToNormalizedResult{}, fmt.Errorf("vehicle %s has no pathId", vehicle.ID)
		}
		track, ok := trackByID[vehicle.PathID]
		if !ok {
			return LegacyLayoutToNormalizedResult{}, fmt.Errorf("vehicle %s references unknown pathId %s", vehicle.ID, vehicle.PathID)
		}
		if vehicle.PathIndex < 0 || vehicle.PathIndex >= track.Capacity {
			return LegacyLayoutToNormalizedResult{}, fmt.Errorf("vehicle %s has invalid pathIndex %d for track %s with capacity %d", vehicle.ID, vehicle.PathIndex, vehicle.PathID, track.Capacity)
		}
	}

	wagons := make([]normalized.Wagon, 0)
	locomotives := make([]normalized.Locomotive, 0)
	for _, vehicle := range state.Vehicles {
		switch vehicle.Type {
		case "wagon":
			wagons = append(wagons, normalized.Wagon{
				WagonID:    vehicle.ID,
				Name:       normalizedObjectName(vehicle.Code, vehicle.ID),
				Color:      vehicle.Color,
				TrackID:    vehicle.PathID,
				TrackIndex: vehicle.PathIndex,
			})
		case "locomotive":
			locomotives = append(locomotives, normalized.Locomotive{
				LocoID:     vehicle.ID,
				Name:       normalizedObjectName(vehicle.Code, vehicle.ID),
				Color:      vehicle.Color,
				TrackID:    vehicle.PathID,
				TrackIndex: vehicle.PathIndex,
			})
		default:
			return LegacyLayoutToNormalizedResult{}, fmt.Errorf("unsupported vehicle type %s for vehicle %s", vehicle.Type, vehicle.ID)
		}
	}

	couplings := make([]normalized.Coupling, 0, len(state.Couplings))
	for _, coupling := range state.Couplings {
		if _, ok := vehiclesByID[coupling.A]; !ok {
			return LegacyLayoutToNormalizedResult{}, fmt.Errorf("coupling %s references unknown object %s", coupling.ID, coupling.A)
		}
		if _, ok := vehiclesByID[coupling.B]; !ok {
			return LegacyLayoutToNormalizedResult{}, fmt.Errorf("coupling %s references unknown object %s", coupling.ID, coupling.B)
		}
		couplingID := coupling.ID
		if strings.TrimSpace(couplingID) == "" {
			couplingID = fmt.Sprintf("cp-%s-%s", coupling.A, coupling.B)
			warnings = append(warnings, fmt.Sprintf("legacy coupling without id was assigned generated normalized id %s", couplingID))
		}
		couplings = append(couplings, normalized.Coupling{
			CouplingID: couplingID,
			Object1ID:  coupling.A,
			Object2ID:  coupling.B,
		})
	}

	connections := buildNormalizedTrackConnectionsFromLegacySegments(state.Segments)
	if len(connections) == 0 && len(state.Segments) > 1 {
		warnings = append(warnings, "no normalized track connections could be inferred from legacy segment geometry")
	}

	return LegacyLayoutToNormalizedResult{
		Scheme: normalized.Scheme{
			Name:             name,
			Tracks:           tracks,
			TrackConnections: connections,
			Wagons:           wagons,
			Locomotives:      locomotives,
			Couplings:        couplings,
		},
		Warnings: dedupeWarnings(warnings),
	}, nil
}

func BuildNormalizedScenarioStepsFromLegacyCommands(commands []CommandSpec) (LegacyCommandsToNormalizedResult, error) {
	steps := make([]normalized.ScenarioStep, 0, len(commands))
	warnings := []string{}
	for _, command := range commands {
		if strings.TrimSpace(command.ID) == "" {
			return LegacyCommandsToNormalizedResult{}, fmt.Errorf("legacy command id is required")
		}
		stepType, err := normalizedStepTypeFromLegacyCommand(command.Type)
		if err != nil {
			return LegacyCommandsToNormalizedResult{}, err
		}

		step := normalized.ScenarioStep{
			StepID:    command.ID,
			StepOrder: command.Order,
			StepType:  stepType,
		}

		if command.Payload.FromPathID != "" {
			value := command.Payload.FromPathID
			step.FromTrackID = &value
		}
		if command.Payload.FromIndex != 0 {
			value := command.Payload.FromIndex
			step.FromIndex = &value
		}

		toTrackID := command.Payload.ToPathID
		if toTrackID == "" {
			toTrackID = command.Payload.TargetPathID
		}
		if toTrackID != "" {
			value := toTrackID
			step.ToTrackID = &value
		}

		toIndex := command.Payload.ToIndex
		if toIndex == 0 {
			toIndex = command.Payload.TargetIndex
		}
		if toIndex != 0 || command.Payload.TargetIndex == 0 || command.Payload.ToIndex == 0 {
			value := toIndex
			step.ToIndex = &value
		}

		switch stepType {
		case "move_loco":
			if command.Payload.LocoID != "" {
				value := command.Payload.LocoID
				step.Object1ID = &value
			} else if command.Payload.AID != "" {
				value := command.Payload.AID
				step.Object1ID = &value
				warnings = append(warnings, fmt.Sprintf("legacy MOVE_LOCO command %s had no locoId; aId was used as object1_id", command.ID))
			}
			if command.Payload.BID != "" {
				value := command.Payload.BID
				step.Object2ID = &value
			}
		case "couple", "decouple", "move_group":
			if command.Payload.AID != "" {
				value := command.Payload.AID
				step.Object1ID = &value
			}
			if command.Payload.BID != "" {
				value := command.Payload.BID
				step.Object2ID = &value
			}
		}

		payloadJSON, err := json.Marshal(command.Payload)
		if err != nil {
			return LegacyCommandsToNormalizedResult{}, fmt.Errorf("failed to encode legacy command payload for %s: %w", command.ID, err)
		}
		step.PayloadJSON = payloadJSON
		steps = append(steps, step)
	}

	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].StepOrder == steps[j].StepOrder {
			return steps[i].StepID < steps[j].StepID
		}
		return steps[i].StepOrder < steps[j].StepOrder
	})

	return LegacyCommandsToNormalizedResult{
		Steps:    steps,
		Warnings: dedupeWarnings(warnings),
	}, nil
}

func deriveNormalizedTrackCapacity(trackID string, vehicles []Vehicle, pathState PathState, hasPathState bool) int {
	if hasPathState && pathState.Capacity > 0 {
		return pathState.Capacity
	}
	maxIndex := -1
	for _, vehicle := range vehicles {
		if vehicle.PathID == trackID && vehicle.PathIndex > maxIndex {
			maxIndex = vehicle.PathIndex
		}
	}
	if maxIndex >= 0 {
		return maxIndex + 1
	}
	return 1
}

func normalizedTrackTypeFromLegacySegment(legacyType string) (string, bool, []string) {
	switch strings.ToLower(strings.TrimSpace(legacyType)) {
	case "main":
		return "main", false, nil
	case "sorting":
		return "sorting", true, nil
	case "lead":
		return "lead", true, nil
	case "bypass":
		return "bypass", false, nil
	case "normal":
		return "normal", false, []string{"legacy segment type normal does not carry storage_allowed; normalized track defaulted to storage_allowed=false"}
	case "other", "":
		return "normal", false, []string{"legacy segment type other/empty was mapped to normalized type normal with storage_allowed=false"}
	default:
		return "normal", false, []string{fmt.Sprintf("unknown legacy segment type %s was mapped to normalized type normal with storage_allowed=false", legacyType)}
	}
}

func normalizedTrackName(trackID string) string {
	return fmt.Sprintf("Track %s", trackID)
}

func normalizedObjectName(code string, fallbackID string) string {
	if strings.TrimSpace(code) != "" {
		return code
	}
	return fallbackID
}

func buildNormalizedTrackConnectionsFromLegacySegments(segments []Segment) []normalized.TrackConnection {
	type endpointRef struct {
		TrackID string
		Side    string
	}

	refsByPoint := map[string][]endpointRef{}
	for _, segment := range segments {
		refsByPoint[slotID(segment.From.X, segment.From.Y)] = append(refsByPoint[slotID(segment.From.X, segment.From.Y)], endpointRef{
			TrackID: segment.ID,
			Side:    "start",
		})
		refsByPoint[slotID(segment.To.X, segment.To.Y)] = append(refsByPoint[slotID(segment.To.X, segment.To.Y)], endpointRef{
			TrackID: segment.ID,
			Side:    "end",
		})
	}

	connections := make([]normalized.TrackConnection, 0)
	seen := map[string]struct{}{}
	for pointKey, refs := range refsByPoint {
		if len(refs) < 2 {
			continue
		}
		connectionType := "serial"
		if len(refs) > 2 {
			connectionType = "switch"
		}
		for i := 0; i < len(refs); i++ {
			for j := i + 1; j < len(refs); j++ {
				a := refs[i]
				b := refs[j]
				key := a.TrackID + "|" + a.Side + "|" + b.TrackID + "|" + b.Side
				if a.TrackID > b.TrackID {
					key = b.TrackID + "|" + b.Side + "|" + a.TrackID + "|" + a.Side
				}
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				connections = append(connections, normalized.TrackConnection{
					ConnectionID:   fmt.Sprintf("conn-%s-%s-%s", strings.ReplaceAll(pointKey, ":", "_"), a.TrackID, b.TrackID),
					Track1ID:       a.TrackID,
					Track2ID:       b.TrackID,
					Track1Side:     a.Side,
					Track2Side:     b.Side,
					ConnectionType: connectionType,
				})
			}
		}
	}

	sort.SliceStable(connections, func(i, j int) bool {
		return connections[i].ConnectionID < connections[j].ConnectionID
	})
	return connections
}

func normalizedStepTypeFromLegacyCommand(commandType string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(commandType)) {
	case "MOVE_LOCO":
		return "move_loco", nil
	case "COUPLE":
		return "couple", nil
	case "DECOUPLE":
		return "decouple", nil
	case "MOVE_GROUP":
		return "move_group", nil
	default:
		return "", fmt.Errorf("unsupported legacy command type %s", commandType)
	}
}
