package main

import "fmt"

type NormalizedExecutionRuntime struct {
	LayoutState LayoutState
	Commands    []CommandSpec
}

func buildExecutionRuntimeFromNormalized(store Store, userID int, scenarioID string) (*NormalizedExecutionRuntime, error) {
	scenario, err := getLegacyScenarioFromNormalizedStore(store, userID, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("failed to build execution commands from normalized scenario: %w", err)
	}

	layout, err := getLegacyLayoutFromNormalizedStore(store, userID, scenario.LayoutID)
	if err != nil {
		return nil, fmt.Errorf("failed to build execution state from normalized scheme: %w", err)
	}

	return &NormalizedExecutionRuntime{
		LayoutState: cloneLayoutState(layout.State),
		Commands:    append([]CommandSpec{}, scenario.Commands...),
	}, nil
}
