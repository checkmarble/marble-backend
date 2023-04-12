package app

import (
	"context"
	"errors"
)

type App struct {
	repository RepositoryInterface
}

type RepositoryInterface interface {

	// Data models & scenarios
	GetDataModel(ctx context.Context, orgID string) (DataModel, error)
	GetScenario(ctx context.Context, orgID string, scenarioID string) (Scenario, error)
	PostScenario(ctx context.Context, orgID string, scenario Scenario) (Scenario, error)
	GetScenarios(ctx context.Context, orgID string) ([]Scenario, error)

	// token validation
	GetOrganizationIDFromToken(ctx context.Context, token string) (orgID string, err error)

	// Decisions
	StoreDecision(ctx context.Context, orgID string, decision Decision) (Decision, error)
	GetDecision(ctx context.Context, orgID string, decisionID string) (Decision, error)

	// Ingestion
	IngestObject(ctx context.Context, dynamicStructWithReader DynamicStructWithReader, table Table) (err error)

	// DB field access
	GetDbField(ctx context.Context, readParams DbFieldReadParams) (interface{}, error)

	// Organization
	GetOrganizations(ctx context.Context) ([]Organization, error)
	CreateOrganization(ctx context.Context, organisation CreateOrganizationInput) (Organization, error)
	GetOrganization(ctx context.Context, orgID string) (Organization, error)
}

func New(r RepositoryInterface) (*App, error) {
	return &App{repository: r}, nil
}

// Sentinel errors that the repository can use
// We define those here because we can't import the repository package in the app itself
var ErrNotFoundInRepository = errors.New("item not found in repository")
