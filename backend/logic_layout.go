package main

import (
	"errors"
	"fmt"
	"time"

	"trains/backend/normalized"
)

func applyScenarioStep(state RuntimeState, step normalized.ScenarioStep) (RuntimeState, string, error) {
	switch step.StepType {
	case "move_loco":
		if step.Object1ID == nil || step.ToTrackID == nil || step.ToIndex == nil {
			return state, "", errors.New("move_loco requires object1_id, to_track_id and to_index")
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
			return state, "", err
		}
		if len(plan.Timeline) == 0 {
			return state, "", errors.New("movement timeline is empty")
		}
		lastStep := plan.Timeline[len(plan.Timeline)-1]
		posByID := map[string]Position{}
		for _, p := range lastStep {
			posByID[p.ID] = p
		}
		nextVehicles := make([]Vehicle, 0, len(state.Vehicles))
		for _, v := range state.Vehicles {
			pos, ok := posByID[v.ID]
			if !ok {
				nextVehicles = append(nextVehicles, v)
				continue
			}
			nextVehicles = append(nextVehicles, Vehicle{
				ID:        v.ID,
				Type:      v.Type,
				Code:      v.Code,
				Color:     v.Color,
				PathID:    v.PathID,
				PathIndex: v.PathIndex,
				X:         pos.X,
				Y:         pos.Y,
			})
		}
		state.Vehicles = finalizeRuntimeState(RuntimeState{
			Segments:  state.Segments,
			Vehicles:  nextVehicles,
			Couplings: state.Couplings,
		}, 32).Vehicles
		return state, "move applied", nil

	case "couple":
		if step.Object1ID == nil || step.Object2ID == nil {
			return state, "", errors.New("couple requires object1_id and object2_id")
		}
		next, _, err := applyLayoutOperation(LayoutOperationRequest{
			GridSize: 32,
			State:    state,
			Action:   "couple",
			SelectedVehicleIDs: []string{
				*step.Object1ID,
				*step.Object2ID,
			},
		})
		if err != nil {
			return state, "", err
		}
		return next, "couple applied", nil

	case "decouple":
		if step.Object1ID == nil || step.Object2ID == nil {
			return state, "", errors.New("decouple requires object1_id and object2_id")
		}
		next, _, err := applyLayoutOperation(LayoutOperationRequest{
			GridSize: 32,
			State:    state,
			Action:   "decouple",
			SelectedVehicleIDs: []string{
				*step.Object1ID,
				*step.Object2ID,
			},
		})
		if err != nil {
			return state, "", err
		}
		return next, "decouple applied", nil

	default:
		return state, "", errors.New("unsupported scenario step type")
	}
}

