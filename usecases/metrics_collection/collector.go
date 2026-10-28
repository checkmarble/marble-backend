package metrics_collection

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
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

var screeningProviderList = []models.ScreeningProvider{
	models.ScreeningProviderOpenSanctions,
	models.ScreeningProviderLexisNexis,
}

type CollectorRepository interface {
	AllOrganizations(ctx context.Context, exec repositories.Executor) ([]models.Organization, error)
	GetMetadata(ctx context.Context, exec repositories.Executor, orgID *uuid.UUID,
		key models.MetadataKey) (*models.Metadata, error)
	PostgresVersionRepository
	CaseCollectorRepository
	DecisionCollectorRepository
	ScreeningCollectorRepository
	AiCaseReviewCollectorRepository
	MonitoredObjectMarbleDbRepository
	ContinuousScreeningCollectorRepository
	FreeformSearchCollectorRepository
}
type CollectorClientRepository interface {
	MonitoredClientDbRepository
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

// CounterCollector is a collector of counters over a time range. Its name identifies
// the watermark used to collect it incrementally.
type CounterCollector interface {
	Collector
	// Name is stable and used as watermark spec: never rename it.
	Name() string
}

var DeploymentIDCache = expirable.NewLRU[string, uuid.UUID](1, nil, 0)

type Collectors struct {
	version            string
	globalCollectors   []GlobalCollector
	snapshotCollectors []Collector
	counterCollectors  []CounterCollector

	licenseConfig   models.LicenseConfiguration
	repository      CollectorRepository
	executorFactory executor_factory.ExecutorFactory
}

// CollectMetrics runs all collectors for the given request.
//
// Two distinct timeouts are at play:
//   - the parent ctx carries the global budget of the run: once ctx.Err() != nil, nothing else is started and the
//     remaining counters are reported as skipped (CompletedUntil == From);
//   - req.CollectorTimeout bounds each collector on its own: when it expires, only that collector stops.
//
// Counters are run sequentially, oldest watermark first, so a collector starved by the budget goes first next run.
func (c Collectors) CollectMetrics(ctx context.Context, req models.MetricsCollectionRequest) (models.MetricsCollectionResult, error) {
	deploymentID, err := c.GetDeploymentID(ctx)
	if err != nil {
		return models.MetricsCollectionResult{}, errors.Wrap(err, "failed to get deployment ID")
	}

	orgs, err := c.getListOfOrganizations(ctx)
	if err != nil {
		return models.MetricsCollectionResult{}, errors.Wrap(err, "failed to list organizations")
	}

	metrics := []models.MetricData{}

	// Global and snapshot collectors run once over [SnapshotFrom, Now], they are never split.
	metrics = append(metrics, c.collectGlobalMetrics(ctx, req.SnapshotFrom, req.Now, req.CollectorTimeout)...)
	metrics = append(metrics, c.collectSnapshotMetrics(ctx, orgs, req.SnapshotFrom, req.Now, req.CollectorTimeout)...)

	// Counter collectors: only those with a watermark in the request, oldest first (tie-break on name).
	counters := make([]CounterCollector, 0, len(c.counterCollectors))
	for _, cc := range c.counterCollectors {
		if _, ok := req.CounterFrom[cc.Name()]; ok {
			counters = append(counters, cc)
		}
	}
	slices.SortStableFunc(counters, func(a, b CounterCollector) int {
		if cmp := req.CounterFrom[a.Name()].Compare(req.CounterFrom[b.Name()]); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Name(), b.Name())
	})

	progress := make([]models.MetricsCounterProgress, 0, len(counters))
	for _, cc := range counters {
		from := req.CounterFrom[cc.Name()]

		// The parent (budget) context is exhausted: do not start this collector.
		if err := ctx.Err(); err != nil {
			progress = append(progress, models.MetricsCounterProgress{
				CollectorName:  cc.Name(),
				From:           from,
				CompletedUntil: from,
				Err:            errors.Wrap(err, "metrics collection budget exhausted"),
			})
			continue
		}

		counterMetrics, p := c.collectCounter(ctx, cc, orgs, from, req.Now, req.CollectorTimeout)
		metrics = append(metrics, counterMetrics...)
		progress = append(progress, p)
	}

	return models.MetricsCollectionResult{
		Collection: models.MetricsCollection{
			CollectionID: pure_utils.NewId(),
			Timestamp:    time.Now(),
			Metrics:      metrics,
			Version:      c.version,
			LicenseKey:   c.GetLicenseKey(),
			DeploymentID: deploymentID,
		},
		Progress: progress,
	}, nil
}

