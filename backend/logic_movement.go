package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
)

func buildMovementPlan(req PlanMovementRequest) (PlanMovementResponse, error) {
	fail := func(message string) (PlanMovementResponse, error) {
		return PlanMovementResponse{}, errors.New(message)
	}

	if req.SelectedLocomotiveID == "" {
		return fail("Select locomotive.")
	}
	if strings.TrimSpace(req.TargetPathID) == "" {
		return fail("Select target path.")
	}

	trackConnections := req.TrackConnections
	if len(trackConnections) == 0 {
		trackConnections = buildTrackConnectionsFromSegments(req.Segments)
	}

	pathSlots := collectPathSlotsWithConnections(req.Segments, req.GridSize, trackConnections)
	if len(pathSlots) == 0 {
		return fail("No rail slots available.")
	}

	targetSegmentFound := false
	targetCapacity := 0
	for _, segment := range req.Segments {
		if segment.ID != req.TargetPathID {
			continue
		}
		targetSegmentFound = true
		targetCapacity = len(getSegmentSlots(segment, req.GridSize))
		break
	}
	if !targetSegmentFound {
		return fail(fmt.Sprintf("Target path was not found: %s.", req.TargetPathID))
	}
	if req.TargetIndex < 0 || req.TargetIndex >= targetCapacity {
		return fail(fmt.Sprintf(
			"Target index is outside path capacity: track_id=%s index=%d capacity=%d.",
			req.TargetPathID,
			req.TargetIndex,
			targetCapacity,
		))
	}

	if _, ok := findPathSlot(pathSlots, req.TargetPathID, req.TargetIndex); !ok {
		return fail(fmt.Sprintf(
			"Target slot is unavailable: track_id=%s index=%d.",
			req.TargetPathID,
			req.TargetIndex,
		))
	}

	normalizedVehicles := make([]Vehicle, 0, len(req.Vehicles))
	vehicleByID := make(map[string]Vehicle, len(req.Vehicles))
	for _, v := range req.Vehicles {
		nv := normalizeVehicleToPath(v, pathSlots)
		normalizedVehicles = append(normalizedVehicles, nv)
		vehicleByID[nv.ID] = nv
	}

	locomotive, exists := vehicleByID[req.SelectedLocomotiveID]
	if !exists || locomotive.Type != "locomotive" {
		return fail("Selected unit is not a locomotive.")
	}

	slots := collectRailSlotsWithConnections(req.Segments, req.GridSize, trackConnections)
	slotByID := make(map[string]Slot, len(slots))
	for _, s := range slots {
		slotByID[s.ID] = s
	}

	slotAdj := buildSlotAdjacencyWithConnections(req.Segments, req.GridSize, trackConnections)
	trainOrder, err := buildTrainOrder(req.SelectedLocomotiveID, normalizedVehicles, req.Couplings)
	if err != nil {
		return fail(err.Error())
	}

	currentSlotByVehicleID := make(map[string]string, len(trainOrder))
	for _, id := range trainOrder {
		v, ok := vehicleByID[id]
		if !ok {
			return fail("Train contains unknown vehicle.")
		}
		nearest := findNearestSlot(Point{X: v.X, Y: v.Y}, slots)
		if nearest == nil {
			return fail("No rail slots available.")
		}
		currentSlotByVehicleID[id] = nearest.ID
	}

	trail := reverseStrings(trainOrder)
	initialSlots := make([]string, 0, len(trail))
	for _, id := range trail {
		initialSlots = append(initialSlots, currentSlotByVehicleID[id])
	}
	for i := 0; i < len(initialSlots)-1; i++ {
		a := initialSlots[i]
		b := initialSlots[i+1]
		if _, ok := slotAdj[a][b]; !ok {
			return fail("Coupled train must stand on adjacent slots.")
		}
	}

	staticOccupied := make(map[string]struct{})
	occupiedTrackIDs := make(map[string]struct{})
	trainSet := make(map[string]struct{}, len(trainOrder))
	for _, id := range trainOrder {
		trainSet[id] = struct{}{}
	}
	for _, v := range normalizedVehicles {
		if _, ok := trainSet[v.ID]; ok {
			continue
		}
		staticOccupied[slotID(v.X, v.Y)] = struct{}{}
		if v.PathID != "" {
			occupiedTrackIDs[v.PathID] = struct{}{}
		}
	}

	trackPath, trackRoute := dijkstraTrackPathAvoidingTracks(
		trackConnections,
		locomotive.PathID,
		req.TargetPathID,
		occupiedTrackIDs,
	)
	if len(trackPath) == 0 {
		trackPath, trackRoute = dijkstraTrackPath(trackConnections, locomotive.PathID, req.TargetPathID)
		if len(trackPath) > 0 && len(occupiedTrackIDs) > 0 {
			log.Printf(
				"[movement] fallback route uses occupied branch: start=%s:%d target=%s:%d occupied_tracks=%v route=%v",
				locomotive.PathID,
				locomotive.PathIndex,
				req.TargetPathID,
				req.TargetIndex,
				sortedTrackIDs(occupiedTrackIDs),
				trackPath,
			)
		}
	}

	targetIndex := req.TargetIndex
	routeSelectionReason := ""
	if len(trainOrder) == 1 {
		occupiedTargetIndices := occupiedIndicesByTrack(normalizedVehicles, trainSet, req.TargetPathID)
		selectedTrackPath, selectedTrackRoute, adjustedTargetIndex, adjustReason, adjustErr := chooseSingleLocomotiveRouteAndTarget(
			locomotive,
			req.Segments,
			trackConnections,
			occupiedTrackIDs,
			req.TargetPathID,
			targetIndex,
			targetCapacity,
			occupiedTargetIndices,
			trackPath,
			trackRoute,
			req.GridSize,
			staticOccupied,
		)
		if adjustErr != nil {
			log.Printf(
				"[movement] target track rejected: track=%s requested_index=%d occupied_indices=%v reason=%v",
				req.TargetPathID,
				req.TargetIndex,
				occupiedTargetIndices,
				adjustErr,
			)
			return fail(adjustErr.Error())
		}
		trackPath = selectedTrackPath
		trackRoute = selectedTrackRoute
		if adjustedTargetIndex != targetIndex {
			log.Printf(
				"[movement] target track adjusted: track=%s requested_index=%d occupied_indices=%v chosen_index=%d route=%v reason=%s",
				req.TargetPathID,
				req.TargetIndex,
				occupiedTargetIndices,
				adjustedTargetIndex,
				trackPath,
				adjustReason,
			)
			targetIndex = adjustedTargetIndex
			routeSelectionReason = adjustReason
		} else if adjustReason != "" {
			routeSelectionReason = adjustReason
			log.Printf(
				"[movement] target track route adjusted: track=%s requested_index=%d occupied_indices=%v route=%v reason=%s",
				req.TargetPathID,
				req.TargetIndex,
				occupiedTargetIndices,
				trackPath,
				adjustReason,
			)
		}
	}

	path, routeSlotByID, pathErr := buildSlotPathFromTrackRoute(
		req.Segments,
		trackConnections,
		locomotive.PathID,
		locomotive.PathIndex,
		req.TargetPathID,
		targetIndex,
		trackPath,
		trackRoute,
		req.GridSize,
	)
	for id, slot := range routeSlotByID {
		slotByID[id] = slot
	}
	if len(path) < 2 {
		log.Printf(
			"[movement] path search failed: start=%s:%d target=%s:%d occupied_tracks=%v route=%v err=%v",
			locomotive.PathID,
			locomotive.PathIndex,
			req.TargetPathID,
			targetIndex,
			sortedTrackIDs(occupiedTrackIDs),
			trackPath,
			pathErr,
		)
		return fail("Path was not found.")
	}

	currentLocoToTail := make([]string, 0, len(trainOrder))
	for _, id := range trainOrder {
		currentLocoToTail = append(currentLocoToTail, currentSlotByVehicleID[id])
	}

	if len(trainOrder) > 1 {
		twoPhaseTimeline, reversalInfo, ok := tryBuildOuterPulloutTimeline(
			req.Segments,
			req.GridSize,
			trackConnections,
			normalizedVehicles,
			vehicleByID,
			trainOrder,
			req.SelectedLocomotiveID,
			req.TargetPathID,
			targetIndex,
			slotAdj,
			slotByID,
			staticOccupied,
		)
		if ok {
			log.Printf(
				"[movement] outer pull-out consist reversal: start=%s:%d target=%s:%d outer_track=%s reversal_slot=%s reason=%s",
				locomotive.PathID,
				locomotive.PathIndex,
				req.TargetPathID,
				targetIndex,
				reversalInfo.OuterTrackID,
				reversalInfo.ReversalSlotID,
				reversalInfo.Reason,
			)
			return PlanMovementResponse{
				OK:          true,
				Message:     "Movement started.",
				Timeline:    twoPhaseTimeline,
				CellsPassed: len(twoPhaseTimeline),
			}, nil
		}
	}

	isBackwardPush := len(trainOrder) > 1 && len(path) > 1 && path[1] == currentLocoToTail[1]
	drivingPath := path
	if isBackwardPush && len(trainOrder) > 1 {
		extended, extErr := extendPathForBackwardPush(path, slotAdj, slotByID, len(trainOrder)-1)
		if extErr != nil {
			return fail(extErr.Error())
		}
		drivingPath = extended
	}

	maxSteps := len(path) - 1
	if maxSteps < 1 {
		return fail("Not enough path length.")
	}

	timeline := make([][]Position, 0, maxSteps)
	for step := 1; step <= maxSteps; step++ {
		stepPositions := make([]Position, 0, len(trainOrder))
		used := make(map[string]struct{})
		valid := true

		for i := 0; i < len(trainOrder); i++ {
			var slotKey string
			if isBackwardPush {
				idx := step + i
				if idx >= len(drivingPath) {
					valid = false
					break
				}
				slotKey = drivingPath[idx]
			} else {
				historyIndex := step - i
				if historyIndex > 0 {
					if historyIndex >= len(path) {
						valid = false
						break
					}
					slotKey = path[historyIndex]
				} else {
					idx := -historyIndex
					if idx >= len(currentLocoToTail) {
						valid = false
						break
					}
					slotKey = currentLocoToTail[idx]
				}
			}

			slot, ok := slotByID[slotKey]
			if !ok {
				valid = false
				break
			}
			if _, blocked := staticOccupied[slotKey]; blocked {
				valid = false
				break
			}
			if _, duplicated := used[slotKey]; duplicated {
				valid = false
				break
			}

			used[slotKey] = struct{}{}
			stepPositions = append(stepPositions, Position{
				ID: trainOrder[i],
				X:  slot.X,
				Y:  slot.Y,
			})
		}

		if !valid {
			log.Printf(
				"[movement] route rejected by occupancy: start=%s:%d target=%s:%d route=%v occupied_slots=%v occupied_tracks=%v",
				locomotive.PathID,
				locomotive.PathIndex,
				req.TargetPathID,
				targetIndex,
				trackPath,
				keysOfMap(staticOccupied),
				sortedTrackIDs(occupiedTrackIDs),
			)
			if routeSelectionReason != "" {
				log.Printf("[movement] last target-track decision: %s", routeSelectionReason)
			}
			return fail("Movement is blocked: not enough free slots.")
		}

		timeline = append(timeline, stepPositions)
	}

	return PlanMovementResponse{
		OK:          true,
		Message:     "Movement started.",
		Timeline:    timeline,
		CellsPassed: len(timeline),
	}, nil
}

