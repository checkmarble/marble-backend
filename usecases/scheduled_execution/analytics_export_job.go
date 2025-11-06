package scheduled_execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"golang.org/x/sync/errgroup"
)

type analyticsExportRepository interface {
	AllOrganizations(ctx context.Context, exec repositories.Executor) ([]models.Organization, error)
	GetWatermark(ctx context.Context, exec repositories.Executor, orgId *string,
		watermarkType models.WatermarkType) (*models.Watermark, error)
	SaveWatermark(ctx context.Context, exec repositories.Executor,
		orgId *string, watermarkType models.WatermarkType, watermarkId *string, watermarkTime time.Time, params json.RawMessage) error

	GetDataModel(ctx context.Context, exec repositories.Executor, organizationID string, fetchEnumValues bool,
		useCache bool) (models.DataModel, error)
	GetAnalyticsSettings(ctx context.Context, exec repositories.Executor, orgId string) (map[string]analytics.Settings, error)
}

func NewAnalyticsExportJob(orgId string, interval time.Duration) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.AnalyticsExportArgs{
					OrgId: orgId,
				}, &river.InsertOpts{
					Queue: orgId,
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: interval,
					},
				}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type AnalyticsExportWorker struct {
	river.WorkerDefaults[models.AnalyticsExportArgs]

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	analyticsFactory   executor_factory.AnalyticsExecutorFactory
	license            models.LicenseValidation
	repository         analyticsExportRepository
	config             infra.AnalyticsConfig
}

func NewAnalyticsExportWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	analyticsFactory executor_factory.AnalyticsExecutorFactory,
	license models.LicenseValidation,
	repository analyticsExportRepository,
	config infra.AnalyticsConfig,
) *AnalyticsExportWorker {
	return &AnalyticsExportWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		analyticsFactory:   analyticsFactory,
		license:            license,
		repository:         repository,
		config:             config,
	}
}

func (w *AnalyticsExportWorker) Timeout(job *river.Job[models.AnalyticsExportArgs]) time.Duration {
	// A timeout of JobInterval is okay because we set a custom timeout that will be slightly lower.
	return w.config.JobInterval
}

func (w AnalyticsExportWorker) Work(ctx context.Context, job *river.Job[models.AnalyticsExportArgs]) error {
	logger := utils.LoggerFromContext(ctx)

	if job.CreatedAt.Before(time.Now().Add(-w.config.JobInterval)) {
		logger.DebugContext(ctx, "skipping offloading job instance because it was created too long ago. A new one should have been created.", "job_created_at", job.CreatedAt)
		return nil
	}

	grace := time.Duration(w.config.JobInterval.Seconds()*0.75) * time.Second
	if grace.Minutes() > 3 {
		grace = 3 * time.Minute
	}

	timeout := time.After(w.config.JobInterval - grace)

	dbExec := w.executorFactory.NewExecutor()
	exec, err := w.analyticsFactory.GetExecutorWithSource(ctx, "pg")
	if err != nil {
		return errors.Wrap(err, "could not build executor")
	}

	dataModel, err := w.repository.GetDataModel(ctx, dbExec, job.Args.OrgId, false, false)
	if err != nil {
		return errors.Wrap(err, "could not retrieve data model")
	}

RepeatLoop:
	for {
		select {
		case <-timeout:
			break RepeatLoop
		default:
		}

		var wg errgroup.Group

		insertedRows := false

		for _, table := range dataModel.Tables {
			ctx = utils.StoreLoggerInContext(ctx, logger.With("table", table.Name))
			logger.DebugContext(ctx, fmt.Sprintf(`exporting data for table "%s"`, table.Name))
			triggerFields := make([]models.Field, 0)
			dbFields := make([]models.Field, 0)

			if settings, err := w.repository.GetAnalyticsSettings(ctx, dbExec, job.Args.OrgId); err == nil {
				if setting, ok := settings[table.Name]; ok {
					triggerFields = pure_utils.Map(setting.TriggerFields, func(name string) models.Field {
						return table.Fields[name]
					})

					for _, f := range setting.DbFields {
						if field, ok := dataModel.FindField(table, f.Path, f.Name); ok {
							field.Name = f.Ident()

							dbFields = append(dbFields, field)
						}
					}
				}
			}

			wg.Go(func() error {
				req := repositories.AnalyticsCopyRequest{
					OrgId:               job.Args.OrgId,
					Table:               w.analyticsFactory.BuildTablePrefix("decisions"),
					TriggerObject:       table.Name,
					TriggerObjectFields: triggerFields,
					ExtraDbFields:       dbFields,
					EndTime:             job.CreatedAt,
					Limit:               w.config.ExportBatchSize,
				}

				nRows, err := w.exportDecisions(ctx, exec, req)

				if nRows > 0 {
					insertedRows = true
				}

				return err
			})

			if w.license.Analytics {
				wg.Go(func() error {
					req := repositories.AnalyticsCopyRequest{
						OrgId:               job.Args.OrgId,
						Table:               w.analyticsFactory.BuildTablePrefix("decision_rules"),
						TriggerObject:       table.Name,
						TriggerObjectFields: triggerFields,
						ExtraDbFields:       dbFields,
						EndTime:             job.CreatedAt,
						Limit:               w.config.ExportBatchSize,
					}

					nRows, err := w.exportDecisionRules(ctx, exec, req)

					if nRows > 0 {
						insertedRows = true
					}

					return err
				})

				wg.Go(func() error {
					req := repositories.AnalyticsCopyRequest{
						OrgId:               job.Args.OrgId,
						Table:               w.analyticsFactory.BuildTablePrefix("screenings"),
						TriggerObject:       table.Name,
						TriggerObjectFields: triggerFields,
						ExtraDbFields:       dbFields,
						EndTime:             job.CreatedAt,
						Limit:               w.config.ExportBatchSize,
					}

					nRows, err := w.exportScreenings(ctx, exec, req)

					if nRows > 0 {
						insertedRows = true
					}

					return err
				})
			}

			if err := wg.Wait(); err != nil {
				logger.ErrorContext(ctx, "failed to export analytics data", "error", err.Error())
				return errors.Wrap(err, "failed to export data")
			}

		}

		if !insertedRows {
			break
		}
	}

	return nil
}