func applyLayoutOperation(req LayoutOperationRequest) (RuntimeState, string, error) {
	state := req.State

	switch req.Action {
	case "add_segment":
		if req.From == nil || req.To == nil {
			return state, "", errors.New("from/to are required")
		}
		if req.From.X == req.To.X && req.From.Y == req.To.Y {
			return state, "", errors.New("segment length must be non-zero")
		}
		state.Segments = append(state.Segments, Segment{
			ID:   nextPathID(state.Segments),
			Type: "normal",
			From: *req.From,
			To:   *req.To,
		})
		return state, "", nil

	case "delete_segments":
		toDelete := map[string]struct{}{}
		for _, id := range req.IDs {
			toDelete[id] = struct{}{}
		}
		if len(toDelete) == 0 {
			return state, "", nil
		}
		filtered := make([]Segment, 0, len(state.Segments))
		for _, segment := range state.Segments {
			if _, drop := toDelete[segment.ID]; drop {
				continue
			}
			filtered = append(filtered, segment)
		}
		state.Segments = filtered
		if len(state.Segments) == 0 {
			state.Vehicles = nil
			state.Couplings = nil
			return state, "", nil
		}
		resolved, err := resolveVehicles(ResolveVehiclesRequest{
			GridSize:        req.GridSize,
			Segments:        state.Segments,
			Vehicles:        state.Vehicles,
			Couplings:       state.Couplings,
			StrictCouplings: false,
		})
		if err != nil {
			state.Vehicles = nil
			state.Couplings = nil
			return state, "vehicles reset: rails changed", nil
		}
		state.Vehicles = resolved
		return state, "", nil

	case "delete_vehicles":
		toDelete := map[string]struct{}{}
		for _, id := range req.IDs {
			toDelete[id] = struct{}{}
		}
		if len(toDelete) == 0 {
			return state, "", nil
		}
		filteredVehicles := make([]Vehicle, 0, len(state.Vehicles))
		for _, v := range state.Vehicles {
			if _, drop := toDelete[v.ID]; drop {
				continue
			}
			filteredVehicles = append(filteredVehicles, v)
		}
		filteredCouplings := make([]Coupling, 0, len(state.Couplings))
		for _, c := range state.Couplings {
			if _, drop := toDelete[c.A]; drop {
				continue
			}
			if _, drop := toDelete[c.B]; drop {
				continue
			}
			filteredCouplings = append(filteredCouplings, c)
		}
		state.Vehicles = filteredVehicles
		state.Couplings = filteredCouplings
		return state, "", nil

	case "clear":
		return RuntimeState{}, "", nil

	case "place_vehicle":
		resp, err := placeVehicleInternal(PlaceVehicleRequest{
			GridSize:     req.GridSize,
			Segments:     state.Segments,
			Vehicles:     state.Vehicles,
			VehicleType:  req.VehicleType,
			TargetPathID: req.TargetPathID,
			TargetIndex:  req.TargetIndex,
		})
		if err != nil {
			return state, "", err
		}
		state.Vehicles = append(state.Vehicles, *resp.Vehicle)
		return state, "", nil

	case "resolve_vehicles":
		resolved, err := resolveVehicles(ResolveVehiclesRequest{
			GridSize:        req.GridSize,
			Segments:        state.Segments,
			Vehicles:        state.Vehicles,
			Couplings:       state.Couplings,
			MovedVehicleIDs: req.MovedVehicleIDs,
			StrictCouplings: req.StrictCouplings,
		})
		if err != nil {
			return state, "", err
		}
		state.Vehicles = resolved
		return state, "", nil

	case "couple":
		if len(req.SelectedVehicleIDs) < 2 {
			return state, "", errors.New("select two vehicles")
		}
		validateResp, err := validateCouplingInternal(ValidateCouplingRequest{
			GridSize:           req.GridSize,
			Segments:           state.Segments,
			Vehicles:           state.Vehicles,
			Couplings:          state.Couplings,
			SelectedVehicleIDs: req.SelectedVehicleIDs,
		})
		if err != nil {
			return state, "", err
		}
		if !validateResp.OK {
			return state, "", errors.New(validateResp.Message)
		}
		a := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-2]
		b := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-1]
		state.Couplings = append(state.Couplings, Coupling{
			ID: fmt.Sprintf("%d", time.Now().UnixNano()),
			A:  a,
			B:  b,
		})
		return state, "", nil

	case "decouple":
		if len(req.SelectedVehicleIDs) < 2 {
			return state, "", errors.New("select two vehicles")
		}
		a := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-2]
		b := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-1]
		key := pairKey(a, b)
		filtered := make([]Coupling, 0, len(state.Couplings))
		for _, coupling := range state.Couplings {
			if pairKey(coupling.A, coupling.B) == key {
				continue
			}
			filtered = append(filtered, coupling)
		}
		state.Couplings = filtered
		return state, "", nil

	default:
		return state, "", errors.New("unknown action")
	}
}

func placeVehicleInternal(req PlaceVehicleRequest) (PlaceVehicleResponse, error) {
	if req.VehicleType != "wagon" && req.VehicleType != "locomotive" {
		return PlaceVehicleResponse{}, errors.New("Vehicle type must be wagon or locomotive.")
	}

	pathSlots := collectPathSlots(req.Segments, req.GridSize)
	target, ok := findPathSlot(pathSlots, req.TargetPathID, req.TargetIndex)
	if !ok {
		return PlaceVehicleResponse{}, errors.New("Target slot is not on rail.")
	}

	occupied := map[string]struct{}{}
	for _, v := range req.Vehicles {
		if v.PathID != "" {
			occupied[pathSlotKey(v.PathID, v.PathIndex)] = struct{}{}
			continue
		}
		occupied[slotID(v.X, v.Y)] = struct{}{}
	}
	if _, exists := occupied[pathSlotKey(target.PathID, target.Index)]; exists {
		return PlaceVehicleResponse{}, errors.New("Target slot is occupied.")
	}

	vehicle := Vehicle{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:      req.VehicleType,
		Code:      nextVehicleCode(req.Vehicles, req.VehicleType),
		PathID:    target.PathID,
		PathIndex: target.Index,
		X:         target.X,
		Y:         target.Y,
	}
	return PlaceVehicleResponse{
		OK:      true,
		Vehicle: &vehicle,
	}, nil
}

