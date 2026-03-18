package main

import "testing"

func TestBuildMovementPlanSingleLocomotive(t *testing.T) {
	req := PlanMovementRequest{
		GridSize: 32,
		Segments: []Segment{
			{
				ID:   "1",
				From: Point{X: 0, Y: 0},
				To:   Point{X: 128, Y: 0},
			},
		},
		Vehicles: []Vehicle{
			{
				ID:        "l1",
				Type:      "locomotive",
				Code:      "l1",
				PathID:    "1",
				PathIndex: 0,
				X:         0,
				Y:         0,
			},
		},
		SelectedLocomotiveID: "l1",
		TargetPathID:         "1",
		TargetIndex:          3,
	}

	resp, err := buildMovementPlan(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}
	if len(resp.Timeline) != 3 {
		t.Fatalf("expected 3 timeline steps, got %d", len(resp.Timeline))
	}
	lastStep := resp.Timeline[len(resp.Timeline)-1]
	if len(lastStep) != 1 {
		t.Fatalf("expected 1 position in last step, got %d", len(lastStep))
	}
	if lastStep[0].ID != "l1" {
		t.Fatalf("expected locomotive id l1, got %s", lastStep[0].ID)
	}
	if lastStep[0].X != 96 || lastStep[0].Y != 0 {
		t.Fatalf("expected final coordinates (96,0), got (%.2f,%.2f)", lastStep[0].X, lastStep[0].Y)
	}
}

func TestBuildMovementPlanFailsWithoutLocomotive(t *testing.T) {
	_, err := buildMovementPlan(PlanMovementRequest{
		GridSize:             32,
		Segments:             []Segment{{ID: "1", From: Point{X: 0, Y: 0}, To: Point{X: 64, Y: 0}}},
		SelectedLocomotiveID: "",
		TargetPathID:         "1",
		TargetIndex:          1,
	})
	if err == nil {
		t.Fatal("expected error for empty locomotive selection")
	}
}

