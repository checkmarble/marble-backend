package usecases

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type SuspiciousActivityReportCaseUsecase interface {
	GetCase(ctx context.Context, id string) (models.Case, error)
	PerformCaseActionSideEffects(ctx context.Context, tx repositories.Transaction, c models.Case) error

	getAvailableInboxIds(ctx context.Context, exec repositories.Executor, organizationId string) ([]uuid.UUID, error)
}

type SuspiciousActivityReportRepository interface {
	ListSuspiciousActivityReportsByCaseId(ctx context.Context, exec repositories.Executor,
		caseId string) ([]models.SuspiciousActivityReport, error)
	GetSuspiciousActivityReportById(ctx context.Context, exec repositories.Executor, caseId, id string,
		forUpdate bool) (models.SuspiciousActivityReport, error)
	CreateSuspiciousActivityReport(ctx context.Context, exec repositories.Executor,
		req models.SuspiciousActivityReportRequest) (models.SuspiciousActivityReport, error)
	UpdateSuspiciousActivityReport(ctx context.Context, exec repositories.Executor,
		req models.SuspiciousActivityReportRequest) (models.SuspiciousActivityReport, error)
	UploadSuspiciousActivityReport(ctx context.Context, tx repositories.Transaction,
		sar models.SuspiciousActivityReport, req models.SuspiciousActivityReportRequest) (models.SuspiciousActivityReport, error)
	DeleteSuspiciousActivityReport(ctx context.Context, exec repositories.Executor,
		req models.SuspiciousActivityReportRequest) error

	CreateCaseEvent(ctx context.Context, exec repositories.Executor,
		createCaseEventAttributes models.CreateCaseEventAttributes) error
}

type SuspiciousActivityReportUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	enforceCaseSecurity security.EnforceSecurityCase

	caseUsecase          SuspiciousActivityReportCaseUsecase
	repository           SuspiciousActivityReportRepository
	blobRepository       repositories.BlobRepository
	caseManagerBucketUrl string
}

func (uc SuspiciousActivityReportUsecase) ListReports(
	ctx context.Context,
	orgId, caseId string,
) ([]models.SuspiciousActivityReport, error) {
	exec := uc.executorFactory.NewExecutor()

	if _, err := uc.hasCasePermissions(ctx, exec, orgId, caseId); err != nil {
		return nil, err
	}

	sars, err := uc.repository.ListSuspiciousActivityReportsByCaseId(ctx, exec, caseId)
	if err != nil {
		return nil, err
	}

	return sars, nil
}

func (uc SuspiciousActivityReportUsecase) CreateReport(
	ctx context.Context,
	orgId string,
	req models.SuspiciousActivityReportRequest,
) (models.SuspiciousActivityReport, error) {
	exec := uc.executorFactory.NewExecutor()

	c, err := uc.hasCasePermissions(ctx, exec, orgId, req.CaseId)
	if err != nil {
		return models.SuspiciousActivityReport{}, err
	}

	if req.Status == nil {
		req.Status = utils.Ptr(models.SarPending)
	}

	if req.File != nil {
		blobKey := fmt.Sprintf("%s/%s/sar/%s", orgId, req.CaseId, uuid.NewString())

		if err := uc.writeToBlobStorage(ctx, *req.File, blobKey); err != nil {
			return models.SuspiciousActivityReport{}, err
		}

		req.Bucket = &uc.caseManagerBucketUrl
		req.BlobKey = &blobKey
	}

	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(
		tx repositories.Transaction,
	) (models.SuspiciousActivityReport, error) {
		sar, err := uc.repository.CreateSuspiciousActivityReport(ctx, exec, req)
		if err != nil {
			return models.SuspiciousActivityReport{}, err
		}

		var userId *string

		if creds, ok := utils.CredentialsFromCtx(ctx); ok {
			userId = utils.Ptr(string(creds.ActorIdentity.UserId))
		}

		if err := uc.caseUsecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return models.SuspiciousActivityReport{}, err
		}

		if err := uc.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			CaseId:       sar.CaseId,
			UserId:       userId,
			EventType:    models.SarCreated,
			ResourceType: utils.Ptr(models.SarResourceType),
			ResourceId:   utils.Ptr(sar.Id),
		}); err != nil {
			return models.SuspiciousActivityReport{}, err
		}

		if req.File != nil {
			if err := uc.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
				CaseId:       sar.CaseId,
				UserId:       userId,
				EventType:    models.SarFileUploaded,
				ResourceType: utils.Ptr(models.SarResourceType),
				ResourceId:   utils.Ptr(sar.Id),
				NewValue:     utils.Ptr(req.File.Filename),
			}); err != nil {
				return models.SuspiciousActivityReport{}, err
			}
		}

		return sar, nil
	})
}

