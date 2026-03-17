package main

import (
	"fmt"
	"time"
)

func listLegacyLayoutsFromNormalized(userID int) ([]Layout, error) {
	return listLegacyLayoutsFromNormalizedStore(appStore, userID)
}

func listLegacyLayoutsFromNormalizedStore(store Store, userID int) ([]Layout, error) {
	schemes, err := store.ListNormalizedSchemes(userID)
	if err != nil {
		return nil, err
	}

	result := make([]Layout, 0, len(schemes))
	for _, scheme := range schemes {
		layout, err := getLegacyLayoutFromNormalizedStore(store, userID, scheme.SchemeID)
		if err != nil {
			return nil, err
		}
		result = append(result, *layout)
	}
	return result, nil
}

func getLegacyLayoutFromNormalized(userID int, schemeID int) (*Layout, error) {
	return getLegacyLayoutFromNormalizedStore(appStore, userID, schemeID)
}

func getLegacyLayoutFromNormalizedStore(store Store, userID int, schemeID int) (*Layout, error) {
	scheme, err := store.GetNormalizedScheme(schemeID, userID)
	if err != nil {
		return nil, fmt.Errorf("normalized scheme %d not found: %w", schemeID, err)
	}

	tracks, err := store.ListTracksByScheme(userID, schemeID)
	if err != nil {
		return nil, fmt.Errorf("failed to load normalized tracks for scheme %d: %w", schemeID, err)
	}
	connections, err := store.ListTrackConnectionsByScheme(userID, schemeID)
	if err != nil {
		return nil, fmt.Errorf("failed to load normalized track connections for scheme %d: %w", schemeID, err)
	}
	wagons, err := store.ListWagonsByScheme(userID, schemeID)
	if err != nil {
		return nil, fmt.Errorf("failed to load normalized wagons for scheme %d: %w", schemeID, err)
	}
	locomotives, err := store.ListLocomotivesByScheme(userID, schemeID)
	if err != nil {
		return nil, fmt.Errorf("failed to load normalized locomotives for scheme %d: %w", schemeID, err)
	}
	couplings, err := store.ListNormalizedCouplingsByScheme(userID, schemeID)
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
	return listLegacyScenariosFromNormalizedStore(appStore, userID)
}

func listLegacyScenariosFromNormalizedStore(store Store, userID int) ([]Scenario, error) {
	scenarios, err := store.ListNormalizedScenarios(userID)
	if err != nil {
		return nil, err
	}

	result := make([]Scenario, 0, len(scenarios))
	for _, normalizedScenario := range scenarios {
		scenario, err := getLegacyScenarioFromNormalizedStore(store, userID, normalizedScenario.ScenarioID)
		if err != nil {
			return nil, err
		}
		result = append(result, *scenario)
	}
	return result, nil
}

func getLegacyScenarioFromNormalized(userID int, scenarioID string) (*Scenario, error) {
	return getLegacyScenarioFromNormalizedStore(appStore, userID, scenarioID)
}

func getLegacyScenarioFromNormalizedStore(store Store, userID int, scenarioID string) (*Scenario, error) {
	scenario, err := store.GetNormalizedScenario(scenarioID, userID)
	if err != nil {
		return nil, fmt.Errorf("normalized scenario %s not found: %w", scenarioID, err)
	}

	steps, err := store.ListScenarioStepsByScenario(userID, scenarioID)
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
