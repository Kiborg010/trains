package main

import (
	"math"
	"testing"
)

func TestGetSegmentSlotsSplitsTrackIntoEqualSegments(t *testing.T) {
	segment := Segment{
		ID:   "s1",
		From: Point{X: 0, Y: 0},
		To:   Point{X: 100, Y: 0},
	}

	slots := getSegmentSlots(segment, 40)
	if len(slots) != 4 {
		t.Fatalf("expected 4 slots for 3 equal segments, got %d", len(slots))
	}

	expectedX := []float64{0, 100.0 / 3.0, 200.0 / 3.0, 100}
	for i, point := range slots {
		if math.Abs(point.X-expectedX[i]) > 1e-9 || math.Abs(point.Y) > 1e-9 {
			t.Fatalf("unexpected slot %d: got (%.12f, %.12f)", i, point.X, point.Y)
		}
	}
}
