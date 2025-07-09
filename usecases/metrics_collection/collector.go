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

type CollectorRepository interface {
	DecisionCollectorRepository
	CaseCollectorRepository
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
	Collect(ctx context.Context, orgIds []string, from time.Time, to time.Time) ([]models.MetricData, error)
}

type Collectors struct {
	version          string
	globalCollectors []GlobalCollector
	collectors       []Collector

	licenseConfig          models.LicenseConfiguration
	organizationRepository CollectorRepository
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
		LicenseKey:   c.GetLicenseKey(),
		DeploymentID: c.GetDeploymentID(),
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
	orgIds := make([]string, len(orgs))
	for i, org := range orgs {
		orgIds[i] = org.Id
	}

	for _, collector := range c.collectors {
		value, err := collector.Collect(ctx, orgIds, from, to)
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
	orgs, err := c.organizationRepository.AllOrganizations(ctx, c.executorFactory.NewExecutor())
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

func (c Collectors) GetDeploymentID() uuid.UUID {
	// TODO: Change by the real deployment ID, DeploymentID TBD
	return uuid.MustParse("c08cce05-ed91-4941-b959-9849c0652640")
}

// Use version to track the version of the collectors, could be used to track changes
// and tell the server which collectors is used by the client
func NewCollectorsTestV1(
	executorFactory executor_factory.ExecutorFactory,
	collectorRepository CollectorRepository,
	apiVersion string,
	licenseConfig models.LicenseConfiguration,
) Collectors {
	return Collectors{
		version: "test-v1",
		collectors: []Collector{
			NewDecisionCollector(collectorRepository, executorFactory),
			NewCaseCollector(collectorRepository, executorFactory),
		},
		globalCollectors: []GlobalCollector{
			NewAppVersionCollector(apiVersion),
		},
		executorFactory:        executorFactory,
		organizationRepository: collectorRepository,
		licenseConfig:          licenseConfig,
	}
}
