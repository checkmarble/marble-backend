package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

type SignupUsecase struct {
	executorFactory        executor_factory.ExecutorFactory
	organizationRepository repositories.OrganizationRepository
	usersRepository        repositories.UserRepository
}

func NewSignupUsecase(
	executorFactory executor_factory.ExecutorFactory,
	organizationRepository repositories.OrganizationRepository,
	usersRepository repositories.UserRepository,
) SignupUsecase {
	return SignupUsecase{
		executorFactory:        executorFactory,
		organizationRepository: organizationRepository,
		usersRepository:        usersRepository,
	}
}

func (uc *SignupUsecase) HasAnOrganization(ctx context.Context) (bool, bool, error) {
	exec := uc.executorFactory.NewExecutor()

	exists, err := uc.organizationRepository.HasOrganizations(ctx, exec)
	if !uc.haveMigrationsRun(err) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}

	return true, exists, nil
}

func (uc *SignupUsecase) HasAUser(ctx context.Context) (bool, bool, error) {
	exec := uc.executorFactory.NewExecutor()

	exists, err := uc.usersRepository.HasUsers(ctx, exec)
	if !uc.haveMigrationsRun(err) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}

	return true, exists, nil
}

func (uc *SignupUsecase) haveMigrationsRun(err error) bool {
	if err == nil {
		return true
	}

	var pgerr *pgconn.PgError

	if errors.As(err, &pgerr) {
		return pgerr.Code != pgerrcode.UndefinedTable
	}
	return true
}