func (uc SuspiciousActivityReportUsecase) UpdateReport(
	ctx context.Context,
	orgId string,
	req models.SuspiciousActivityReportRequest,
) (models.SuspiciousActivityReport, error) {
	exec := uc.executorFactory.NewExecutor()

	c, err := uc.hasCasePermissions(ctx, exec, orgId, req.CaseId)
	if err != nil {
		return models.SuspiciousActivityReport{}, err
	}

	sar, err := uc.repository.GetSuspiciousActivityReportById(ctx, exec, req.CaseId, *req.ReportId, false)
	if err != nil {
		return models.SuspiciousActivityReport{},
			errors.Wrap(models.NotFoundError, err.Error())
	}

	if req.Status != nil && sar.Status == *req.Status && req.File == nil {
		return sar, nil
	}

	if req.Status == nil {
		req.Status = utils.Ptr(models.SarPending)
	}

	if req.File != nil {
		blobKey := fmt.Sprintf("%s/%s/sar/%s", orgId, req.CaseId, uuid.NewString())

		if err := uc.writeToBlobStorage(ctx, *req.File, blobKey); err != nil {
			return models.SuspiciousActivityReport{}, err
		}

		req.Bucket = &uc.caseManagerBucketUrl
		req.BlobKey = &blobKey
	}

	var userId *string

	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		userId = utils.Ptr(string(creds.ActorIdentity.UserId))
	}

	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(
		tx repositories.Transaction,
	) (models.SuspiciousActivityReport, error) {
		var updatedSar models.SuspiciousActivityReport

		if req.File == nil {
			updatedSar, err = uc.repository.UpdateSuspiciousActivityReport(ctx, exec, req)
		} else {
			updatedSar, err = uc.repository.UploadSuspiciousActivityReport(ctx, tx, sar, req)
		}
		if err != nil {
			return models.SuspiciousActivityReport{}, err
		}

		if err := uc.caseUsecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return models.SuspiciousActivityReport{}, err
		}

		if err := uc.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			CaseId:        sar.CaseId,
			UserId:        userId,
			EventType:     models.SarStatusChanged,
			ResourceType:  utils.Ptr(models.SarResourceType),
			ResourceId:    utils.Ptr(updatedSar.Id),
			NewValue:      utils.Ptr(updatedSar.Status.String()),
			PreviousValue: utils.Ptr(sar.Status.String()),
		}); err != nil {
			return models.SuspiciousActivityReport{}, err
		}

		return updatedSar, nil
	})
}

func (uc SuspiciousActivityReportUsecase) GenerateReportUrl(
	ctx context.Context,
	orgId, caseId, reportId string,
) (string, error) {
	exec := uc.executorFactory.NewExecutor()

	if _, err := uc.hasCasePermissions(ctx, exec, orgId, caseId); err != nil {
		return "", err
	}

	sar, err := uc.repository.GetSuspiciousActivityReportById(ctx, exec, caseId, reportId, false)
	if err != nil {
		return "", err
	}

	if sar.Bucket == nil || sar.BlobKey == nil {
		return "", errors.Wrap(models.NotFoundError,
			"this suspicious activity report does not have an attached file")
	}

	return uc.blobRepository.GenerateSignedUrl(ctx, *sar.Bucket, *sar.BlobKey)
}

func (uc SuspiciousActivityReportUsecase) DeleteReport(
	ctx context.Context,
	orgId string,
	req models.SuspiciousActivityReportRequest,
) error {
	exec := uc.executorFactory.NewExecutor()

	sar, err := uc.repository.GetSuspiciousActivityReportById(ctx, exec, req.CaseId, *req.ReportId, true)
	if err != nil {
		return err
	}

	if sar.Status == models.SarCompleted {
		return errors.Wrap(models.UnprocessableEntityError,
			"the suspicious activity report is marked as completed")
	}

	c, err := uc.hasCasePermissions(ctx, exec, orgId, req.CaseId)
	if err != nil {
		return err
	}

	var userId *string

	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		userId = utils.Ptr(string(creds.ActorIdentity.UserId))
	}

	return uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := uc.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			CaseId:       sar.CaseId,
			UserId:       userId,
			EventType:    models.SarDeleted,
			ResourceType: utils.Ptr(models.SarResourceType),
			ResourceId:   utils.Ptr(sar.Id),
		}); err != nil {
			return err
		}

		if err := uc.caseUsecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return err
		}

		return uc.repository.DeleteSuspiciousActivityReport(ctx, exec, req)
	})
}

func (uc SuspiciousActivityReportUsecase) hasCasePermissions(ctx context.Context,
	exec repositories.Executor, orgId, caseId string,
) (models.Case, error) {
	c, err := uc.caseUsecase.GetCase(ctx, caseId)
	if err != nil {
		return models.Case{}, errors.Wrap(models.NotFoundError, err.Error())
	}

	inboxIds, err := uc.caseUsecase.getAvailableInboxIds(ctx, exec, orgId)
	if err != nil {
		return models.Case{}, err
	}

	if err := uc.enforceCaseSecurity.ReadOrUpdateCase(c.GetMetadata(), inboxIds); err != nil {
		return models.Case{}, err
	}

	return c, nil
}

func (uc SuspiciousActivityReportUsecase) writeToBlobStorage(ctx context.Context, fileHeader multipart.FileHeader, newFileReference string,
) error {
	writer, err := uc.blobRepository.OpenStream(ctx, uc.caseManagerBucketUrl, newFileReference, fileHeader.Filename)
	if err != nil {
		return err
	}
	defer writer.Close() // We should still call Close when we are finished writing to check the error if any - this is a no-op if Close has already been called

	file, err := fileHeader.Open()
	if err != nil {
		return errors.Wrap(models.BadParameterError, err.Error())
	}
	if _, err := io.Copy(writer, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	return nil
}
