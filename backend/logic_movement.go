package main

import (
	"errors"
	"math"
	"sort"
	"strings"
)

func buildMovementPlan(req PlanMovementRequest) (PlanMovementResponse, error) {
	if req.SelectedLocomotiveID == "" {
		return PlanMovementResponse{}, errors.New("Select locomotive.")
	}
	if strings.TrimSpace(req.TargetPathID) == "" {
		return PlanMovementResponse{}, errors.New("Select target path.")
	}

	pathSlots := collectPathSlots(req.Segments, req.GridSize)
	if len(pathSlots) == 0 {
		return PlanMovementResponse{}, errors.New("No rail slots available.")
	}
	targetSlot, ok := findPathSlot(pathSlots, req.TargetPathID, req.TargetIndex)
	if !ok {
		return PlanMovementResponse{}, errors.New("Target slot is unavailable.")
	}
	targetSlotID := slotID(targetSlot.X, targetSlot.Y)

	normalizedVehicles := make([]Vehicle, 0, len(req.Vehicles))
	vehicleByID := make(map[string]Vehicle, len(req.Vehicles))
	for _, v := range req.Vehicles {
		nv := normalizeVehicleToPath(v, pathSlots)
		normalizedVehicles = append(normalizedVehicles, nv)
		vehicleByID[nv.ID] = nv
	}

	locomotive, exists := vehicleByID[req.SelectedLocomotiveID]
	if !exists || locomotive.Type != "locomotive" {
		return PlanMovementResponse{}, errors.New("Selected unit is not a locomotive.")
	}

	slots := collectRailSlots(req.Segments, req.GridSize)
	slotByID := make(map[string]Slot, len(slots))
	for _, s := range slots {
		slotByID[s.ID] = s
	}

	slotAdj := buildSlotAdjacency(req.Segments, req.GridSize)
	trainOrder, err := buildTrainOrder(req.SelectedLocomotiveID, normalizedVehicles, req.Couplings)
	if err != nil {
		return PlanMovementResponse{}, err
	}

	currentSlotByVehicleID := make(map[string]string, len(trainOrder))
	for _, id := range trainOrder {
		v, ok := vehicleByID[id]
		if !ok {
			return PlanMovementResponse{}, errors.New("Train contains unknown vehicle.")
		}
		nearest := findNearestSlot(Point{X: v.X, Y: v.Y}, slots)
		if nearest == nil {
			return PlanMovementResponse{}, errors.New("No rail slots available.")
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
			return PlanMovementResponse{}, errors.New("Coupled train must stand on adjacent slots.")
		}
	}

	locoStart := currentSlotByVehicleID[req.SelectedLocomotiveID]
	path := dijkstraPath(slotAdj, locoStart, targetSlotID)
	if len(path) < 2 {
		return PlanMovementResponse{}, errors.New("Path was not found.")
	}

	currentLocoToTail := make([]string, 0, len(trainOrder))
	for _, id := range trainOrder {
		currentLocoToTail = append(currentLocoToTail, currentSlotByVehicleID[id])
	}

	isBackwardPush := len(trainOrder) > 1 && len(path) > 1 && path[1] == currentLocoToTail[1]
	drivingPath := path
	if isBackwardPush && len(trainOrder) > 1 {
		extended, extErr := extendPathForBackwardPush(path, slotAdj, slotByID, len(trainOrder)-1)
		if extErr != nil {
			return PlanMovementResponse{}, extErr
		}
		drivingPath = extended
	}

	staticOccupied := make(map[string]struct{})
	trainSet := make(map[string]struct{}, len(trainOrder))
	for _, id := range trainOrder {
		trainSet[id] = struct{}{}
	}
	for _, v := range normalizedVehicles {
		if _, ok := trainSet[v.ID]; ok {
			continue
		}
		staticOccupied[slotID(v.X, v.Y)] = struct{}{}
	}

	maxSteps := len(path) - 1
	if maxSteps < 1 {
		return PlanMovementResponse{}, errors.New("Not enough path length.")
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
			return PlanMovementResponse{}, errors.New("Movement is blocked: not enough free slots.")
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
