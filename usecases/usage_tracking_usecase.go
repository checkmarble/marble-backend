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
)

var usageTrackingCache = expirable.NewLRU[models.MetadataKey, bool](1, nil, 10*time.Minute)

type usageTrackingReaderRepository interface {
	GetMetadata(context.Context, repositories.Executor, *uuid.UUID, models.MetadataKey) (*models.Metadata, error)
}

type UsageTrackingReader struct {
	repository      usageTrackingReaderRepository
	executorFactory executor_factory.ExecutorFactory
	isMarbleSaas    bool
	disableSegment  bool
}

func (uc UsageTrackingReader) Enabled(ctx context.Context) bool {
	if uc.disableSegment {
		return false
	}
	if uc.isMarbleSaas {
		return true
	}
	key := models.MetadataKeyUsageTrackingEnabled
	if enabled, found := usageTrackingCache.Get(key); found {
		return enabled
	}

	metadata, err := uc.repository.GetMetadata(ctx, uc.executorFactory.NewExecutor(), nil, key)
	if err != nil {
		utils.LoggerFromContext(ctx).ErrorContext(ctx, "Failed to read usage tracking setting", "error", err)
		return false
	}

	enabled := false
	if metadata != nil {
		enabled, err = strconv.ParseBool(metadata.Value)
		if err != nil {
			utils.LoggerFromContext(ctx).ErrorContext(ctx, "Invalid usage tracking setting", "error", err)
			return false
		}
	}
	usageTrackingCache.Add(key, enabled)
	return enabled
}

type usageTrackingWriterRepository interface {
	UpsertMetadata(context.Context, repositories.Executor, models.Metadata) error
}

type UsageTrackingWriter struct {
	repository      usageTrackingWriterRepository
	executorFactory executor_factory.ExecutorFactory
	enforceSecurity security.EnforceSecurity
	isMarbleSaas    bool
}

func (uc UsageTrackingWriter) SetEnabled(ctx context.Context, enabled bool) error {
	if err := uc.enforceSecurity.Permission(models.ORGANIZATIONS_UPDATE); err != nil {
		return err
	}
	if uc.isMarbleSaas {
		return errors.Wrap(models.ForbiddenError, "usage tracking cannot be changed on Marble SaaS")
	}

	key := models.MetadataKeyUsageTrackingEnabled
	if err := uc.repository.UpsertMetadata(ctx, uc.executorFactory.NewExecutor(), models.Metadata{
		Key: key, Value: strconv.FormatBool(enabled),
	}); err != nil {
		return err
	}
	usageTrackingCache.Add(key, enabled)
	return nil
}