func validateCouplingInternal(req ValidateCouplingRequest) (ValidateCouplingResponse, error) {
	if len(req.SelectedVehicleIDs) < 2 {
		return ValidateCouplingResponse{OK: false, Message: "Select two vehicles."}, nil
	}

	pathSlots := collectPathSlots(req.Segments, req.GridSize)
	vehicles := make([]Vehicle, 0, len(req.Vehicles))
	for _, v := range req.Vehicles {
		vehicles = append(vehicles, normalizeVehicleToPath(v, pathSlots))
	}

	a := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-2]
	b := req.SelectedVehicleIDs[len(req.SelectedVehicleIDs)-1]
	if a == b {
		return ValidateCouplingResponse{OK: false, Message: "Cannot couple a vehicle with itself."}, nil
	}

	vehicleByID := make(map[string]Vehicle, len(req.Vehicles))
	for _, v := range vehicles {
		vehicleByID[v.ID] = v
	}

	va, okA := vehicleByID[a]
	vb, okB := vehicleByID[b]
	if !okA || !okB {
		return ValidateCouplingResponse{OK: false, Message: "Selected vehicles were not found."}, nil
	}

	existing := make(map[string]struct{}, len(req.Couplings))
	for _, c := range req.Couplings {
		existing[pairKey(c.A, c.B)] = struct{}{}
	}
	if _, exists := existing[pairKey(a, b)]; exists {
		return ValidateCouplingResponse{OK: false, Message: "These vehicles are already coupled."}, nil
	}

	adjacentPairs := buildAdjacentSlotPairs(req.Segments, req.GridSize)
	slotA := slotID(va.X, va.Y)
	slotB := slotID(vb.X, vb.Y)
	if _, ok := adjacentPairs[pairKey(slotA, slotB)]; !ok {
		return ValidateCouplingResponse{OK: false, Message: "Coupling is allowed only for adjacent slots."}, nil
	}

	return ValidateCouplingResponse{OK: true}, nil
}

func resolveVehicles(req ResolveVehiclesRequest) ([]Vehicle, error) {
	if len(req.Vehicles) == 0 {
		return req.Vehicles, nil
	}
	pathSlots := collectPathSlots(req.Segments, req.GridSize)
	if len(pathSlots) == 0 {
		return nil, errors.New("No rail slots available.")
	}

	movedSet := map[string]struct{}{}
	for _, id := range req.MovedVehicleIDs {
		movedSet[id] = struct{}{}
	}
	if len(movedSet) == 0 {
		for _, v := range req.Vehicles {
			movedSet[v.ID] = struct{}{}
		}
	}

	blocked := map[string]struct{}{}
	for _, v := range req.Vehicles {
		if _, moved := movedSet[v.ID]; moved {
			continue
		}
		if v.PathID != "" {
			blocked[pathSlotKey(v.PathID, v.PathIndex)] = struct{}{}
			continue
		}
		blocked[slotID(v.X, v.Y)] = struct{}{}
	}

	next := make([]Vehicle, 0, len(req.Vehicles))
	nextByID := map[string]Vehicle{}

	for _, v := range req.Vehicles {
		if _, moved := movedSet[v.ID]; !moved {
			normalized := normalizeVehicleToPath(v, pathSlots)
			next = append(next, normalized)
			nextByID[v.ID] = normalized
			continue
		}

		nearest := findNearestPathSlot(Point{X: v.X, Y: v.Y}, pathSlots, blocked)
		if nearest == nil {
			return nil, errors.New("Cannot place moved vehicles on free rail slots.")
		}

		resolved := Vehicle{
			ID:        v.ID,
			Type:      v.Type,
			Code:      v.Code,
			Color:     v.Color,
			PathID:    nearest.PathID,
			PathIndex: nearest.Index,
			X:         nearest.X,
			Y:         nearest.Y,
		}
		next = append(next, resolved)
		nextByID[v.ID] = resolved
		blocked[pathSlotKey(nearest.PathID, nearest.Index)] = struct{}{}
	}

	if req.StrictCouplings {
		pathAdjPairs := buildAdjacentPathSlotPairs(req.Segments, req.GridSize)
		for _, c := range req.Couplings {
			va, okA := nextByID[c.A]
			vb, okB := nextByID[c.B]
			if !okA || !okB {
				continue
			}
			if _, ok := pathAdjPairs[pathSlotPairKey(va.PathID, va.PathIndex, vb.PathID, vb.PathIndex)]; !ok {
				return nil, errors.New("Coupled vehicles must stay on adjacent slots.")
			}
		}
	}

	return next, nil
}

