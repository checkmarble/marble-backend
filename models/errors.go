package models

import (
	"fmt"

	"github.com/cockroachdb/errors"
)

// Base errors, related to default API status codes
var (
	// BadParameterError is rendered with the http status code 400
	BadParameterError = errors.New("bad parameter")

	// UnAuthorizedError is rendered with the http status code 401
	UnAuthorizedError = errors.New("unauthorized")

	// ForbiddenError is rendered with the http status code 403
	ForbiddenError = errors.New("forbidden")

	// NotFoundError is rendered with the http status code 404
	NotFoundError = errors.New("not found")

	// ConflictError is rendered with the http status code 409
	ConflictError = errors.New("duplicate value")
)

// Authentication related errors
var ErrUnknownUser = errors.Wrap(NotFoundError, "unknown user")

// DB related errors
var (
	ErrIgnoreRollBackError = errors.New("ignore rollback error")
)

// Scenario status related errors
var (
	// iteration edition
	ErrScenarioIterationNotDraft = errors.Wrap(BadParameterError, "scenario iteration is not a draft")

	// publication
	ErrScenarioIterationIsDraft = errors.Wrap(BadParameterError,
		"scenario iteration version a draft and cannot published")
	ErrScenarioIterationRequiresPreparation = errors.Wrap(
		BadParameterError,
		"scenario iteration requires preparation")
	ErrScenarioIterationNotValid = errors.Wrap(
		BadParameterError,
		"scenario iteration is not valid for publication")
	ErrDataPreparationServiceUnavailable = errors.Wrap(
		ConflictError,
		"data preparation service is unavailable: an index is being created in the client db schema")

	// execution
	ErrScenarioHasNoLiveVersion                         = errors.Wrap(BadParameterError, "scenario has no live version")
	ErrScenarioTriggerTypeAndTiggerObjectTypeMismatch   = errors.Wrap(BadParameterError, "scenario's trigger_type and provided trigger_object type are different")
	ErrScenarioTriggerConditionAndTriggerObjectMismatch = errors.Wrap(BadParameterError, "trigger_object does not match the scenario's trigger conditions")
	ErrInvalidAST                                       = errors.Wrap(BadParameterError, "invalid AST")
	ErrPanicInScenarioEvalution                         = errors.New("panic during scenario evaluation")
)

// ingestion and decision creating payload related errors
var FormatValidationError = errors.New("The input object is not valid")

// transfercheck errors
type FieldValidationError map[string]string

func (e FieldValidationError) Error() string {
	return fmt.Sprintf("%v", map[string]string(e))
}
