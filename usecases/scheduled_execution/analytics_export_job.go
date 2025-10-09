package scheduled_execution

import (
	"context"
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
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

	dbExec := w.executorFactory.NewExecutor()
	exec, err := w.analyticsFactory.GetExecutorWithSource(ctx, "pg")
	if err != nil {
		return errors.Wrap(err, "could not build executor")
	}

	dataModel, err := w.repository.GetDataModel(ctx, dbExec, job.Args.OrgId, false, false)
	if err != nil {
		return errors.Wrap(err, "could not retrieve data model")
	}

	var wg errgroup.Group

	for _, table := range dataModel.Tables {
		triggerFields := make([]models.Field, 0)
		dbFields := make([]models.Field, 0)

		if settings, err := w.repository.GetAnalyticsSettings(ctx, w.executorFactory.NewExecutor(), job.Args.OrgId); err == nil {
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
				Limit:               50000,
			}

			return w.exportDecisions(ctx, exec, req)
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
					Limit:               50000,
				}

				return w.exportDecisionRules(ctx, exec, req)
			})

			wg.Go(func() error {
				req := repositories.AnalyticsCopyRequest{
					OrgId:               job.Args.OrgId,
					Table:               w.analyticsFactory.BuildTablePrefix("screenings"),
					TriggerObject:       table.Name,
					TriggerObjectFields: triggerFields,
					ExtraDbFields:       dbFields,
					EndTime:             job.CreatedAt,
					Limit:               50000,
				}

				return w.exportScreenings(ctx, exec, req)
			})
		}

		if err := wg.Wait(); err != nil {
			logger.ErrorContext(ctx, "failed to export analytics data", "error", err.Error())
			return errors.Wrap(err, "failed to export data")
		}
	}

	return nil
}

func (w AnalyticsExportWorker) exportDecisions(
	ctx context.Context,
	exec repositories.AnalyticsExecutor,
	req repositories.AnalyticsCopyRequest) error {

	id, createdAt, err := repositories.AnalyticsGetLatestRow(ctx, exec, w.analyticsFactory.BuildTarget("decisions", &req.TriggerObject))
	if err != nil {
		return errors.Wrap(err, "failed to get latest exported row")
	}

	req.Watermark = &models.Watermark{WatermarkId: utils.Ptr(id.String()), WatermarkTime: createdAt}

	_, err = repositories.AnalyticsCopyDecisions(ctx, exec, req)
	if err != nil {
		return errors.Wrap(err, "failed to copy decisions")
	}

	return nil
}

func (w AnalyticsExportWorker) exportDecisionRules(
	ctx context.Context,
	exec repositories.AnalyticsExecutor,
	req repositories.AnalyticsCopyRequest) error {

	id, createdAt, err := repositories.AnalyticsGetLatestRow(ctx, exec, w.analyticsFactory.BuildTarget("decision_rules", &req.TriggerObject))
	if err != nil {
		return errors.Wrap(err, "failed to get latest exported row")
	}

	req.Watermark = &models.Watermark{WatermarkId: utils.Ptr(id.String()), WatermarkTime: createdAt}

	_, err = repositories.AnalyticsCopyDecisionRules(ctx, exec, req)
	if err != nil {
		return errors.Wrap(err, "failed to copy decision rules")
	}

	return nil
}

func (w AnalyticsExportWorker) exportScreenings(
	ctx context.Context,
	exec repositories.AnalyticsExecutor,
	req repositories.AnalyticsCopyRequest) error {

	id, createdAt, err := repositories.AnalyticsGetLatestRow(ctx, exec, w.analyticsFactory.BuildTarget("screenings", &req.TriggerObject))
	if err != nil {
		return errors.Wrap(err, "failed to get latest exported row")
	}

	req.Watermark = &models.Watermark{WatermarkId: utils.Ptr(id.String()), WatermarkTime: createdAt}

	_, err = repositories.AnalyticsCopyScreenings(ctx, exec, req)
	if err != nil {
		return errors.Wrap(err, "failed to copy screenings")
	}

	return nil
}
