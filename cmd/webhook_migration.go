package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
)

// MigrateConvoyWebhooks migrates webhooks from Convoy to the internal webhook system.
// This is a one-time migration that should run at server startup.
// It will be removed after a few releases.
//
// The migration:
// - Checks if already migrated (via metadata flag)
// - Fetches all webhooks from Convoy for all organizations
// - Creates them in the internal system with their secrets (in a transaction)
// - Skips non-HTTPS endpoints (logs a warning)
// - Sets the migration flag when complete (in the same transaction)
func MigrateConvoyWebhooks(ctx context.Context, repos repositories.Repositories, hasConvoyServerSetup bool) error {
	logger := utils.LoggerFromContext(ctx)

	// Check if already migrated
	if IsWebhookSystemMigrated(ctx, repos) {
		logger.InfoContext(ctx, "Webhook system already migrated, skipping migration")
		return nil
	}

	// Check if Convoy is configured
	if !hasConvoyServerSetup {
		logger.InfoContext(ctx, "Convoy not configured, marking webhook system as migrated")
		return markWebhookSystemMigratedWithTx(ctx, repos)
	}

	logger.InfoContext(ctx, "Starting webhook migration from Convoy to internal system")

	// Add timeout for Convoy API calls to prevent hanging on unreachable Convoy
	convoyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	exec, err := repos.ExecutorGetter.GetExecutor(ctx, models.DATABASE_SCHEMA_TYPE_MARBLE, nil)
	if err != nil {
		return errors.Wrap(err, "failed to get executor")
	}

	// Get all organizations (read outside transaction)
	orgs, err := repos.MarbleDbRepository.AllOrganizations(ctx, exec)
	if err != nil {
		return errors.Wrap(err, "failed to list organizations")
	}

	// Collect all webhooks to migrate (reads from Convoy, outside transaction)
	type webhookToMigrate struct {
		orgId   uuid.UUID
		webhook models.Webhook
	}
	var webhooksToMigrate []webhookToMigrate
	var totalSkipped int

	for _, org := range orgs {
		convoyWebhooks, err := repos.ConvoyRepository.ListWebhooks(convoyCtx, org.Id, null.String{})
		if err != nil {
			logger.WarnContext(ctx, "Failed to fetch Convoy webhooks for organization",
				"organization_id", org.Id,
				"error", err.Error())
			continue
		}

		for _, webhook := range convoyWebhooks {
			// Skip non-HTTPS endpoints
			if !strings.HasPrefix(strings.ToLower(webhook.Url), "https://") {
				logger.WarnContext(ctx, "Skipping non-HTTPS webhook during migration",
					"organization_id", org.Id,
					"webhook_id", webhook.Id,
					"url", webhook.Url)
				totalSkipped++
				continue
			}
			webhooksToMigrate = append(webhooksToMigrate, webhookToMigrate{orgId: org.Id, webhook: webhook})
		}
	}

	// Perform migration in a single transaction
	err = repos.ExecutorGetter.Transaction(ctx, models.DATABASE_SCHEMA_TYPE_MARBLE, nil,
		func(tx repositories.Transaction) error {
			for _, item := range webhooksToMigrate {
				if err := migrateWebhook(ctx, repos, tx, item.webhook); err != nil {
					return errors.Wrapf(err, "failed to migrate webhook %s for org %s",
						item.webhook.Id, item.orgId)
				}
			}

			// Mark migration as complete in the same transaction
			return createMigrationMetadata(ctx, repos, tx)
		},
	)
	if err != nil {
		return errors.Wrap(err, "webhook migration transaction failed")
	}

	logger.InfoContext(ctx, "Webhook migration completed",
		"total_migrated", len(webhooksToMigrate),
		"total_skipped", totalSkipped)

	return nil
}

func migrateWebhook(
	ctx context.Context,
	repos repositories.Repositories,
	tx repositories.Transaction,
	webhook models.Webhook,
) error {
	webhookId := uuid.Must(uuid.NewV7())
	now := time.Now()

	// Convert timeout
	httpTimeout := 30 // default
	if webhook.HttpTimeout != nil {
		httpTimeout = *webhook.HttpTimeout
	}

	// Ensure event_types is never nil (empty means subscribe to all)
	eventTypes := webhook.EventTypes
	if eventTypes == nil {
		eventTypes = []string{}
	}

	newWebhook := models.NewWebhook{
		Id:                       webhookId,
		OrganizationId:           webhook.OrganizationId,
		Url:                      webhook.Url,
		EventTypes:               eventTypes,
		HttpTimeoutSeconds:       httpTimeout,
		RateLimit:                webhook.RateLimit,
		RateLimitDurationSeconds: webhook.RateLimitDuration,
		Enabled:                  true,
		CreatedAt:                now,
		UpdatedAt:                now,
	}

	// Create the webhook
	if err := repos.MarbleDbRepository.CreateWebhook(ctx, tx, newWebhook); err != nil {
		return errors.Wrap(err, "failed to create webhook")
	}

	// Migrate active secrets (skip deleted/expired)
	for _, secret := range webhook.Secrets {
		if secret.DeletedAt != "" {
			continue // Skip deleted secrets
		}
		if secret.ExpiresAt != "" {
			expiresAt, err := time.Parse(time.RFC3339, secret.ExpiresAt)
			if err == nil && expiresAt.Before(now) {
				continue // Skip expired secrets
			}
		}

		var expiresAt *time.Time
		if secret.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, secret.ExpiresAt); err == nil {
				expiresAt = &t
			}
		}

		newSecret := models.NewWebhookSecret{
			Id:        uuid.Must(uuid.NewV7()),
			WebhookId: webhookId,
			Value:     secret.Value,
			CreatedAt: now,
			ExpiresAt: expiresAt,
		}

		if err := repos.MarbleDbRepository.AddWebhookSecret(ctx, tx, newSecret); err != nil {
			return errors.Wrap(err, "failed to add webhook secret")
		}
	}

	return nil
}

// createMigrationMetadata creates the metadata record marking the webhook system as migrated.
// Used within a transaction.
func createMigrationMetadata(ctx context.Context, repos repositories.Repositories, tx repositories.Transaction) error {
	metadata := models.Metadata{
		ID:        uuid.Must(uuid.NewV7()),
		CreatedAt: time.Now(),
		Key:       models.MetadataKeyWebhookSystemMigrated,
		Value:     "true",
	}
	return repos.MarbleDbRepository.CreateMetadata(ctx, tx, metadata)
}

// markWebhookSystemMigratedWithTx marks the webhook system as migrated in its own transaction.
// Used when Convoy is not configured.
func markWebhookSystemMigratedWithTx(ctx context.Context, repos repositories.Repositories) error {
	return repos.ExecutorGetter.Transaction(ctx, models.DATABASE_SCHEMA_TYPE_MARBLE, nil,
		func(tx repositories.Transaction) error {
			return createMigrationMetadata(ctx, repos, tx)
		},
	)
}
