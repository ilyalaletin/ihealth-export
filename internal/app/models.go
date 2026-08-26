package app

import "encoding/json"

type Sample struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	Kind           string          `json:"kind"`
	StartDate      string          `json:"start_date"`
	EndDate        string          `json:"end_date"`
	Value          *float64        `json:"value,omitempty"`
	TextValue      string          `json:"text_value,omitempty"`
	Unit           string          `json:"unit,omitempty"`
	ActivityType   *int64          `json:"activity_type,omitempty"`
	ActivityName   string          `json:"activity_name,omitempty"`
	SourceName     string          `json:"source_name,omitempty"`
	SourceBundleID string          `json:"source_bundle_id,omitempty"`
	DeviceName     string          `json:"device_name,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

type Profile struct {
	DateOfBirth   string `json:"date_of_birth,omitempty"`
	BiologicalSex string `json:"biological_sex,omitempty"`
	BloodType     string `json:"blood_type,omitempty"`
	Fitzpatrick   string `json:"fitzpatrick_skin_type,omitempty"`
	WheelchairUse string `json:"wheelchair_use,omitempty"`
}

type UploadBatch struct {
	DeviceID   string   `json:"device_id"`
	ExportedAt string   `json:"exported_at"`
	Type       string   `json:"type"`
	Samples    []Sample `json:"samples"`
	DeletedIDs []string `json:"deleted_ids"`
	Profile    *Profile `json:"profile,omitempty"`
}

type UploadResult struct {
	Accepted int `json:"accepted"`
	Deleted  int `json:"deleted"`
}
