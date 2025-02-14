package dto

import "encoding/json"

type APIErrorResponse struct {
	Message   string         `json:"message"`
	Details   json.Marshaler `json:"details,omitempty"`
	ErrorCode ErrorCode      `json:"error_code"`
}

type ErrorCode string

const (
	// iteration edition related
	CanOnlyEditDraft ErrorCode = "can_only_edit_draft"

	// publication related
	CannotPublishDraft                ErrorCode = "scenario_iteration_is_draft"
	CannotPublishRequiresPreparation  ErrorCode = "scenario_iteration_requires_preparation"
	ScenarioIterationInvalid          ErrorCode = "scenario_iteration_is_invalid"
	DataPreparationServiceUnavailable ErrorCode = "data_preparation_service_unavailable"

	// decision related
	TriggerConditionNotMatched ErrorCode = "trigger_condition_not_matched"

	// ingestion related
	SchemaMismatchError ErrorCode = "data_does_not_match_schema"
	InvalidJSON         ErrorCode = "invalid_json"

	// general
	UnknownUser ErrorCode = "unknown_user"
)
