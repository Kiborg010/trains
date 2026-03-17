package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"trains/backend/normalized"
)

// NormalizedSchemeToLegacyLayoutResult contains the reconstructed legacy state
// plus explicit warnings for fields that could only be approximated.
type NormalizedSchemeToLegacyLayoutResult struct {
	State    LayoutState `json:"state"`
	Warnings []string    `json:"warnings,omitempty"`
}

// NormalizedScenarioToLegacyCommandsResult contains reconstructed legacy commands
// plus warnings about lossy payload projection.
type NormalizedScenarioToLegacyCommandsResult struct {
	Commands []CommandSpec `json:"commands"`
	Warnings []string      `json:"warnings,omitempty"`
}

// BuildLegacyLayoutStateFromNormalizedSchemeDetails reconstructs the legacy
// LayoutState from the normalized scheme model.
//
// Explicit mappings:
//   - tracks -> segments
//   - wagons + locomotives -> vehicles
//   - couplings -> couplings
//
// Lossy parts are returned via Warnings:
//   - vehicle x/y are interpolated from track geometry + capacity
//   - legacy vehicle code is approximated from normalized name
//   - legacy PathState.Neighbors ignores connection side semantics and keeps
//     only undirected neighboring track ids
func BuildLegacyLayoutStateFromNormalizedSchemeDetails(
	scheme normalized.Scheme,
	tracks []normalized.Track,
	connections []normalized.TrackConnection,
	wagons []normalized.Wagon,
	locomotives []normalized.Locomotive,
	couplings []normalized.Coupling,
) NormalizedSchemeToLegacyLayoutResult {
	warnings := []string{}
	if scheme.SchemeID == 0 {
		warnings = append(warnings, "normalized scheme metadata is incomplete: scheme_id is empty or zero")
	}
	if strings.TrimSpace(scheme.Name) == "" {
		warnings = append(warnings, "normalized scheme metadata is incomplete: scheme name is empty")
	}
	trackByID := make(map[string]normalized.Track, len(tracks))
	for _, track := range tracks {
		trackByID[track.TrackID] = track
	}

	segments := make([]Segment, 0, len(tracks))
	for _, track := range tracks {
		segments = append(segments, legacySegmentFromTrack(track))
	}

	vehicles := make([]Vehicle, 0, len(wagons)+len(locomotives))
	for _, wagon := range wagons {
		vehicle, vehicleWarnings := legacyVehicleFromWagon(wagon, trackByID[wagon.TrackID])
		vehicles = append(vehicles, vehicle)
		warnings = append(warnings, vehicleWarnings...)
	}
	for _, loco := range locomotives {
		vehicle, vehicleWarnings := legacyVehicleFromLocomotive(loco, trackByID[loco.TrackID])
		vehicles = append(vehicles, vehicle)
		warnings = append(warnings, vehicleWarnings...)
	}

	legacyCouplings := make([]Coupling, 0, len(couplings))
	for _, coupling := range couplings {
		legacyCouplings = append(legacyCouplings, Coupling{
			ID: coupling.CouplingID,
			A:  coupling.Object1ID,
			B:  coupling.Object2ID,
		})
	}

	pathStates := buildLegacyPathStates(tracks, connections, vehicles)
	if len(tracks) > 0 {
		warnings = append(warnings, "legacy vehicle x/y coordinates are reconstructed by linear interpolation from normalized track geometry and capacity")
		warnings = append(warnings, "legacy path neighbors are reconstructed as an undirected track graph without preserving connection side semantics")
	}
	if len(wagons) > 0 || len(locomotives) > 0 {
		warnings = append(warnings, "legacy vehicle code is approximated from normalized object name because normalized model has no separate code field")
	}

	return NormalizedSchemeToLegacyLayoutResult{
		State: LayoutState{
			Segments:  segments,
			Vehicles:  vehicles,
			Couplings: legacyCouplings,
			Paths:     pathStates,
		},
		Warnings: dedupeWarnings(warnings),
	}
}

