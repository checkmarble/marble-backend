package app

import (
	"context"
	"errors"

	"golang.org/x/exp/slog"
)

type Repositories struct {
	organizationsRepo          RepositoryOrganizations
	scenariosRepo              RepositoryScenarios
	scenarioIterationsRepo     RepositoryScenarioItertions
	scenarioIterationRulesRepo RepositoryScenarioItertionRules
	scenarioPublicationsRepo   RepositoryScenarioPublications
	decisionsRepo              RepositoryDecisions
	dataIngestionRepo          RepositoryDataIngestion
	commonRepo                 RepositoryCommon
}

type App struct {
	repository   Repository
	repositories Repositories
}

type RepositoryDataIngestion interface {
	IngestObject(ctx context.Context, dynamicStructWithReader DynamicStructWithReader, table Table, logger *slog.Logger) (err error)
	GetDbField(ctx context.Context, readParams DbFieldReadParams) (interface{}, error)
}

type RepositoryCommon interface {
	GetOrganizationIDFromToken(ctx context.Context, token string) (orgID string, err error)
	GetDataModel(ctx context.Context, orgID string) (DataModel, error)
}

type Repository interface {
	RepositoryScenarios
	RepositoryScenarioItertions
	RepositoryScenarioItertionRules
	RepositoryScenarioPublications
	RepositoryOrganizations
	RepositoryDecisions
	RepositoryDataIngestion
	RepositoryCommon
} // bouger dans un folder "ports" ?

func New(r Repository) (*App, error) {
	return &App{
		repository: r, // à drop
		repositories: Repositories{
			organizationsRepo:          r,
			scenariosRepo:              r,
			scenarioIterationsRepo:     r,
			scenarioIterationRulesRepo: r,
			scenarioPublicationsRepo:   r,
			decisionsRepo:              r,
			dataIngestionRepo:          r,
			commonRepo:                 r,
		},
	}, nil
}

// Si je veux faire tourner des tests unitaires sur l'app, j'initialise seulement la partie des repositories dont j'ai besoin pour le test (et
// j'implémente/mock leurs méthodes):
// app := App{repositories: Repositories{dataIngestionRepo: mockDataIngestionRepo}}
// concrètement, j'évite le problème "je me trimbale une énorme interface impossible à mocker car 30+ méthodes à implémenter"

// Sentinel errors that the repository can use
// We define those here because we can't import the repository package in the app itself
var (
	ErrNotFoundInRepository      = errors.New("item not found in repository")
	ErrScenarioIterationNotDraft = errors.New("scenario iteration is not a draft")
	ErrScenarioIterationNotValid = errors.New("scenario iteration is not valid for publication")
)
