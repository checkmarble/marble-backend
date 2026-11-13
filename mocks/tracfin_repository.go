package mocks

import (
	"context"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type TracfinRepository struct{ mock.Mock }

var _ repositories.TracfinRepository = (*TracfinRepository)(nil)

func (r *TracfinRepository) GetTracfinConfiguration(ctx context.Context, exec repositories.Executor, orgID string) (models.TracfinConfiguration, error) {
	args := r.Called(ctx, exec, orgID)
	return args.Get(0).(models.TracfinConfiguration), args.Error(1)
}
func (r *TracfinRepository) SaveTracfinConfiguration(ctx context.Context, exec repositories.Executor, orgID string, config models.TracfinConfiguration, expectedRevision int64) error {
	return r.Called(ctx, exec, orgID, config, expectedRevision).Error(0)
}
func (r *TracfinRepository) GetTracfinDraft(ctx context.Context, exec repositories.Executor, orgID, caseID string) (models.TracfinDraft, error) {
	args := r.Called(ctx, exec, orgID, caseID)
	return args.Get(0).(models.TracfinDraft), args.Error(1)
}
func (r *TracfinRepository) CreateTracfinDraft(ctx context.Context, exec repositories.Executor, orgID, caseID string, draft models.TracfinDraft) error {
	return r.Called(ctx, exec, orgID, caseID, draft).Error(0)
}
func (r *TracfinRepository) SaveTracfinDraft(ctx context.Context, exec repositories.Executor, orgID, caseID string, draft models.TracfinDraft, expectedRevision int64) error {
	return r.Called(ctx, exec, orgID, caseID, draft, expectedRevision).Error(0)
}
func (r *TracfinRepository) SaveTracfinExport(ctx context.Context, exec repositories.Executor, orgID, caseID string, export models.TracfinExport, expectedRevision int64, expectedPreviousExportID *string) error {
	return r.Called(ctx, exec, orgID, caseID, export, expectedRevision, expectedPreviousExportID).Error(0)
}

func (r *TracfinRepository) GetTracfinDecisionSources(ctx context.Context, exec repositories.Executor, orgID, caseID string, ids []string) ([]models.TracfinDecisionSource, error) {
	args := r.Called(ctx, exec, orgID, caseID, ids)
	return args.Get(0).([]models.TracfinDecisionSource), args.Error(1)
}
