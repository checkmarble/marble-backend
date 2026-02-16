package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
	"github.com/pkg/errors"
)

type convoyWebhooksRepository interface {
	GetWebhook(ctx context.Context, webhookId string) (models.Webhook, error)
	ListWebhooks(ctx context.Context, organizationId uuid.UUID, partnerId null.String) ([]models.Webhook, error)
	RegisterWebhook(ctx context.Context, organizationId uuid.UUID, partnerId null.String,
		input models.WebhookRegister) (models.Webhook, error)
	UpdateWebhook(ctx context.Context, input models.Webhook) (models.Webhook, error)
	DeleteWebhook(ctx context.Context, webhookId string) error
}

type webhooksRepository interface {
	CreateWebhook(ctx context.Context, exec repositories.Executor, webhook models.NewWebhook) error
	GetWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.NewWebhook, error)
	ListWebhooks(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) ([]models.NewWebhook, error)
	UpdateWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID, update models.NewWebhookUpdate) error
	DeleteWebhook(ctx context.Context, exec repositories.Executor, id uuid.UUID) error
	AddWebhookSecret(ctx context.Context, exec repositories.Executor, secret models.NewWebhookSecret) error
	ListActiveWebhookSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID) ([]models.NewWebhookSecret, error)
	ListAllWebhookSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID) ([]models.NewWebhookSecret, error)
	GetWebhookSecret(ctx context.Context, exec repositories.Executor, secretId uuid.UUID) (models.NewWebhookSecret, error)
	RevokeWebhookSecret(ctx context.Context, exec repositories.Executor, secretId uuid.UUID) error
	ExpireWebhookSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID, expiresAt time.Time) error
	CountPermanentWebhookSecrets(ctx context.Context, exec repositories.Executor, webhookId uuid.UUID) (int, error)
}

type enforceSecurityWebhook interface {
	CanCreateWebhook(ctx context.Context, organizationId uuid.UUID, partnerId null.String) error
	CanReadWebhook(ctx context.Context, webhook models.Webhook) error
	CanModifyWebhook(ctx context.Context, webhook models.Webhook) error
}

type webhookEndpointValidator interface {
	ValidateEndpoint(ctx context.Context, url string) error
}

type WebhooksUsecase struct {
	enforceSecurity       enforceSecurityWebhook
	executorFactory       executor_factory.ExecutorFactory
	transactionFactory    executor_factory.TransactionFactory
	convoyRepository      convoyWebhooksRepository
	webhookRepository     webhooksRepository
	endpointValidator     webhookEndpointValidator
	webhookSystemMigrated bool
}

func NewWebhooksUsecase(
	enforceSecurity enforceSecurityWebhook,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	convoyRepository convoyWebhooksRepository,
	webhookRepository webhooksRepository,
	endpointValidator webhookEndpointValidator,
	webhookSystemMigrated bool,
) WebhooksUsecase {
	return WebhooksUsecase{
		enforceSecurity:       enforceSecurity,
		executorFactory:       executorFactory,
		transactionFactory:    transactionFactory,
		convoyRepository:      convoyRepository,
		webhookRepository:     webhookRepository,
		endpointValidator:     endpointValidator,
		webhookSystemMigrated: webhookSystemMigrated,
	}
}

// adaptNewWebhookToLegacy converts a NewWebhook to the legacy Webhook format
// used by existing DTOs and handlers.
func adaptNewWebhookToLegacy(nw models.NewWebhook) models.Webhook {
	var httpTimeout *int
	if nw.HttpTimeoutSeconds > 0 {
		httpTimeout = &nw.HttpTimeoutSeconds
	}

	return models.Webhook{
		Id:                nw.Id.String(),
		OrganizationId:    nw.OrganizationId,
		PartnerId:         null.String{}, // New system doesn't use partners
		EventTypes:        nw.EventTypes,
		Url:               nw.Url,
		HttpTimeout:       httpTimeout,
		RateLimit:         nw.RateLimit,
		RateLimitDuration: nw.RateLimitDurationSeconds,
		Secrets:           pure_utils.Map(nw.Secrets, adaptNewSecretToLegacy),
	}
}

