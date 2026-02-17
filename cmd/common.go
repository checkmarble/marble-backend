package cmd

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

func GetDeploymentMetadata(ctx context.Context, repositories repositories.Repositories) (models.Metadata, error) {
	// Get deployment ID from Marble DB
	executor, err := repositories.ExecutorGetter.GetExecutor(
		ctx,
		models.DATABASE_SCHEMA_TYPE_MARBLE,
		nil,
	)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return models.Metadata{}, errors.Wrap(err, "failed to get executor from Marble DB")
	}
	deploymentMetadata, err := repositories.MarbleDbRepository.GetMetadata(ctx, executor, nil, models.MetadataKeyDeploymentID)
	if err != nil {
		utils.LogAndReportSentryError(ctx, err)
		return models.Metadata{}, errors.Wrap(err, "failed to get deployment ID from Marble DB")
	}

	// Expect the deployment ID to be set
	if deploymentMetadata == nil {
		return models.Metadata{}, errors.Wrap(models.NotFoundError, "deployment ID not found")
	}

	return *deploymentMetadata, nil
}

// IsWebhookSystemMigrated checks if the webhook system has been migrated to the new internal system.
// Returns false if the metadata key is not found (pre-migration state).
func IsWebhookSystemMigrated(ctx context.Context, repositories repositories.Repositories) bool {
	executor, err := repositories.ExecutorGetter.GetExecutor(
		ctx,
		models.DATABASE_SCHEMA_TYPE_MARBLE,
		nil,
	)
	if err != nil {
		return false
	}

	metadata, err := repositories.MarbleDbRepository.GetMetadata(ctx, executor, nil, models.MetadataKeyWebhookSystemMigrated)
	if err != nil || metadata == nil {
		return false
	}

	return metadata.Value == "true"
}