func logExportResult(ctx context.Context, table string, start time.Time, nRows int, startWatermark time.Time, err error) {
	logger := utils.LoggerFromContext(ctx)
	switch {
	case err != nil:
		logger.ErrorContext(ctx, fmt.Sprintf("%s export failed", table), "duration",
			time.Since(start), "error", err.Error())
	case nRows > 0:
		logger.DebugContext(ctx, fmt.Sprintf("%s export succeeded", table), "duration", time.Since(start), "rows", nRows)
	default:
		logger.DebugContext(ctx, fmt.Sprintf("%s export is up to date", table), "duration", time.Since(start))
	}
}

func (w AnalyticsExportWorker) exportDecisions(
	ctx context.Context,
	exec repositories.AnalyticsExecutor,
	req repositories.AnalyticsCopyRequest,
) (nRows int, err error) {
	start := time.Now()
	var startWatermark time.Time
	defer func() {
		logExportResult(ctx, "decisions", start, nRows, startWatermark, err)
	}()

	var id uuid.UUID
	id, startWatermark, err = repositories.AnalyticsGetLatestRow(ctx, exec,
		req.OrgId, req.TriggerObject,
		w.analyticsFactory.BuildTarget("decisions"))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get latest exported row")
	}

	req.Watermark = &models.Watermark{WatermarkId: utils.Ptr(id.String()), WatermarkTime: startWatermark}

	nRows, err = repositories.AnalyticsCopyDecisions(ctx, exec, req)
	if err != nil {
		return 0, errors.Wrap(err, "failed to copy decisions")
	}

	return nRows, nil
}

func (w AnalyticsExportWorker) exportDecisionRules(
	ctx context.Context,
	exec repositories.AnalyticsExecutor,
	req repositories.AnalyticsCopyRequest,
) (nRows int, err error) {
	start := time.Now()
	var startWatermark time.Time
	defer func() {
		logExportResult(ctx, "decision rules", start, nRows, startWatermark, err)
	}()

	var id uuid.UUID
	id, startWatermark, err = repositories.AnalyticsGetLatestRow(ctx, exec,
		req.OrgId, req.TriggerObject,
		w.analyticsFactory.BuildTarget("decision_rules"))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get latest exported row")
	}

	req.Watermark = &models.Watermark{WatermarkId: utils.Ptr(id.String()), WatermarkTime: startWatermark}

	nRows, err = repositories.AnalyticsCopyDecisionRules(ctx, exec, req)
	if err != nil {
		return 0, errors.Wrap(err, "failed to copy decision rules")
	}

	return nRows, nil
}

func (w AnalyticsExportWorker) exportScreenings(
	ctx context.Context,
	exec repositories.AnalyticsExecutor,
	req repositories.AnalyticsCopyRequest,
) (nRows int, err error) {
	start := time.Now()
	var startWatermark time.Time
	defer func() {
		logExportResult(ctx, "screenings", start, nRows, startWatermark, err)
	}()

	var id uuid.UUID
	id, startWatermark, err = repositories.AnalyticsGetLatestRow(ctx, exec,
		req.OrgId, req.TriggerObject,
		w.analyticsFactory.BuildTarget("screenings"))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get latest exported row")
	}

	req.Watermark = &models.Watermark{WatermarkId: utils.Ptr(id.String()), WatermarkTime: startWatermark}

	nRows, err = repositories.AnalyticsCopyScreenings(ctx, exec, req)
	if err != nil {
		return 0, errors.Wrap(err, "failed to copy screenings")
	}

	return nRows, nil
}