func normalizeVehicleToPath(vehicle Vehicle, pathSlots []PathSlot) Vehicle {
	if vehicle.PathID != "" && vehicle.PathIndex >= 0 {
		if slot, ok := findPathSlot(pathSlots, vehicle.PathID, vehicle.PathIndex); ok {
			vehicle.X = slot.X
			vehicle.Y = slot.Y
			return vehicle
		}
	}
	nearest := findNearestPathSlot(Point{X: vehicle.X, Y: vehicle.Y}, pathSlots, nil)
	if nearest == nil {
		return vehicle
	}
	vehicle.PathID = nearest.PathID
	vehicle.PathIndex = nearest.Index
	vehicle.X = nearest.X
	vehicle.Y = nearest.Y
	return vehicle
}

func finalizeRuntimeState(state RuntimeState, gridSize float64) RuntimeState {
	pathSlots := collectPathSlots(state.Segments, gridSize)
	normalized := make([]Vehicle, 0, len(state.Vehicles))
	for _, v := range state.Vehicles {
		normalized = append(normalized, normalizeVehicleToPath(v, pathSlots))
	}
	state.Vehicles = normalized
	state.Paths = buildPathStates(state.Segments, state.Vehicles, gridSize)
	return state
}

func buildPathStates(segments []Segment, vehicles []Vehicle, gridSize float64) []RuntimePathState {
	vehicleByPath := map[string][]string{}
	for _, v := range vehicles {
		if v.PathID == "" {
			continue
		}
		vehicleByPath[v.PathID] = append(vehicleByPath[v.PathID], v.ID)
	}

	neighbors := buildPathAdjacency(segments)
	states := make([]RuntimePathState, 0, len(segments))
	for _, segment := range segments {
		capacity := len(getSegmentSlots(segment, gridSize))
		states = append(states, RuntimePathState{
			ID:         segment.ID,
			Capacity:   capacity,
			VehicleIDs: vehicleByPath[segment.ID],
			Neighbors:  neighbors[segment.ID],
		})
	}
	return states
}

func buildPathAdjacency(segments []Segment) map[string][]string {
	endpointMap := map[string][]string{}
	for _, segment := range segments {
		fromKey := slotID(segment.From.X, segment.From.Y)
		toKey := slotID(segment.To.X, segment.To.Y)
		endpointMap[fromKey] = append(endpointMap[fromKey], segment.ID)
		endpointMap[toKey] = append(endpointMap[toKey], segment.ID)
	}

	adj := map[string]map[string]struct{}{}
	for _, ids := range endpointMap {
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				a := ids[i]
				b := ids[j]
				if _, ok := adj[a]; !ok {
					adj[a] = map[string]struct{}{}
				}
				if _, ok := adj[b]; !ok {
					adj[b] = map[string]struct{}{}
				}
				adj[a][b] = struct{}{}
				adj[b][a] = struct{}{}
			}
		}
	}

	result := map[string][]string{}
	for id, neighbors := range adj {
		for neighbor := range neighbors {
			result[id] = append(result[id], neighbor)
		}
	}
	return result
}

