package heuristic

import (
	"testing"

	"trains/backend/normalized"
)

func TestBuildFixedClassProblem(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "sorting-2", TrackIndex: 1},
			{WagonID: "w3", Color: "red", TrackID: "lead-2", TrackIndex: 2},
		},
	}

	problem, err := BuildFixedClassProblem(scheme, "red", "lead-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if problem.MainTrack.TrackID != "main-1" {
		t.Fatalf("expected main track main-1, got %s", problem.MainTrack.TrackID)
	}
	if problem.BypassTrack.TrackID != "bypass-1" {
		t.Fatalf("expected bypass track bypass-1, got %s", problem.BypassTrack.TrackID)
	}
	if problem.FormationTrack.TrackID != "lead-2" {
		t.Fatalf("expected formation track lead-2, got %s", problem.FormationTrack.TrackID)
	}
	if problem.BufferTrack.TrackID != "lead-1" {
		t.Fatalf("expected buffer track lead-1, got %s", problem.BufferTrack.TrackID)
	}
	if len(problem.TargetWagons) != 2 {
		t.Fatalf("expected 2 target wagons, got %d", len(problem.TargetWagons))
	}
	if len(problem.NonTargetWagons) != 1 {
		t.Fatalf("expected 1 non-target wagon, got %d", len(problem.NonTargetWagons))
	}
	if len(problem.WagonsByTrack["sorting-1"]) != 1 {
		t.Fatalf("expected wagon index on sorting-1")
	}
}

func TestBuildFixedClassProblemRejectsWrongTrackCounts(t *testing.T) {
	scheme := normalized.Scheme{
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "lead-1", TrackIndex: 1},
		},
	}

	if _, err := BuildFixedClassProblem(scheme, "red", "lead-1"); err == nil {
		t.Fatal("expected error for invalid fixed-class scheme")
	}
}

func TestCheckFixedClassFeasibilityFeasibleAutoSelection(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 6},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 8},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "red", TrackID: "sorting-2", TrackIndex: 1},
			{WagonID: "w3", Color: "red", TrackID: "lead-2", TrackIndex: 2},
			{WagonID: "w4", Color: "blue", TrackID: "lead-1", TrackIndex: 0},
		},
	}

	result := CheckFixedClassFeasibility(scheme, "red", 3, "")
	if !result.Feasible {
		t.Fatalf("expected feasible result, got reasons: %v", result.Reasons)
	}
	if result.ChosenFormationTrackID != "lead-2" {
		t.Fatalf("expected lead-2 as formation track, got %s", result.ChosenFormationTrackID)
	}
	if result.ChosenBufferTrackID != "lead-1" {
		t.Fatalf("expected lead-1 as buffer track, got %s", result.ChosenBufferTrackID)
	}
	if result.TargetCount != 3 {
		t.Fatalf("expected target count 3, got %d", result.TargetCount)
	}
	if result.RequiredTargetCount != 3 {
		t.Fatalf("expected required target count 3, got %d", result.RequiredTargetCount)
	}
}

func TestCheckFixedClassFeasibilityInfeasible(t *testing.T) {
	scheme := normalized.Scheme{
		SchemeID: 1,
		Tracks: []normalized.Track{
			{TrackID: "main-1", Type: "main", StorageAllowed: false, Capacity: 8},
			{TrackID: "bypass-1", Type: "bypass", StorageAllowed: false, Capacity: 6},
			{TrackID: "sorting-1", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "sorting-2", Type: "sorting", StorageAllowed: true, Capacity: 8},
			{TrackID: "lead-1", Type: "lead", StorageAllowed: true, Capacity: 2},
			{TrackID: "lead-2", Type: "lead", StorageAllowed: true, Capacity: 2},
		},
		Wagons: []normalized.Wagon{
			{WagonID: "w1", Color: "red", TrackID: "sorting-1", TrackIndex: 0},
			{WagonID: "w2", Color: "blue", TrackID: "sorting-2", TrackIndex: 1},
		},
	}

	result := CheckFixedClassFeasibility(scheme, "red", 3, "")
	if result.Feasible {
		t.Fatal("expected infeasible result")
	}
	if len(result.Reasons) == 0 {
		t.Fatal("expected infeasible reasons")
	}
}
