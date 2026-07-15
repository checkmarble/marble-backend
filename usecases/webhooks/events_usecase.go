package webhooks

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const (
	MAX_CONCURRENT_WEBHOOKS_SENT      = 20
	WEBHOOKS_SEND_MAX_RETRIES         = 24
	DEFAULT_FAILED_WEBHOOKS_PAGE_SIZE = 1000
	ASYNC_WEBHOOKS_SEND_TIMEOUT       = 5 * time.Minute
)

type enforceSecurityWebhookEvents interface {
	SendWebhookEvent(ctx context.Context, organizationId uuid.UUID) error
}

// webhookEventV2Repository is the interface for the new webhook event system.
type webhookEventV2Repository interface {
	CreateWebhookEventV2(ctx context.Context, exec repositories.Executor, event models.WebhookEventV2) error
}

// webhookTaskQueue is the interface for enqueueing webhook dispatch jobs.
type webhookTaskQueue interface {
	EnqueueWebhookDispatch(ctx context.Context, tx repositories.Transaction, organizationId uuid.UUID, webhookEventId uuid.UUID) error
}

type WebhookEventsUsecase struct {
	enforceSecurity             enforceSecurityWebhookEvents
	executorFactory             executor_factory.ExecutorFactory
	transactionFactory          executor_factory.TransactionFactory
	webhookEventV2Repository    webhookEventV2Repository
	taskQueue                   webhookTaskQueue
	failedWebhooksRetryPageSize int
	hasLicense                  bool
	publicApiAdaptor            types.PublicApiDataAdapter
}

func NewWebhookEventsUsecase(
	enforceSecurity enforceSecurityWebhookEvents,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	webhookEventV2Repository webhookEventV2Repository,
	taskQueue webhookTaskQueue,
	failedWebhooksRetryPageSize int,
	hasLicense bool,
	publicApiAdaptor types.PublicApiDataAdapter,
) WebhookEventsUsecase {
	if failedWebhooksRetryPageSize == 0 {
		failedWebhooksRetryPageSize = DEFAULT_FAILED_WEBHOOKS_PAGE_SIZE
	}

	return WebhookEventsUsecase{
		enforceSecurity:             enforceSecurity,
		executorFactory:             executorFactory,
		transactionFactory:          transactionFactory,
		webhookEventV2Repository:    webhookEventV2Repository,
		taskQueue:                   taskQueue,
		failedWebhooksRetryPageSize: failedWebhooksRetryPageSize,
		hasLicense:                  hasLicense,
		publicApiAdaptor:            publicApiAdaptor,
	}
}

// Does nothing (returns nil) if the license is not active.
func (usecase WebhookEventsUsecase) CreateWebhookEvent(
	ctx context.Context,
	tx repositories.Transaction,
	input models.WebhookEventCreate,
) error {
	// Check license
	if !usecase.hasLicense {
		return nil
	}

	err := usecase.enforceSecurity.SendWebhookEvent(ctx, input.OrganizationId)
	if err != nil {
		return err
	}

	// Serialize event data using the public API DTOs
	apiVersion, eventData, err := dto.AdaptWebhookEventData(ctx, tx, usecase.publicApiAdaptor, input.EventContent.Data)
	if err != nil {
		return errors.Wrap(err, "error adapting webhook event data")
	}

	eventId := pure_utils.NewId()

	// Create webhook event v2
	event := models.WebhookEventV2{
		Id:             eventId,
		OrganizationId: input.OrganizationId,
		EventType:      string(input.EventContent.Type),
		ApiVersion:     apiVersion,
		EventData:      eventData,
	}

	err = usecase.webhookEventV2Repository.CreateWebhookEventV2(ctx, tx, event)
	if err != nil {
		return errors.Wrap(err, "error creating webhook event")
	}

	// Enqueue dispatch job (fan-out happens asynchronously)
	err = usecase.taskQueue.EnqueueWebhookDispatch(ctx, tx, input.OrganizationId, eventId)
	if err != nil {
		return errors.Wrap(err, "error enqueueing webhook dispatch job")
	}

	return nil
}
