package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

var (
	appStore  Store
	jwtSecret string
)

func main() {
	_ = godotenv.Load()
	// Get JWT secret from environment or use default (NOT SAFE FOR PRODUCTION)
	jwtSecret = os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
		log.Println("WARNING: using default JWT secret. Set JWT_SECRET environment variable in production.")
	}

	// Initialize database store
	dbConnString := os.Getenv("DATABASE_URL")
	if dbConnString == "" {
		dbConnString = "postgres://user:password@localhost:5432/trains?sslmode=disable"
		log.Println("WARNING: using default database connection string. Set DATABASE_URL environment variable.")
	}

	var err error
	appStore, err = NewPostgresStore(dbConnString)
	if err != nil {
		log.Printf("failed to connect to database: %v", err)
		log.Println("falling back to in-memory store (data will be lost on restart)")
		appStore = NewInMemoryStore()
	}

	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/api/health", healthHandler)
	mux.HandleFunc("/api/auth/register", registerHandler)
	mux.HandleFunc("/api/auth/login", loginHandler)

	// Protected endpoints
	mux.Handle("/api/auth/me", authMiddleware(http.HandlerFunc(meHandler)))
	mux.Handle("/api/executions/", authMiddleware(http.HandlerFunc(executionByIDHandler)))
	mux.Handle("/api/normalized/schemes", authMiddleware(http.HandlerFunc(normalizedSchemesHandler)))
	mux.Handle("/api/normalized/schemes/", authMiddleware(http.HandlerFunc(normalizedSchemeByIDHandler)))
	mux.Handle("/api/normalized/heuristic/generate", authMiddleware(http.HandlerFunc(normalizedHeuristicGenerateHandler)))
	mux.Handle("/api/normalized/heuristic/generate-and-save", authMiddleware(http.HandlerFunc(normalizedHeuristicGenerateAndSaveHandler)))
	mux.Handle("/api/normalized/heuristic/scenarios", authMiddleware(http.HandlerFunc(normalizedHeuristicScenariosHandler)))
	mux.Handle("/api/normalized/heuristic/scenarios/", authMiddleware(http.HandlerFunc(normalizedHeuristicScenarioByIDHandler)))
	mux.Handle("/api/normalized/scenarios", authMiddleware(http.HandlerFunc(normalizedScenariosHandler)))
	mux.Handle("/api/normalized/scenarios/", authMiddleware(http.HandlerFunc(normalizedScenarioByIDHandler)))

	// Original endpoints (will need updates for user binding)
	mux.HandleFunc("/api/couplings/validate", validateCouplingHandler)
	mux.HandleFunc("/api/movement/plan", planMovementHandler)
	mux.HandleFunc("/api/vehicles/place", placeVehicleHandler)
	mux.HandleFunc("/api/vehicles/resolve", resolveVehiclesHandler)
	mux.HandleFunc("/api/layout/apply", layoutApplyHandler)
	// Scenario/layout persistence endpoints are protected and bound to account via JWT.

	handler := withCORS(mux)
	log.Println("backend started on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}