// Collects global metrics from all collectors, each under its own timeout.
// If a collector fails, it is reported and the next one runs (don't fail the whole function).
func (c Collectors) collectGlobalMetrics(ctx context.Context, from, to time.Time, timeout time.Duration) []models.MetricData {
	metrics := []models.MetricData{}

	for _, collector := range c.globalCollectors {
		if ctx.Err() != nil {
			break
		}
		cctx, cancel := context.WithTimeout(ctx, timeout)
		value, err := collector.Collect(cctx, from, to)
		cancel()
		if err != nil {
			utils.LoggerFromContext(ctx).WarnContext(ctx, "metrics gauge collector failed",
				"collector", fmt.Sprintf("%T", collector), "error", err.Error())
			utils.LogAndReportSentryError(ctx, err)
			continue
		}
		metrics = append(metrics, value...)
	}

	return metrics
}

// Collects snapshot (gauge) metrics once over [from, to], each collector under its own timeout.
// If a collector fails, it is reported and the next one runs (don't fail the whole function).
func (c Collectors) collectSnapshotMetrics(ctx context.Context, orgs []models.Organization,
	from, to time.Time, timeout time.Duration,
) []models.MetricData {
	metrics := []models.MetricData{}

	for _, collector := range c.snapshotCollectors {
		if ctx.Err() != nil {
			break
		}
		cctx, cancel := context.WithTimeout(ctx, timeout)
		value, err := collector.Collect(cctx, orgs, from, to)
		cancel()
		if err != nil {
			utils.LoggerFromContext(ctx).WarnContext(ctx, "metrics gauge collector failed",
				"collector", fmt.Sprintf("%T", collector), "error", err.Error())
			utils.LogAndReportSentryError(ctx, err)
			continue
		}
		metrics = append(metrics, value...)
	}

	return metrics
}

// collectCounter walks one counter collector through its backlog, one day at a time, under a single
// per-collector timeout (which is also capped by the parent budget since cctx derives from ctx).
// It stops at the first failing period and discards that period: CompletedUntil is the end of the last
// fully collected period, so the watermark can safely advance to it.
func (c Collectors) collectCounter(ctx context.Context, collector CounterCollector, orgs []models.Organization,
	from, now time.Time, timeout time.Duration,
) ([]models.MetricData, models.MetricsCounterProgress) {
	progress := models.MetricsCounterProgress{
		CollectorName:  collector.Name(),
		From:           from,
		CompletedUntil: from,
	}
	if !from.Before(now) {
		return nil, progress
	}

	periods, err := pure_utils.SplitTimeRangeByFrequency(from, now, pure_utils.FrequencyDaily)
	if err != nil {
		progress.Err = errors.Wrap(err, "failed to split time range for "+collector.Name())
		return nil, progress
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	metrics := []models.MetricData{}
	for _, period := range periods {
		if err := cctx.Err(); err != nil {
			progress.Err = errors.Wrapf(err, "collector %s stopped before period %s - %s",
				collector.Name(), period.From.Format(time.RFC3339), period.To.Format(time.RFC3339))
			break
		}

		value, err := collector.Collect(cctx, orgs, period.From, period.To)
		if err != nil {
			progress.Err = errors.Wrapf(err, "collector %s failed on period %s - %s",
				collector.Name(), period.From.Format(time.RFC3339), period.To.Format(time.RFC3339))
			break
		}

		metrics = append(metrics, value...)
		progress.CompletedUntil = period.To
	}

	utils.LoggerFromContext(ctx).DebugContext(ctx, "counter collector finished",
		"collector", collector.Name(),
		"from", from,
		"completed_until", progress.CompletedUntil,
		"error", progress.Err,
	)

	return metrics, progress
}

// CounterCollectorNames returns the names of the counter collectors, in list order.
func (c Collectors) CounterCollectorNames() []string {
	names := make([]string, 0, len(c.counterCollectors))
	for _, cc := range c.counterCollectors {
		names = append(names, cc.Name())
	}
	return names
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
		counterCollectors: []CounterCollector{
			NewDecisionCollector(repository, executorFactory),
			NewCaseCollector(repository, executorFactory),
			NewScreeningCollector(repository, executorFactory),
			NewAiCaseReviewCollector(repository, executorFactory),
			NewScreeningByProviderCollector(repository, executorFactory, screeningProviderList),
			NewContinuousScreeningByProviderCollector(repository, executorFactory, screeningProviderList),
			NewFreeformSearchByProviderCollector(repository, executorFactory, screeningProviderList),
		},
		snapshotCollectors: []Collector{
			NewMonitoredObjectCollector(repository, clientDbRepo, executorFactory),
		},
		globalCollectors: []GlobalCollector{
			NewAppVersionCollector(apiVersion),
			NewPostgresVersionCollector(repository, executorFactory),
		},
		executorFactory: executorFactory,
		licenseConfig:   licenseConfig,
		repository:      repository,
	}
}
