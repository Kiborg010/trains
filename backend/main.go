package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", healthHandler)
	mux.HandleFunc("/api/couplings/validate", validateCouplingHandler)
	mux.HandleFunc("/api/movement/plan", planMovementHandler)
	mux.HandleFunc("/api/vehicles/place", placeVehicleHandler)
	mux.HandleFunc("/api/vehicles/resolve", resolveVehiclesHandler)
	mux.HandleFunc("/api/layout/apply", layoutApplyHandler)
	mux.HandleFunc("/api/scenarios", scenariosHandler)
	mux.HandleFunc("/api/scenarios/", scenarioByIDHandler)
	mux.HandleFunc("/api/executions/", executionByIDHandler)

	handler := withCORS(mux)
	log.Println("backend started on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}
