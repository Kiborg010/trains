package main

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Segment struct {
	ID   string `json:"id"`
	Type string `json:"type,omitempty"`
	From Point  `json:"from"`
	To   Point  `json:"to"`
}

type Vehicle struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Code      string  `json:"code,omitempty"`
	Color     string  `json:"color"`
	PathID    string  `json:"pathId,omitempty"`
	PathIndex int     `json:"pathIndex"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
}

type Coupling struct {
	ID string `json:"id"`
	A  string `json:"a"`
	B  string `json:"b"`
}

type Slot struct {
	ID string
	X  float64
	Y  float64
}

type ValidateCouplingRequest struct {
	GridSize           float64    `json:"gridSize"`
	Segments           []Segment  `json:"segments"`
	Vehicles           []Vehicle  `json:"vehicles"`
	Couplings          []Coupling `json:"couplings"`
	SelectedVehicleIDs []string   `json:"selectedVehicleIds"`
}

type ValidateCouplingResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type PlanMovementRequest struct {
	GridSize             float64    `json:"gridSize"`
	Segments             []Segment  `json:"segments"`
	TrackConnections     []MovementTrackConnection `json:"trackConnections,omitempty"`
	Vehicles             []Vehicle  `json:"vehicles"`
	Couplings            []Coupling `json:"couplings"`
	SelectedLocomotiveID string     `json:"selectedLocomotiveId"`
	TargetPathID         string     `json:"targetPathId"`
	TargetIndex          int        `json:"targetIndex"`
}

type Position struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

type PathSlot struct {
	PathID string
	Index  int
	X      float64
	Y      float64
}

type MovementTrackConnection struct {
	Track1ID       string `json:"track1_id"`
	Track2ID       string `json:"track2_id"`
	Track1Side     string `json:"track1_side"`
	Track2Side     string `json:"track2_side"`
	ConnectionType string `json:"connection_type,omitempty"`
}

type PlanMovementResponse struct {
	OK          bool         `json:"ok"`
	Message     string       `json:"message,omitempty"`
	Timeline    [][]Position `json:"timeline,omitempty"`
	CellsPassed int          `json:"cellsPassed,omitempty"`
}

type PlaceVehicleRequest struct {
	GridSize     float64   `json:"gridSize"`
	Segments     []Segment `json:"segments"`
	Vehicles     []Vehicle `json:"vehicles"`
	VehicleType  string    `json:"vehicleType"`
	Color        string    `json:"color,omitempty"`
	TargetPathID string    `json:"targetPathId"`
	TargetIndex  int       `json:"targetIndex"`
}

type PlaceVehicleResponse struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message,omitempty"`
	Vehicle *Vehicle `json:"vehicle,omitempty"`
}

type ResolveVehiclesRequest struct {
	GridSize        float64    `json:"gridSize"`
	Segments        []Segment  `json:"segments"`
	Vehicles        []Vehicle  `json:"vehicles"`
	Couplings       []Coupling `json:"couplings"`
	MovedVehicleIDs []string   `json:"movedVehicleIds"`
	StrictCouplings bool       `json:"strictCouplings"`
}

type ResolveVehiclesResponse struct {
	OK       bool      `json:"ok"`
	Message  string    `json:"message,omitempty"`
	Vehicles []Vehicle `json:"vehicles,omitempty"`
}

type RuntimeState struct {
	Segments  []Segment          `json:"segments"`
	Vehicles  []Vehicle          `json:"vehicles"`
	Couplings []Coupling         `json:"couplings"`
	Paths     []RuntimePathState `json:"paths,omitempty"`
}

type RuntimePathState struct {
	ID         string   `json:"id"`
	Capacity   int      `json:"capacity"`
	VehicleIDs []string `json:"vehicleIds,omitempty"`
	Neighbors  []string `json:"neighbors,omitempty"`
}

type LayoutOperationRequest struct {
	GridSize float64      `json:"gridSize"`
	State    RuntimeState `json:"state"`
	Action   string       `json:"action"`

	From               *Point   `json:"from,omitempty"`
	To                 *Point   `json:"to,omitempty"`
	IDs                []string `json:"ids,omitempty"`
	SelectedVehicleIDs []string `json:"selectedVehicleIds,omitempty"`
	VehicleType        string   `json:"vehicleType,omitempty"`
	Color              string   `json:"color,omitempty"`
	TargetPathID       string   `json:"targetPathId,omitempty"`
	TargetIndex        int      `json:"targetIndex,omitempty"`
	MovedVehicleIDs    []string `json:"movedVehicleIds,omitempty"`
	StrictCouplings    bool     `json:"strictCouplings,omitempty"`
}

type LayoutOperationResponse struct {
	OK      bool         `json:"ok"`
	Message string       `json:"message,omitempty"`
	State   RuntimeState `json:"state"`
}

type Execution struct {
	ID          string       `json:"id"`
	ScenarioID  string       `json:"scenarioId"`
	UserID      int          `json:"userId,omitempty"`
	Status      string       `json:"status"`
	CurrentStep int          `json:"currentStep"`
	State       RuntimeState `json:"state"`
	Log         []string     `json:"log"`
}

type RunScenarioResponse struct {
	OK        bool      `json:"ok"`
	Message   string    `json:"message,omitempty"`
	Execution Execution `json:"execution"`
}

type StepExecutionResponse struct {
	OK        bool      `json:"ok"`
	Message   string    `json:"message,omitempty"`
	Execution Execution `json:"execution"`
}
