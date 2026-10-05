package usecases

import (
	"context"
	"strconv"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

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
	// ORGANIZATIONS_UPDATE currently allows changing tracking for all organizations on the instance.
	// Consider a dedicated instance-level permission if this scope needs to be restricted.
	if err := uc.enforceSecurity.Permission(models.ORGANIZATIONS_UPDATE); err != nil {
		return err
	}
	if uc.isMarbleSaas {
		return errors.Wrap(models.ForbiddenError, "usage tracking cannot be changed on Marble SaaS")
	}

	key := models.MetadataKeyUsageTrackingEnabled
	return uc.repository.UpsertMetadata(ctx, uc.executorFactory.NewExecutor(), models.Metadata{
		Key: key, Value: strconv.FormatBool(enabled),
	})
}
