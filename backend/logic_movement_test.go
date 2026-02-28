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
