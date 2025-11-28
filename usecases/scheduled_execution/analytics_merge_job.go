package scheduled_execution

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"gocloud.dev/blob"
)

func NewAnalyticsMergeJob() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.AnalyticsMergeArgs{}, &river.InsertOpts{
				Queue: "analytics_merge",
				UniqueOpts: river.UniqueOpts{
					ByQueue:  true,
					ByPeriod: time.Hour,
				},
			}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

// AnalyticsMergeWorker runs periodically to compact multiple Parquet files into a single one.
//
// As Parquet files are immutable once created, the export job creates a
// discrete file on each run (every hour), meaning we end up with way too many
// files for our workload (a month would be roughly 720 files). This is bad for
// performance.
//
// Within a partition (org_id, year, month, trigger_object_type), we can easily
// compact all those files into a consolidated mega-file once the partition is
// finalized (a partition is finalized when its period is passed, so current month - 1).
//
// When run, this worker will find the first partition between the watermark and the
// latest finalized partition, and copy all rows within that partition into
// new Parquet files. On write, those rows will still respect their destination
// partition depending on their columns, but will be written into a single file.
//
// This consolidated file, to be easily matchable, will be called
// `merged.parquet`, as opposed to the initial `<uuid>.parquet`.
//
// Once all the partitions have been consolidated, all other files will be
// deleted. S3 API only allows the deletion of one file at a time. This poses
// two issues:
//
//   - There is a time frame where both the split and consolidated files exist
//     in the bucket. During that time, analytics would be duplicated.
//   - If the process stops in the middle of deleting the files, some of them might
//     linger, undeleted, once again offering duplicating analytics. This worker is
//     made so it will catch up, on the next run, to finish the interrupted deletion.
//
// Once all files within a partition are deleted, the watermark will be set to
// that month, so that the next run can move on to the next partition, if it is
// finalized.
type AnalyticsMergeWorker struct {
	river.WorkerDefaults[models.AnalyticsMergeArgs]

	executorFactory  executor_factory.ExecutorFactory
	analyticsFactory executor_factory.AnalyticsExecutorFactory
	license          models.LicenseValidation
	repository       analyticsExportRepository
	config           infra.AnalyticsConfig
	blobRepository   repositories.BlobRepository
}

func NewAnalyticsMergeWorker(
	executorFactory executor_factory.ExecutorFactory,
	analyticsFactory executor_factory.AnalyticsExecutorFactory,
	license models.LicenseValidation,
	repository analyticsExportRepository,
	config infra.AnalyticsConfig,
	blobRepository repositories.BlobRepository,
) *AnalyticsMergeWorker {
	return &AnalyticsMergeWorker{
		executorFactory:  executorFactory,
		analyticsFactory: analyticsFactory,
		license:          license,
		repository:       repository,
		config:           config,
		blobRepository:   blobRepository,
	}
}

func (w *AnalyticsMergeWorker) Timeout(job *river.Job[models.AnalyticsMergeArgs]) time.Duration {
	// A timeout of JobInterval is okay because we set a custom timeout that will be slightly lower.
	return w.config.JobInterval
}