// adaptNewSecretToLegacy converts a NewWebhookSecret to the legacy Secret format.
func adaptNewSecretToLegacy(s models.NewWebhookSecret) models.Secret {
	secret := models.Secret{
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
		Uid:       s.Id.String(),
		Value:     s.Value,
	}
	if s.ExpiresAt != nil {
		secret.ExpiresAt = s.ExpiresAt.Format(time.RFC3339)
	}
	if s.RevokedAt != nil {
		secret.DeletedAt = s.RevokedAt.Format(time.RFC3339)
	}
	return secret
}

// generateSecretValue generates a random 32-byte hex-encoded secret.
func generateSecretValue() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.Wrap(err, "failed to generate secret")
	}
	return hex.EncodeToString(bytes), nil
}

func (usecase WebhooksUsecase) ListWebhooks(ctx context.Context, organizationId uuid.UUID, partnerId null.String) ([]models.Webhook, error) {
	if usecase.webhookSystemMigrated {
		return usecase.listWebhooksNew(ctx, organizationId)
	}
	return usecase.listWebhooksConvoy(ctx, organizationId, partnerId)
}

func (usecase WebhooksUsecase) listWebhooksConvoy(ctx context.Context, organizationId uuid.UUID, partnerId null.String) ([]models.Webhook, error) {
	webhooks, err := usecase.convoyRepository.ListWebhooks(ctx, organizationId, partnerId)
	if err != nil {
		return nil, errors.Wrap(err, "error listing webhooks")
	}

	for _, webhook := range webhooks {
		if err := usecase.enforceSecurity.CanReadWebhook(ctx, webhook); err != nil {
			return nil, err
		}
	}

	return webhooks, nil
}

func (usecase WebhooksUsecase) listWebhooksNew(ctx context.Context, organizationId uuid.UUID) ([]models.Webhook, error) {
	exec := usecase.executorFactory.NewExecutor()
	newWebhooks, err := usecase.webhookRepository.ListWebhooks(ctx, exec, organizationId)
	if err != nil {
		return nil, errors.Wrap(err, "error listing webhooks")
	}

	webhooks := make([]models.Webhook, 0, len(newWebhooks))
	for _, nw := range newWebhooks {
		// Fetch all secrets (including revoked) for internal API
		secrets, err := usecase.webhookRepository.ListAllWebhookSecrets(ctx, exec, nw.Id)
		if err != nil {
			return nil, errors.Wrap(err, "error listing webhook secrets")
		}
		nw.Secrets = secrets

		webhook := adaptNewWebhookToLegacy(nw)
		if err := usecase.enforceSecurity.CanReadWebhook(ctx, webhook); err != nil {
			return nil, err
		}
		webhooks = append(webhooks, webhook)
	}

	return webhooks, nil
}

func (usecase WebhooksUsecase) RegisterWebhook(
	ctx context.Context,
	organizationId uuid.UUID,
	partnerId null.String,
	input models.WebhookRegister,
) (models.Webhook, error) {
	err := usecase.enforceSecurity.CanCreateWebhook(ctx, organizationId, partnerId)
	if err != nil {
		return models.Webhook{}, err
	}

	if err = input.Validate(); err != nil {
		return models.Webhook{}, err
	}

	if usecase.webhookSystemMigrated {
		return usecase.registerWebhookNew(ctx, organizationId, input)
	}

	webhook, err := usecase.convoyRepository.RegisterWebhook(ctx, organizationId, partnerId, input)
	return webhook, errors.Wrap(err, "error registering webhook")
}

