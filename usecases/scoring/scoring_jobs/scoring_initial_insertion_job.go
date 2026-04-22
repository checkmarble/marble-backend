package scoring_jobs

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/feature_access"
	"github.com/checkmarble/marble-backend/usecases/scoring"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type initialScoringRepository interface {
	GetMetadata(ctx context.Context, exec repositories.Executor, orgID *uuid.UUID, key models.MetadataKey) (*models.Metadata, error)
	UpsertMetadata(ctx context.Context, exec repositories.Executor, metadata models.Metadata) error
}

func NewInitialInsertionJob(orgId uuid.UUID, interval time.Duration) *river.PeriodicJob {
	return worker_jobs.NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.ScoringInitialInsertionArgs{
					OrgId: orgId,
				}, &river.InsertOpts{
					Queue: orgId.String(),
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: interval,
					},
				}
		},
	)
}

type InitialInsertionWorker struct {
	river.WorkerDefaults[models.ScoringInitialInsertionArgs]

	executorFactory          executor_factory.ExecutorFactory
	featureAccessReader      feature_access.FeatureAccessReader
	rulesetUsecase           scoring.ScoringRulesetsUsecase
	repository               scoring.ScoringRepository
	initialScoringRepository initialScoringRepository
	ingestedDataReader       repositories.IngestedDataReadRepository
}

func NewInitialInsertionWorker(
	executorFactory executor_factory.ExecutorFactory,
	featureAccessReader feature_access.FeatureAccessReader,
	rulesetUsecase scoring.ScoringRulesetsUsecase,
	repository scoring.ScoringRepository,
	initialScoringRepository initialScoringRepository,
	ingestedDataReader repositories.IngestedDataReadRepository,
) *InitialInsertionWorker {
	return &InitialInsertionWorker{
		executorFactory:          executorFactory,
		featureAccessReader:      featureAccessReader,
		rulesetUsecase:           rulesetUsecase,
		repository:               repository,
		initialScoringRepository: initialScoringRepository,
		ingestedDataReader:       ingestedDataReader,
	}
}

func (w *InitialInsertionWorker) Work(ctx context.Context, job *river.Job[models.ScoringInitialInsertionArgs]) error {
	featureAccess, err := w.featureAccessReader.GetOrganizationFeatureAccess(ctx, job.Args.OrgId, nil)
	if err != nil {
		return err
	}
	if !featureAccess.UserScoring.IsAllowed() {
		return nil
	}

	exec := w.executorFactory.NewExecutor()

	rulesets, err := w.repository.ListScoringRulesets(ctx, exec, job.Args.OrgId)
	if err != nil {
		return err
	}

	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	for _, ruleset := range rulesets {
		metadata, err := w.initialScoringRepository.GetMetadata(ctx, exec, &job.Args.OrgId, buildMetadataKey(ruleset.RecordType))
		if err != nil {
			return errors.Wrap(err, "could not retrieve initial scoring status metadata")
		}
		if metadata != nil && metadata.Value == "true" {
			continue
		}

		wm := uuid.Nil

		for {
			internalIds, objectIds, err := w.ingestedDataReader.GetObjectsFromInternalId(ctx, clientDbExec, ruleset.RecordType, wm, 1000)
			if err != nil {
				return err
			}
			if len(internalIds) == 0 {
				break
			}

			for idx, objectId := range objectIds {
				internalId := internalIds[idx]

				req := models.InsertScoreRequest{
					OrgId:      job.Args.OrgId,
					RecordType: ruleset.RecordType,
					RecordId:   objectId,
					RulesetId:  &ruleset.Id,
				}

				if err := w.repository.InsertEmptyScore(ctx, exec, req); err != nil {
					return err
				}

				if idx+1 == len(objectIds) {
					wm = internalId
				}
			}
		}

		err = w.initialScoringRepository.UpsertMetadata(ctx, exec, models.Metadata{
			OrgID: &job.Args.OrgId,
			Key:   buildMetadataKey(ruleset.RecordType),
			Value: "true",
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func buildMetadataKey(table string) models.MetadataKey {
	return models.MetadataKey(string(models.ScoringInitialInsertionDone) + ":" + table)
}
