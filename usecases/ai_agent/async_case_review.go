package ai_agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type CaseReviewUsecase interface {
	CreateCaseReviewSync(ctx context.Context, caseId string) (agent_dto.AiCaseReviewDto, error)
}

type caseReviewWorkerRepository interface {
	CreateCaseReviewFile(
		ctx context.Context,
		exec repositories.Executor,
		caseReview models.AiCaseReview,
	) error
	GetCaseReviewById(
		ctx context.Context,
		exec repositories.Executor,
		aiCaseReviewId uuid.UUID,
	) (models.AiCaseReview, error)
	UpdateCaseReviewFile(
		ctx context.Context,
		exec repositories.Executor,
		caseReviewId uuid.UUID,
		status models.UpdateAiCaseReview,
	) error
	ListCaseReviewFiles(
		ctx context.Context,
		exec repositories.Executor,
		caseId uuid.UUID,
	) ([]models.AiCaseReview, error)
	GetCaseById(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error)
	GetOrganizationById(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
	) (models.Organization, error)
}

type CaseReviewWorker struct {
	river.WorkerDefaults[models.CaseReviewArgs]

	blobRepository    repositories.BlobRepository
	caseReviewUsecase CaseReviewUsecase
	executorFactory   executor_factory.ExecutorFactory
	repository        caseReviewWorkerRepository
	timeout           time.Duration
	bucketUrl         string
}

func NewCaseReviewWorker(
	blobRepository repositories.BlobRepository,
	bucketUrl string,
	caseReviewUsecase CaseReviewUsecase,
	executorFactory executor_factory.ExecutorFactory,
	repository caseReviewWorkerRepository,
	timeout time.Duration,
) CaseReviewWorker {
	return CaseReviewWorker{
		blobRepository:    blobRepository,
		bucketUrl:         bucketUrl,
		caseReviewUsecase: caseReviewUsecase,
		executorFactory:   executorFactory,
		repository:        repository,
		timeout:           timeout,
	}
}

func (w *CaseReviewWorker) Timeout(job *river.Job[models.CaseReviewArgs]) time.Duration {
	return w.timeout
}

func (w *CaseReviewWorker) Work(ctx context.Context, job *river.Job[models.CaseReviewArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	c, err := w.repository.GetCaseById(ctx, w.executorFactory.NewExecutor(), job.Args.CaseId.String())
	if err != nil {
		return errors.Wrap(err, "Error while getting case")
	}

	// Check if the organization has AI case review enabled, fetch the organization and check the flag
	org, err := w.repository.GetOrganizationById(ctx, w.executorFactory.NewExecutor(), c.OrganizationId)
	if err != nil {
		return errors.Wrap(err, "Error while getting organization")
	}
	if !org.AiCaseReviewEnabled {
		logger.DebugContext(ctx, "AI case review is not enabled for organization", "organization_id", c.OrganizationId)
		return nil
	}

	// Get the case review file object from the database
	aiCaseReview, err := w.repository.GetCaseReviewById(ctx,
		w.executorFactory.NewExecutor(), job.Args.AiCaseReviewId)
	if err != nil {
		return errors.Wrap(err, "Error while getting case review file")
	}

	cr, err := w.caseReviewUsecase.CreateCaseReviewSync(ctx, job.Args.CaseId.String())
	if err != nil {
		errUpdate := w.repository.UpdateCaseReviewFile(ctx, w.executorFactory.NewExecutor(),
			aiCaseReview.Id, models.UpdateAiCaseReview{
				Status: models.AiCaseReviewStatusFailed,
			})
		if errUpdate != nil {
			return errors.Join(
				errors.Wrap(errUpdate, "Error while updating case review file status"),
				errors.Wrap(err, "Error while generating case review"))
		}
		return errors.Wrap(err, "Error while generating case review")
	}
	logger.DebugContext(ctx, "Finished generating case review", "case_id", job.Args.CaseId)

	stream, err := w.blobRepository.OpenStream(ctx, w.bucketUrl, aiCaseReview.FileReference, aiCaseReview.FileReference)
	if err != nil {
		errUpdate := w.repository.UpdateCaseReviewFile(ctx, w.executorFactory.NewExecutor(),
			aiCaseReview.Id, models.UpdateAiCaseReview{
				Status: models.AiCaseReviewStatusFailed,
			})
		if errUpdate != nil {
			return errors.Join(
				errors.Wrap(errUpdate, "Error while updating case review file status"),
				errors.Wrap(err, "Error while opening stream"))
		}
		return errors.Wrap(err, "Error while opening stream")
	}
	defer stream.Close()

	err = json.NewEncoder(stream).Encode(cr)
	if err != nil {
		errUpdate := w.repository.UpdateCaseReviewFile(ctx, w.executorFactory.NewExecutor(),
			aiCaseReview.Id, models.UpdateAiCaseReview{
				Status: models.AiCaseReviewStatusFailed,
			})
		if errUpdate != nil {
			return errors.Join(
				errors.Wrap(errUpdate, "Error while updating case review file status"),
				errors.Wrap(err, "Error while encoding case review"))
		}
		return errors.Wrap(err, "Error while encoding case review")
	}

	err = w.repository.UpdateCaseReviewFile(ctx, w.executorFactory.NewExecutor(),
		aiCaseReview.Id, models.UpdateAiCaseReview{
			Status: models.AiCaseReviewStatusCompleted,
		})
	if err != nil {
		return errors.Wrap(err, "Error while updating case review file status")
	}
	logger.DebugContext(ctx, "Finished creating case review file", "case_id", job.Args.CaseId, "review_id", aiCaseReview.Id)

	return nil
}
