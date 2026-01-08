package usecases

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type userSettingsRepository interface {
	GetCurrentUnavailability(context.Context, repositories.Executor, string) (*models.UserUnavailability, error)
	InsertUnavailability(context.Context, repositories.Executor, string, string, time.Time) error
	UpdateUnavailability(context.Context, repositories.Executor, uuid.UUID, time.Time) error
	DeleteUnavailability(context.Context, repositories.Executor, string) error
}

type UserSettingsUsecase struct {
	executorFactory executor_factory.ExecutorFactory
	enforceSecurity security.EnforceSecurity
	repository      userSettingsRepository
}

func (uc *UserSettingsUsecase) GetUnavailability(ctx context.Context) (*models.UserUnavailability, error) {
	userId := *uc.enforceSecurity.UserId()

	current, err := uc.repository.GetCurrentUnavailability(ctx, uc.executorFactory.NewExecutor(), userId)
	if err != nil {
		return nil, errors.Wrap(err, "could not check user's current availabilities")
	}

	return current, nil
}

func (uc *UserSettingsUsecase) SetUnavailability(ctx context.Context, until time.Time) error {
	if !until.After(time.Now()) {
		return errors.New("unavailability end must be in the future")
	}

	userId := *uc.enforceSecurity.UserId()

	current, err := uc.repository.GetCurrentUnavailability(ctx, uc.executorFactory.NewExecutor(), userId)
	if err != nil {
		return errors.Wrap(err, "could not check user's current availabilities")
	}

	if current == nil {
		return uc.repository.InsertUnavailability(ctx, uc.executorFactory.NewExecutor(), uc.enforceSecurity.OrgId().String(), userId, until)
	}

	return uc.repository.UpdateUnavailability(ctx, uc.executorFactory.NewExecutor(), current.Id, until)
}

func (uc *UserSettingsUsecase) DeleteUnavailability(ctx context.Context) error {
	userId := *uc.enforceSecurity.UserId()

	return uc.repository.DeleteUnavailability(ctx, uc.executorFactory.NewExecutor(), userId)
}