func (usecase WebhooksUsecase) registerWebhookNew(
	ctx context.Context,
	organizationId uuid.UUID,
	input models.WebhookRegister,
) (models.Webhook, error) {
	// Validate endpoint reachability before creating
	if err := usecase.endpointValidator.ValidateEndpoint(ctx, input.Url); err != nil {
		return models.Webhook{}, errors.Wrap(err, "webhook endpoint unreachable")
	}

	webhookId := uuid.Must(uuid.NewV7())
	httpTimeout := 30 // default
	if input.HttpTimeout != nil {
		httpTimeout = *input.HttpTimeout
	}

	// Generate secret before transaction
	secretValue, err := generateSecretValue()
	if err != nil {
		return models.Webhook{}, err
	}

	newWebhook := models.NewWebhook{
		Id:                       webhookId,
		OrganizationId:           organizationId,
		Url:                      input.Url,
		EventTypes:               input.EventTypes,
		HttpTimeoutSeconds:       httpTimeout,
		RateLimit:                input.RateLimit,
		RateLimitDurationSeconds: input.RateLimitDuration,
		Enabled:                  true,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}

	secret := models.NewWebhookSecret{
		Id:        uuid.Must(uuid.NewV7()),
		WebhookId: webhookId,
		Value:     secretValue,
		CreatedAt: time.Now(),
	}

	// Create webhook and secret in a single transaction
	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.webhookRepository.CreateWebhook(ctx, tx, newWebhook); err != nil {
			return errors.Wrap(err, "error creating webhook")
		}
		if err := usecase.webhookRepository.AddWebhookSecret(ctx, tx, secret); err != nil {
			return errors.Wrap(err, "error adding webhook secret")
		}
		return nil
	})
	if err != nil {
		return models.Webhook{}, err
	}

	newWebhook.Secrets = []models.NewWebhookSecret{secret}
	return adaptNewWebhookToLegacy(newWebhook), nil
}

func (usecase WebhooksUsecase) GetWebhook(
	ctx context.Context, organizationId uuid.UUID, partnerId null.String, webhookId string,
) (models.Webhook, error) {
	if usecase.webhookSystemMigrated {
		return usecase.getWebhookNew(ctx, webhookId)
	}

	webhook, err := usecase.convoyRepository.GetWebhook(ctx, webhookId)
	if err != nil {
		return models.Webhook{}, models.NotFoundError
	}
	if err = usecase.enforceSecurity.CanReadWebhook(ctx, webhook); err != nil {
		return models.Webhook{}, err
	}
	return webhook, nil
}

func (usecase WebhooksUsecase) getWebhookNew(ctx context.Context, webhookId string) (models.Webhook, error) {
	exec := usecase.executorFactory.NewExecutor()

	id, err := uuid.Parse(webhookId)
	if err != nil {
		return models.Webhook{}, models.NotFoundError
	}

	newWebhook, err := usecase.webhookRepository.GetWebhook(ctx, exec, id)
	if err != nil {
		return models.Webhook{}, models.NotFoundError
	}

	// Return all secrets (including revoked) for the internal API
	secrets, err := usecase.webhookRepository.ListAllWebhookSecrets(ctx, exec, id)
	if err != nil {
		return models.Webhook{}, errors.Wrap(err, "error listing webhook secrets")
	}
	newWebhook.Secrets = secrets

	webhook := adaptNewWebhookToLegacy(newWebhook)
	if err = usecase.enforceSecurity.CanReadWebhook(ctx, webhook); err != nil {
		return models.Webhook{}, err
	}
	return webhook, nil
}

func (usecase WebhooksUsecase) DeleteWebhook(
	ctx context.Context, organizationId uuid.UUID, partnerId null.String, webhookId string,
) error {
	if usecase.webhookSystemMigrated {
		return usecase.deleteWebhookNew(ctx, webhookId)
	}

	webhook, err := usecase.convoyRepository.GetWebhook(ctx, webhookId)
	if err != nil {
		return models.NotFoundError
	}
	if err = usecase.enforceSecurity.CanModifyWebhook(ctx, webhook); err != nil {
		return err
	}

	err = usecase.convoyRepository.DeleteWebhook(ctx, webhook.Id)
	return errors.Wrap(err, "error deleting webhook")
}

