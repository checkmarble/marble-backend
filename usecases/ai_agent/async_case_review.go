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
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/riverqueue/river"
)

type CaseReviewUsecase interface {
	CreateCaseReview(ctx context.Context, caseId string) (models.AiCaseReview, error)
}

type caseReviewRepository interface {
	CreateCaseReviewFile(
		ctx context.Context,
		exec repositories.Executor,
		caseReview models.AiCaseReviewFile,
	) error
}

type CaseReviewWorker struct {
	river.WorkerDefaults[models.CaseReviewArgs]

	blobRepository    repositories.BlobRepository
	caseReviewUsecase CaseReviewUsecase
	executorFactory   executor_factory.ExecutorFactory
	repository        caseReviewRepository
	timeout           time.Duration
	bucketUrl         string
}

func NewCaseReviewWorker(
	blobRepository repositories.BlobRepository,
	bucketUrl string,
	caseReviewUsecase CaseReviewUsecase,
	executorFactory executor_factory.ExecutorFactory,
	repository caseReviewRepository,
	timeout time.Duration,
) CaseReviewWorker {
	return CaseReviewWorker{
		blobRepository:    blobRepository,
		bucketUrl:         "file://./tempFiles/case-manager-bucket?create_dir=true",
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
	cr, err := w.caseReviewUsecase.CreateCaseReview(ctx, job.Args.CaseId)
	if err != nil {
		return errors.Wrap(err, "Error while generating case review")
	}

	crDto := agent_dto.AdaptCaseReviewV1(cr)

	id := uuid.Must(uuid.NewV7())
	fileRef := fmt.Sprintf("ai_case_reviews/%s/%s.json", job.Args.CaseId, id)
	stream, err := w.blobRepository.OpenStream(ctx, w.bucketUrl, fileRef, fileRef)
	if err != nil {
		return errors.Wrap(err, "Error while opening stream")
	}
	defer stream.Close()

	err = json.NewEncoder(stream).Encode(crDto)
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
	return err
}
