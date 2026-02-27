package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type ScoringRepository struct {
	mock.Mock
}

func (m *ScoringRepository) GetScoringSettings(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) (*models.ScoringSettings, error) {
	args := m.Called(ctx, exec, orgId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ScoringSettings), args.Error(1)
}

func (m *ScoringRepository) UpdateScoringSettings(ctx context.Context, exec repositories.Executor, settings models.ScoringSettings) (models.ScoringSettings, error) {
	args := m.Called(ctx, exec, settings)
	return args.Get(0).(models.ScoringSettings), args.Error(1)
}

func (m *ScoringRepository) ListScoringRulesets(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) ([]models.ScoringRuleset, error) {
	args := m.Called(ctx, exec, orgId)
	return args.Get(0).([]models.ScoringRuleset), args.Error(1)
}

func (m *ScoringRepository) GetScoringRuleset(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, entityType string, status models.ScoreRulesetStatus) (models.ScoringRuleset, error) {
	args := m.Called(ctx, exec, orgId, entityType, status)
	return args.Get(0).(models.ScoringRuleset), args.Error(1)
}

func (m *ScoringRepository) InsertScoringRulesetVersion(ctx context.Context, tx repositories.Transaction, orgId uuid.UUID, ruleset models.CreateScoringRulesetRequest) (models.ScoringRuleset, error) {
	args := m.Called(ctx, tx, orgId, ruleset)
	return args.Get(0).(models.ScoringRuleset), args.Error(1)
}

func (m *ScoringRepository) DeleteAllRulesetRules(ctx context.Context, tx repositories.Transaction, ruleset models.ScoringRuleset) error {
	args := m.Called(ctx, tx, ruleset)
	return args.Error(0)
}

func (m *ScoringRepository) InsertScoringRulesetVersionRule(ctx context.Context, tx repositories.Transaction, ruleset models.ScoringRuleset, rule models.CreateScoringRuleRequest) (models.ScoringRule, error) {
	args := m.Called(ctx, tx, ruleset, rule)
	return args.Get(0).(models.ScoringRule), args.Error(1)
}

func (m *ScoringRepository) CommitRuleset(ctx context.Context, exec repositories.Executor, ruleset models.ScoringRuleset) (models.ScoringRuleset, error) {
	args := m.Called(ctx, exec, ruleset)
	return args.Get(0).(models.ScoringRuleset), args.Error(1)
}

func (m *ScoringRepository) GetScoreHistory(ctx context.Context, exec repositories.Executor, entityRef models.ScoringEntityRef) ([]models.ScoringScore, error) {
	args := m.Called(ctx, exec, entityRef)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ScoringScore), args.Error(1)
}

func (m *ScoringRepository) GetActiveScore(ctx context.Context, exec repositories.Executor, entityRef models.ScoringEntityRef) (*models.ScoringScore, error) {
	args := m.Called(ctx, exec, entityRef)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ScoringScore), args.Error(1)
}

func (m *ScoringRepository) InsertScore(ctx context.Context, tx repositories.Transaction, req models.InsertScoreRequest) (models.ScoringScore, error) {
	args := m.Called(ctx, tx, req)
	return args.Get(0).(models.ScoringScore), args.Error(1)
}
