package main

import (
	"math"
	"strings"
)

type segmentEndpoint struct {
	segment Segment
	side    string
}

func collectRailSlotsWithConnections(segments []Segment, gridSize float64, connections []MovementTrackConnection) []Slot {
	endpointOverrides := buildSharedEndpointOverrideMap(segments, connections)
	uniq := map[string]Slot{}
	for _, segment := range segments {
		points := getSegmentSlotsWithEndpointOverrides(segment, gridSize, endpointOverrides)
		for _, p := range points {
			id := slotID(p.X, p.Y)
			uniq[id] = Slot{ID: id, X: p.X, Y: p.Y}
		}
	}
	result := make([]Slot, 0, len(uniq))
	for _, s := range uniq {
		result = append(result, s)
	}
	return result
}

func collectRailSlots(segments []Segment, gridSize float64) []Slot {
	return collectRailSlotsWithConnections(segments, gridSize, buildTrackConnectionsFromSegments(segments))
}

func collectPathSlotsWithConnections(segments []Segment, gridSize float64, connections []MovementTrackConnection) []PathSlot {
	endpointOverrides := buildSharedEndpointOverrideMap(segments, connections)
	slots := make([]PathSlot, 0)
	for _, segment := range segments {
		points := getSegmentSlotsWithEndpointOverrides(segment, gridSize, endpointOverrides)
		for i, p := range points {
			slots = append(slots, PathSlot{
				PathID: segment.ID,
				Index:  i,
				X:      p.X,
				Y:      p.Y,
			})
		}
	}
	return slots
}

func collectPathSlots(segments []Segment, gridSize float64) []PathSlot {
	return collectPathSlotsWithConnections(segments, gridSize, buildTrackConnectionsFromSegments(segments))
}

func findPathSlot(slots []PathSlot, pathID string, index int) (PathSlot, bool) {
	for _, slot := range slots {
		if slot.PathID == pathID && slot.Index == index {
			return slot, true
		}
	}
	return PathSlot{}, false
}

func findNearestPathSlot(point Point, slots []PathSlot, blocked map[string]struct{}) *PathSlot {
	var best *PathSlot
	bestDist := math.Inf(1)
	for i := range slots {
		if blocked != nil {
			if _, used := blocked[pathSlotKey(slots[i].PathID, slots[i].Index)]; used {
				continue
			}
		}
		dx := point.X - slots[i].X
		dy := point.Y - slots[i].Y
		dist := dx*dx + dy*dy
		if dist < bestDist {
			bestDist = dist
			best = &slots[i]
		}
	}
	return best
}

func buildSlotAdjacency(segments []Segment, gridSize float64) map[string]map[string]struct{} {
	return buildSlotAdjacencyWithConnections(segments, gridSize, buildTrackConnectionsFromSegments(segments))
}

