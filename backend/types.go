package main

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Segment struct {
	ID   string `json:"id"`
	From Point  `json:"from"`
	To   Point  `json:"to"`
}

type Vehicle struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Code      string  `json:"code,omitempty"`
	Color     string  `json:"color"`
	PathID    string  `json:"pathId,omitempty"`
	PathIndex int     `json:"pathIndex,omitempty"`
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

type LayoutState struct {
	Segments  []Segment   `json:"segments"`
	Vehicles  []Vehicle   `json:"vehicles"`
	Couplings []Coupling  `json:"couplings"`
	Paths     []PathState `json:"paths,omitempty"`
}

type PathState struct {
	ID         string   `json:"id"`
	Capacity   int      `json:"capacity"`
	VehicleIDs []string `json:"vehicleIds,omitempty"`
	Neighbors  []string `json:"neighbors,omitempty"`
}

type LayoutOperationRequest struct {
	GridSize float64     `json:"gridSize"`
	State    LayoutState `json:"state"`
	Action   string      `json:"action"`

	From               *Point   `json:"from,omitempty"`
	To                 *Point   `json:"to,omitempty"`
	IDs                []string `json:"ids,omitempty"`
	SelectedVehicleIDs []string `json:"selectedVehicleIds,omitempty"`
	VehicleType        string   `json:"vehicleType,omitempty"`
	TargetPathID       string   `json:"targetPathId,omitempty"`
	TargetIndex        int      `json:"targetIndex,omitempty"`
	MovedVehicleIDs    []string `json:"movedVehicleIds,omitempty"`
	StrictCouplings    bool     `json:"strictCouplings,omitempty"`
}

type LayoutOperationResponse struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	State   LayoutState `json:"state"`
}

type Scenario struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	UserID       int                    `json:"userId,omitempty"`
	LayoutID     int                    `json:"layoutId"`
	InitialState LayoutState            `json:"initialState,omitempty"` // legacy compatibility only
	Commands     []CommandSpec          `json:"commands"`
	CommandsMap  map[string]CommandSpec `json:"-"`
}

type CommandSpec struct {
	ID      string         `json:"id"`
	Order   int            `json:"order"`
	Type    string         `json:"type"`
	Payload CommandPayload `json:"payload"`
}

type CommandPayload struct {
	LocoID       string `json:"locoId,omitempty"`
	FromPathID   string `json:"fromPathId,omitempty"`
	FromIndex    int    `json:"fromIndex,omitempty"`
	ToPathID     string `json:"toPathId,omitempty"`
	ToIndex      int    `json:"toIndex,omitempty"`
	TargetPathID string `json:"targetPathId,omitempty"`
	TargetIndex  int    `json:"targetIndex,omitempty"`
	AID          string `json:"aId,omitempty"`
	BID          string `json:"bId,omitempty"`
}

type Execution struct {
	ID             string      `json:"id"`
	ScenarioID     string      `json:"scenarioId"`
	UserID         int         `json:"userId,omitempty"`
	Status         string      `json:"status"`
	CurrentCommand int         `json:"currentCommand"`
	State          LayoutState `json:"state"`
	Log            []string    `json:"log"`
}

type CreateScenarioRequest struct {
	Name     string `json:"name"`
	LayoutID int    `json:"layoutId"`
}

type CreateScenarioResponse struct {
	OK       bool     `json:"ok"`
	Message  string   `json:"message,omitempty"`
	Scenario Scenario `json:"scenario"`
}

type AddCommandRequest struct {
	Type    string         `json:"type"`
	Payload CommandPayload `json:"payload"`
}

type AddCommandResponse struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	Command CommandSpec `json:"command"`
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

type SaveLayoutRequest struct {
	Name  string      `json:"name"`
	State LayoutState `json:"state"`
}

type SaveLayoutResponse struct {
	OK      bool    `json:"ok"`
	Message string  `json:"message,omitempty"`
	Layout  *Layout `json:"layout,omitempty"`
}

type ListLayoutsResponse struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message,omitempty"`
	Layouts []Layout `json:"layouts,omitempty"`
}

type ListScenariosResponse struct {
	OK        bool       `json:"ok"`
	Message   string     `json:"message,omitempty"`
	Scenarios []Scenario `json:"scenarios,omitempty"`
}
