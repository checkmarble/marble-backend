package usecases

import (
	"context"
	"strconv"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/sync/semaphore"
)

type usageTrackingRepository interface {
	GetMetadata(context.Context, repositories.Executor, *uuid.UUID, models.MetadataKey) (*models.Metadata, error)
	UpsertMetadata(context.Context, repositories.Executor, models.Metadata) error
}

type UsageTrackingUsecase struct {
	repository      usageTrackingRepository
	executorFactory executor_factory.ExecutorFactory
	enforceSecurity security.EnforceSecurity
	isMarbleSaas    bool
	disableSegment  bool
	cache           *expirable.LRU[models.MetadataKey, bool]
	mu              *semaphore.Weighted
}

func NewUsageTrackingUsecase(repository usageTrackingRepository, executorFactory executor_factory.ExecutorFactory,
	isMarbleSaas, disableSegment bool,
) *UsageTrackingUsecase {
	return &UsageTrackingUsecase{
		repository:      repository,
		executorFactory: executorFactory,
		enforceSecurity: security.NewEnforceSecurity(models.Credentials{}),
		isMarbleSaas:    isMarbleSaas,
		disableSegment:  disableSegment,
		cache:           expirable.NewLRU[models.MetadataKey, bool](1, nil, 10*time.Minute),
		mu:              semaphore.NewWeighted(1),
	}
}

func (uc UsageTrackingUsecase) Enabled(ctx context.Context) bool {
	if uc.disableSegment {
		return false
	}
	if uc.isMarbleSaas {
		return true
	}
	key := models.MetadataKeyUsageTrackingEnabled
	if enabled, found := uc.cache.Get(key); found {
		return enabled
	}

	// Bound both locking and the DB read so slow metadata does not block tracking indefinitely.
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := uc.mu.Acquire(ctx, 1); err != nil {
		return false
	}
	defer uc.mu.Release(1)

	if enabled, found := uc.cache.Get(key); found {
		return enabled
	}

	metadata, err := uc.repository.GetMetadata(ctx, uc.executorFactory.NewExecutor(), nil, key)
	if err != nil {
		utils.LoggerFromContext(ctx).ErrorContext(ctx, "Failed to read usage tracking setting", "error", err)
		return false
	}

	enabled := true
	if metadata != nil {
		enabled, err = strconv.ParseBool(metadata.Value)
		if err != nil {
			utils.LoggerFromContext(ctx).ErrorContext(ctx, "Invalid usage tracking setting", "error", err)
			return false
		}
	}
	uc.cache.Add(key, enabled)
	return enabled
}

func (uc UsageTrackingUsecase) SetEnabled(ctx context.Context, enabled bool) error {
	if err := uc.enforceSecurity.Permission(models.ORGANIZATIONS_UPDATE); err != nil {
		return err
	}
	if uc.isMarbleSaas {
		return errors.Wrap(models.ForbiddenError, "usage tracking cannot be changed on Marble SaaS")
	}

	// The cache and lock are shared by the usecases created with credentials.
	if err := uc.mu.Acquire(ctx, 1); err != nil {
		return err
	}
	defer uc.mu.Release(1)

	key := models.MetadataKeyUsageTrackingEnabled
	if err := uc.repository.UpsertMetadata(ctx, uc.executorFactory.NewExecutor(), models.Metadata{
		Key: key, Value: strconv.FormatBool(enabled),
	}); err != nil {
		return err
	}
	uc.cache.Add(key, enabled)
	return nil
}
