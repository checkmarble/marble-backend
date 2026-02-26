package lago_repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// Max event per batch for event ingestion
// cf: https://getlago.com/docs/api-reference/events/batch
const MAX_EVENTS_PER_BATCH = 100

type LagoRepository struct {
	client     *http.Client
	lagoConfig infra.LagoConfig
}

func NewLagoRepository(client *http.Client, lagoConfig infra.LagoConfig) LagoRepository {
	return LagoRepository{
		client:     client,
		lagoConfig: lagoConfig,
	}
}

func (repo LagoRepository) IsConfigured() bool {
	return repo.lagoConfig.BaseUrl != "" && repo.lagoConfig.ApiKey != "" && repo.lagoConfig.ParsedUrl != nil
}

// Build the request with the correct headers
func (repo LagoRepository) getRequest(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", repo.lagoConfig.ApiKey))
	return req, nil
}

// doRequestWithRetry performs an HTTP request with retry logic
// Retries on network errors, 5xx errors, and 429 (rate limit) errors
func (repo LagoRepository) doRequestWithRetry(ctx context.Context, method string, url string, body []byte) (*http.Response, error) {
	var resp *http.Response
	err := retry.Do(
		func() error {
			var reqBody io.Reader
			if body != nil {
				reqBody = bytes.NewReader(body)
			}

			req, err := repo.getRequest(ctx, method, url, reqBody)
			if err != nil {
				return err
			}

			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err = repo.client.Do(req)
			if err != nil {
				return err
			}

			if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
				resp.Body.Close()
				return errors.Newf("received status code %d", resp.StatusCode)
			}

			return nil
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
		retry.Delay(100*time.Millisecond),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(ctx),
	)

	return resp, err
}

// Get Wallet for an organization
func (repo LagoRepository) GetWallets(ctx context.Context, orgId uuid.UUID) ([]models.Wallet, error) {
	if !repo.IsConfigured() {
		return nil, errors.New("lago repository is not configured")
	}

	baseUrl := *repo.lagoConfig.ParsedUrl
	baseUrl.Path = "/api/v1/wallets"
	query := baseUrl.Query()
	query.Add("external_customer_id", orgId.String())
	baseUrl.RawQuery = query.Encode()

	resp, err := repo.doRequestWithRetry(ctx, http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send 'get wallet' request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Newf("failed to get wallet: %s", resp.Status)
	}

	var wallets WalletsDto
	if err = json.NewDecoder(resp.Body).Decode(&wallets); err != nil {
		return nil, errors.Wrap(err, "failed to decode wallet")
	}

	adaptedWallets := AdaptWalletsDtoToModel(wallets)
	return adaptedWallets, nil
}

// Get Active Subscriptions (not detailed) for an organization
func (repo LagoRepository) GetSubscriptions(ctx context.Context, orgId uuid.UUID) ([]models.Subscription, error) {
	if !repo.IsConfigured() {
		return nil, errors.New("lago repository is not configured")
	}

	baseUrl := *repo.lagoConfig.ParsedUrl
	baseUrl.Path = "/api/v1/subscriptions"
	query := baseUrl.Query()
	query.Add("external_customer_id", orgId.String())
	query.Add("status[]", "active")
	baseUrl.RawQuery = query.Encode()

	resp, err := repo.doRequestWithRetry(ctx, http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get subscriptions")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Newf("failed to get subscriptions: %s", resp.Status)
	}

	var subscriptions SubscriptionsDto
	if err = json.NewDecoder(resp.Body).Decode(&subscriptions); err != nil {
		return nil, errors.Wrap(err, "failed to decode subscriptions")
	}

	return AdaptSubscriptionsDtoToModel(subscriptions), nil
}

// Get subscription with more details
// Be careful, use the external ID to get the subscription (cf: https://getlago.com/docs/api-reference/subscriptions/get-specific)
func (repo LagoRepository) GetSubscription(ctx context.Context, subscriptionExternalId string) (models.Subscription, error) {
	if !repo.IsConfigured() {
		return models.Subscription{}, errors.New("lago repository is not configured")
	}

	baseUrl := *repo.lagoConfig.ParsedUrl
	baseUrl.Path = fmt.Sprintf("/api/v1/subscriptions/%s", subscriptionExternalId)

	resp, err := repo.doRequestWithRetry(ctx, http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		return models.Subscription{}, errors.Wrap(err, "failed to get subscription")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.Subscription{}, errors.Newf("failed to get subscription: %s", resp.Status)
	}

	var subscription SubscriptionDto
	if err = json.NewDecoder(resp.Body).Decode(&subscription); err != nil {
		return models.Subscription{}, errors.Wrap(err, "failed to decode subscription")
	}

	return AdaptSubscriptionDtoToModel(subscription), nil
}

