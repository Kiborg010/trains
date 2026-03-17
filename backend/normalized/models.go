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
	ScenarioID string         `json:"scenario_id"`
	SchemeID   int            `json:"scheme_id"`
	Name       string         `json:"name"`
	Steps      []ScenarioStep `json:"steps,omitempty"`
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