func sortedTrackIDs(trackIDs map[string]struct{}) []string {
	result := make([]string, 0, len(trackIDs))
	for trackID := range trackIDs {
		result = append(result, trackID)
	}
	sort.Strings(result)
	return result
}

func occupiedIndicesByTrack(vehicles []Vehicle, excludedVehicleIDs map[string]struct{}, trackID string) []int {
	indices := make([]int, 0)
	for _, vehicle := range vehicles {
		if _, excluded := excludedVehicleIDs[vehicle.ID]; excluded {
			continue
		}
		if vehicle.PathID != trackID {
			continue
		}
		indices = append(indices, vehicle.PathIndex)
	}
	sort.Ints(indices)
	return indices
}

func chooseSingleLocomotiveRouteAndTarget(
	locomotive Vehicle,
	segments []Segment,
	trackConnections []MovementTrackConnection,
	occupiedTrackIDs map[string]struct{},
	targetTrackID string,
	requestedIndex int,
	targetCapacity int,
	occupiedIndices []int,
	defaultTrackPath []string,
	defaultTrackRoute []trackRouteEdge,
	gridSize float64,
	staticOccupied map[string]struct{},
) ([]string, []trackRouteEdge, int, string, error) {
	type routeCandidate struct {
		TrackPath    []string
		TrackRoute   []trackRouteEdge
		TargetIndex  int
		Reason       string
		Distance     float64
		UsesFallback bool
	}

	candidates := make([]routeCandidate, 0, 6)
	appendCandidate := func(trackPath []string, trackRoute []trackRouteEdge, reason string, usesFallback bool) {
		if len(trackPath) == 0 {
			return
		}
		tryAppendCandidate := func(candidateTrackPath []string, candidateTrackRoute []trackRouteEdge, candidateIndex int, candidateReason string) bool {
			path, _, err := buildSlotPathFromTrackRoute(
				segments,
				trackConnections,
				locomotive.PathID,
				locomotive.PathIndex,
				targetTrackID,
				candidateIndex,
				candidateTrackPath,
				candidateTrackRoute,
				gridSize,
			)
			if err != nil || len(path) < 2 {
				return false
			}
			if !isSingleLocoPathClear(path, staticOccupied) {
				return false
			}
			candidates = append(candidates, routeCandidate{
				TrackPath:    append([]string{}, candidateTrackPath...),
				TrackRoute:   append([]trackRouteEdge{}, candidateTrackRoute...),
				TargetIndex:  candidateIndex,
				Reason:       candidateReason,
				Distance:     math.Abs(float64(candidateIndex - requestedIndex)),
				UsesFallback: usesFallback,
			})
			log.Printf(
				"[movement] candidate route accepted: start=%s:%d target=%s:%d candidate_index=%d route=%v reason=%s",
				locomotive.PathID,
				locomotive.PathIndex,
				targetTrackID,
				requestedIndex,
				candidateIndex,
				candidateTrackPath,
				candidateReason,
			)
			return true
		}

		for _, candidateIndex := range sortedFreeTargetIndices(requestedIndex, targetCapacity, occupiedIndices) {
			finalReason := reason
			if candidateIndex != requestedIndex {
				if finalReason != "" {
					finalReason += "; "
				}
				finalReason += "requested slot is occupied or blocked on approach, using nearest reachable free slot"
			}
			tryAppendCandidate(trackPath, trackRoute, candidateIndex, finalReason)
		}

		if locomotive.PathID != targetTrackID {
			for _, goalSide := range []string{"start", "end"} {
				loopTrackPath, loopTrackRoute := dijkstraTrackLoopPathWithGoalSideAvoidingTracks(
					trackConnections,
					targetTrackID,
					goalSide,
					occupiedTrackIDs,
				)
				if len(loopTrackPath) == 0 {
					continue
				}
				combinedTrackPath := append(append([]string{}, trackPath...), loopTrackPath[1:]...)
				combinedTrackRoute := append(append([]trackRouteEdge{}, trackRoute...), loopTrackRoute...)
				combinedReason := reason
				if combinedReason != "" {
					combinedReason += "; "
				}
				combinedReason += fmt.Sprintf("extended with non-trivial loop on target track via %s", goalSide)
				tryAppendCandidate(combinedTrackPath, combinedTrackRoute, requestedIndex, combinedReason)
			}
		}
	}

	appendCandidate(defaultTrackPath, defaultTrackRoute, "", false)
	if locomotive.PathID == targetTrackID {
		for _, side := range []string{"start", "end"} {
			loopPath, loopRoute := dijkstraTrackLoopPathWithGoalSideAvoidingTracks(
				trackConnections,
				locomotive.PathID,
				side,
				occupiedTrackIDs,
			)
			appendCandidate(loopPath, loopRoute, fmt.Sprintf("non-trivial loop to re-enter track via %s", side), false)
			if len(loopPath) == 0 {
				fallbackLoopPath, fallbackLoopRoute := dijkstraTrackLoopPathWithGoalSide(
					trackConnections,
					locomotive.PathID,
					side,
				)
				appendCandidate(fallbackLoopPath, fallbackLoopRoute, fmt.Sprintf("fallback non-trivial loop to re-enter track via %s", side), true)
			}
		}
	}
	if locomotive.PathID != targetTrackID {
		for _, side := range []string{"start", "end"} {
			sidePath, sideRoute := dijkstraTrackPathWithGoalSideAvoidingTracks(
				trackConnections,
				locomotive.PathID,
				targetTrackID,
				side,
				occupiedTrackIDs,
			)
			appendCandidate(sidePath, sideRoute, fmt.Sprintf("preferred entry side %s", side), false)
			if len(sidePath) == 0 {
				fallbackPath, fallbackRoute := dijkstraTrackPathWithGoalSide(
					trackConnections,
					locomotive.PathID,
					targetTrackID,
					side,
				)
				appendCandidate(fallbackPath, fallbackRoute, fmt.Sprintf("fallback entry side %s", side), true)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, nil, 0, "", fmt.Errorf(
			"Target track is blocked: no reachable free slot on track_id=%s requested_index=%d occupied_indices=%v.",
			targetTrackID,
			requestedIndex,
			occupiedIndices,
		)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Distance != candidates[j].Distance {
			return candidates[i].Distance < candidates[j].Distance
		}
		if len(candidates[i].TrackPath) != len(candidates[j].TrackPath) {
			return len(candidates[i].TrackPath) < len(candidates[j].TrackPath)
		}
		if candidates[i].UsesFallback != candidates[j].UsesFallback {
			return !candidates[i].UsesFallback && candidates[j].UsesFallback
		}
		return strings.Join(candidates[i].TrackPath, "|") < strings.Join(candidates[j].TrackPath, "|")
	})

	best := candidates[0]
	return best.TrackPath, best.TrackRoute, best.TargetIndex, best.Reason, nil
}

func sortedFreeTargetIndices(requestedIndex, targetCapacity int, occupiedIndices []int) []int {
	occupiedSet := make(map[int]struct{}, len(occupiedIndices))
	for _, occupiedIndex := range occupiedIndices {
		occupiedSet[occupiedIndex] = struct{}{}
	}
	indices := make([]int, 0, targetCapacity)
	for idx := 0; idx < targetCapacity; idx++ {
		if _, occupied := occupiedSet[idx]; occupied {
			continue
		}
		indices = append(indices, idx)
	}
	sort.SliceStable(indices, func(i, j int) bool {
		leftDistance := math.Abs(float64(indices[i] - requestedIndex))
		rightDistance := math.Abs(float64(indices[j] - requestedIndex))
		if leftDistance != rightDistance {
			return leftDistance < rightDistance
		}
		return indices[i] < indices[j]
	})
	return indices
}

func isSingleLocoPathClear(path []string, staticOccupied map[string]struct{}) bool {
	for i := 1; i < len(path); i++ {
		if _, blocked := staticOccupied[path[i]]; blocked {
			return false
		}
	}
	return true
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func resolveSingleLocomotiveTargetIndex(
	locomotive Vehicle,
	targetTrackID string,
	requestedIndex int,
	targetCapacity int,
	occupiedIndices []int,
	route []trackRouteEdge,
) (int, string, error) {
	if len(occupiedIndices) == 0 {
		return requestedIndex, "", nil
	}

	reachableIndices := make([]int, 0)
	if locomotive.PathID == targetTrackID {
		if requestedIndex >= locomotive.PathIndex {
			limit := targetCapacity - 1
			for _, occupiedIndex := range occupiedIndices {
				if occupiedIndex > locomotive.PathIndex {
					limit = occupiedIndex - 1
					break
				}
			}
			for idx := locomotive.PathIndex + 1; idx <= limit; idx++ {
				reachableIndices = append(reachableIndices, idx)
			}
		} else {
			limit := 0
			for i := len(occupiedIndices) - 1; i >= 0; i-- {
				if occupiedIndices[i] < locomotive.PathIndex {
					limit = occupiedIndices[i] + 1
					break
				}
			}
			for idx := locomotive.PathIndex - 1; idx >= limit; idx-- {
				reachableIndices = append(reachableIndices, idx)
			}
		}
	} else {
		entrySide := "start"
		if len(route) > 0 {
			entrySide = route[len(route)-1].ToSide
		}
		if entrySide == "end" {
			limit := 0
			if len(occupiedIndices) > 0 {
				limit = occupiedIndices[len(occupiedIndices)-1] + 1
			}
			for idx := targetCapacity - 1; idx >= limit; idx-- {
				reachableIndices = append(reachableIndices, idx)
			}
		} else {
			limit := targetCapacity - 1
			if len(occupiedIndices) > 0 {
				limit = occupiedIndices[0] - 1
			}
			for idx := 0; idx <= limit; idx++ {
				reachableIndices = append(reachableIndices, idx)
			}
		}
	}

	if len(reachableIndices) == 0 {
		return 0, "", fmt.Errorf(
			"Target track is blocked: no reachable free slot on track_id=%s requested_index=%d occupied_indices=%v.",
			targetTrackID,
			requestedIndex,
			occupiedIndices,
		)
	}

	bestIndex := reachableIndices[0]
	bestDistance := math.Abs(float64(reachableIndices[0] - requestedIndex))
	for _, candidateIndex := range reachableIndices[1:] {
		distance := math.Abs(float64(candidateIndex - requestedIndex))
		if distance < bestDistance {
			bestIndex = candidateIndex
			bestDistance = distance
		}
	}
	if bestIndex == requestedIndex {
		return requestedIndex, "", nil
	}
	return bestIndex, "requested slot is occupied or lies behind occupied wagons on the target track", nil
}

func keysOfMap(items map[string]struct{}) []string {
	result := make([]string, 0, len(items))
	for key := range items {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

type outerPulloutReversalInfo struct {
	OuterTrackID   string
	ReversalSlotID string
	Reason         string
}

type stationOrientation struct {
	LeftOuterTrackID    string
	RightOuterTrackID   string
	ExternalSideByTrack map[string]string
}

func tryBuildOuterPulloutTimeline(
	segments []Segment,
	gridSize float64,
	trackConnections []MovementTrackConnection,
	vehicles []Vehicle,
	vehicleByID map[string]Vehicle,
	trainOrder []string,
	locomotiveID string,
	targetTrackID string,
	targetIndex int,
	slotAdj map[string]map[string]struct{},
	slotByID map[string]Slot,
	staticOccupied map[string]struct{},
) ([][]Position, outerPulloutReversalInfo, bool) {
	orientation, ok := detectStationOrientation(segments, trackConnections)
	if !ok {
		return nil, outerPulloutReversalInfo{}, false
	}

	locomotive := vehicleByID[locomotiveID]
	if locomotive.PathID == targetTrackID {
		return nil, outerPulloutReversalInfo{}, false
	}
	if locomotive.PathID == orientation.LeftOuterTrackID || locomotive.PathID == orientation.RightOuterTrackID {
		return nil, outerPulloutReversalInfo{}, false
	}
	if targetTrackID == orientation.LeftOuterTrackID || targetTrackID == orientation.RightOuterTrackID {
		return nil, outerPulloutReversalInfo{}, false
	}

	attachedSide, ok := locomotiveAttachedTrackSide(vehicles, vehicleByID, trainOrder, segments)
	if !ok {
		return nil, outerPulloutReversalInfo{}, false
	}

	outerTrackID := orientation.RightOuterTrackID
	if attachedSide == "left" {
		outerTrackID = orientation.LeftOuterTrackID
	}
	externalSide := orientation.ExternalSideByTrack[outerTrackID]
	if externalSide == "" {
		return nil, outerPulloutReversalInfo{}, false
	}
	outerTargetIndex, err := minimalOuterPulloutIndex(segments, gridSize, outerTrackID, externalSide, len(trainOrder))
	if err != nil {
		return nil, outerPulloutReversalInfo{}, false
	}

	currentLocoToTail := make([]string, 0, len(trainOrder))
	for _, id := range trainOrder {
		current := vehicleByID[id]
		currentLocoToTail = append(currentLocoToTail, slotID(current.X, current.Y))
	}

	pullTrackPath, pullTrackRoute := dijkstraTrackPath(trackConnections, locomotive.PathID, outerTrackID)
	if len(pullTrackPath) == 0 {
		return nil, outerPulloutReversalInfo{}, false
	}
	pullPath, pullSlots, err := buildSlotPathFromTrackRoute(
		segments,
		trackConnections,
		locomotive.PathID,
		locomotive.PathIndex,
		outerTrackID,
		outerTargetIndex,
		pullTrackPath,
		pullTrackRoute,
		gridSize,
	)
	if err != nil || len(pullPath) < len(trainOrder)+1 {
		return nil, outerPulloutReversalInfo{}, false
	}
	for id, slot := range pullSlots {
		slotByID[id] = slot
	}

	phaseOneTimeline, phaseOneEndState, ok := simulatePullTimeline(pullPath, currentLocoToTail, trainOrder, slotByID, staticOccupied)
	if !ok {
		return nil, outerPulloutReversalInfo{}, false
	}

	pushTrackPath, pushTrackRoute := dijkstraTrackPath(trackConnections, outerTrackID, targetTrackID)
	if len(pushTrackPath) == 0 {
		return nil, outerPulloutReversalInfo{}, false
	}
	pushPath, pushSlots, err := buildSlotPathFromTrackRoute(
		segments,
		trackConnections,
		outerTrackID,
		outerTargetIndex,
		targetTrackID,
		targetIndex,
		pushTrackPath,
		pushTrackRoute,
		gridSize,
	)
	if err != nil || len(pushPath) < 2 {
		return nil, outerPulloutReversalInfo{}, false
	}
	for id, slot := range pushSlots {
		slotByID[id] = slot
	}
	pushDrivingPath, err := extendPathForBackwardPush(pushPath, slotAdj, slotByID, len(trainOrder)-1)
	if err != nil {
		return nil, outerPulloutReversalInfo{}, false
	}
	if !matchesPushPrefix(phaseOneEndState, pushDrivingPath) {
		return nil, outerPulloutReversalInfo{}, false
	}

	phaseTwoTimeline, ok := simulatePushTimeline(pushPath, pushDrivingPath, trainOrder, slotByID, staticOccupied)
	if !ok {
		return nil, outerPulloutReversalInfo{}, false
	}

	return append(phaseOneTimeline, phaseTwoTimeline...), outerPulloutReversalInfo{
		OuterTrackID:   outerTrackID,
		ReversalSlotID: pullPath[len(pullPath)-1],
		Reason:         "internal-to-internal consist transfer must fully pull out to the station outer track before pushing into the target branch",
	}, true
}

func minimalOuterPulloutIndex(
	segments []Segment,
	gridSize float64,
	trackID string,
	externalSide string,
	trainLength int,
) (int, error) {
	if trainLength < 1 {
		trainLength = 1
	}
	var targetSegment *Segment
	for i := range segments {
		if segments[i].ID == trackID {
			targetSegment = &segments[i]
			break
		}
	}
	if targetSegment == nil {
		return 0, fmt.Errorf("track %s was not found", trackID)
	}
	points := getSegmentSlots(*targetSegment, gridSize)
	if len(points) == 0 {
		return 0, fmt.Errorf("track %s has no slots", trackID)
	}
	maxIndex := len(points) - 1
	switch externalSide {
	case "start":
		idx := maxIndex - trainLength
		if idx < 0 {
			idx = 0
		}
		return idx, nil
	case "end":
		idx := trainLength
		if idx > maxIndex {
			idx = maxIndex
		}
		return idx, nil
	default:
		return 0, fmt.Errorf("unsupported external side %q for track %s", externalSide, trackID)
	}
}

func simulatePullTimeline(
	path []string,
	initialState []string,
	trainOrder []string,
	slotByID map[string]Slot,
	staticOccupied map[string]struct{},
) ([][]Position, []string, bool) {
	state := append([]string{}, initialState...)
	timeline := make([][]Position, 0, len(path)-1)
	for step := 1; step < len(path); step++ {
		next := make([]string, len(state))
		next[0] = path[step]
		for i := 1; i < len(state); i++ {
			next[i] = state[i-1]
		}
		stepPositions, ok := buildStepPositionsFromState(next, trainOrder, slotByID, staticOccupied)
		if !ok {
			return nil, nil, false
		}
		timeline = append(timeline, stepPositions)
		state = next
	}
	return timeline, state, true
}

func simulatePushTimeline(
	path []string,
	drivingPath []string,
	trainOrder []string,
	slotByID map[string]Slot,
	staticOccupied map[string]struct{},
) ([][]Position, bool) {
	timeline := make([][]Position, 0, len(path)-1)
	for step := 1; step < len(path); step++ {
		state := make([]string, len(trainOrder))
		for i := 0; i < len(trainOrder); i++ {
			idx := step + i
			if idx >= len(drivingPath) {
				return nil, false
			}
			state[i] = drivingPath[idx]
		}
		stepPositions, ok := buildStepPositionsFromState(state, trainOrder, slotByID, staticOccupied)
		if !ok {
			return nil, false
		}
		timeline = append(timeline, stepPositions)
	}
	return timeline, true
}

func buildStepPositionsFromState(
	state []string,
	trainOrder []string,
	slotByID map[string]Slot,
	staticOccupied map[string]struct{},
) ([]Position, bool) {
	if len(state) != len(trainOrder) {
		return nil, false
	}
	used := make(map[string]struct{}, len(state))
	stepPositions := make([]Position, 0, len(state))
	for i, slotKey := range state {
		slot, ok := slotByID[slotKey]
		if !ok {
			return nil, false
		}
		if _, blocked := staticOccupied[slotKey]; blocked {
			return nil, false
		}
		if _, duplicated := used[slotKey]; duplicated {
			return nil, false
		}
		used[slotKey] = struct{}{}
		stepPositions = append(stepPositions, Position{
			ID: trainOrder[i],
			X:  slot.X,
			Y:  slot.Y,
		})
	}
	return stepPositions, true
}

func matchesPushPrefix(state []string, drivingPath []string) bool {
	if len(drivingPath) < len(state) {
		return false
	}
	for i := range state {
		if state[i] != drivingPath[i] {
			return false
		}
	}
	return true
}

func detectStationOrientation(segments []Segment, trackConnections []MovementTrackConnection) (stationOrientation, bool) {
	connectedSides := map[string]map[string]int{}
	for _, connection := range trackConnections {
		if connectedSides[connection.Track1ID] == nil {
			connectedSides[connection.Track1ID] = map[string]int{}
		}
		if connectedSides[connection.Track2ID] == nil {
			connectedSides[connection.Track2ID] = map[string]int{}
		}
		connectedSides[connection.Track1ID][connection.Track1Side]++
		connectedSides[connection.Track2ID][connection.Track2Side]++
	}

	type outerCandidate struct {
		TrackID      string
		CenterX      float64
		ExternalSide string
	}
	candidates := make([]outerCandidate, 0, 2)
	for _, segment := range segments {
		startConnected := connectedSides[segment.ID]["start"]
		endConnected := connectedSides[segment.ID]["end"]
		if (startConnected == 0 && endConnected == 0) || (startConnected > 0 && endConnected > 0) {
			continue
		}
		externalSide := "start"
		if startConnected > 0 {
			externalSide = "end"
		}
		candidates = append(candidates, outerCandidate{
			TrackID:      segment.ID,
			CenterX:      (segment.From.X + segment.To.X) / 2,
			ExternalSide: externalSide,
		})
	}
	if len(candidates) < 2 {
		return stationOrientation{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CenterX < candidates[j].CenterX
	})
	left := candidates[0]
	right := candidates[len(candidates)-1]
	return stationOrientation{
		LeftOuterTrackID:  left.TrackID,
		RightOuterTrackID: right.TrackID,
		ExternalSideByTrack: map[string]string{
			left.TrackID:  left.ExternalSide,
			right.TrackID: right.ExternalSide,
		},
	}, true
}

func locomotiveAttachedTrackSide(
	vehicles []Vehicle,
	vehicleByID map[string]Vehicle,
	trainOrder []string,
	segments []Segment,
) (string, bool) {
	if len(trainOrder) < 2 {
		return "", false
	}
	locomotive := vehicleByID[trainOrder[0]]
	firstWagon := vehicleByID[trainOrder[1]]
	if locomotive.PathID != firstWagon.PathID {
		return "", false
	}

	attachedTrack := locomotive.PathID
	attachedSide := "start"
	if locomotive.PathIndex > firstWagon.PathIndex {
		attachedSide = "end"
	}
	for _, segment := range segments {
		if segment.ID != attachedTrack {
			continue
		}
		switch attachedSide {
		case "start":
			if segment.From.X <= segment.To.X {
				return "left", true
			}
			return "right", true
		case "end":
			if segment.To.X >= segment.From.X {
				return "right", true
			}
			return "left", true
		}
	}
	return "", false
}

func trackSideIndex(segments []Segment, gridSize float64, trackID, side string) (int, error) {
	for _, segment := range segments {
		if segment.ID != trackID {
			continue
		}
		points := getSegmentSlots(segment, gridSize)
		if len(points) == 0 {
			return 0, fmt.Errorf("track %s has no slots", trackID)
		}
		switch side {
		case "start":
			return 0, nil
		case "end":
			return len(points) - 1, nil
		default:
			return 0, fmt.Errorf("unsupported side %q for track %s", side, trackID)
		}
	}
	return 0, fmt.Errorf("track %s not found", trackID)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func buildTrainOrder(locomotiveID string, vehicles []Vehicle, couplings []Coupling) ([]string, error) {
	graph := make(map[string]map[string]struct{}, len(vehicles))
	for _, v := range vehicles {
		graph[v.ID] = map[string]struct{}{}
	}
	for _, c := range couplings {
		if _, ok := graph[c.A]; !ok {
			continue
		}
		if _, ok := graph[c.B]; !ok {
			continue
		}
		graph[c.A][c.B] = struct{}{}
		graph[c.B][c.A] = struct{}{}
	}

	connected := map[string]struct{}{locomotiveID: {}}
	queue := []string{locomotiveID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for next := range graph[cur] {
			if _, seen := connected[next]; seen {
				continue
			}
			connected[next] = struct{}{}
			queue = append(queue, next)
		}
	}

	if len(connected) == 1 {
		return []string{locomotiveID}, nil
	}

	for id := range connected {
		degree := 0
		for next := range graph[id] {
			if _, ok := connected[next]; ok {
				degree++
			}
		}
		if degree > 2 {
			return nil, errors.New("Only linear train order is supported.")
		}
	}

	locoDegree := 0
	for next := range graph[locomotiveID] {
		if _, ok := connected[next]; ok {
			locoDegree++
		}
	}
	if locoDegree > 1 {
		return nil, errors.New("Locomotive must be at train head.")
	}

	endpoints := make([]string, 0, 2)
	for id := range connected {
		degree := 0
		for next := range graph[id] {
			if _, ok := connected[next]; ok {
				degree++
			}
		}
		if degree <= 1 {
			endpoints = append(endpoints, id)
		}
	}

	var tail string
	for _, id := range endpoints {
		if id != locomotiveID {
			tail = id
			break
		}
	}
	if tail == "" {
		return nil, errors.New("Locomotive must be at train head.")
	}

	orderTailToLoco := []string{}
	prev := ""
	cur := tail
	for cur != "" {
		orderTailToLoco = append(orderTailToLoco, cur)
		if cur == locomotiveID {
			break
		}
		next := ""
		for n := range graph[cur] {
			if n != prev {
				if _, ok := connected[n]; ok {
					next = n
					break
				}
			}
		}
		prev = cur
		cur = next
	}

	if len(orderTailToLoco) == 0 || orderTailToLoco[len(orderTailToLoco)-1] != locomotiveID {
		return nil, errors.New("Locomotive must be at train head.")
	}

	return reverseStrings(orderTailToLoco), nil
}

type trackEdge struct {
	NextID         string
	CurrentSide    string
	NextSide       string
	ConnectionType string
}

type trackRouteEdge struct {
	FromTrackID    string
	ToTrackID      string
	FromSide       string
	ToSide         string
	ConnectionType string
}

func dijkstraTrackPath(connections []MovementTrackConnection, startTrackID, goalTrackID string) ([]string, []trackRouteEdge) {
	return dijkstraTrackPathAvoidingTracks(connections, startTrackID, goalTrackID, nil)
}

func dijkstraTrackPathWithGoalSide(
	connections []MovementTrackConnection,
	startTrackID, goalTrackID, goalSide string,
) ([]string, []trackRouteEdge) {
	return dijkstraTrackPathWithGoalSideAvoidingTracks(connections, startTrackID, goalTrackID, goalSide, nil)
}

func dijkstraTrackLoopPathWithGoalSide(
	connections []MovementTrackConnection,
	startTrackID, goalSide string,
) ([]string, []trackRouteEdge) {
	return dijkstraTrackLoopPathWithGoalSideAvoidingTracks(connections, startTrackID, goalSide, nil)
}

func dijkstraTrackPathAvoidingTracks(
	connections []MovementTrackConnection,
	startTrackID, goalTrackID string,
	blockedTrackIDs map[string]struct{},
) ([]string, []trackRouteEdge) {
	return dijkstraTrackPathWithGoalSideAvoidingTracks(connections, startTrackID, goalTrackID, "", blockedTrackIDs)
}

func dijkstraTrackLoopPathWithGoalSideAvoidingTracks(
	connections []MovementTrackConnection,
	startTrackID, goalSide string,
	blockedTrackIDs map[string]struct{},
) ([]string, []trackRouteEdge) {
	if startTrackID == "" {
		return nil, nil
	}

	adjacency := buildTrackAdjacency(connections)
	if len(adjacency[startTrackID]) == 0 {
		return nil, nil
	}

	type loopState struct {
		TrackPath []string
		Route     []trackRouteEdge
	}

	queue := make([]loopState, 0)
	for _, edge := range adjacency[startTrackID] {
		if blockedTrackIDs != nil && edge.NextID != startTrackID {
			if _, blocked := blockedTrackIDs[edge.NextID]; blocked {
				continue
			}
		}
		queue = append(queue, loopState{
			TrackPath: []string{startTrackID, edge.NextID},
			Route: []trackRouteEdge{{
				FromTrackID:    startTrackID,
				ToTrackID:      edge.NextID,
				FromSide:       edge.CurrentSide,
				ToSide:         edge.NextSide,
				ConnectionType: edge.ConnectionType,
			}},
		})
	}

	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]
		cur := state.TrackPath[len(state.TrackPath)-1]
		for _, edge := range adjacency[cur] {
			if edge.NextID == startTrackID {
				if len(state.TrackPath) < 3 {
					continue
				}
				if goalSide != "" && edge.NextSide != goalSide {
					continue
				}
				return append(append([]string{}, state.TrackPath...), startTrackID), append(
					append([]trackRouteEdge{}, state.Route...),
					trackRouteEdge{
						FromTrackID:    cur,
						ToTrackID:      startTrackID,
						FromSide:       edge.CurrentSide,
						ToSide:         edge.NextSide,
						ConnectionType: edge.ConnectionType,
					},
				)
			}
			if blockedTrackIDs != nil && edge.NextID != startTrackID {
				if _, blocked := blockedTrackIDs[edge.NextID]; blocked {
					continue
				}
			}
			if containsString(state.TrackPath, edge.NextID) {
				continue
			}
			nextTrackPath := append([]string{}, state.TrackPath...)
			nextTrackPath = append(nextTrackPath, edge.NextID)
			nextRoute := append([]trackRouteEdge{}, state.Route...)
			nextRoute = append(nextRoute, trackRouteEdge{
				FromTrackID:    cur,
				ToTrackID:      edge.NextID,
				FromSide:       edge.CurrentSide,
				ToSide:         edge.NextSide,
				ConnectionType: edge.ConnectionType,
			})
			queue = append(queue, loopState{
				TrackPath: nextTrackPath,
				Route:     nextRoute,
			})
		}
	}
	return nil, nil
}

func dijkstraTrackPathWithGoalSideAvoidingTracks(
	connections []MovementTrackConnection,
	startTrackID, goalTrackID, goalSide string,
	blockedTrackIDs map[string]struct{},
) ([]string, []trackRouteEdge) {
	if startTrackID == "" || goalTrackID == "" {
		return nil, nil
	}
	if startTrackID == goalTrackID && (goalSide == "" || goalSide == "start" || goalSide == "end") {
		return []string{startTrackID}, nil
	}

	adjacency := buildTrackAdjacency(connections)
	if len(adjacency[startTrackID]) == 0 || len(adjacency[goalTrackID]) == 0 {
		return nil, nil
	}

	prevTrack := map[string]string{}
	prevEdge := map[string]trackRouteEdge{}
	visited := map[string]struct{}{startTrackID: {}}
	queue := []string{startTrackID}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == goalTrackID {
			break
		}
		for _, edge := range adjacency[cur] {
			if edge.NextID == goalTrackID && goalSide != "" && edge.NextSide != goalSide {
				continue
			}
			if blockedTrackIDs != nil && edge.NextID != startTrackID && edge.NextID != goalTrackID {
				if _, blocked := blockedTrackIDs[edge.NextID]; blocked {
					continue
				}
			}
			if _, seen := visited[edge.NextID]; seen {
				continue
			}
			visited[edge.NextID] = struct{}{}
			prevTrack[edge.NextID] = cur
			prevEdge[edge.NextID] = trackRouteEdge{
				FromTrackID:    cur,
				ToTrackID:      edge.NextID,
				FromSide:       edge.CurrentSide,
				ToSide:         edge.NextSide,
				ConnectionType: edge.ConnectionType,
			}
			queue = append(queue, edge.NextID)
		}
	}

	if _, ok := prevTrack[goalTrackID]; !ok {
		return nil, nil
	}

	trackPath := []string{goalTrackID}
	route := make([]trackRouteEdge, 0)
	cur := goalTrackID
	for cur != startTrackID {
		edge := prevEdge[cur]
		route = append(route, edge)
		cur = prevTrack[cur]
		trackPath = append(trackPath, cur)
	}

	for i, j := 0, len(trackPath)-1; i < j; i, j = i+1, j-1 {
		trackPath[i], trackPath[j] = trackPath[j], trackPath[i]
	}
	for i, j := 0, len(route)-1; i < j; i, j = i+1, j-1 {
		route[i], route[j] = route[j], route[i]
	}
	return trackPath, route
}

func buildTrackAdjacency(connections []MovementTrackConnection) map[string][]trackEdge {
	adjacency := map[string][]trackEdge{}
	for _, connection := range connections {
		adjacency[connection.Track1ID] = append(adjacency[connection.Track1ID], trackEdge{
			NextID:         connection.Track2ID,
			CurrentSide:    connection.Track1Side,
			NextSide:       connection.Track2Side,
			ConnectionType: connection.ConnectionType,
		})
		adjacency[connection.Track2ID] = append(adjacency[connection.Track2ID], trackEdge{
			NextID:         connection.Track1ID,
			CurrentSide:    connection.Track2Side,
			NextSide:       connection.Track1Side,
			ConnectionType: connection.ConnectionType,
		})
	}
	return adjacency
}

func buildSlotPathFromTrackRoute(
	segments []Segment,
	trackConnections []MovementTrackConnection,
	startTrackID string,
	startIndex int,
	targetTrackID string,
	targetIndex int,
	trackPath []string,
	route []trackRouteEdge,
	gridSize float64,
) ([]string, map[string]Slot, error) {
	if len(trackPath) == 0 {
		return nil, nil, errors.New("empty track path")
	}

	slotsByTrack := map[string][]Point{}
	endpointOverrides := buildSharedEndpointOverrideMap(segments, trackConnections)
	for _, segment := range segments {
		slotsByTrack[segment.ID] = getSegmentSlotsWithEndpointOverrides(segment, gridSize, endpointOverrides)
	}

	slotByID := map[string]Slot{}
	result := make([]string, 0)
	appendTrackSlice := func(trackID string, fromIndex, toIndex int) error {
		points := slotsByTrack[trackID]
		if len(points) == 0 {
			return fmt.Errorf("track %s has no slots", trackID)
		}
		if fromIndex < 0 || fromIndex >= len(points) || toIndex < 0 || toIndex >= len(points) {
			return fmt.Errorf("track %s slice is outside capacity: %d -> %d of %d", trackID, fromIndex, toIndex, len(points))
		}
		step := 1
		if fromIndex > toIndex {
			step = -1
		}
		for idx := fromIndex; ; idx += step {
			point := points[idx]
			id := slotID(point.X, point.Y)
			slotByID[id] = Slot{ID: id, X: point.X, Y: point.Y}
			if len(result) == 0 || result[len(result)-1] != id {
				result = append(result, id)
			}
			if idx == toIndex {
				break
			}
		}
		return nil
	}

	sideIndex := func(trackID, side string) (int, error) {
		points := slotsByTrack[trackID]
		if len(points) == 0 {
			return 0, fmt.Errorf("track %s has no slots", trackID)
		}
		switch side {
		case "start":
			return 0, nil
		case "end":
			return len(points) - 1, nil
		default:
			return 0, fmt.Errorf("unsupported side %q for track %s", side, trackID)
		}
	}

	if len(trackPath) == 1 {
		if err := appendTrackSlice(startTrackID, startIndex, targetIndex); err != nil {
			return nil, nil, err
		}
		return result, slotByID, nil
	}

	firstBoundary, err := sideIndex(startTrackID, route[0].FromSide)
	if err != nil {
		return nil, nil, err
	}
	if err := appendTrackSlice(startTrackID, startIndex, firstBoundary); err != nil {
		return nil, nil, err
	}

	for i := 1; i < len(trackPath)-1; i++ {
		trackID := trackPath[i]
		entryIndex, err := sideIndex(trackID, route[i-1].ToSide)
		if err != nil {
			return nil, nil, err
		}
		exitIndex, err := sideIndex(trackID, route[i].FromSide)
		if err != nil {
			return nil, nil, err
		}
		if err := appendTrackSlice(trackID, entryIndex, exitIndex); err != nil {
			return nil, nil, err
		}
	}

	lastEntryIndex, err := sideIndex(targetTrackID, route[len(route)-1].ToSide)
	if err != nil {
		return nil, nil, err
	}
	if err := appendTrackSlice(targetTrackID, lastEntryIndex, targetIndex); err != nil {
		return nil, nil, err
	}

	return result, slotByID, nil
}

func dijkstraPath(adjacency map[string]map[string]struct{}, startID, goalID string) []string {
	if startID == goalID {
		return []string{startID}
	}
	if _, ok := adjacency[startID]; !ok {
		return nil
	}
	if _, ok := adjacency[goalID]; !ok {
		return nil
	}

	dist := map[string]int{startID: 0}
	prev := map[string]string{}
	visited := map[string]struct{}{}
	queue := []string{startID}

	for len(queue) > 0 {
		sort.Slice(queue, func(i, j int) bool {
			return dist[queue[i]] < dist[queue[j]]
		})
		cur := queue[0]
		queue = queue[1:]
		if _, seen := visited[cur]; seen {
			continue
		}
		visited[cur] = struct{}{}
		if cur == goalID {
			break
		}
		for next := range adjacency[cur] {
			if _, seen := visited[next]; seen {
				continue
			}
			nd := dist[cur] + 1
			old, ok := dist[next]
			if !ok || nd < old {
				dist[next] = nd
				prev[next] = cur
				queue = append(queue, next)
			}
		}
	}

	if _, ok := prev[goalID]; !ok {
		return nil
	}

	path := []string{goalID}
	node := goalID
	for {
		p, ok := prev[node]
		if !ok {
			break
		}
		path = append(path, p)
		node = p
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func extendPathForBackwardPush(path []string, adjacency map[string]map[string]struct{}, slotByID map[string]Slot, neededTailSlots int) ([]string, error) {
	extended := append([]string{}, path...)
	var prev string
	if len(path) >= 2 {
		prev = path[len(path)-2]
	}
	cur := path[len(path)-1]

	for i := 0; i < neededTailSlots; i++ {
		candidates := []string{}
		for id := range adjacency[cur] {
			if id != prev {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) == 0 {
			return nil, errors.New("Not enough space after target for backward push.")
		}

		next := candidates[0]
		if len(candidates) > 1 && prev != "" {
			pPrev, okPrev := slotByID[prev]
			pCur, okCur := slotByID[cur]
			if okPrev && okCur {
				inX := pCur.X - pPrev.X
				inY := pCur.Y - pPrev.Y
				bestScore := math.Inf(-1)
				for _, candidateID := range candidates {
					pNext, okNext := slotByID[candidateID]
					if !okNext {
						continue
					}
					outX := pNext.X - pCur.X
					outY := pNext.Y - pCur.Y
					score := inX*outX + inY*outY
					if score > bestScore {
						bestScore = score
						next = candidateID
					}
				}
			}
		}

		extended = append(extended, next)
		prev = cur
		cur = next
	}

	return extended, nil
}