func buildSlotAdjacencyWithConnections(segments []Segment, gridSize float64, connections []MovementTrackConnection) map[string]map[string]struct{} {
	adj := map[string]map[string]struct{}{}
	endpointOverrides := buildSharedEndpointOverrideMap(segments, connections)
	for _, segment := range segments {
		points := getSegmentSlotsWithEndpointOverrides(segment, gridSize, endpointOverrides)
		for i := 0; i < len(points)-1; i++ {
			a := slotID(points[i].X, points[i].Y)
			b := slotID(points[i+1].X, points[i+1].Y)
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

	connectNearbyEndpointsWithOverrides(adj, segments, gridSize, endpointOverrides)
	return adj
}

type endpointSlot struct {
	slotID string
	x      float64
	y      float64
}

func connectNearbyEndpoints(adj map[string]map[string]struct{}, segments []Segment, gridSize float64) {
	connectNearbyEndpointsWithOverrides(adj, segments, gridSize, buildSharedEndpointOverrideMap(segments, buildTrackConnectionsFromSegments(segments)))
}

func connectNearbyEndpointsWithOverrides(adj map[string]map[string]struct{}, segments []Segment, gridSize float64, endpointOverrides map[string]Point) {
	endpoints := make([]endpointSlot, 0, len(segments)*2)
	for _, segment := range segments {
		points := getSegmentSlotsWithEndpointOverrides(segment, gridSize, endpointOverrides)
		if len(points) == 0 {
			continue
		}
		start := points[0]
		end := points[len(points)-1]
		endpoints = append(endpoints,
			endpointSlot{slotID: slotID(start.X, start.Y), x: start.X, y: start.Y},
			endpointSlot{slotID: slotID(end.X, end.Y), x: end.X, y: end.Y},
		)
	}

	epsilon := math.Max(0.5, gridSize*0.05)
	for i := 0; i < len(endpoints); i++ {
		for j := i + 1; j < len(endpoints); j++ {
			a := endpoints[i]
			b := endpoints[j]
			if a.slotID == b.slotID {
				continue
			}
			if math.Hypot(a.x-b.x, a.y-b.y) > epsilon {
				continue
			}
			if _, ok := adj[a.slotID]; !ok {
				adj[a.slotID] = map[string]struct{}{}
			}
			if _, ok := adj[b.slotID]; !ok {
				adj[b.slotID] = map[string]struct{}{}
			}
			adj[a.slotID][b.slotID] = struct{}{}
			adj[b.slotID][a.slotID] = struct{}{}
		}
	}
}

func buildAdjacentSlotPairs(segments []Segment, gridSize float64) map[string]struct{} {
	pairs := map[string]struct{}{}
	endpointOverrides := buildSharedEndpointOverrideMap(segments, buildTrackConnectionsFromSegments(segments))
	for _, segment := range segments {
		points := getSegmentSlotsWithEndpointOverrides(segment, gridSize, endpointOverrides)
		for i := 0; i < len(points)-1; i++ {
			a := slotID(points[i].X, points[i].Y)
			b := slotID(points[i+1].X, points[i+1].Y)
			pairs[pairKey(a, b)] = struct{}{}
		}
	}
	return pairs
}

func buildAdjacentPathSlotPairs(segments []Segment, gridSize float64) map[string]struct{} {
	pairs := map[string]struct{}{}
	endpointOverrides := buildSharedEndpointOverrideMap(segments, buildTrackConnectionsFromSegments(segments))
	for _, segment := range segments {
		points := getSegmentSlotsWithEndpointOverrides(segment, gridSize, endpointOverrides)
		for i := 0; i < len(points)-1; i++ {
			pairs[pathSlotPairKey(segment.ID, i, segment.ID, i+1)] = struct{}{}
		}
	}
	return pairs
}

func buildTrackConnectionsFromSegments(segments []Segment) []MovementTrackConnection {
	byNode := map[string][]segmentEndpoint{}
	for _, segment := range segments {
		byNode[slotID(segment.From.X, segment.From.Y)] = append(byNode[slotID(segment.From.X, segment.From.Y)], segmentEndpoint{
			segment: segment,
			side:    "start",
		})
		byNode[slotID(segment.To.X, segment.To.Y)] = append(byNode[slotID(segment.To.X, segment.To.Y)], segmentEndpoint{
			segment: segment,
			side:    "end",
		})
	}

	seen := map[string]struct{}{}
	result := make([]MovementTrackConnection, 0)
	for _, entries := range byNode {
		if len(entries) < 2 {
			continue
		}
		connectionType := "serial"
		if len(entries) > 2 {
			connectionType = "switch"
		}
		for i := 0; i < len(entries); i++ {
			for j := i + 1; j < len(entries); j++ {
				a := entries[i]
				b := entries[j]
				id := a.segment.ID + ":" + b.segment.ID + ":" + a.side + ":" + b.side
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				result = append(result, MovementTrackConnection{
					Track1ID:       a.segment.ID,
					Track2ID:       b.segment.ID,
					Track1Side:     a.side,
					Track2Side:     b.side,
					ConnectionType: connectionType,
				})
			}
		}
	}
	return result
}

func getSegmentSlots(segment Segment, step float64) []Point {
	return getSegmentSlotsWithEndpointOverrides(segment, step, nil)
}

func getSegmentSlotsWithEndpointOverrides(segment Segment, step float64, endpointOverrides map[string]Point) []Point {
	from := segment.From
	to := segment.To
	if endpointOverrides != nil {
		if override, ok := endpointOverrides[endpointRefKey(segment.ID, "start")]; ok {
			from = override
		}
		if override, ok := endpointOverrides[endpointRefKey(segment.ID, "end")]; ok {
			to = override
		}
	}

	dx := to.X - from.X
	dy := to.Y - from.Y
	length := math.Hypot(dx, dy)

	if length == 0 {
		return []Point{{X: from.X, Y: from.Y}}
	}

	count := int(math.Round(length / step))
	if count < 1 {
		count = 1
	}
	ux := dx / length
	uy := dy / length
	actualStep := length / float64(count)
	slots := make([]Point, 0, count+1)
	for i := 0; i <= count; i++ {
		distance := actualStep * float64(i)
		slots = append(slots, Point{
			X: from.X + ux*distance,
			Y: from.Y + uy*distance,
		})
	}
	slots[len(slots)-1] = Point{X: to.X, Y: to.Y}

	return slots
}

func endpointRefKey(trackID, side string) string {
	return trackID + ":" + side
}

func buildSharedEndpointOverrideMap(segments []Segment, connections []MovementTrackConnection) map[string]Point {
	if len(segments) == 0 {
		return nil
	}

	segmentByID := make(map[string]Segment, len(segments))
	parent := make(map[string]string)

	addEndpoint := func(trackID, side string) {
		key := endpointRefKey(trackID, side)
		if _, ok := parent[key]; !ok {
			parent[key] = key
		}
	}

	for _, segment := range segments {
		segmentByID[segment.ID] = segment
		addEndpoint(segment.ID, "start")
		addEndpoint(segment.ID, "end")
	}

	var find func(string) string
	find = func(key string) string {
		root, ok := parent[key]
		if !ok {
			parent[key] = key
			return key
		}
		if root == key {
			return key
		}
		parent[key] = find(root)
		return parent[key]
	}

	union := func(a, b string) {
		ra := find(a)
		rb := find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	for _, connection := range connections {
		if connection.Track1ID == "" || connection.Track2ID == "" {
			continue
		}
		a := endpointRefKey(connection.Track1ID, connection.Track1Side)
		b := endpointRefKey(connection.Track2ID, connection.Track2Side)
		addEndpoint(connection.Track1ID, connection.Track1Side)
		addEndpoint(connection.Track2ID, connection.Track2Side)
		union(a, b)
	}

	componentRefs := make(map[string][]string)
	for ref := range parent {
		root := find(ref)
		componentRefs[root] = append(componentRefs[root], ref)
	}

	type pointSum struct {
		x     float64
		y     float64
		count int
	}
	componentSums := make(map[string]pointSum)
	for root, refs := range componentRefs {
		sum := pointSum{}
		for _, ref := range refs {
			parts := strings.SplitN(ref, ":", 2)
			if len(parts) != 2 {
				continue
			}
			segment, ok := segmentByID[parts[0]]
			if !ok {
				continue
			}
			point := segment.From
			if parts[1] == "end" {
				point = segment.To
			}
			sum.x += point.X
			sum.y += point.Y
			sum.count++
		}
		if sum.count > 0 {
			componentSums[root] = sum
		}
	}

	overrides := make(map[string]Point)
	for root, refs := range componentRefs {
		sum, ok := componentSums[root]
		if !ok || sum.count == 0 {
			continue
		}
		point := Point{X: sum.x / float64(sum.count), Y: sum.y / float64(sum.count)}
		for _, ref := range refs {
			overrides[ref] = point
		}
	}
	return overrides
}

func findNearestSlot(point Point, slots []Slot) *Slot {
	var best *Slot
	bestDist := math.Inf(1)
	for i := range slots {
		dx := point.X - slots[i].X
		dy := point.Y - slots[i].Y
		dist := dx*dx + dy*dy
		if dist < bestDist {
			bestDist = dist
			best = &slots[i]
		}
	}
	return best
}

func findNearestFreeSlot(point Point, slots []Slot, blocked map[string]struct{}) *Slot {
	var best *Slot
	bestDist := math.Inf(1)
	for i := range slots {
		if _, used := blocked[slots[i].ID]; used {
			continue
		}
		dx := point.X - slots[i].X
		dy := point.Y - slots[i].Y
		dist := dx*dx + dy*dy
		if dist < bestDist {
			bestDist = dist
			best = &slots[i]
		}
	}
	return best
}
