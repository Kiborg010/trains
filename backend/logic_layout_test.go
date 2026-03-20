package main

import (
	"testing"

	"trains/backend/normalized"
)

func testStringPtr(v string) *string { return &v }
func testIntPtr(v int) *int          { return &v }

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
	if err == nil || err.Error() != "Целевой слот занят." {
		t.Fatalf("expected shared endpoint occupancy to block placement, got %v", err)
	}
}

func TestValidateCouplingAllowsAdjacentSwitchAreaSlots(t *testing.T) {
	segments := []Segment{
		{ID: "22", From: Point{X: 0, Y: 0}, To: Point{X: 160, Y: 0}},
		{ID: "23", From: Point{X: -64, Y: 32}, To: Point{X: 0, Y: 0}},
		{ID: "18", From: Point{X: -64, Y: -32}, To: Point{X: 0, Y: 0}},
	}

	resp, err := validateCouplingInternal(ValidateCouplingRequest{
		GridSize: 32,
		Segments: segments,
		Vehicles: []Vehicle{
			{ID: "w1", Type: "wagon", PathID: "23", PathIndex: 1, X: -32, Y: 16},
			{ID: "w2", Type: "wagon", PathID: "22", PathIndex: 0, X: 0, Y: 0},
		},
		SelectedVehicleIDs: []string{"w1", "w2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected coupling to be allowed for adjacent switch-area slots, got %+v", resp)
	}
}

func TestApplyScenarioStepMovesWholeCoupledGroupForward(t *testing.T) {
	state := RuntimeState{
		Segments: []Segment{
			{ID: "22", From: Point{X: 0, Y: 0}, To: Point{X: 192, Y: 0}},
		},
		Vehicles: []Vehicle{
			{ID: "l1", Type: "locomotive", Code: "l1", PathID: "22", PathIndex: 0, X: 0, Y: 0},
			{ID: "w1", Type: "wagon", Code: "w1", PathID: "22", PathIndex: 1, X: 32, Y: 0},
			{ID: "w8", Type: "wagon", Code: "w8", PathID: "22", PathIndex: 2, X: 64, Y: 0},
		},
		Couplings: []Coupling{
			{ID: "c1", A: "l1", B: "w1"},
			{ID: "c2", A: "w1", B: "w8"},
		},
	}

	next, _, err := applyScenarioStep(state, normalized.ScenarioStep{
		StepType:  "move_loco",
		Object1ID: testStringPtr("l1"),
		ToTrackID: testStringPtr("22"),
		ToIndex:   testIntPtr(3),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byID := map[string]Vehicle{}
	for _, vehicle := range next.Vehicles {
		byID[vehicle.ID] = vehicle
	}
	if byID["l1"].PathID != "22" || byID["l1"].PathIndex != 3 {
		t.Fatalf("expected locomotive to move to 22:3, got %+v", byID["l1"])
	}
	if byID["w1"].PathID != "22" || byID["w1"].PathIndex != 4 {
		t.Fatalf("expected first wagon to move with the consist to 22:4, got %+v", byID["w1"])
	}
	if byID["w8"].PathID != "22" || byID["w8"].PathIndex != 5 {
		t.Fatalf("expected second wagon to move with the consist to 22:5, got %+v", byID["w8"])
	}
}

