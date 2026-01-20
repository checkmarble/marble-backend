package metrics_collection

import (
	"context"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

const (
	DeploymentIDCacheKey = "metadata_deployment_id"
)

type CollectorRepository interface {
	AllOrganizations(ctx context.Context, exec repositories.Executor) ([]models.Organization, error)
	GetMetadata(ctx context.Context, exec repositories.Executor, orgID *uuid.UUID,
		key models.MetadataKey) (*models.Metadata, error)
	CaseCollectorRepository
	DecisionCollectorRepository
	ScreeningCollectorRepository
	AiCaseReviewCollectorRepository
	ContinuousScreeningMarbleDbRepository
}
type CollectorClientRepository interface {
	ContinuousScreeningClientDbRepository
}

// GlobalCollector is a collector that is not specific to an organization.
// For example, the app version, the number of users
type GlobalCollector interface {
	Collect(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error)
}

// Collector is a collector that is specific to an organization.
type Collector interface {
	Collect(ctx context.Context, orgs []models.Organization, from time.Time, to time.Time) ([]models.MetricData, error)
}

var DeploymentIDCache = expirable.NewLRU[string, uuid.UUID](1, nil, 0)

type Collectors struct {
	version          string
	globalCollectors []GlobalCollector
	collectors       []Collector

	licenseConfig   models.LicenseConfiguration
	repository      CollectorRepository
	executorFactory executor_factory.ExecutorFactory
}

func (c Collectors) CollectMetrics(ctx context.Context, from time.Time, to time.Time) (models.MetricsCollection, error) {
	metrics := []models.MetricData{}

	// Collect global metrics
	globalMetrics, err := c.collectGlobalMetrics(ctx, from, to)
	if err != nil {
		return models.MetricsCollection{}, err
	}
	metrics = slices.Concat(metrics, globalMetrics)

	// Collect organization-specific metrics
	orgMetrics, err := c.collectOrganizationMetrics(ctx, from, to)
	if err != nil {
		return models.MetricsCollection{}, err
	}
	metrics = slices.Concat(metrics, orgMetrics)

	deploymentID, err := c.GetDeploymentID(ctx)
	if err != nil {
		return models.MetricsCollection{}, err
	}

	payload := models.MetricsCollection{
		CollectionID: uuid.New(),
		Timestamp:    time.Now(),
		Metrics:      metrics,
		Version:      c.version,
		LicenseKey:   c.GetLicenseKey(),
		DeploymentID: deploymentID,
	}

	return payload, nil
}

// Collects global metrics from all collectors
// If a collector fails, it will log a warning and continue to the next collector (don't fail the whole function)
func (c Collectors) collectGlobalMetrics(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	metrics := []models.MetricData{}

	for _, collector := range c.globalCollectors {
		value, err := collector.Collect(ctx, from, to)
		if err != nil {
			utils.LogAndReportSentryError(ctx, err)
			continue
		}
		metrics = slices.Concat(metrics, value)
	}

	return metrics, nil
}

// Collects organization metrics from all collectors, fetching all organizations from the database first
// If a collector fails, it will log a warning and continue to the next collector (don't fail the whole function)
func (c Collectors) collectOrganizationMetrics(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	metrics := []models.MetricData{}
	logger := utils.LoggerFromContext(ctx)

	orgs, err := c.getListOfOrganizations(ctx)
	if err != nil {
		return []models.MetricData{}, err
	}

	for _, collector := range c.collectors {
		value, err := collector.Collect(ctx, orgs, from, to)
		if err != nil {
			logger.WarnContext(ctx, "Failed to collect organization metrics", "error", err)
			continue
		}
		metrics = slices.Concat(metrics, value)
	}

	return metrics, nil
}

// Fetches all organizations from the database
func (c Collectors) getListOfOrganizations(ctx context.Context) ([]models.Organization, error) {
	orgs, err := c.repository.AllOrganizations(ctx, c.executorFactory.NewExecutor())
	if err != nil {
		return []models.Organization{}, err
	}
	return orgs, nil
}

func (c Collectors) GetLicenseKey() *string {
	if c.licenseConfig.LicenseKey == "" {
		return nil
	}
	return &c.licenseConfig.LicenseKey
}

func (c Collectors) GetDeploymentID(ctx context.Context) (uuid.UUID, error) {
	if deploymentID, exists := DeploymentIDCache.Get(DeploymentIDCacheKey); exists {
		return deploymentID, nil
	}

	metadata, err := c.repository.GetMetadata(ctx, c.executorFactory.NewExecutor(), nil, models.MetadataKeyDeploymentID)
	if err != nil {
		return uuid.Nil, err
	}
	if metadata == nil {
		return uuid.Nil, errors.Wrap(models.NotFoundError, "deployment ID not found")
	}
	deploymentID, err := uuid.Parse(metadata.Value)
	if err != nil {
		return uuid.Nil, err
	}
	DeploymentIDCache.Add(DeploymentIDCacheKey, deploymentID)
	return deploymentID, nil
}

// Use version to track the version of the collectors, could be used to track changes
// and tell the server which collectors is used by the client
func NewCollectorsV1(
	executorFactory executor_factory.ExecutorFactory,
	repository CollectorRepository,
	clientDbRepo CollectorClientRepository,
	apiVersion string,
	licenseConfig models.LicenseConfiguration,
) Collectors {
	return Collectors{
		version: "v1",
		collectors: []Collector{
			NewDecisionCollector(repository, executorFactory),
			NewCaseCollector(repository, executorFactory),
			NewScreeningCollector(repository, executorFactory),
			NewAiCaseReviewCollector(repository, executorFactory),
			NewContinuousScreeningCollector(repository, clientDbRepo, executorFactory),
		},
		globalCollectors: []GlobalCollector{
			NewAppVersionCollector(apiVersion),
		},
		executorFactory: executorFactory,
		licenseConfig:   licenseConfig,
		repository:      repository,
	}
}
