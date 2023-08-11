package models

import (
	"errors"
	"fmt"
)

// BadParameterError is rendered with the http status code 400
var BadParameterError = errors.New("Bad Parameter")

// UnAuthorizedError is rendered with the http status code 401
var UnAuthorizedError = errors.New("UnAuthorized")

// ForbiddenError is rendered with the http status code 403
var ForbiddenError = errors.New("Forbidden")

// NotFoundError is rendered with the http status code 404
var NotFoundError = errors.New("Not found")

// DuplicateValue is rendered with the http status code 409
var DuplicateValueError = errors.New("Duplicate value")

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
var (
	NullFieldReadError   = errors.New("Null field read")
	NoRowsReadError      = errors.New("No row read")
	DivisionByZeroError  = errors.New("Division by zero")
)

var RuleExecutionAuthorizedErrors = []error{NullFieldReadError, NoRowsReadError, DivisionByZeroError}
