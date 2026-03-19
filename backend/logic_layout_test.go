package main

import "testing"

func TestApplyLayoutOperationAddSegmentAssignsNumericID(t *testing.T) {
	state := RuntimeState{}
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
	state := RuntimeState{
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

func TestArePathSlotsEquivalentForConnectedEndpoints(t *testing.T) {
	slots := collectPathSlotsWithConnections([]Segment{
		{ID: "22", From: Point{X: 0, Y: 0}, To: Point{X: 96, Y: 0}},
		{ID: "23", From: Point{X: 96, Y: 0}, To: Point{X: 160, Y: 0}},
	}, 32, []MovementTrackConnection{
		{Track1ID: "22", Track2ID: "23", Track1Side: "end", Track2Side: "start", ConnectionType: "serial"},
	})

	if !arePathSlotsEquivalent(slots, "22", 3, "23", 0) {
		t.Fatal("expected connected end/start slots to be treated as the same logical junction slot")
	}
}

func TestPlaceVehicleTreatsSharedEndpointAsOccupied(t *testing.T) {
	segments := []Segment{
		{ID: "22", From: Point{X: 0, Y: 0}, To: Point{X: 96, Y: 0}},
		{ID: "23", From: Point{X: 96, Y: 0}, To: Point{X: 160, Y: 0}},
	}

	_, err := placeVehicleInternal(PlaceVehicleRequest{
		GridSize:     32,
		Segments:     segments,
		VehicleType:  "locomotive",
		TargetPathID: "23",
		TargetIndex:  0,
		Vehicles: []Vehicle{
			{ID: "l1", Type: "locomotive", PathID: "22", PathIndex: 3, X: 96, Y: 0},
		},
	})
	if err == nil || err.Error() != "Target slot is occupied." {
		t.Fatalf("expected shared endpoint occupancy to block placement, got %v", err)
	}
}

