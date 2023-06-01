package repositories

import (
	"context"
	"marble/marble-backend/app"
)

type DecisionRepository interface {
	StoreDecision(ctx context.Context, orgID string, decision app.Decision) (app.Decision, error)
	GetDecision(ctx context.Context, orgID string, decisionID string) (app.Decision, error)
	ListDecisions(ctx context.Context, orgID string) ([]app.Decision, error)
}
