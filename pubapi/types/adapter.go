package types

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type PublicApiDataAdapter interface {
	ListUsers(ctx context.Context, exec repositories.Executor) ([]models.User, error)
	ListTags(ctx context.Context, exec repositories.Executor) ([]models.Tag, error)
	GetCaseReferents(ctx context.Context, exec repositories.Executor, caseIds []string) ([]models.CaseReferents, error)
}
