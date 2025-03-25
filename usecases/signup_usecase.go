package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
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

func (uc *SignupUsecase) HasAnOrganization(ctx context.Context) (bool, error) {
	exec := uc.executorFactory.NewExecutor()

	exists, err := uc.organizationRepository.HasOrganizations(ctx, exec)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (uc *SignupUsecase) HasAUser(ctx context.Context) (bool, error) {
	exec := uc.executorFactory.NewExecutor()

	exists, err := uc.usersRepository.HasUsers(ctx, exec)
	if err != nil {
		return false, err
	}
	return exists, nil
}
