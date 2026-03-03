package params

import "encoding/json"

type CreateAsyncDecisionParams struct {
	TriggerObjectType string          `json:"trigger_object_type" binding:"required"`
	TriggerObject     json.RawMessage `json:"trigger_object" binding:"required"`
	ScenarioId        *string         `json:"scenario_id" binding:"omitempty,uuid"`
	Ingest            bool            `json:"ingest"`
}

type CreateAsyncDecisionBatchParams struct {
	TriggerObjectType string            `json:"trigger_object_type" binding:"required"`
	Objects           []json.RawMessage `json:"objects" binding:"required,min=1,max=100"`
	ScenarioId        *string           `json:"scenario_id" binding:"omitempty,uuid"`
	Ingest            bool              `json:"ingest"`
}
