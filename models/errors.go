package models

import (
	"errors"
	"fmt"
)

// BadParameterError is rendered with the http status code 400
var BadParameterError = errors.New("bad parameter")

// UnAuthorizedError is rendered with the http status code 401
var UnAuthorizedError = errors.New("unauthorized")

var (
	ErrUnknownUser = errors.New("unknown user")
)

// ForbiddenError is rendered with the http status code 403
var ForbiddenError = errors.New("forbidden")

// NotFoundError is rendered with the http status code 404
var NotFoundError = errors.New("not found")

// DuplicateValueError is rendered with the http status code 409
var DuplicateValueError = errors.New("duplicate value")

var ErrIgnoreRollBackError = errors.New("ignore rollback error")

var (
	PanicInScenarioEvalutionError = errors.New("panic during scenario evaluation")
	NotFoundInRepositoryError     = fmt.Errorf("item not found in repository: %w", NotFoundError)
)

var (
	ErrScenarioIterationNotDraft                          = fmt.Errorf("scenario iteration is not a draft %w", BadParameterError)
	ErrScenarioIterationNotValid                          = fmt.Errorf("scenario iteration is not valid for publication %w", BadParameterError)
	ScenarioHasNoLiveVersionError                         = fmt.Errorf("scenario has no live version %w", BadParameterError)
	ScenarioTriggerTypeAndTiggerObjectTypeMismatchError   = fmt.Errorf("scenario's trigger_type and provided trigger_object type are different %w", BadParameterError)
	ScenarioTriggerConditionAndTriggerObjectMismatchError = fmt.Errorf("trigger_object does not match the scenario's trigger conditions %w", BadParameterError)
)

var (
	FormatValidationError = errors.New("The input object is not valid")
)

// Rule execution related errors
var ErrRuntimeExpression = errors.New("expression runtime error")
var (
	NullFieldReadError        = fmt.Errorf("Null field read: %w", ErrRuntimeExpression)
	NoRowsReadError           = fmt.Errorf("No rows read %w", ErrRuntimeExpression)
	DivisionByZeroError       = fmt.Errorf("Division by zero %w", ErrRuntimeExpression)
	PayloadFieldNotFoundError = fmt.Errorf("Payload field not found %w", ErrRuntimeExpression)
)

var RuleExecutionAuthorizedErrors = []error{NullFieldReadError, NoRowsReadError, DivisionByZeroError, PayloadFieldNotFoundError}

func IsAuthorizedError(err error) bool {
	for _, authorizedError := range RuleExecutionAuthorizedErrors {
		if errors.Is(err, authorizedError) {
			return true
		}
	}
	return false
}

type PayloadValidationErrors struct {
	message string
	errors  map[string]string
}

func (p PayloadValidationErrors) Error() string {
	return p.message
}

func (p PayloadValidationErrors) Errors() map[string]string {
	return p.errors
}

func NewPayloadValidationErrors(message string, errors map[string]string) PayloadValidationErrors {
	return PayloadValidationErrors{
		message: message,
		errors:  errors,
	}
}
