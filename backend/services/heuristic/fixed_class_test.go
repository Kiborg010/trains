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
