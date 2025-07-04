package metrics_collection

import (
	"context"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type OrganizationRepository interface {
	AllOrganizations(ctx context.Context, exec repositories.Executor) ([]models.Organization, error)
}

// GlobalCollector is a collector that is not specific to an organization.
// It is used to collect metrics that are not specific to an organization.
// For example, the app version, the number of users
type GlobalCollector interface {
	Collect(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error)
}

// Collector is a collector that is specific to an organization.
type Collector interface {
	Collect(ctx context.Context, orgId string, from time.Time, to time.Time) ([]models.MetricData, error)
}

type Collectors struct {
	version          string
	globalCollectors []GlobalCollector
	collectors       []Collector

	organizationRepository OrganizationRepository
	executorFactory        executor_factory.ExecutorFactory
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

	payload := models.MetricsCollection{
		CollectionID: uuid.New(),
		Timestamp:    time.Now(),
		Metrics:      metrics,
		Version:      c.version,
	}

	return payload, nil
}

// Collects global metrics from all collectors
// If a collector fails, it will log a warning and continue to the next collector (don't fail the whole function)
func (c Collectors) collectGlobalMetrics(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	metrics := []models.MetricData{}
	logger := utils.LoggerFromContext(ctx)

	for _, collector := range c.globalCollectors {
		value, err := collector.Collect(ctx, from, to)
		if err != nil {
			logger.WarnContext(ctx, "Failed to collect global metrics", "error", err)
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

	for _, org := range orgs {
		for _, collector := range c.collectors {
			value, err := collector.Collect(ctx, org.Id, from, to)
			if err != nil {
				logger.WarnContext(ctx, "Failed to collect organization metrics", "error", err)
				continue
			}
			metrics = slices.Concat(metrics, value)
		}
	}

	return metrics, nil
}

// Fetches all organizations from the database
// NOTE: Add caching to avoid fetching the same organizations every time (but how can we invalidate the cache?)
func (c Collectors) getListOfOrganizations(ctx context.Context) ([]models.Organization, error) {
	orgs, err := c.organizationRepository.AllOrganizations(ctx, c.executorFactory.NewExecutor())
	if err != nil {
		return []models.Organization{}, err
	}
	return orgs, nil
}

// Use version to track the version of the collectors, could be used to track changes
// and tell the server which collectors is used by the client
func NewCollectorsTestV1(
	executorFactory executor_factory.ExecutorFactory,
	organizationRepository OrganizationRepository,
	apiVersion string,
) Collectors {
	return Collectors{
		version: "test-v1",
		collectors: []Collector{
			NewStubOrganizationCollector(),
		},
		globalCollectors: []GlobalCollector{
			NewStubGlobalCollector(),
			NewLicenseKeyCollector(),
			NewAppVersionCollector(apiVersion),
		},
		executorFactory:        executorFactory,
		organizationRepository: organizationRepository,
	}
}
