package heuristic

import (
	"fmt"
	"sort"
	"strings"

	"trains/backend/normalized"
)

type FixedClassProblem struct {
	SchemeID        int
	TargetColor     string
	MainTrack       normalized.Track
	BypassTrack     normalized.Track
	SortingTracks   []normalized.Track
	LeadTracks      []normalized.Track
	FormationTrack  normalized.Track
	BufferTrack     normalized.Track
	TargetWagons    []normalized.Wagon
	NonTargetWagons []normalized.Wagon
	WagonsByTrack   map[string][]normalized.Wagon
}

func BuildFixedClassProblem(scheme normalized.Scheme, targetColor string, formationTrackID string) (FixedClassProblem, error) {
	targetColor = strings.TrimSpace(targetColor)
	if targetColor == "" {
		return FixedClassProblem{}, fmt.Errorf("target color is required")
	}

	mainTracks := make([]normalized.Track, 0)
	bypassTracks := make([]normalized.Track, 0)
	sortingTracks := make([]normalized.Track, 0)
	leadTracks := make([]normalized.Track, 0)

	for _, track := range scheme.Tracks {
		switch strings.TrimSpace(track.Type) {
		case "main":
			mainTracks = append(mainTracks, track)
		case "bypass":
			bypassTracks = append(bypassTracks, track)
		case "sorting":
			sortingTracks = append(sortingTracks, track)
		case "lead":
			leadTracks = append(leadTracks, track)
		}
	}

	sortTracks(mainTracks)
	sortTracks(bypassTracks)
	sortTracks(sortingTracks)
	sortTracks(leadTracks)

	if len(mainTracks) != 1 {
		return FixedClassProblem{}, fmt.Errorf("expected exactly 1 main track, got %d", len(mainTracks))
	}
	if len(bypassTracks) != 1 {
		return FixedClassProblem{}, fmt.Errorf("expected exactly 1 bypass track, got %d", len(bypassTracks))
	}
	if len(sortingTracks) != 2 {
		return FixedClassProblem{}, fmt.Errorf("expected exactly 2 sorting tracks, got %d", len(sortingTracks))
	}
	if len(leadTracks) != 2 {
		return FixedClassProblem{}, fmt.Errorf("expected exactly 2 lead tracks, got %d", len(leadTracks))
	}

	mainTrack := mainTracks[0]
	bypassTrack := bypassTracks[0]
	if mainTrack.StorageAllowed {
		return FixedClassProblem{}, fmt.Errorf("main track %s must not allow storage", mainTrack.TrackID)
	}
	if bypassTrack.StorageAllowed {
		return FixedClassProblem{}, fmt.Errorf("bypass track %s must not allow storage", bypassTrack.TrackID)
	}
	for _, track := range sortingTracks {
		if !track.StorageAllowed {
			return FixedClassProblem{}, fmt.Errorf("sorting track %s must allow storage", track.TrackID)
		}
	}
	for _, track := range leadTracks {
		if !track.StorageAllowed {
			return FixedClassProblem{}, fmt.Errorf("lead track %s must allow storage", track.TrackID)
		}
	}

	formationTrack, bufferTrack, err := chooseLeadTracks(leadTracks, formationTrackID)
	if err != nil {
		return FixedClassProblem{}, err
	}

	targetWagons := make([]normalized.Wagon, 0)
	nonTargetWagons := make([]normalized.Wagon, 0)
	wagonsByTrack := make(map[string][]normalized.Wagon)
	colors := map[string]struct{}{}
	for _, wagon := range scheme.Wagons {
		color := strings.TrimSpace(wagon.Color)
		if color == "" {
			return FixedClassProblem{}, fmt.Errorf("wagon %s has empty color", wagon.WagonID)
		}
		colors[color] = struct{}{}
		wagonsByTrack[wagon.TrackID] = append(wagonsByTrack[wagon.TrackID], wagon)
		if color == targetColor {
			targetWagons = append(targetWagons, wagon)
		} else {
			nonTargetWagons = append(nonTargetWagons, wagon)
		}
	}

	if len(colors) > 2 {
		return FixedClassProblem{}, fmt.Errorf("expected at most 2 wagon colors, got %d", len(colors))
	}
	if len(scheme.Wagons) > 0 && len(colors) < 2 {
		return FixedClassProblem{}, fmt.Errorf("expected exactly 2 wagon colors in scheme with wagons, got %d", len(colors))
	}
	if len(targetWagons) == 0 {
		return FixedClassProblem{}, fmt.Errorf("no wagons of target color %q found", targetColor)
	}

	for trackID := range wagonsByTrack {
		sort.Slice(wagonsByTrack[trackID], func(i, j int) bool {
			return wagonsByTrack[trackID][i].TrackIndex < wagonsByTrack[trackID][j].TrackIndex
		})
	}

	return FixedClassProblem{
		SchemeID:        scheme.SchemeID,
		TargetColor:     targetColor,
		MainTrack:       mainTrack,
		BypassTrack:     bypassTrack,
		SortingTracks:   append([]normalized.Track{}, sortingTracks...),
		LeadTracks:      append([]normalized.Track{}, leadTracks...),
		FormationTrack:  formationTrack,
		BufferTrack:     bufferTrack,
		TargetWagons:    append([]normalized.Wagon{}, targetWagons...),
		NonTargetWagons: append([]normalized.Wagon{}, nonTargetWagons...),
		WagonsByTrack:   wagonsByTrack,
	}, nil
}

func chooseLeadTracks(leadTracks []normalized.Track, formationTrackID string) (normalized.Track, normalized.Track, error) {
	if len(leadTracks) != 2 {
		return normalized.Track{}, normalized.Track{}, fmt.Errorf("exactly 2 lead tracks are required")
	}
	formationTrackID = strings.TrimSpace(formationTrackID)
	if formationTrackID == "" {
		return leadTracks[0], leadTracks[1], nil
	}
	if leadTracks[0].TrackID == formationTrackID {
		return leadTracks[0], leadTracks[1], nil
	}
	if leadTracks[1].TrackID == formationTrackID {
		return leadTracks[1], leadTracks[0], nil
	}
	return normalized.Track{}, normalized.Track{}, fmt.Errorf("formation track %s is not one of the lead tracks", formationTrackID)
}

func sortTracks(tracks []normalized.Track) {
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].TrackID < tracks[j].TrackID
	})
}
