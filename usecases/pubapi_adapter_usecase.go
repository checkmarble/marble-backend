package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type publicApiAdapterRepository interface {
	ListUsers(ctx context.Context, exec repositories.Executor, organizationId *uuid.UUID) ([]models.User, error)
	ListOrganizationTags(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID, target models.TagTarget, withCaseCount bool) ([]models.Tag, error)
	GetCaseReferents(ctx context.Context, exec repositories.Executor, caseIds []string) ([]models.CaseReferents, error)
}

type PublicApiAdapterUsecase struct {
	enforceSecurity security.EnforceSecurity
	repository      publicApiAdapterRepository
}

func (uc PublicApiAdapterUsecase) ListUsers(ctx context.Context, exec repositories.Executor) ([]models.User, error) {
	return uc.repository.ListUsers(ctx, exec, utils.Ptr(uc.enforceSecurity.OrgId()))
}

func (uc PublicApiAdapterUsecase) ListTags(ctx context.Context, exec repositories.Executor) ([]models.Tag, error) {
	return uc.repository.ListOrganizationTags(ctx, exec, uc.enforceSecurity.OrgId(), models.TagTargetCase, false)
}

func (uc PublicApiAdapterUsecase) GetCaseReferents(ctx context.Context, exec repositories.Executor, caseIds []string) ([]models.CaseReferents, error) {
	return uc.repository.GetCaseReferents(ctx, exec, caseIds)
}