func TestBuildMovementPlanAllowsNearCoincidentSwitchEndpoints(t *testing.T) {
	req := PlanMovementRequest{
		GridSize: 40,
		Segments: []Segment{
			{ID: "1", From: Point{X: 0, Y: 0}, To: Point{X: 40, Y: 0}},
			{ID: "2", From: Point{X: 40.4, Y: 0}, To: Point{X: 80, Y: 0}},
			{ID: "3", From: Point{X: 80.4, Y: 0}, To: Point{X: 120, Y: 0}},
			{ID: "4", From: Point{X: 120.4, Y: 0}, To: Point{X: 160, Y: 0}},
		},
		TrackConnections: []MovementTrackConnection{
			{Track1ID: "1", Track2ID: "2", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "2", Track2ID: "3", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "3", Track2ID: "4", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
		},
		Vehicles: []Vehicle{
			{
				ID:        "l1",
				Type:      "locomotive",
				Code:      "l1",
				PathID:    "1",
				PathIndex: 0,
				X:         0,
				Y:         0,
			},
		},
		SelectedLocomotiveID: "l1",
		TargetPathID:         "4",
		TargetIndex:          1,
	}

	resp, err := buildMovementPlan(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}
	if len(resp.Timeline) == 0 {
		t.Fatal("expected non-empty timeline")
	}
}

func TestBuildMovementPlanUsesTrackConnectionsAsRoutingSource(t *testing.T) {
	req := PlanMovementRequest{
		GridSize: 40,
		Segments: []Segment{
			{ID: "1", From: Point{X: -560, Y: 200}, To: Point{X: -360, Y: 200}},
			{ID: "3", From: Point{X: -360, Y: 200}, To: Point{X: -280, Y: 280}},
			{ID: "4", From: Point{X: -280, Y: 280}, To: Point{X: -160, Y: 240}},
			{ID: "6", From: Point{X: -160, Y: 240}, To: Point{X: -40, Y: 200}},
			{ID: "14", From: Point{X: -40, Y: 200}, To: Point{X: 200, Y: 200}},
		},
		TrackConnections: []MovementTrackConnection{
			{Track1ID: "1", Track2ID: "3", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "3", Track2ID: "4", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "4", Track2ID: "6", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "6", Track2ID: "14", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
		},
		Vehicles: []Vehicle{
			{
				ID:        "l1",
				Type:      "locomotive",
				Code:      "l1",
				PathID:    "1",
				PathIndex: 1,
				X:         -520,
				Y:         200,
			},
		},
		SelectedLocomotiveID: "l1",
		TargetPathID:         "14",
		TargetIndex:          4,
	}

	resp, err := buildMovementPlan(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}
	if len(resp.Timeline) == 0 {
		t.Fatal("expected non-empty timeline")
	}
}

func TestBuildMovementPlanFindsUserRouteThroughSwitchGraph(t *testing.T) {
	req := PlanMovementRequest{
		GridSize: 40,
		Segments: []Segment{
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-1", From: Point{X: -560, Y: 200}, To: Point{X: -360, Y: 200}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-2", From: Point{X: -360, Y: 200}, To: Point{X: -160, Y: 80}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-3", From: Point{X: -360, Y: 200}, To: Point{X: -280, Y: 280}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-4", From: Point{X: -280, Y: 280}, To: Point{X: -160, Y: 240}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-5", From: Point{X: -280, Y: 280}, To: Point{X: -160, Y: 360}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-6", From: Point{X: -160, Y: 240}, To: Point{X: -40, Y: 200}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-7", From: Point{X: -160, Y: 240}, To: Point{X: -40, Y: 280}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-8", From: Point{X: -160, Y: 80}, To: Point{X: -40, Y: 40}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-9", From: Point{X: -160, Y: 80}, To: Point{X: -40, Y: 120}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-10", From: Point{X: -40, Y: 40}, To: Point{X: 200, Y: 40}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-11", From: Point{X: -40, Y: 120}, To: Point{X: 200, Y: 120}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-12", From: Point{X: 200, Y: 40}, To: Point{X: 320, Y: 80}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-13", From: Point{X: 200, Y: 120}, To: Point{X: 320, Y: 80}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-14", From: Point{X: -40, Y: 200}, To: Point{X: 200, Y: 200}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-15", From: Point{X: -40, Y: 280}, To: Point{X: 200, Y: 280}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-16", From: Point{X: 200, Y: 200}, To: Point{X: 320, Y: 240}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-17", From: Point{X: 200, Y: 280}, To: Point{X: 320, Y: 240}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-18", From: Point{X: -160, Y: 360}, To: Point{X: 320, Y: 360}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-19", From: Point{X: 320, Y: 80}, To: Point{X: 520, Y: 200}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-20", From: Point{X: 320, Y: 240}, To: Point{X: 440, Y: 280}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-21", From: Point{X: 320, Y: 360}, To: Point{X: 440, Y: 280}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-22", From: Point{X: 440, Y: 280}, To: Point{X: 520, Y: 200}},
			{ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-23", From: Point{X: 520, Y: 200}, To: Point{X: 800, Y: 200}},
		},
		TrackConnections: []MovementTrackConnection{
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-1", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-2", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-1", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-3", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-2", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-3", Track1Side: "start", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-2", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-8", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-2", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-9", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-3", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-4", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-3", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-5", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-4", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-5", Track1Side: "start", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-4", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-6", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-4", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-7", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-5", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-18", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-6", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-14", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-6", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-7", Track1Side: "start", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-7", Track2ID: "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-15", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
		},
		Vehicles: []Vehicle{
			{
				ID:        "l1",
				Type:      "locomotive",
				Code:      "l1",
				PathID:    "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-1",
				PathIndex: 1,
				X:         -520,
				Y:         200,
			},
		},
		SelectedLocomotiveID: "l1",
		TargetPathID:         "draft-e3bf4304-5ff0-4d6c-901f-0d1da8eae081-track-14",
		TargetIndex:          4,
	}

	resp, err := buildMovementPlan(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}
	if len(resp.Timeline) == 0 {
		t.Fatal("expected non-empty timeline")
	}
}

func TestBuildMovementPlanAvoidsOccupiedParallelBranchToMergedTrack(t *testing.T) {
	req := PlanMovementRequest{
		GridSize: 40,
		Segments: []Segment{
			{ID: "1", From: Point{X: -80, Y: 0}, To: Point{X: 0, Y: 0}},
			{ID: "2", From: Point{X: 0, Y: 0}, To: Point{X: 80, Y: -80}},
			{ID: "3", From: Point{X: 0, Y: 0}, To: Point{X: 80, Y: 80}},
			{ID: "4", From: Point{X: 80, Y: -80}, To: Point{X: 160, Y: 0}},
			{ID: "5", From: Point{X: 80, Y: 80}, To: Point{X: 160, Y: 0}},
			{ID: "6", From: Point{X: 160, Y: 0}, To: Point{X: 240, Y: 0}},
		},
		TrackConnections: []MovementTrackConnection{
			{Track1ID: "1", Track2ID: "2", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "1", Track2ID: "3", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "2", Track2ID: "4", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "3", Track2ID: "5", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "4", Track2ID: "5", Track1Side: "end", Track2Side: "end", ConnectionType: "switch"},
			{Track1ID: "4", Track2ID: "6", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "5", Track2ID: "6", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
		},
		Vehicles: []Vehicle{
			{
				ID:        "l1",
				Type:      "locomotive",
				Code:      "l1",
				PathID:    "1",
				PathIndex: 0,
				X:         -80,
				Y:         0,
			},
			{
				ID:        "w1",
				Type:      "wagon",
				Code:      "w1",
				Color:     "#f59e0b",
				PathID:    "2",
				PathIndex: 1,
				X:         40,
				Y:         -40,
			},
		},
		SelectedLocomotiveID: "l1",
		TargetPathID:         "6",
		TargetIndex:          1,
	}

	resp, err := buildMovementPlan(req)
	if err != nil {
		t.Fatalf("expected route via free lower branch, got error: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}
	if len(resp.Timeline) == 0 {
		t.Fatal("expected non-empty timeline")
	}
	lastStep := resp.Timeline[len(resp.Timeline)-1]
	if len(lastStep) != 1 {
		t.Fatalf("expected one locomotive position, got %d", len(lastStep))
	}
	if lastStep[0].X != 200 || lastStep[0].Y != 0 {
		t.Fatalf("expected target slot on track 6 at (200,0), got (%.2f,%.2f)", lastStep[0].X, lastStep[0].Y)
	}
}

func TestBuildMovementPlanAvoidsOccupiedParallelBranchToFreeBranchTarget(t *testing.T) {
	req := PlanMovementRequest{
		GridSize: 40,
		Segments: []Segment{
			{ID: "1", From: Point{X: -80, Y: 0}, To: Point{X: 0, Y: 0}},
			{ID: "2", From: Point{X: 0, Y: 0}, To: Point{X: 80, Y: -80}},
			{ID: "3", From: Point{X: 0, Y: 0}, To: Point{X: 80, Y: 80}},
			{ID: "4", From: Point{X: 80, Y: -80}, To: Point{X: 160, Y: 0}},
			{ID: "5", From: Point{X: 80, Y: 80}, To: Point{X: 160, Y: 0}},
		},
		TrackConnections: []MovementTrackConnection{
			{Track1ID: "1", Track2ID: "2", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "1", Track2ID: "3", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "2", Track2ID: "4", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "3", Track2ID: "5", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "4", Track2ID: "5", Track1Side: "end", Track2Side: "end", ConnectionType: "switch"},
		},
		Vehicles: []Vehicle{
			{
				ID:        "l1",
				Type:      "locomotive",
				Code:      "l1",
				PathID:    "1",
				PathIndex: 0,
				X:         -80,
				Y:         0,
			},
			{
				ID:        "w1",
				Type:      "wagon",
				Code:      "w1",
				Color:     "#f59e0b",
				PathID:    "2",
				PathIndex: 1,
				X:         40,
				Y:         -40,
			},
		},
		SelectedLocomotiveID: "l1",
		TargetPathID:         "5",
		TargetIndex:          1,
	}

	resp, err := buildMovementPlan(req)
	if err != nil {
		t.Fatalf("expected direct route to free lower branch, got error: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}
	if len(resp.Timeline) == 0 {
		t.Fatal("expected non-empty timeline")
	}
	lastStep := resp.Timeline[len(resp.Timeline)-1]
	if len(lastStep) != 1 {
		t.Fatalf("expected one locomotive position, got %d", len(lastStep))
	}
	if lastStep[0].X != 108.2842712474619 || lastStep[0].Y != 51.7157287525381 {
		t.Fatalf("expected target slot on track 5 at (108.28,51.72), got (%.2f,%.2f)", lastStep[0].X, lastStep[0].Y)
	}
}

func TestBuildMovementPlanAllowsSingleLocomotiveToEnterPartiallyOccupiedTargetTrack(t *testing.T) {
	req := PlanMovementRequest{
		GridSize: 40,
		Segments: []Segment{
			{ID: "1", From: Point{X: 0, Y: 0}, To: Point{X: 80, Y: 0}},
			{ID: "2", From: Point{X: 80, Y: 0}, To: Point{X: 280, Y: 0}},
		},
		TrackConnections: []MovementTrackConnection{
			{Track1ID: "1", Track2ID: "2", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
		},
		Vehicles: []Vehicle{
			{
				ID:        "l1",
				Type:      "locomotive",
				Code:      "l1",
				PathID:    "1",
				PathIndex: 0,
				X:         0,
				Y:         0,
			},
			{
				ID:        "w1",
				Type:      "wagon",
				Code:      "w1",
				Color:     "#f59e0b",
				PathID:    "2",
				PathIndex: 4,
				X:         240,
				Y:         0,
			},
		},
		SelectedLocomotiveID: "l1",
		TargetPathID:         "2",
		TargetIndex:          4,
	}

	resp, err := buildMovementPlan(req)
	if err != nil {
		t.Fatalf("expected planner to adjust target to a free slot, got error: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}
	if len(resp.Timeline) == 0 {
		t.Fatal("expected non-empty timeline")
	}
	lastStep := resp.Timeline[len(resp.Timeline)-1]
	if len(lastStep) != 1 {
		t.Fatalf("expected one locomotive position, got %d", len(lastStep))
	}
	if lastStep[0].X != 200 || lastStep[0].Y != 0 {
		t.Fatalf("expected adjusted destination on free slot index 3 at (200,0), got (%.2f,%.2f)", lastStep[0].X, lastStep[0].Y)
	}
}

func TestBuildMovementPlanRejectsTargetTrackWhenNoReachableFreeSlotExists(t *testing.T) {
	req := PlanMovementRequest{
		GridSize: 40,
		Segments: []Segment{
			{ID: "1", From: Point{X: 0, Y: 0}, To: Point{X: 80, Y: 0}},
			{ID: "2", From: Point{X: 80, Y: 0}, To: Point{X: 200, Y: 0}},
		},
		TrackConnections: []MovementTrackConnection{
			{Track1ID: "1", Track2ID: "2", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
		},
		Vehicles: []Vehicle{
			{
				ID:        "l1",
				Type:      "locomotive",
				Code:      "l1",
				PathID:    "1",
				PathIndex: 0,
				X:         0,
				Y:         0,
			},
			{
				ID:        "w1",
				Type:      "wagon",
				Code:      "w1",
				Color:     "#f59e0b",
				PathID:    "2",
				PathIndex: 0,
				X:         80,
				Y:         0,
			},
			{
				ID:        "w2",
				Type:      "wagon",
				Code:      "w2",
				Color:     "#0ea5e9",
				PathID:    "2",
				PathIndex: 1,
				X:         120,
				Y:         0,
			},
			{
				ID:        "w3",
				Type:      "wagon",
				Code:      "w3",
				Color:     "#22c55e",
				PathID:    "2",
				PathIndex: 2,
				X:         160,
				Y:         0,
			},
		},
		SelectedLocomotiveID: "l1",
		TargetPathID:         "2",
		TargetIndex:          2,
	}

	_, err := buildMovementPlan(req)
	if err == nil {
		t.Fatal("expected clear rejection when target track has no reachable free slots")
	}
	if err.Error() != "Target track is blocked: no reachable free slot on track_id=2 requested_index=2 occupied_indices=[0 1 2]." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildMovementPlanChoosesAlternateEntrySideForPartiallyOccupiedTargetTrack(t *testing.T) {
	req := PlanMovementRequest{
		GridSize: 40,
		Segments: []Segment{
			{ID: "1", From: Point{X: -80, Y: 0}, To: Point{X: 0, Y: 0}},
			{ID: "2", From: Point{X: 0, Y: 0}, To: Point{X: 80, Y: -80}},
			{ID: "3", From: Point{X: 0, Y: 0}, To: Point{X: 80, Y: 200}},
			{ID: "4", From: Point{X: 80, Y: -80}, To: Point{X: 160, Y: 0}},
			{ID: "5", From: Point{X: 80, Y: 200}, To: Point{X: 160, Y: 200}},
			{ID: "8", From: Point{X: 160, Y: 0}, To: Point{X: 160, Y: 200}},
		},
		TrackConnections: []MovementTrackConnection{
			{Track1ID: "1", Track2ID: "2", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "1", Track2ID: "3", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "2", Track2ID: "4", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "3", Track2ID: "5", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "4", Track2ID: "8", Track1Side: "end", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "5", Track2ID: "8", Track1Side: "end", Track2Side: "end", ConnectionType: "switch"},
		},
		Vehicles: []Vehicle{
			{
				ID:        "l1",
				Type:      "locomotive",
				Code:      "l1",
				PathID:    "1",
				PathIndex: 1,
				X:         -40,
				Y:         0,
			},
			{
				ID:        "w1",
				Type:      "wagon",
				Code:      "w1",
				Color:     "#f59e0b",
				PathID:    "8",
				PathIndex: 2,
				X:         160,
				Y:         80,
			},
			{
				ID:        "w2",
				Type:      "wagon",
				Code:      "w2",
				Color:     "#0ea5e9",
				PathID:    "8",
				PathIndex: 3,
				X:         160,
				Y:         120,
			},
		},
		SelectedLocomotiveID: "l1",
		TargetPathID:         "8",
		TargetIndex:          4,
	}

	resp, err := buildMovementPlan(req)
	if err != nil {
		t.Fatalf("expected alternate entry side to keep index 4 reachable, got error: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}
	if len(resp.Timeline) == 0 {
		t.Fatal("expected non-empty timeline")
	}
	lastStep := resp.Timeline[len(resp.Timeline)-1]
	if len(lastStep) != 1 {
		t.Fatalf("expected one locomotive position, got %d", len(lastStep))
	}
	if lastStep[0].X != 160 || lastStep[0].Y != 160 {
		t.Fatalf("expected arrival to target track 8 index 4 at (160,160), got (%.2f,%.2f)", lastStep[0].X, lastStep[0].Y)
	}
}

func TestBuildMovementPlanFindsLoopRouteOnSameTrackBeyondBlockingWagons(t *testing.T) {
	req := PlanMovementRequest{
		GridSize: 40,
		Segments: []Segment{
			{ID: "7", From: Point{X: 0, Y: 0}, To: Point{X: 240, Y: 0}},
			{ID: "12", From: Point{X: 0, Y: 0}, To: Point{X: 120, Y: -120}},
			{ID: "18", From: Point{X: 120, Y: -120}, To: Point{X: 240, Y: 0}},
		},
		TrackConnections: []MovementTrackConnection{
			{Track1ID: "7", Track2ID: "12", Track1Side: "start", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "12", Track2ID: "18", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "18", Track2ID: "7", Track1Side: "end", Track2Side: "end", ConnectionType: "switch"},
		},
		Vehicles: []Vehicle{
			{
				ID:        "l1",
				Type:      "locomotive",
				Code:      "l1",
				PathID:    "7",
				PathIndex: 1,
				X:         40,
				Y:         0,
			},
			{
				ID:        "w1",
				Type:      "wagon",
				Code:      "w1",
				Color:     "#f59e0b",
				PathID:    "7",
				PathIndex: 2,
				X:         80,
				Y:         0,
			},
			{
				ID:        "w2",
				Type:      "wagon",
				Code:      "w2",
				Color:     "#0ea5e9",
				PathID:    "7",
				PathIndex: 3,
				X:         120,
				Y:         0,
			},
			{
				ID:        "w3",
				Type:      "wagon",
				Code:      "w3",
				Color:     "#22c55e",
				PathID:    "7",
				PathIndex: 4,
				X:         160,
				Y:         0,
			},
		},
		SelectedLocomotiveID: "l1",
		TargetPathID:         "7",
		TargetIndex:          5,
	}

	resp, err := buildMovementPlan(req)
	if err != nil {
		t.Fatalf("expected loop route back onto the same track, got error: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}
	if len(resp.Timeline) == 0 {
		t.Fatal("expected non-empty timeline")
	}
	lastStep := resp.Timeline[len(resp.Timeline)-1]
	if len(lastStep) != 1 {
		t.Fatalf("expected one locomotive position, got %d", len(lastStep))
	}
	if lastStep[0].X != 200 || lastStep[0].Y != 0 {
		t.Fatalf("expected arrival to track 7 index 5 at (200,0), got (%.2f,%.2f)", lastStep[0].X, lastStep[0].Y)
	}
}

func TestDijkstraTrackLoopPathWithGoalSideAvoidingTracks(t *testing.T) {
	path, route := dijkstraTrackLoopPathWithGoalSideAvoidingTracks(
		[]MovementTrackConnection{
			{Track1ID: "7", Track2ID: "12", Track1Side: "start", Track2Side: "start", ConnectionType: "switch"},
			{Track1ID: "12", Track2ID: "18", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
			{Track1ID: "18", Track2ID: "7", Track1Side: "end", Track2Side: "end", ConnectionType: "switch"},
		},
		"7",
		"end",
		map[string]struct{}{"7": {}},
	)
	if len(path) == 0 {
		t.Fatal("expected non-trivial loop path")
	}
	if len(route) == 0 {
		t.Fatal("expected non-trivial loop route edges")
	}
	expected := []string{"7", "12", "18", "7"}
	if len(path) != len(expected) {
		t.Fatalf("unexpected path length: got %v", path)
	}
	for i := range expected {
		if path[i] != expected[i] {
			t.Fatalf("unexpected loop path: got %v want %v", path, expected)
		}
	}
}
