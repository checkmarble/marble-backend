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
	CreateCaseReviewSync(ctx context.Context, caseId string, caseReviewContext *CaseReviewContext) (agent_dto.AiCaseReviewDto, error)
	HasAiCaseReviewEnabled(ctx context.Context, orgId string) (bool, error)
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

	// Check if the organization has AI case review enabled
	hasAiCaseReviewEnabled, err := w.caseReviewUsecase.HasAiCaseReviewEnabled(ctx, c.OrganizationId)
	if err != nil {
		return errors.Wrap(err, "Error while checking if AI case review is enabled")
	}
	if !hasAiCaseReviewEnabled {
		logger.DebugContext(ctx, "AI case review is not enabled for organization", "organization_id", c.OrganizationId)
		return nil
	}

	// Get the case review file object from the database
	// aiCaseReview, err := w.repository.GetCaseReviewById(ctx,
	// 	w.executorFactory.NewExecutor(), job.Args.AiCaseReviewId)
	aiCaseReview, err := w.repository.GetCaseReviewById(
		ctx,
		w.executorFactory.NewExecutor(),
		uuid.MustParse("0198d157-18cc-743d-8252-6479bfc78884"),
	)
	if err != nil {
		return errors.Wrap(err, "Error while getting case review file")
	}

	// Get case review temporary file
	caseReviewContext, err := w.getPreviousCaseReviewContext(ctx, aiCaseReview)
	if err != nil {
		return errors.Wrap(err, "Error while getting previous case review context")
	}

	cr, err := w.caseReviewUsecase.CreateCaseReviewSync(ctx, job.Args.CaseId.String(), &caseReviewContext)
	if err != nil {
		return w.handleCreateCaseReviewSyncError(
			ctx,
			aiCaseReview,
			&caseReviewContext,
			errors.Wrap(err, "Error while generating case review"),
		)
	}

	logger.DebugContext(ctx, "Finished generating case review", "case_id", job.Args.CaseId)

	stream, err := w.blobRepository.OpenStream(ctx, w.bucketUrl, aiCaseReview.FileReference, aiCaseReview.FileReference)
	if err != nil {
		return w.handleCreateCaseReviewSyncError(
			ctx,
			aiCaseReview,
			&caseReviewContext,
			errors.Wrap(err, "Error while opening stream"),
		)
	}
	defer stream.Close()

	err = json.NewEncoder(stream).Encode(cr)
	if err != nil {
		return w.handleCreateCaseReviewSyncError(
			ctx,
			aiCaseReview,
			&caseReviewContext,
			errors.Wrap(err, "Error while encoding case review"),
		)
	}

	err = w.repository.UpdateCaseReviewFile(ctx, w.executorFactory.NewExecutor(),
		aiCaseReview.Id, models.UpdateAiCaseReview{
			Status: models.AiCaseReviewStatusCompleted,
		})
	if err != nil {
		return w.handleCreateCaseReviewSyncError(
			ctx,
			aiCaseReview,
			&caseReviewContext,
			errors.Wrap(err, "Error while updating case review file status"),
		)
	}
	logger.DebugContext(ctx, "Finished creating case review file", "case_id", job.Args.CaseId, "review_id", aiCaseReview.Id)

	return nil
}

// handleCreateCaseReviewSyncError is a helper function to handle errors during the case review process
// It stores the case review context into a blob and updates the case review file status to failed
// It returns the original error
func (w *CaseReviewWorker) handleCreateCaseReviewSyncError(
	ctx context.Context,
	aiCaseReview models.AiCaseReview,
	caseReviewContext *CaseReviewContext,
	err error,
) error {
	// Store the case review context into a blob
	stream, errStream := w.blobRepository.OpenStream(ctx, w.bucketUrl,
		aiCaseReview.FileTempReference, aiCaseReview.FileTempReference)
	if errStream != nil {
		return errors.Join(err, errors.Wrap(errStream,
			"Error while opening temporary file stream"))
	}
	defer stream.Close()

	errEncode := json.NewEncoder(stream).Encode(caseReviewContext)
	if errEncode != nil {
		return errors.Join(
			err,
			errors.Wrap(errEncode, "Error while encoding case review context to temporary file"),
		)
	}

	errUpdate := w.repository.UpdateCaseReviewFile(ctx, w.executorFactory.NewExecutor(),
		aiCaseReview.Id, models.UpdateAiCaseReview{
			Status: models.AiCaseReviewStatusFailed,
		})
	if errUpdate != nil {
		return errors.Join(
			err,
			errors.Wrap(errUpdate, "Error while updating case review file status"),
		)
	}

	return err
}

// Get from blob the previous case review context if it exists
// If not, return an empty case review context
func (w *CaseReviewWorker) getPreviousCaseReviewContext(
	ctx context.Context,
	aiCaseReview models.AiCaseReview,
) (CaseReviewContext, error) {
	caseReviewContext := CaseReviewContext{}
	caseReviewContextBlob, err := w.blobRepository.GetBlob(
		ctx,
		w.bucketUrl,
		aiCaseReview.FileTempReference,
	)
	if err == nil {
		defer caseReviewContextBlob.ReadCloser.Close()
		err = json.NewDecoder(caseReviewContextBlob.ReadCloser).Decode(&caseReviewContext)
		if err != nil {
			return caseReviewContext, err
		}
	}

	return caseReviewContext, nil
}
