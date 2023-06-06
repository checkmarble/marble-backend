package app

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
)

// Sentinel errors that the repository can use
// We define those here because we can't import the repository package in the app itself
var (
	ErrNotFoundInRepository      = fmt.Errorf("item not found in repository: %w", models.NotFoundError)
	ErrScenarioIterationNotDraft = errors.New("scenario iteration is not a draft")
	ErrScenarioIterationNotValid = errors.New("scenario iteration is not valid for publication")
)
