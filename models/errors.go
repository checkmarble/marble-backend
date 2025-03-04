package models

import (
	"fmt"

	"github.com/cockroachdb/errors"
)

// Base errors, related to default API status codes
var (
	// BadParameterError is rendered with the http status code 400
	BadParameterError = errors.New("bad parameter")

	// UnprocessableEntityError is rendered with the http status code 422
	UnprocessableEntityError = errors.New("unprocessable entity")

	// UnAuthorizedError is rendered with the http status code 401
	UnAuthorizedError = errors.New("unauthorized")

	// ForbiddenError is rendered with the http status code 403
	ForbiddenError = errors.New("forbidden")

	// NotFoundError is rendered with the http status code 404
	NotFoundError = errors.New("not found")

	// ConflictError is rendered with the http status code 409
	ConflictError = errors.New("duplicate value")

	// MissingRequirement means this features required infrastructure or configuration that was not provided
	MissingRequirement = errors.New("missing requirement")
)

// Authentication related errors
var ErrUnknownUser = errors.Wrap(NotFoundError, "unknown user")

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
	ErrScenarioHasNoLiveVersion                       = errors.Wrap(BadParameterError, "scenario has no live version")
	ErrScenarioTriggerTypeAndTiggerObjectTypeMismatch = errors.Wrap(BadParameterError,
		"scenario's trigger_type and provided trigger_object type are different")
	ErrInvalidAST               = errors.Wrap(BadParameterError, "invalid AST")
	ErrPanicInScenarioEvalution = errors.New("panic during scenario evaluation")

	ErrTestRunAlreadyExist      = errors.Wrap(ConflictError, "there is an already existing testrun for this scenario")
	ErrNoTestRunFound           = errors.Wrap(NotFoundError, "there is no testrun for this scenario")
	ErrWrongIterationForTestRun = errors.Wrap(ConflictError, "the current scenario iteration is a live version and cannot be used")
)

// ingestion and decision creating payload related errors
var FormatValidationError = errors.New("The input object is not valid")

// transfercheck errors
type FieldValidationError map[string]string

func (e FieldValidationError) Error() string {
	return fmt.Sprintf("%v", map[string]string(e))
}

type (
	RequirementError       string
	RequirementErrorReason string
)

const (
	REQUIREMENT_OPEN_SANCTIONS RequirementError = "open_sanctions"

	REQUIREMENT_REASON_MISSING_CONFIGURATION = "missing_configuration"
	REQUIREMENT_REASON_INVALID_CONFIGURATION = "invalid_configuration"
	REQUIREMENT_REASON_HEALTHCHECK_FAILED    = "healthcheck_failed"
)

type MissingRequirementError struct {
	Requirement RequirementError
	Reason      RequirementErrorReason
	Err         error
}

func (err MissingRequirementError) Error() string {
	return string(err.Requirement)
}

func (e MissingRequirementError) Is(target error) bool {
	var req MissingRequirementError

	return errors.As(target, &req)
}
