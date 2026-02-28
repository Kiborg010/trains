package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func resetScenarioStoreForTests() {
	scenarioStore.mu.Lock()
	defer scenarioStore.mu.Unlock()
	scenarioStore.scenarios = map[string]Scenario{}
	scenarioStore.executions = map[string]Execution{}
}

func newTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/scenarios", scenariosHandler)
	mux.HandleFunc("/api/scenarios/", scenarioByIDHandler)
	mux.HandleFunc("/api/executions/", executionByIDHandler)
	return mux
}

func doJSONRequest(t *testing.T, mux *http.ServeMux, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode json: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestScenarioEndpointsStepExecution(t *testing.T) {
	resetScenarioStoreForTests()
	mux := newTestMux()

	initial := LayoutState{
		Segments: []Segment{
			{ID: "1", From: Point{X: 0, Y: 0}, To: Point{X: 128, Y: 0}},
		},
		Vehicles: []Vehicle{
			{ID: "l1", Type: "locomotive", PathID: "1", PathIndex: 0, X: 0, Y: 0},
		},
	}

	createRR := doJSONRequest(t, mux, http.MethodPost, "/api/scenarios", CreateScenarioRequest{
		Name:         "test",
		InitialState: initial,
	})
	if createRR.Code != http.StatusOK {
		t.Fatalf("create scenario status: %d", createRR.Code)
	}
	var createResp CreateScenarioResponse
	if err := json.Unmarshal(createRR.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if !createResp.OK || createResp.Scenario.ID == "" {
		t.Fatalf("unexpected create response: %+v", createResp)
	}

	addCommandRR := doJSONRequest(t, mux, http.MethodPost, "/api/scenarios/"+createResp.Scenario.ID+"/commands", AddCommandRequest{
		Type: "MOVE_LOCO",
		Payload: CommandPayload{
			LocoID:       "l1",
			TargetPathID: "1",
			TargetIndex:  2,
		},
	})
	if addCommandRR.Code != http.StatusOK {
		t.Fatalf("add command status: %d", addCommandRR.Code)
	}
	var addResp AddCommandResponse
	if err := json.Unmarshal(addCommandRR.Body.Bytes(), &addResp); err != nil {
		t.Fatalf("decode add command response: %v", err)
	}
	if !addResp.OK {
		t.Fatalf("add command failed: %+v", addResp)
	}

	runRR := doJSONRequest(t, mux, http.MethodPost, "/api/scenarios/"+createResp.Scenario.ID+"/run", nil)
	if runRR.Code != http.StatusOK {
		t.Fatalf("run scenario status: %d", runRR.Code)
	}
	var runResp RunScenarioResponse
	if err := json.Unmarshal(runRR.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("decode run response: %v", err)
	}
	if !runResp.OK || runResp.Execution.ID == "" {
		t.Fatalf("unexpected run response: %+v", runResp)
	}

	stepRR := doJSONRequest(t, mux, http.MethodPost, "/api/executions/"+runResp.Execution.ID+"/step", nil)
	if stepRR.Code != http.StatusOK {
		t.Fatalf("step execution status: %d", stepRR.Code)
	}
	var stepResp StepExecutionResponse
	if err := json.Unmarshal(stepRR.Body.Bytes(), &stepResp); err != nil {
		t.Fatalf("decode step response: %v", err)
	}
	if !stepResp.OK {
		t.Fatalf("step failed: %+v", stepResp)
	}
	if stepResp.Execution.CurrentCommand != 1 {
		t.Fatalf("expected current command 1, got %d", stepResp.Execution.CurrentCommand)
	}
	if stepResp.Execution.Status != "completed" {
		t.Fatalf("expected completed status, got %s", stepResp.Execution.Status)
	}
}
