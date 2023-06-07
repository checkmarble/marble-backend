package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type DecisionRepository interface {
	StoreDecision(ctx context.Context, orgID string, decision models.Decision) (models.Decision, error)
	GetDecision(ctx context.Context, orgID string, decisionID string) (models.Decision, error)
	ListDecisions(ctx context.Context, orgID string) ([]models.Decision, error)
}
