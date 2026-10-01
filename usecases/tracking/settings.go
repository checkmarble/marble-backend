package tracking

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

type metadataRepository interface {
	GetMetadata(context.Context, repositories.Executor, *uuid.UUID, models.MetadataKey) (*models.Metadata, error)
	UpsertMetadata(context.Context, repositories.Executor, models.Metadata) error
}

type Settings struct {
	repository      metadataRepository
	executorFactory executor_factory.ExecutorFactory
	isMarbleSaas    bool
	cache           *expirable.LRU[models.MetadataKey, bool]
	mu              sync.Mutex
}

func NewSettings(repository metadataRepository, executorFactory executor_factory.ExecutorFactory, isMarbleSaas bool) *Settings {
	return &Settings{
		repository:      repository,
		executorFactory: executorFactory,
		isMarbleSaas:    isMarbleSaas,
		cache:           expirable.NewLRU[models.MetadataKey, bool](1, nil, 24*time.Hour),
	}
}

func (s *Settings) Enabled(ctx context.Context) bool {
	if s.isMarbleSaas {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := models.MetadataKeyUsageTrackingEnabled
	if enabled, found := s.cache.Get(key); found {
		return enabled
	}

	metadata, err := s.repository.GetMetadata(ctx, s.executorFactory.NewExecutor(), nil, key)
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
	s.cache.Add(key, enabled)
	return enabled
}

func (s *Settings) SetEnabled(ctx context.Context, enabled bool) error {
	if s.isMarbleSaas {
		return errors.Wrap(models.ForbiddenError, "usage tracking cannot be changed on Marble SaaS")
	}

	// Serialize reads and writes so an earlier read cannot overwrite the updated cache.
	s.mu.Lock()
	defer s.mu.Unlock()

	key := models.MetadataKeyUsageTrackingEnabled
	if err := s.repository.UpsertMetadata(ctx, s.executorFactory.NewExecutor(), models.Metadata{
		Key: key, Value: strconv.FormatBool(enabled),
	}); err != nil {
		return err
	}
	s.cache.Add(key, enabled)
	return nil
}
