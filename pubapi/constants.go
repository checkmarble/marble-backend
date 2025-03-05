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
	ErrInternalServerError = errors.New("unknown error, please contact your administrator")

	ErrFeatureDisabled = errors.New("feature is not enabled in your organization and requires a Marble license")
	ErrNotConfigured   = errors.New("feature in not configured in your organization")

	ErrInvalidId      = errors.New("provided resource ID is invalid")
	ErrInvalidPayload = errors.New("the provided payload failed validations")
	ErrNotFound       = errors.New("requested resource was not found")
)
