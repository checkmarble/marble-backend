package app

import (
	"errors"
	"marble/marble-backend/app/data_model"
	"marble/marble-backend/app/dynamic_reading"
	"marble/marble-backend/app/scenarios"
)

type App struct {
	repository RepositoryInterface
}

type RepositoryInterface interface {

	// Data models & scenarios
	GetDataModel(orgID string) (data_model.DataModel, error)
	GetScenario(orgID string, scenarioID string) (scenarios.Scenario, error)

	// token validation
	GetOrganizationIDFromToken(token string) (orgID string, err error)

	// Decisions
	StoreDecision(orgID string, decision scenarios.Decision) (id string, err error)
	GetDecision(orgID string, decisionID string) (scenarios.Decision, error)

	// Ingestion
	IngestObject(dynamicStructWithReader dynamic_reading.DynamicStructWithReader, table data_model.Table) (err error)
}

func New(r RepositoryInterface) (*App, error) {
	return &App{repository: r}, nil
}

// Sentinel errors that the repository can use
// We define those here because we can't import the repository package in the app itself
var ErrNotFoundInRepository = errors.New("item not found in repository")
