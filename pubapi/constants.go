package pubapi

import (
	"errors"
)

const (
	LinkDecisions            = "decisions"
	LinkSanctionChecks       = "sanction_checks"
	LinkSanctionCheckMatches = "sanction_check_matches"
)

var (
	ErrInternalServerError = errors.New("server_error")

	ErrFeatureDisabled = errors.New("feature_disabled")
	ErrNotConfigured   = errors.New("feature_not_configured")

	ErrForbidden           = errors.New("forbidden")
	ErrNotFound            = errors.New("not_found")
	ErrInvalidPayload      = errors.New("invalid_payload")
	ErrConflict            = errors.New("conflict")
	ErrUnprocessableEntity = errors.New("unprocessable_entity")
	ErrTimeout             = errors.New("timeout")
)