func (w AnalyticsMergeWorker) Work(ctx context.Context, job *river.Job[models.AnalyticsMergeArgs]) error {
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

	orgs, err := w.repository.AllOrganizations(ctx, dbExec)
	if err != nil {
		return errors.Wrap(err, "could not retrieve organizations")
	}

	for _, org := range orgs {
		dataModel, err := w.repository.GetDataModel(ctx, dbExec, org.Id, false, false)
		if err != nil {
			return errors.Wrap(err, "could not retrieve data model")
		}

		for _, table := range dataModel.Tables {
			if err := w.merge(ctx, job, org.Id, dbExec, exec,
				models.WatermarkTypeMergedAnalyticsDecisions, "decisions", table.Name); err != nil {
				return err
			}
			if err := w.merge(ctx, job, org.Id, dbExec, exec,
				models.WatermarkTypeMergedAnalyticsDecisionRules, "decision_rules", table.Name); err != nil {
				return err
			}
			if err := w.merge(ctx, job, org.Id, dbExec, exec,
				models.WatermarkTypeMergedAnalyticsScreenings, "screenings", table.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w AnalyticsMergeWorker) merge(
	ctx context.Context,
	job *river.Job[models.AnalyticsMergeArgs],
	orgId string,
	dbExec repositories.Executor,
	exec repositories.AnalyticsExecutor,
	watermarkType models.WatermarkType,
	kind, tableName string,
) error {
	logger := utils.LoggerFromContext(ctx).With("org_id", orgId, "kind", kind, "trigger_object_type", tableName)

	// Let's find the first partition (e.g. month) that can be merged. The eligible partition is the first month:
	//  * containing at least one row
	//  * after the (year, month) of the saved watermark
	//  * before the current unfinished month
	lhs, foundPartition, err := w.findFirstMergeablePartition(ctx, job, orgId, dbExec, exec, watermarkType, kind, tableName)
	if err != nil {
		return errors.Wrap(err, "could not find first mergeable partition")
	}
	if !foundPartition {
		logger.DebugContext(ctx, "no partition was found")
		return nil
	}

	logger = logger.With("partition", fmt.Sprintf("%d/%d", lhs.Year(), lhs.Month()))
	prefix := fmt.Sprintf("%s/org_id=%s/year=%d/month=%d/trigger_object_type=%s", kind, orgId, lhs.Year(), lhs.Month(), tableName)

	bucket, err := w.blobRepository.RawBucket(ctx, w.config.BucketUrl)
	if err != nil {
		return errors.Wrap(err, "could not retrieve analytics bucket")
	}

	isMerged, err := bucket.Exists(ctx, fmt.Sprintf("%s/merged.parquet", prefix))
	if err != nil {
		return errors.Wrap(err, "could not check if partition was already merged")
	}

	// If we already have a `merged.parquet` files, it means the merge process
	// finished in a previous job but could not be committed to the watermark,
	// we do not run the merge again.
	if !isMerged {
		inner := repositories.NewQueryBuilder().
			Select("*").
			From(w.analyticsFactory.BuildTarget(kind, orgId, tableName)).
			Where("org_id = ?", orgId).
			Where("trigger_object_type = ?", tableName).
			Where("(year, month) = (?, ?)", lhs.Year(), lhs.Month())

		innerSql, args, err := inner.ToSql()
		if err != nil {
			return err
		}

		query := fmt.Sprintf(`copy ( %s ) to '%s/%s/merged.parquet' (format parquet, compression zstd)`, innerSql, w.config.Bucket, prefix)

		if _, err := exec.ExecContext(ctx, query, args...); err != nil {
			return errors.Wrap(err, "could not merge partition")
		}

		logger.InfoContext(ctx, "merged analytics files")
	} else {
		logger.WarnContext(ctx, "previous job might not have finished properly, cleaning up stray files")
	}

	// Delete all files except for `merged.parquet`.
	if err := w.deleteMergedFiles(ctx, bucket, prefix); err != nil {
		return errors.Wrap(err, "could not delete merged files")
	}

	if err := w.repository.SaveWatermark(ctx, dbExec, &orgId,
		models.SpecializedWatermark(watermarkType, tableName), utils.Ptr(uuid.NewString()), lhs, nil); err != nil {
		return errors.Wrap(err, "failed to save watermark")
	}

	return nil
}

func (w AnalyticsMergeWorker) findFirstMergeablePartition(
	ctx context.Context,
	job *river.Job[models.AnalyticsMergeArgs],
	orgId string,
	dbExec repositories.Executor,
	exec repositories.AnalyticsExecutor,
	watermarkType models.WatermarkType,
	kind, tableName string,
) (time.Time, bool, error) {
	// The watermark represents the lower bound of our search for a month to compact
	wm, err := w.repository.GetWatermark(ctx, dbExec, &orgId,
		models.SpecializedWatermark(watermarkType, tableName))
	if err != nil {
		return time.Time{}, false, errors.Wrap(err, "failed to get watermark")
	}

	// From then, we search first the first date with data (minimum of tuple
	// (year, month) above the watermark), unnesting the row() into two separate
	// columns.
	minQuery := repositories.NewQueryBuilder().
		Select("unnest(coalesce(min((year, month)), (0, 0)))").
		From(w.analyticsFactory.BuildTarget(kind, orgId, tableName)).
		Where("org_id = ?", orgId).
		Where("trigger_object_type = ?", tableName)

	if wm != nil {
		minQuery = minQuery.Where("(year, month) > (?, ?)", wm.WatermarkTime.Year(), wm.WatermarkTime.Month())
	}

	sql, args, err := minQuery.ToSql()
	if err != nil {
		return time.Time{}, false, err
	}

	row := exec.QueryRowContext(ctx, sql, args...)
	if err := row.Err(); err != nil {
		if repositories.IsDuckDBNoFilesError(err) {
			return time.Time{}, false, nil
		}

		return time.Time{}, false, err
	}

	var year, month int

	if err := row.Scan(&year, &month); err != nil {
		return time.Time{}, false, err
	}

	// If both are zeroes, it means there is no data for that trigger object type
	if year == 0 && month == 0 {
		return time.Time{}, false, nil
	}

	date := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(job.CreatedAt.Year(), job.CreatedAt.Month(), 1, 0, 0, 0, 0, time.UTC)

	if date.Year() >= now.Year() && date.Month() >= now.Month() {
		return time.Time{}, false, nil
	}

	return date, true, nil
}

func (w *AnalyticsMergeWorker) deleteMergedFiles(ctx context.Context, bucket *blob.Bucket, prefix string) error {
	files := bucket.List(&blob.ListOptions{
		Prefix: prefix,
	})

	// Delete all old files. We have a failure condition here, if the process
	// dies or crashes out in the middle of the loop, some files will not be
	// deleted, which means some data will be duplicated.
	//
	// The next run will be able to clean up stray files.
	for {
		file, err := files.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		if strings.HasSuffix(file.Key, "merged.parquet") {
			continue
		}

		if err := bucket.Delete(ctx, file.Key); err != nil {
			return err
		}
	}

	return nil
}
