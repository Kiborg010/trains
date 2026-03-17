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

type FixedClassFeasibility struct {
	Feasible                bool
	Reasons                 []string
	ChosenFormationTrackID  string
	ChosenBufferTrackID     string
	TargetCount             int
	RequiredTargetCount     int
	AvailableBufferCapacity int
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

func CheckFixedClassFeasibility(scheme normalized.Scheme, targetColor string, requiredTargetCount int, formationTrackID string) FixedClassFeasibility {
	result := FixedClassFeasibility{
		Feasible:            false,
		Reasons:             []string{},
		RequiredTargetCount: requiredTargetCount,
	}

	if requiredTargetCount <= 0 {
		result.Reasons = append(result.Reasons, "required target count K must be positive")
		return result
	}

	targetColor = strings.TrimSpace(targetColor)
	if targetColor == "" {
		result.Reasons = append(result.Reasons, "target color is required")
		return result
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
		result.Reasons = append(result.Reasons, fmt.Sprintf("expected exactly 1 main track, got %d", len(mainTracks)))
	}
	if len(bypassTracks) != 1 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("expected exactly 1 bypass track, got %d", len(bypassTracks)))
	}
	if len(sortingTracks) != 2 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("expected exactly 2 sorting tracks, got %d", len(sortingTracks)))
	}
	if len(leadTracks) != 2 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("expected exactly 2 lead tracks, got %d", len(leadTracks)))
	}
	if len(result.Reasons) > 0 {
		return result
	}

	if mainTracks[0].StorageAllowed {
		result.Reasons = append(result.Reasons, fmt.Sprintf("main track %s must not allow storage", mainTracks[0].TrackID))
	}
	if bypassTracks[0].StorageAllowed {
		result.Reasons = append(result.Reasons, fmt.Sprintf("bypass track %s must not allow storage", bypassTracks[0].TrackID))
	}
	for _, track := range sortingTracks {
		if !track.StorageAllowed {
			result.Reasons = append(result.Reasons, fmt.Sprintf("sorting track %s must allow storage", track.TrackID))
		}
	}
	for _, track := range leadTracks {
		if !track.StorageAllowed {
			result.Reasons = append(result.Reasons, fmt.Sprintf("lead track %s must allow storage", track.TrackID))
		}
	}

	targetCount := 0
	colors := map[string]struct{}{}
	occupiedByTrack := map[string]int{}
	for _, wagon := range scheme.Wagons {
		color := strings.TrimSpace(wagon.Color)
		if color == "" {
			result.Reasons = append(result.Reasons, fmt.Sprintf("wagon %s has empty color", wagon.WagonID))
			continue
		}
		colors[color] = struct{}{}
		occupiedByTrack[wagon.TrackID]++
		if color == targetColor {
			targetCount++
		}
	}
	result.TargetCount = targetCount

	if len(colors) > 2 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("expected at most 2 wagon colors, got %d", len(colors)))
	}
	if len(scheme.Wagons) > 0 && len(colors) < 2 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("expected exactly 2 wagon colors in scheme with wagons, got %d", len(colors)))
	}
	if targetCount < requiredTargetCount {
		result.Reasons = append(result.Reasons, fmt.Sprintf("not enough target wagons: have %d, need %d", targetCount, requiredTargetCount))
	}

	formationTrack, bufferTrack, chooseReasons := selectFormationAndBufferTracks(leadTracks, occupiedByTrack, requiredTargetCount, formationTrackID)
	result.Reasons = append(result.Reasons, chooseReasons...)
	if formationTrack.TrackID != "" {
		result.ChosenFormationTrackID = formationTrack.TrackID
	}
	if bufferTrack.TrackID != "" {
		result.ChosenBufferTrackID = bufferTrack.TrackID
		bufferCapacity := bufferTrack.Capacity - occupiedByTrack[bufferTrack.TrackID]
		if bufferCapacity < 0 {
			bufferCapacity = 0
		}
		result.AvailableBufferCapacity = bufferCapacity
		if bufferCapacity <= 0 {
			result.Reasons = append(result.Reasons, fmt.Sprintf("buffer track %s has no available capacity", bufferTrack.TrackID))
		}
	}

	if len(result.Reasons) > 0 {
		return result
	}
	result.Feasible = true
	return result
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

func selectFormationAndBufferTracks(
	leadTracks []normalized.Track,
	occupiedByTrack map[string]int,
	requiredTargetCount int,
	formationTrackID string,
) (normalized.Track, normalized.Track, []string) {
	reasons := []string{}
	if len(leadTracks) != 2 {
		return normalized.Track{}, normalized.Track{}, []string{"exactly 2 lead tracks are required"}
	}

	formationTrackID = strings.TrimSpace(formationTrackID)
	if formationTrackID != "" {
		formationTrack, bufferTrack, err := chooseLeadTracks(leadTracks, formationTrackID)
		if err != nil {
			return normalized.Track{}, normalized.Track{}, []string{err.Error()}
		}
		if formationTrack.Capacity < requiredTargetCount {
			reasons = append(reasons, fmt.Sprintf(
				"formation track %s capacity %d is less than required target count %d",
				formationTrack.TrackID,
				formationTrack.Capacity,
				requiredTargetCount,
			))
		}
		return formationTrack, bufferTrack, reasons
	}

	candidates := make([]normalized.Track, 0, len(leadTracks))
	for _, track := range leadTracks {
		if track.Capacity >= requiredTargetCount {
			candidates = append(candidates, track)
		}
	}
	if len(candidates) == 0 {
		return normalized.Track{}, normalized.Track{}, []string{
			fmt.Sprintf("no lead track has capacity >= %d", requiredTargetCount),
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		leftOccupied := occupiedByTrack[candidates[i].TrackID]
		rightOccupied := occupiedByTrack[candidates[j].TrackID]
		if leftOccupied != rightOccupied {
			return leftOccupied < rightOccupied
		}
		if candidates[i].Capacity != candidates[j].Capacity {
			return candidates[i].Capacity > candidates[j].Capacity
		}
		return candidates[i].TrackID < candidates[j].TrackID
	})

	formationTrack := candidates[0]
	var bufferTrack normalized.Track
	for _, track := range leadTracks {
		if track.TrackID != formationTrack.TrackID {
			bufferTrack = track
			break
		}
	}
	if bufferTrack.TrackID == "" {
		return normalized.Track{}, normalized.Track{}, []string{"buffer track was not found"}
	}

	return formationTrack, bufferTrack, nil
}
