package main

import (
	"fmt"
	"time"
)

func listLegacyLayoutsFromNormalized(userID int) ([]Layout, error) {
	schemes, err := appStore.ListNormalizedSchemes(userID)
	if err != nil {
		return nil, err
	}

	result := make([]Layout, 0, len(schemes))
	for _, scheme := range schemes {
		layout, err := getLegacyLayoutFromNormalized(userID, scheme.SchemeID)
		if err != nil {
			return nil, err
		}
		result = append(result, *layout)
	}
	return result, nil
}

func getLegacyLayoutFromNormalized(userID int, schemeID int) (*Layout, error) {
	scheme, err := appStore.GetNormalizedScheme(schemeID, userID)
	if err != nil {
		return nil, fmt.Errorf("normalized scheme %d not found: %w", schemeID, err)
	}

	tracks, err := appStore.ListTracksByScheme(userID, schemeID)
	if err != nil {
		return nil, fmt.Errorf("failed to load normalized tracks for scheme %d: %w", schemeID, err)
	}
	connections, err := appStore.ListTrackConnectionsByScheme(userID, schemeID)
	if err != nil {
		return nil, fmt.Errorf("failed to load normalized track connections for scheme %d: %w", schemeID, err)
	}
	wagons, err := appStore.ListWagonsByScheme(userID, schemeID)
	if err != nil {
		return nil, fmt.Errorf("failed to load normalized wagons for scheme %d: %w", schemeID, err)
	}
	locomotives, err := appStore.ListLocomotivesByScheme(userID, schemeID)
	if err != nil {
		return nil, fmt.Errorf("failed to load normalized locomotives for scheme %d: %w", schemeID, err)
	}
	couplings, err := appStore.ListNormalizedCouplingsByScheme(userID, schemeID)
	if err != nil {
		return nil, fmt.Errorf("failed to load normalized couplings for scheme %d: %w", schemeID, err)
	}

	adapted := BuildLegacyLayoutStateFromNormalizedSchemeDetails(
		*scheme,
		tracks,
		connections,
		wagons,
		locomotives,
		couplings,
	)

	if len(adapted.State.Segments) == 0 && len(tracks) > 0 {
		return nil, fmt.Errorf("failed to reconstruct legacy layout state for scheme %d", schemeID)
	}

	return &Layout{
		ID:        scheme.SchemeID,
		UserID:    userID,
		Name:      scheme.Name,
		State:     adapted.State,
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
	}, nil
}

func listLegacyScenariosFromNormalized(userID int) ([]Scenario, error) {
	scenarios, err := appStore.ListNormalizedScenarios(userID)
	if err != nil {
		return nil, err
	}

	result := make([]Scenario, 0, len(scenarios))
	for _, normalizedScenario := range scenarios {
		scenario, err := getLegacyScenarioFromNormalized(userID, normalizedScenario.ScenarioID)
		if err != nil {
			return nil, err
		}
		result = append(result, *scenario)
	}
	return result, nil
}

func getLegacyScenarioFromNormalized(userID int, scenarioID string) (*Scenario, error) {
	scenario, err := appStore.GetNormalizedScenario(scenarioID, userID)
	if err != nil {
		return nil, fmt.Errorf("normalized scenario %s not found: %w", scenarioID, err)
	}

	steps, err := appStore.ListScenarioStepsByScenario(userID, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("failed to load normalized scenario steps for scenario %s: %w", scenarioID, err)
	}

	adapted := BuildLegacyCommandsFromNormalizedScenarioDetails(*scenario, steps)
	commandsMap := make(map[string]CommandSpec, len(adapted.Commands))
	for _, command := range adapted.Commands {
		commandsMap[command.ID] = command
	}

	return &Scenario{
		ID:          scenario.ScenarioID,
		Name:        scenario.Name,
		UserID:      userID,
		LayoutID:    scenario.SchemeID,
		Commands:    adapted.Commands,
		CommandsMap: commandsMap,
	}, nil
}