func (usecase WebhooksUsecase) deleteWebhookNew(ctx context.Context, webhookId string) error {
	exec := usecase.executorFactory.NewExecutor()

	id, err := uuid.Parse(webhookId)
	if err != nil {
		return models.NotFoundError
	}

	newWebhook, err := usecase.webhookRepository.GetWebhook(ctx, exec, id)
	if err != nil {
		return models.NotFoundError
	}

	webhook := adaptNewWebhookToLegacy(newWebhook)
	if err = usecase.enforceSecurity.CanModifyWebhook(ctx, webhook); err != nil {
		return err
	}

	err = usecase.webhookRepository.DeleteWebhook(ctx, exec, id)
	return errors.Wrap(err, "error deleting webhook")
}

func (usecase WebhooksUsecase) UpdateWebhook(
	ctx context.Context, organizationId uuid.UUID, partnerId null.String, webhookId string, input models.WebhookUpdate,
) (models.Webhook, error) {
	if err := input.Validate(); err != nil {
		return models.Webhook{}, err
	}

	if usecase.webhookSystemMigrated {
		return usecase.updateWebhookNew(ctx, webhookId, input)
	}

	webhook, err := usecase.convoyRepository.GetWebhook(ctx, webhookId)
	if err != nil {
		return models.Webhook{}, models.NotFoundError
	}
	if err = usecase.enforceSecurity.CanModifyWebhook(ctx, webhook); err != nil {
		return models.Webhook{}, err
	}

	updatedWebhook, err := usecase.convoyRepository.UpdateWebhook(ctx,
		models.MergeWebhookWithUpdate(webhook, input))
	return updatedWebhook, errors.Wrap(err, "error updating webhook")
}

func (usecase WebhooksUsecase) updateWebhookNew(
	ctx context.Context, webhookId string, input models.WebhookUpdate,
) (models.Webhook, error) {
	// Validate new URL if being changed
	if input.Url != nil {
		if err := usecase.endpointValidator.ValidateEndpoint(ctx, *input.Url); err != nil {
			return models.Webhook{}, errors.Wrap(err, "webhook endpoint validation failed")
		}
	}

	exec := usecase.executorFactory.NewExecutor()

	id, err := uuid.Parse(webhookId)
	if err != nil {
		return models.Webhook{}, models.NotFoundError
	}

	newWebhook, err := usecase.webhookRepository.GetWebhook(ctx, exec, id)
	if err != nil {
		return models.Webhook{}, models.NotFoundError
	}

	webhook := adaptNewWebhookToLegacy(newWebhook)
	if err = usecase.enforceSecurity.CanModifyWebhook(ctx, webhook); err != nil {
		return models.Webhook{}, err
	}

	// Convert legacy update to new update format
	update := models.NewWebhookUpdate{
		EventTypes: input.EventTypes,
		Url:        input.Url,
	}
	if input.HttpTimeout != nil {
		update.HttpTimeoutSeconds = input.HttpTimeout
	}
	if input.RateLimit != nil {
		update.RateLimit = input.RateLimit
	}
	if input.RateLimitDuration != nil {
		update.RateLimitDurationSeconds = input.RateLimitDuration
	}

	if err := usecase.webhookRepository.UpdateWebhook(ctx, exec, id, update); err != nil {
		return models.Webhook{}, errors.Wrap(err, "error updating webhook")
	}

	// Fetch updated webhook with secrets (including revoked for internal API)
	updatedWebhook, err := usecase.webhookRepository.GetWebhook(ctx, exec, id)
	if err != nil {
		return models.Webhook{}, errors.Wrap(err, "error fetching updated webhook")
	}

	secrets, err := usecase.webhookRepository.ListAllWebhookSecrets(ctx, exec, id)
	if err != nil {
		return models.Webhook{}, errors.Wrap(err, "error listing webhook secrets")
	}
	updatedWebhook.Secrets = secrets

	return adaptNewWebhookToLegacy(updatedWebhook), nil
}

