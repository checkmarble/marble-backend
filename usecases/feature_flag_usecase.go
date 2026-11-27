package usecases

import (
	"context"
	"strings"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type featureFlagRepository interface {
	ListFeatureFlags(ctx context.Context, exec repositories.Executor) (map[string]string, error)
	ReplaceFeatureFlags(ctx context.Context, exec repositories.Transaction, flags map[string]string) error
}

type enforceSecurityFeatureFlags interface {
	ReadFeatureFlags() error
	WriteFeatureFlags() error
}

type FeatureFlagUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	enforceSecurity    enforceSecurityFeatureFlags
	repository         featureFlagRepository
}

func (uc FeatureFlagUsecase) ListFeatureFlags(ctx context.Context) (map[string]string, error) {
	if err := uc.enforceSecurity.ReadFeatureFlags(); err != nil {
		return nil, err
	}

	stored, err := uc.repository.ListFeatureFlags(ctx, uc.executorFactory.NewExecutor())
	if err != nil {
		return nil, err
	}

	return withAllKnownFeatureFlags(stored), nil
}

func (uc FeatureFlagUsecase) ReplaceFeatureFlags(ctx context.Context, flags map[string]string) (map[string]string, error) {
	if err := uc.enforceSecurity.WriteFeatureFlags(); err != nil {
		return nil, err
	}

	toStore := make(map[string]string, len(flags))

	for flag, value := range flags {
		if !infra.IsKnownFeatureFlag(flag) {
			return nil, errors.Wrapf(models.BadParameterError, "unknown feature flag %s", flag)
		}
		if value == "" {
			continue
		}
		if err := validateFeatureFlagValue(value); err != nil {
			return nil, errors.Wrapf(err, "invalid value for feature flag %s", flag)
		}

		toStore[flag] = value
	}

	stored, err := executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (map[string]string, error) {
		if err := uc.repository.ReplaceFeatureFlags(ctx, tx, toStore); err != nil {
			return nil, err
		}

		return uc.repository.ListFeatureFlags(ctx, tx)
	})
	if err != nil {
		return nil, err
	}

	return withAllKnownFeatureFlags(stored), nil
}

func withAllKnownFeatureFlags(stored map[string]string) map[string]string {
	known := infra.KnownFeatureFlags()
	flags := make(map[string]string, len(known))

	for _, flag := range known {
		flags[flag] = stored[flag]
	}

	return flags
}

func validateFeatureFlagValue(value string) error {
	if value == "all" {
		return nil
	}

	for org := range strings.SplitSeq(value, ",") {
		if _, err := uuid.Parse(org); err != nil {
			return errors.Wrapf(models.BadParameterError, "%q is not 'all' or an organization ID", org)
		}
	}

	return nil
}
