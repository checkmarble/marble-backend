package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/riverqueue/river"
)

type CaseReviewUsecase interface {
	CreateCaseReviewSync(ctx context.Context, caseId string) (agent_dto.AiCaseReviewDto, error)
}

type caseReviewWorkerRepository interface {
	CreateCaseReviewFile(
		ctx context.Context,
		exec repositories.Executor,
		caseReview models.AiCaseReviewFile,
	) error
	ListCaseReviewFiles(
		ctx context.Context,
		exec repositories.Executor,
		caseId uuid.UUID,
	) ([]models.AiCaseReviewFile, error)
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
	c, err := w.repository.GetCaseById(ctx, w.executorFactory.NewExecutor(), job.Args.CaseId)
	if err != nil {
		return errors.Wrap(err, "Error while getting case")
	}

	org, err := w.repository.GetOrganizationById(ctx, w.executorFactory.NewExecutor(), c.OrganizationId)
	if err != nil {
		return errors.Wrap(err, "Error while getting organization")
	}

	if !org.AiCaseReviewEnabled {
		logger.DebugContext(ctx, "AI case review is not enabled for organization", "organization_id", c.OrganizationId)
		return nil
	}

	cr, err := w.caseReviewUsecase.CreateCaseReviewSync(ctx, job.Args.CaseId)
	if err != nil {
		return errors.Wrap(err, "Error while generating case review")
	}
	logger.DebugContext(ctx, "Finished generating case review", "case_id", job.Args.CaseId)

	id := uuid.Must(uuid.NewV7())
	fileRef := fmt.Sprintf("ai_case_reviews/%s/%s.json", job.Args.CaseId, id)
	stream, err := w.blobRepository.OpenStream(ctx, w.bucketUrl, fileRef, fileRef)
	if err != nil {
		return errors.Wrap(err, "Error while opening stream")
	}
	defer stream.Close()

	err = json.NewEncoder(stream).Encode(cr)
	if err != nil {
		return errors.Wrap(err, "Error while encoding case review")
	}

	caseId, err := uuid.Parse(job.Args.CaseId)
	if err != nil {
		return errors.Wrap(err, "Error while parsing case id")
	}

	err = w.repository.CreateCaseReviewFile(ctx, w.executorFactory.NewExecutor(), models.AiCaseReviewFile{
		ID:            id,
		CaseID:        caseId,
		Status:        models.AiCaseReviewFileStatusCompleted.String(),
		BucketName:    w.bucketUrl,
		FileReference: fileRef,
	})
	if err != nil {
		return errors.Wrap(err, "Error while creating case review file")
	}
	logger.DebugContext(ctx, "Finished creating case review file", "case_id", job.Args.CaseId)

	return nil
}
