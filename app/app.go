package app

import "errors"

type App struct {
	repository RepositoryInterface
}

type RepositoryInterface interface {

	// Data models & scenarios
	GetDataModel(orgID string) (DataModel, error)
	GetScenario(orgID string, scenarioID string) (Scenario, error)

	// token validation
	GetOrganizationIDFromToken(token string) (orgID string, err error)

	// Decisions
	StoreDecision(orgID string, decision Decision) (id string, err error)
	GetDecision(orgID string, decisionID string) (Decision, error)

	// Ingestion
	IngestObject(orgID string, ingestPayload IngestPayload) (err error)
}

func New(r RepositoryInterface) (*App, error) {
	return &App{repository: r}, nil
}

// Sentinel errors that the repository can use
// We define those here because we can't import the repository package in the app itself
var ErrNotFoundInRepository = errors.New("item not found in repository")
