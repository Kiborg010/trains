package main

import "testing"

func TestApplyLayoutOperationAddSegmentAssignsNumericID(t *testing.T) {
	state := LayoutState{}
	from := Point{X: 0, Y: 0}
	to := Point{X: 64, Y: 0}

	next, _, err := applyLayoutOperation(LayoutOperationRequest{
		GridSize: 32,
		State:    state,
		Action:   "add_segment",
		From:     &from,
		To:       &to,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(next.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(next.Segments))
	}
	if next.Segments[0].ID != "1" {
		t.Fatalf("expected segment id 1, got %s", next.Segments[0].ID)
	}
}

func TestApplyLayoutOperationCoupleFailsForNonAdjacent(t *testing.T) {
	state := LayoutState{
		Segments: []Segment{
			{
				ID:   "1",
				From: Point{X: 0, Y: 0},
				To:   Point{X: 128, Y: 0},
			},
		},
		Vehicles: []Vehicle{
			{ID: "v1", Type: "wagon", PathID: "1", PathIndex: 0, X: 0, Y: 0},
			{ID: "v2", Type: "wagon", PathID: "1", PathIndex: 3, X: 96, Y: 0},
		},
	}

	_, _, err := applyLayoutOperation(LayoutOperationRequest{
		GridSize:           32,
		State:              state,
		Action:             "couple",
		SelectedVehicleIDs: []string{"v1", "v2"},
		StrictCouplings:    true,
		MovedVehicleIDs:    nil,
		VehicleType:        "",
		TargetPathID:       "",
	})
	if err == nil {
		t.Fatal("expected coupling validation error, got nil")
	}
}