// Get customer usage for an organization on a specific subscription
// Use subscription external ID (cf: https://getlago.com/docs/api-reference/customer-usage/get-current)
func (repo LagoRepository) GetCustomerUsage(
	ctx context.Context,
	orgId uuid.UUID,
	subscriptionExternalId string,
) (models.CustomerUsage, error) {
	if !repo.IsConfigured() {
		return models.CustomerUsage{}, errors.New("lago repository is not configured")
	}

	baseUrl := *repo.lagoConfig.ParsedUrl
	baseUrl.Path = fmt.Sprintf("/api/v1/customers/%s/current_usage", orgId)
	query := baseUrl.Query()
	query.Add("external_subscription_id", subscriptionExternalId)
	baseUrl.RawQuery = query.Encode()

	resp, err := repo.doRequestWithRetry(ctx, http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		return models.CustomerUsage{}, errors.Wrap(err, "failed to get customer usage")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.CustomerUsage{}, errors.Newf("failed to get customer usage: %s", resp.Status)
	}

	var customerUsage CustomerUsageDto
	if err = json.NewDecoder(resp.Body).Decode(&customerUsage); err != nil {
		return models.CustomerUsage{}, errors.Wrap(err, "failed to decode customer usage")
	}

	return AdaptCustomerUsageDtoToModel(customerUsage), nil
}

func (repo LagoRepository) SendEvent(ctx context.Context, event models.BillingEvent) error {
	logger := utils.LoggerFromContext(ctx)
	if !repo.IsConfigured() {
		return errors.New("lago repository is not configured")
	}

	baseUrl := *repo.lagoConfig.ParsedUrl
	baseUrl.Path = "/api/v1/events"

	body, err := json.Marshal(AdaptModelToBillingEventDto(event))
	if err != nil {
		return errors.Wrap(err, "failed to marshal event")
	}

	resp, err := repo.doRequestWithRetry(ctx, http.MethodPost, baseUrl.String(), body)
	if err != nil {
		return errors.Wrap(err, "failed to send event")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.ErrorContext(ctx, "failed to send billing event", "status", resp.StatusCode, "response", string(bodyBytes))
		return errors.Newf("failed to send billing event")
	}

	return nil
}

// Send Events in batch
// Can only send 100 events per batch (cf: https://getlago.com/docs/api-reference/events/batch)
// In case or error, need to retry the entire batch
func (repo LagoRepository) SendEvents(ctx context.Context, events []models.BillingEvent) error {
	if !repo.IsConfigured() {
		return errors.New("lago repository is not configured")
	}

	baseUrl := *repo.lagoConfig.ParsedUrl
	baseUrl.Path = "/api/v1/events/batch"

	for i := 0; i < len(events); i += MAX_EVENTS_PER_BATCH {
		batch := events[i:min(i+MAX_EVENTS_PER_BATCH, len(events))]
		body, err := json.Marshal(AdaptModelToBillingEventsDto(batch))
		if err != nil {
			return errors.Wrap(err, "failed to marshal events")
		}

		if err := repo.sendBatch(ctx, baseUrl.String(), body); err != nil {
			return err
		}
	}

	return nil
}

func (repo LagoRepository) sendBatch(ctx context.Context, baseUrl string, body []byte) error {
	logger := utils.LoggerFromContext(ctx)

	resp, err := repo.doRequestWithRetry(ctx, http.MethodPost, baseUrl, body)
	if err != nil {
		return errors.Wrap(err, "failed to send events")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.WarnContext(ctx, "failed to send events", "status", resp.StatusCode, "response", string(bodyBytes))
		return errors.New("failed to send events")
	}
	return nil
}

func (repo LagoRepository) GetEntitlements(ctx context.Context, subscriptionExternalId string) ([]models.BillingEntitlement, error) {
	if !repo.IsConfigured() {
		return nil, errors.New("lago repository is not configured")
	}

	baseUrl := *repo.lagoConfig.ParsedUrl
	baseUrl.Path = fmt.Sprintf("/api/v1/subscriptions/%s/entitlements", subscriptionExternalId)

	resp, err := repo.doRequestWithRetry(ctx, http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get customer usage")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Newf("failed to get customer usage: %s", resp.Status)
	}

	var result EntitlementsDto
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Wrap(err, "failed to decode entitlements")
	}

	return pure_utils.Map(result.Entitlements, AdaptEntitlementDtoToModel), nil
}
