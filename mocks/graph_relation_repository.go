package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type GraphRelationRepository struct {
	mock.Mock
}

func (r *GraphRelationRepository) ListGraphRelations(
	ctx context.Context, exec repositories.Executor, orgId uuid.UUID,
) ([]models.GraphRelation, error) {
	args := r.Called(ctx, exec, orgId)
	return args.Get(0).([]models.GraphRelation), args.Error(1)
}

func (r *GraphRelationRepository) GetGraphRelation(
	ctx context.Context, exec repositories.Executor, id uuid.UUID,
) (models.GraphRelation, error) {
	args := r.Called(ctx, exec, id)
	return args.Get(0).(models.GraphRelation), args.Error(1)
}

func (r *GraphRelationRepository) CreateGraphRelation(
	ctx context.Context, exec repositories.Executor, relation models.CreateGraphRelation,
) (models.GraphRelation, error) {
	args := r.Called(ctx, exec, relation)
	return args.Get(0).(models.GraphRelation), args.Error(1)
}

func (r *GraphRelationRepository) DeleteGraphRelation(
	ctx context.Context, exec repositories.Executor, id uuid.UUID,
) error {
	args := r.Called(ctx, exec, id)
	return args.Error(0)
}
