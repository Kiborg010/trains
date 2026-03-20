package normalized

import "encoding/json"

type Scheme struct {
	SchemeID         int               `json:"scheme_id"`
	Name             string            `json:"name"`
	Tracks           []Track           `json:"tracks,omitempty"`
	TrackConnections []TrackConnection `json:"track_connections,omitempty"`
	Wagons           []Wagon           `json:"wagons,omitempty"`
	Locomotives      []Locomotive      `json:"locomotives,omitempty"`
	Couplings        []Coupling        `json:"couplings,omitempty"`
}

type Track struct {
	TrackID        string  `json:"track_id"`
	SchemeID       int     `json:"scheme_id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	StartX         float64 `json:"start_x"`
	StartY         float64 `json:"start_y"`
	EndX           float64 `json:"end_x"`
	EndY           float64 `json:"end_y"`
	Capacity       int     `json:"capacity"`
	StorageAllowed bool    `json:"storage_allowed"`
}

type TrackConnection struct {
	ConnectionID   string `json:"connection_id"`
	SchemeID       int    `json:"scheme_id"`
	Track1ID       string `json:"track1_id"`
	Track2ID       string `json:"track2_id"`
	Track1Side     string `json:"track1_side"`
	Track2Side     string `json:"track2_side"`
	ConnectionType string `json:"connection_type"`
}

type Wagon struct {
	WagonID    string `json:"wagon_id"`
	SchemeID   int    `json:"scheme_id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	TrackID    string `json:"track_id"`
	TrackIndex int    `json:"track_index"`
}

type Locomotive struct {
	LocoID     string `json:"loco_id"`
	SchemeID   int    `json:"scheme_id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	TrackID    string `json:"track_id"`
	TrackIndex int    `json:"track_index"`
}

type Coupling struct {
	CouplingID string `json:"coupling_id"`
	SchemeID   int    `json:"scheme_id"`
	Object1ID  string `json:"object1_id"`
	Object2ID  string `json:"object2_id"`
}

type Scenario struct {
	ScenarioID                string         `json:"scenario_id"`
	SchemeID                  int            `json:"scheme_id"`
	Name                      string         `json:"name"`
	SourceHeuristicScenarioID *string        `json:"source_heuristic_scenario_id,omitempty"`
	Steps                     []ScenarioStep `json:"steps,omitempty"`
}

type ScenarioMetrics struct {
	ScenarioID           string `json:"scenario_id"`
	TotalLocoDistance    int    `json:"total_loco_distance"`
	TotalCouples         int    `json:"total_couples"`
	TotalDecouples       int    `json:"total_decouples"`
	TotalSwitchCrossings int    `json:"total_switch_crossings"`
}

type ScenarioStep struct {
	StepID      string          `json:"step_id"`
	ScenarioID  string          `json:"scenario_id"`
	StepOrder   int             `json:"step_order"`
	StepType    string          `json:"step_type"`
	FromTrackID *string         `json:"from_track_id,omitempty"`
	FromIndex   *int            `json:"from_index,omitempty"`
	ToTrackID   *string         `json:"to_track_id,omitempty"`
	ToIndex     *int            `json:"to_index,omitempty"`
	Object1ID   *string         `json:"object1_id,omitempty"`
	Object2ID   *string         `json:"object2_id,omitempty"`
	PayloadJSON json.RawMessage `json:"payload_json,omitempty"`
}

type HeuristicScenario struct {
	HeuristicScenarioID string                  `json:"heuristic_scenario_id"`
	SchemeID            int                     `json:"scheme_id"`
	Name                string                  `json:"name"`
	TargetColor         string                  `json:"target_color"`
	RequiredTargetCount int                     `json:"required_target_count"`
	FormationTrackID    string                  `json:"formation_track_id"`
	BufferTrackID       string                  `json:"buffer_track_id"`
	MainTrackID         string                  `json:"main_track_id"`
	Feasible            bool                    `json:"feasible"`
	Reasons             []string                `json:"reasons,omitempty"`
	MetricsJSON         json.RawMessage         `json:"metrics_json,omitempty"`
	Steps               []HeuristicScenarioStep `json:"steps,omitempty"`
}

type HeuristicScenarioStep struct {
	StepID              string `json:"step_id"`
	HeuristicScenarioID string `json:"heuristic_scenario_id"`
	StepOrder           int    `json:"step_order"`
	StepType            string `json:"step_type"`
	SourceTrackID       string `json:"source_track_id"`
	DestinationTrackID  string `json:"destination_track_id"`
	SourceSide          string `json:"source_side,omitempty"`
	WagonCount          int    `json:"wagon_count"`
	TargetColor         string `json:"target_color"`
	FormationTrackID    string `json:"formation_track_id"`
	BufferTrackID       string `json:"buffer_track_id"`
	MainTrackID         string `json:"main_track_id"`
}