// BuildLegacyCommandsFromNormalizedScenarioDetails reconstructs legacy
// []CommandSpec from normalized scenario metadata + scenario steps.
//
// The adapter prefers explicit normalized step columns. payload_json is used as
// a secondary source for recognized legacy payload keys. Unrepresentable fields
// are surfaced through Warnings instead of being dropped silently.
func BuildLegacyCommandsFromNormalizedScenarioDetails(
	scenario normalized.Scenario,
	steps []normalized.ScenarioStep,
) NormalizedScenarioToLegacyCommandsResult {
	sortedSteps := append([]normalized.ScenarioStep{}, steps...)
	sort.SliceStable(sortedSteps, func(i, j int) bool {
		if sortedSteps[i].StepOrder == sortedSteps[j].StepOrder {
			return sortedSteps[i].StepID < sortedSteps[j].StepID
		}
		return sortedSteps[i].StepOrder < sortedSteps[j].StepOrder
	})

	commands := make([]CommandSpec, 0, len(sortedSteps))
	warnings := []string{}
	for _, step := range sortedSteps {
		payload, payloadWarnings := legacyPayloadFromScenarioStep(step)
		warnings = append(warnings, payloadWarnings...)
		commands = append(commands, CommandSpec{
			ID:      step.StepID,
			Order:   step.StepOrder,
			Type:    legacyCommandTypeFromStepType(step.StepType),
			Payload: payload,
		})
	}

	if scenario.ScenarioID == "" {
		warnings = append(warnings, "normalized scenario metadata is incomplete: scenario_id is empty")
	}

	return NormalizedScenarioToLegacyCommandsResult{
		Commands: commands,
		Warnings: dedupeWarnings(warnings),
	}
}

func legacySegmentFromTrack(track normalized.Track) Segment {
	return Segment{
		ID:   track.TrackID,
		Type: track.Type,
		From: Point{X: track.StartX, Y: track.StartY},
		To:   Point{X: track.EndX, Y: track.EndY},
	}
}

func legacyVehicleFromWagon(wagon normalized.Wagon, track normalized.Track) (Vehicle, []string) {
	x, y, warnings := interpolateLegacyVehicleCoordinates(track, wagon.TrackIndex)
	return Vehicle{
		ID:        wagon.WagonID,
		Type:      "wagon",
		Code:      wagon.Name,
		Color:     wagon.Color,
		PathID:    wagon.TrackID,
		PathIndex: wagon.TrackIndex,
		X:         x,
		Y:         y,
	}, warnings
}

func legacyVehicleFromLocomotive(loco normalized.Locomotive, track normalized.Track) (Vehicle, []string) {
	x, y, warnings := interpolateLegacyVehicleCoordinates(track, loco.TrackIndex)
	return Vehicle{
		ID:        loco.LocoID,
		Type:      "locomotive",
		Code:      loco.Name,
		Color:     loco.Color,
		PathID:    loco.TrackID,
		PathIndex: loco.TrackIndex,
		X:         x,
		Y:         y,
	}, warnings
}

func interpolateLegacyVehicleCoordinates(track normalized.Track, trackIndex int) (float64, float64, []string) {
	warnings := []string{}
	if track.TrackID == "" {
		return 0, 0, []string{"legacy vehicle coordinates could not be reconstructed because normalized track reference is missing"}
	}
	if track.Capacity <= 1 {
		if track.Capacity <= 0 {
			warnings = append(warnings, fmt.Sprintf("track %s has non-positive capacity; legacy coordinates were anchored to track start", track.TrackID))
		}
		return track.StartX, track.StartY, warnings
	}

	clampedIndex := trackIndex
	if clampedIndex < 0 {
		clampedIndex = 0
		warnings = append(warnings, fmt.Sprintf("negative track_index for track %s was clamped to 0 for legacy coordinate reconstruction", track.TrackID))
	}
	if clampedIndex >= track.Capacity {
		clampedIndex = track.Capacity - 1
		warnings = append(warnings, fmt.Sprintf("track_index beyond capacity for track %s was clamped for legacy coordinate reconstruction", track.TrackID))
	}

	ratio := float64(clampedIndex) / float64(track.Capacity-1)
	x := track.StartX + (track.EndX-track.StartX)*ratio
	y := track.StartY + (track.EndY-track.StartY)*ratio
	return x, y, warnings
}

