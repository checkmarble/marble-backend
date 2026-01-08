package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type MockCaseReviewWorkerRepository struct {
	mock.Mock
}

func (r *MockCaseReviewWorkerRepository) CreateCaseReviewFile(
	ctx context.Context,
	exec repositories.Executor,
	caseReview models.AiCaseReview,
) error {
	args := r.Called(ctx, exec, caseReview)
	return args.Error(0)
}

func (r *MockCaseReviewWorkerRepository) GetCaseReviewById(
	ctx context.Context,
	exec repositories.Executor,
	aiCaseReviewId uuid.UUID,
) (models.AiCaseReview, error) {
	args := r.Called(ctx, exec, aiCaseReviewId)
	return args.Get(0).(models.AiCaseReview), args.Error(1)
}

func (r *MockCaseReviewWorkerRepository) UpdateCaseReviewFile(
	ctx context.Context,
	exec repositories.Executor,
	caseReviewId uuid.UUID,
	status models.UpdateAiCaseReview,
) error {
	args := r.Called(ctx, exec, caseReviewId, status)
	return args.Error(0)
}

func (r *MockCaseReviewWorkerRepository) ListCaseReviewFiles(
	ctx context.Context,
	exec repositories.Executor,
	caseId uuid.UUID,
) ([]models.AiCaseReview, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).([]models.AiCaseReview), args.Error(1)
}

func (r *MockCaseReviewWorkerRepository) GetCaseById(ctx context.Context,
	exec repositories.Executor, caseId string,
) (models.Case, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).(models.Case), args.Error(1)
}

func (r *MockCaseReviewWorkerRepository) GetOrganizationById(
	ctx context.Context,
	exec repositories.Executor,
	organizationId uuid.UUID,
) (models.Organization, error) {
	args := r.Called(ctx, exec, organizationId)
	return args.Get(0).(models.Organization), args.Error(1)
}
