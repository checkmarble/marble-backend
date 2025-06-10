package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type EntityAnnotationRepository interface {
	CreateCaseEvent(ctx context.Context, exec repositories.Executor,
		createCaseEventAttributes models.CreateCaseEventAttributes) error
	GetEntityAnnotationById(
		ctx context.Context,
		exec repositories.Executor,
		req models.AnnotationByIdRequest,
	) ([]models.EntityAnnotation, error)
	GetEntityAnnotations(ctx context.Context, exec repositories.Executor,
		req models.EntityAnnotationRequest) ([]models.EntityAnnotation, error)
	GetEntityAnnotationsForObjects(ctx context.Context, exec repositories.Executor,
		req models.EntityAnnotationRequestForObjects) (map[string][]models.EntityAnnotation, error)
	GetEntityAnnotationsForCase(ctx context.Context, exec repositories.Executor,
		req models.CaseEntityAnnotationRequest) ([]models.EntityAnnotation, error)
	CreateEntityAnnotation(ctx context.Context, exec repositories.Executor,
		req models.CreateEntityAnnotationRequest) (models.EntityAnnotation, error)
	DeleteEntityAnnotation(ctx context.Context, exec repositories.Executor,
		req models.AnnotationByIdRequest) error
	IsObjectTagSet(ctx context.Context, exec repositories.Executor,
		req models.CreateEntityAnnotationRequest, tagId string) (bool, error)
}

type EntityAnnotationCaseUsecase interface {
	AttachAnnotation(ctx context.Context, tx repositories.Transaction, annotationId string, annotationReq models.CreateEntityAnnotationRequest) error
	AttachAnnotationFiles(ctx context.Context, tx repositories.Transaction, annotationId string, annotationReq models.CreateEntityAnnotationRequest, files []models.EntityAnnotationFilePayloadFile) error
}

type TagRepository interface {
	GetTagById(ctx context.Context, exec repositories.Executor, tagId string) (models.Tag, error)
}

type EntityAnnotationUsecase struct {
	enforceSecurityAnnotation security.EnforceSecurityAnnotation

	repository                 EntityAnnotationRepository
	dataModelRepository        repositories.DataModelRepository
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	caseUsecase                EntityAnnotationCaseUsecase
	tagRepository              TagRepository

	blobRepository repositories.BlobRepository
	bucketUrl      string

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
}

func (uc EntityAnnotationUsecase) List(ctx context.Context, req models.EntityAnnotationRequest) ([]models.EntityAnnotation, error) {
	if err := uc.checkObject(ctx, req.OrgId, req.ObjectType, req.ObjectId); err != nil {
		return nil, errors.Wrap(models.NotFoundError, err.Error())
	}

	return uc.repository.GetEntityAnnotations(ctx, uc.executorFactory.NewExecutor(), req)
}
func (uc EntityAnnotationUsecase) Get(ctx context.Context, orgId, id string) (models.EntityAnnotation, error) {
	annotations, err := uc.repository.GetEntityAnnotationById(ctx, uc.executorFactory.NewExecutor(), models.AnnotationByIdRequest{
		OrgId:          orgId,
		AnnotationId:   id,
		IncludeDeleted: true,
	})

	if err != nil {
		return models.EntityAnnotation{}, err
	}
	if len(annotations) == 0 {
		return models.EntityAnnotation{}, errors.Wrap(models.NotFoundError, "requested annotation was not found")
	}

	return annotations[0], nil
}

func (uc EntityAnnotationUsecase) ListForObjects(ctx context.Context,
	req models.EntityAnnotationRequestForObjects,
) (map[string][]models.EntityAnnotation, error) {
	for _, objectId := range req.ObjectIds {
		if err := uc.checkObject(ctx, req.OrgId, req.ObjectType, objectId); err != nil {
			return nil, errors.Wrap(models.NotFoundError, err.Error())
		}
	}

	return uc.repository.GetEntityAnnotationsForObjects(ctx, uc.executorFactory.NewExecutor(), req)
}

func (uc EntityAnnotationUsecase) ListForCase(ctx context.Context,
	req models.CaseEntityAnnotationRequest,
) ([]models.EntityAnnotation, error) {
	return uc.repository.GetEntityAnnotationsForCase(ctx, uc.executorFactory.NewExecutor(), req)
}

func (uc EntityAnnotationUsecase) Attach(ctx context.Context,
	req models.CreateEntityAnnotationRequest,
) (models.EntityAnnotation, error) {
	if err := uc.checkObject(ctx, req.OrgId, req.ObjectType, req.ObjectId); err != nil {
		return models.EntityAnnotation{}, errors.Wrap(models.NotFoundError, err.Error())
	}

	if err := uc.validateAnnotation(ctx, req); err != nil {
		return models.EntityAnnotation{}, err
	}

	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (models.EntityAnnotation, error) {
		annotation, err := uc.repository.CreateEntityAnnotation(ctx, uc.executorFactory.NewExecutor(), req)
		if err != nil {
			return models.EntityAnnotation{}, err
		}

		if req.CaseId != nil {
			if err := uc.caseUsecase.AttachAnnotation(ctx, tx, annotation.Id, req); err != nil {
				return models.EntityAnnotation{}, err
			}
		}

		return annotation, nil
	})
}