func buildLegacyPathStates(tracks []normalized.Track, connections []normalized.TrackConnection, vehicles []Vehicle) []PathState {
	vehicleIDsByTrack := map[string][]string{}
	for _, vehicle := range vehicles {
		if vehicle.PathID == "" {
			continue
		}
		vehicleIDsByTrack[vehicle.PathID] = append(vehicleIDsByTrack[vehicle.PathID], vehicle.ID)
	}

	neighborsByTrack := map[string]map[string]struct{}{}
	for _, connection := range connections {
		if _, ok := neighborsByTrack[connection.Track1ID]; !ok {
			neighborsByTrack[connection.Track1ID] = map[string]struct{}{}
		}
		if _, ok := neighborsByTrack[connection.Track2ID]; !ok {
			neighborsByTrack[connection.Track2ID] = map[string]struct{}{}
		}
		neighborsByTrack[connection.Track1ID][connection.Track2ID] = struct{}{}
		neighborsByTrack[connection.Track2ID][connection.Track1ID] = struct{}{}
	}

	result := make([]PathState, 0, len(tracks))
	for _, track := range tracks {
		neighbors := make([]string, 0, len(neighborsByTrack[track.TrackID]))
		for neighborID := range neighborsByTrack[track.TrackID] {
			neighbors = append(neighbors, neighborID)
		}
		sort.Strings(neighbors)
		result = append(result, PathState{
			ID:         track.TrackID,
			Capacity:   track.Capacity,
			VehicleIDs: append([]string{}, vehicleIDsByTrack[track.TrackID]...),
			Neighbors:  neighbors,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func legacyCommandTypeFromStepType(stepType string) string {
	switch strings.ToLower(strings.TrimSpace(stepType)) {
	case "move_loco":
		return "MOVE_LOCO"
	case "couple":
		return "COUPLE"
	case "decouple":
		return "DECOUPLE"
	case "move_group":
		return "MOVE_GROUP"
	default:
		return strings.ToUpper(strings.TrimSpace(stepType))
	}
}

func legacyPayloadFromScenarioStep(step normalized.ScenarioStep) (CommandPayload, []string) {
	payload := CommandPayload{}
	warnings := []string{}

	if step.FromTrackID != nil {
		payload.FromPathID = *step.FromTrackID
	}
	if step.FromIndex != nil {
		payload.FromIndex = *step.FromIndex
	}
	if step.ToTrackID != nil {
		payload.ToPathID = *step.ToTrackID
		payload.TargetPathID = *step.ToTrackID
	}
	if step.ToIndex != nil {
		payload.ToIndex = *step.ToIndex
		payload.TargetIndex = *step.ToIndex
	}

	switch strings.ToLower(step.StepType) {
	case "move_loco":
		if step.Object1ID != nil {
			payload.LocoID = *step.Object1ID
			payload.AID = *step.Object1ID
		}
		if step.Object2ID != nil {
			payload.BID = *step.Object2ID
			warnings = append(warnings, fmt.Sprintf("scenario step %s uses object2_id for move_loco; legacy payload keeps it only as bId", step.StepID))
		}
	case "couple", "decouple":
		if step.Object1ID != nil {
			payload.AID = *step.Object1ID
		}
		if step.Object2ID != nil {
			payload.BID = *step.Object2ID
		}
	case "move_group":
		if step.Object1ID != nil {
			payload.AID = *step.Object1ID
		}
		if step.Object2ID != nil {
			payload.BID = *step.Object2ID
		}
		warnings = append(warnings, fmt.Sprintf("scenario step %s has type move_group; legacy CommandPayload has no dedicated group-move fields", step.StepID))
	default:
		if step.Object1ID != nil {
			payload.AID = *step.Object1ID
		}
		if step.Object2ID != nil {
			payload.BID = *step.Object2ID
		}
	}

	jsonWarnings := mergeLegacyPayloadJSON(&payload, step)
	warnings = append(warnings, jsonWarnings...)
	return payload, warnings
}

func mergeLegacyPayloadJSON(payload *CommandPayload, step normalized.ScenarioStep) []string {
	if len(step.PayloadJSON) == 0 {
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal(step.PayloadJSON, &raw); err != nil {
		return []string{fmt.Sprintf("scenario step %s has payload_json that could not be parsed during legacy projection", step.StepID)}
	}

	recognized := map[string]struct{}{
		"locoId":       {},
		"fromPathId":   {},
		"fromIndex":    {},
		"toPathId":     {},
		"toIndex":      {},
		"targetPathId": {},
		"targetIndex":  {},
		"aId":          {},
		"bId":          {},
	}

	if payload.LocoID == "" {
		payload.LocoID = readString(raw, "locoId")
	}
	if payload.FromPathID == "" {
		payload.FromPathID = readString(raw, "fromPathId")
	}
	if payload.FromIndex == 0 {
		payload.FromIndex = readInt(raw, "fromIndex")
	}
	if payload.ToPathID == "" {
		payload.ToPathID = readString(raw, "toPathId")
	}
	if payload.ToIndex == 0 {
		payload.ToIndex = readInt(raw, "toIndex")
	}
	if payload.TargetPathID == "" {
		payload.TargetPathID = readString(raw, "targetPathId")
	}
	if payload.TargetIndex == 0 {
		payload.TargetIndex = readInt(raw, "targetIndex")
	}
	if payload.AID == "" {
		payload.AID = readString(raw, "aId")
	}
	if payload.BID == "" {
		payload.BID = readString(raw, "bId")
	}

	extraKeys := make([]string, 0)
	for key := range raw {
		if _, ok := recognized[key]; !ok {
			extraKeys = append(extraKeys, key)
		}
	}
	sort.Strings(extraKeys)
	if len(extraKeys) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("scenario step %s payload_json contains keys not representable in legacy CommandPayload: %s", step.StepID, strings.Join(extraKeys, ", "))}
}

func readString(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func readInt(raw map[string]any, key string) int {
	value, ok := raw[key]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func dedupeWarnings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
