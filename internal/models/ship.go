package models

// Ship represents the structure of a ship JSON file
type Ship struct {
	PublicName       string                 `json:"publicName"`
	StrName          string                 `json:"strName"`
	StrRegID         string                 `json:"strRegID"`
	NCurrentWaypoint int                    `json:"nCurrentWaypoint"`
	FTimeEngaged     float64                `json:"fTimeEngaged"`
	FWearManeuver    float64                `json:"fWearManeuver"`
	FWearAccrued     float64                `json:"fWearAccrued"`
	ACOs             []interface{}          `json:"aCOs"`
	RawData          map[string]interface{} `json:"-"`
}