func (uc EntityAnnotationUsecase) AttachFile(ctx context.Context,
	req models.CreateEntityAnnotationRequest,
	files []multipart.FileHeader,
) ([]models.EntityAnnotation, error) {
	if err := uc.checkObject(ctx, req.OrgId, req.ObjectType, req.ObjectId); err != nil {
		return nil, errors.Wrap(models.NotFoundError, err.Error())
	}

	metadata := make([]models.EntityAnnotationFilePayloadFile, len(files))

	for idx, file := range files {
		key := fmt.Sprintf("annotations/%s/%s/%s", req.OrgId, req.ObjectType, uuid.NewString())

		if err := uc.writeFileAnnotationToBlobStorage(ctx, file, key); err != nil {
			return nil, err
		}

		metadata[idx] = models.EntityAnnotationFilePayloadFile{
			Id:       uuid.NewString(),
			Key:      key,
			Filename: file.Filename,
		}
	}

	fp, ok := req.Payload.(models.EntityAnnotationFilePayload)
	if !ok {
		return nil, errors.Wrap(models.BadParameterError, "could not understand request")
	}

	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) ([]models.EntityAnnotation, error) {
		annotations := make([]models.EntityAnnotation, len(metadata))

		for idx, file := range metadata {
			fileReq := req

			fileReq.Payload = models.EntityAnnotationFilePayload{
				Caption: fp.Caption,
				Bucket:  uc.bucketUrl,
				Files:   []models.EntityAnnotationFilePayloadFile{file},
			}

			annotation, err := uc.repository.CreateEntityAnnotation(ctx, tx, fileReq)
			if err != nil {
				return nil, err
			}

			if req.CaseId != nil {
				if err := uc.caseUsecase.AttachAnnotationFiles(ctx, tx, annotation.Id, req, metadata); err != nil {
					return nil, err
				}
			}

			annotations[idx] = annotation
		}

		return annotations, nil
	})
}

func (uc EntityAnnotationUsecase) GetFileDownloadUrl(ctx context.Context,
	req models.AnnotationByIdRequest, partId string,
) (string, error) {
	files, err := uc.repository.GetEntityAnnotationById(ctx, uc.executorFactory.NewExecutor(), req)
	if err != nil {
		return "", err
	}
	if len(files) != 1 {
		return "", errors.Wrap(models.NotFoundError, "requested file not found")
	}

	var fp models.EntityAnnotationFilePayload

	if err := json.Unmarshal(files[0].Payload, &fp); err != nil {
		return "", errors.New("could not parse stored annotation payload")
	}

	for _, part := range fp.Files {
		if part.Id == partId {
			return uc.blobRepository.GenerateSignedUrl(ctx, uc.bucketUrl, part.Key)
		}
	}

	return "", errors.Wrap(models.NotFoundError, "could not find requested file part")
}

func (uc EntityAnnotationUsecase) DeleteAnnotation(ctx context.Context,
	req models.AnnotationByIdRequest,
) error {
	if err := uc.enforceSecurityAnnotation.DeleteAnnotation(); err != nil {
		return errors.Wrap(models.ForbiddenError, err.Error())
	}

	return uc.repository.DeleteEntityAnnotation(ctx, uc.executorFactory.NewExecutor(), req)
}

func (uc EntityAnnotationUsecase) checkObject(ctx context.Context, orgId, objectType, objectId string) error {
	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, uc.executorFactory.NewExecutor(), orgId, false)
	if err != nil {
		return err
	}

	table, ok := dataModel.Tables[objectType]
	if !ok {
		return errors.Wrap(models.NotFoundError, "unknown object type")
	}

	db, err := uc.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return err
	}

	if objects, err := uc.ingestedDataReadRepository.QueryIngestedObject(ctx, db, table,
		objectId); err != nil || len(objects) == 0 {
		return errors.Wrap(models.NotFoundError, "unknown object")
	}

	return nil
}

func (uc EntityAnnotationUsecase) validateAnnotation(ctx context.Context, req models.CreateEntityAnnotationRequest) error {
	if req.AnnotationType == models.EntityAnnotationTag {
		payload, ok := req.Payload.(models.EntityAnnotationTagPayload)
		if !ok {
			return errors.New("invalid payload for annotation type")
		}
		tag, err := uc.tagRepository.GetTagById(ctx, uc.executorFactory.NewExecutor(), payload.TagId)
		if err != nil {
			return errors.Wrap(models.NotFoundError, "unknown tag")
		}
		if tag.Target != models.TagTargetObject {
			return errors.Wrap(models.UnprocessableEntityError,
				"provided tag is not targeting ingested objects")
		}
		exists, err := uc.repository.IsObjectTagSet(ctx, uc.executorFactory.NewExecutor(), req, payload.TagId)
		if err != nil {
			return err
		}
		if exists {
			return errors.Wrap(models.ConflictError,
				"tag is already annotated on this object")
		}
	}

	return nil
}

func (uc EntityAnnotationUsecase) writeFileAnnotationToBlobStorage(ctx context.Context, file multipart.FileHeader, key string) error {
	writer, err := uc.blobRepository.OpenStream(ctx, uc.bucketUrl, key, file.Filename)
	if err != nil {
		return err
	}

	defer writer.Close()

	r, err := file.Open()
	if err != nil {
		return err
	}

	if _, err := io.Copy(writer, r); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	return nil
}
