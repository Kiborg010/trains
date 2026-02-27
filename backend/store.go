package main

import "sync"

var scenarioStore = struct {
	mu         sync.Mutex
	scenarios  map[string]Scenario
	executions map[string]Execution
}{
	scenarios:  map[string]Scenario{},
	executions: map[string]Execution{},
}
