package main

import (
	"math"
)

type segmentEndpoint struct {
	segment Segment
	side    string
}

func collectRailSlots(segments []Segment, gridSize float64) []Slot {
	uniq := map[string]Slot{}
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
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

func collectPathSlots(segments []Segment, gridSize float64) []PathSlot {
	slots := make([]PathSlot, 0)
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
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
	adj := map[string]map[string]struct{}{}
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
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

	connectNearbyEndpoints(adj, segments, gridSize)
	return adj
}

type endpointSlot struct {
	slotID string
	x      float64
	y      float64
}

func connectNearbyEndpoints(adj map[string]map[string]struct{}, segments []Segment, gridSize float64) {
	endpoints := make([]endpointSlot, 0, len(segments)*2)
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
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
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
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
	for _, segment := range segments {
		points := getSegmentSlots(segment, gridSize)
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
	dx := segment.To.X - segment.From.X
	dy := segment.To.Y - segment.From.Y
	length := math.Hypot(dx, dy)

	if length == 0 {
		return []Point{{X: segment.From.X, Y: segment.From.Y}}
	}

	count := int(math.Floor(length / step))
	ux := dx / length
	uy := dy / length
	slots := make([]Point, 0, count+2)
	for i := 0; i <= count; i++ {
		slots = append(slots, Point{
			X: segment.From.X + ux*step*float64(i),
			Y: segment.From.Y + uy*step*float64(i),
		})
	}

	last := slots[len(slots)-1]
	if math.Hypot(last.X-segment.To.X, last.Y-segment.To.Y) >= step*0.25 {
		slots = append(slots, Point{X: segment.To.X, Y: segment.To.Y})
	}

	return slots
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
