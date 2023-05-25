package app

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"

	"golang.org/x/exp/slog"
)

type App struct {
	repository RepositoryInterface
}

type RepositoryInterface interface {
	RepositoryScenarioInterface
	RepositoryScenarioItertionInterface
	RepositoryScenarioItertionRuleInterface
	RepositoryScenarioPublicationInterface

	// Data models & scenarios
	GetDataModel(ctx context.Context, orgID string) (models.DataModel, error)

	// Decisions
	StoreDecision(ctx context.Context, orgID string, decision Decision) (Decision, error)
	GetDecision(ctx context.Context, orgID string, decisionID string) (Decision, error)
	ListDecisions(ctx context.Context, orgID string) ([]Decision, error)

	// Ingestion
	IngestObject(ctx context.Context, payload Payload, table models.Table, logger *slog.Logger) (err error)

	// DB field access
	GetDbField(ctx context.Context, readParams DbFieldReadParams) (interface{}, error)
}

func New(r RepositoryInterface) (*App, error) {
	return &App{repository: r}, nil
}

// Sentinel errors that the repository can use
// We define those here because we can't import the repository package in the app itself
var (
	ErrNotFoundInRepository      = fmt.Errorf("item not found in repository: %w", models.NotFoundError)
	ErrScenarioIterationNotDraft = errors.New("scenario iteration is not a draft")
	ErrScenarioIterationNotValid = errors.New("scenario iteration is not valid for publication")
)