// CreateWebhookSecret generates a new secret for a webhook.
// If expireExistingInDays is provided, sets expiration on all existing non-revoked secrets without expiration.
// Only works for migrated webhooks.
func (usecase WebhooksUsecase) CreateWebhookSecret(
	ctx context.Context,
	webhookId uuid.UUID,
	expireExistingInDays *int,
) (models.Secret, error) {
	if !usecase.webhookSystemMigrated {
		return models.Secret{}, errors.Wrap(models.BadParameterError, "secret rotation only available for migrated webhooks")
	}

	exec := usecase.executorFactory.NewExecutor()

	// Get webhook to check permissions
	newWebhook, err := usecase.webhookRepository.GetWebhook(ctx, exec, webhookId)
	if err != nil {
		return models.Secret{}, models.NotFoundError
	}

	webhook := adaptNewWebhookToLegacy(newWebhook)
	if err = usecase.enforceSecurity.CanModifyWebhook(ctx, webhook); err != nil {
		return models.Secret{}, err
	}

	// Generate new secret
	secretValue, err := generateSecretValue()
	if err != nil {
		return models.Secret{}, err
	}

	newSecret := models.NewWebhookSecret{
		Id:        uuid.Must(uuid.NewV7()),
		WebhookId: webhookId,
		Value:     secretValue,
		CreatedAt: time.Now(),
	}

	// Create secret and optionally expire existing ones in a transaction
	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		// Expire existing secrets if requested
		if expireExistingInDays != nil {
			expiresAt := time.Now().AddDate(0, 0, *expireExistingInDays)
			if err := usecase.webhookRepository.ExpireWebhookSecrets(ctx, tx, webhookId, expiresAt); err != nil {
				return errors.Wrap(err, "error expiring existing secrets")
			}
		}

		if err := usecase.webhookRepository.AddWebhookSecret(ctx, tx, newSecret); err != nil {
			return errors.Wrap(err, "error adding webhook secret")
		}
		return nil
	})
	if err != nil {
		return models.Secret{}, err
	}

	return adaptNewSecretToLegacy(newSecret), nil
}

// RevokeWebhookSecret revokes a webhook secret.
// Fails if this would leave the webhook without any permanent (non-expiring) secrets.
// Only works for migrated webhooks.
func (usecase WebhooksUsecase) RevokeWebhookSecret(
	ctx context.Context,
	webhookId uuid.UUID,
	secretId uuid.UUID,
) error {
	if !usecase.webhookSystemMigrated {
		return errors.Wrap(models.BadParameterError, "secret revocation only available for migrated webhooks")
	}

	exec := usecase.executorFactory.NewExecutor()

	// Get webhook to check permissions
	newWebhook, err := usecase.webhookRepository.GetWebhook(ctx, exec, webhookId)
	if err != nil {
		return models.NotFoundError
	}

	webhook := adaptNewWebhookToLegacy(newWebhook)
	if err = usecase.enforceSecurity.CanModifyWebhook(ctx, webhook); err != nil {
		return err
	}

	// Get the secret to verify it belongs to this webhook and check if it's permanent
	secret, err := usecase.webhookRepository.GetWebhookSecret(ctx, exec, secretId)
	if err != nil {
		return models.NotFoundError
	}

	if secret.WebhookId != webhookId {
		return models.NotFoundError
	}

	if secret.RevokedAt != nil {
		return errors.Wrap(models.BadParameterError, "secret is already revoked")
	}

	// Ensure at least one permanent secret remains after revocation
	permanentCount, err := usecase.webhookRepository.CountPermanentWebhookSecrets(ctx, exec, webhookId)
	if err != nil {
		return errors.Wrap(err, "error counting permanent secrets")
	}

	// If revoking a permanent secret, we'll have one less
	remainingPermanent := permanentCount
	if secret.ExpiresAt == nil {
		remainingPermanent--
	}

	if remainingPermanent < 1 {
		return errors.Wrap(models.BadParameterError,
			"cannot revoke: webhook must retain at least one permanent (non-expiring) secret")
	}

	if err := usecase.webhookRepository.RevokeWebhookSecret(ctx, exec, secretId); err != nil {
		return errors.Wrap(err, "error revoking secret")
	}

	return nil
}
