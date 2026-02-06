package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

type EntityAnnotationRepository interface {
	CreateCaseEvent(ctx context.Context, exec repositories.Executor,
		createCaseEventAttributes models.CreateCaseEventAttributes) (models.CaseEvent, error)
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
	UpdateEntityAnnotationPayload(ctx context.Context, exec repositories.Executor,
		orgId uuid.UUID, annotationId string, payload models.EntityAnnotationPayload) (models.EntityAnnotation, error)
	FindEntityAnnotationsWithRiskTopics(ctx context.Context, exec repositories.Executor,
		filter models.EntityAnnotationRiskTopicsFilter) ([]models.EntityAnnotation, error)
}

type EntityAnnotationCaseUsecase interface {
	AttachAnnotation(ctx context.Context, tx repositories.Transaction, annotationId string,
		annotationReq models.CreateEntityAnnotationRequest) error
	AttachAnnotationFiles(ctx context.Context, tx repositories.Transaction, annotationId string,
		annotationReq models.CreateEntityAnnotationRequest, files []models.EntityAnnotationFilePayloadFile) error
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

	blobRepository      repositories.BlobRepository
	bucketUrl           string
	taskQueueRepository repositories.TaskQueueRepository

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
}

func (uc EntityAnnotationUsecase) List(ctx context.Context, req models.EntityAnnotationRequest) ([]models.EntityAnnotation, error) {
	if err := uc.checkObject(ctx, req.OrgId, req.ObjectType); err != nil {
		return nil, errors.Wrap(models.NotFoundError, err.Error())
	}

	annotations, err := uc.repository.GetEntityAnnotations(ctx, uc.executorFactory.NewExecutor(), req)
	if err != nil {
		return nil, err
	}

	if req.LoadThumbnails {
		for annIdx, ann := range annotations {
			if ann.AnnotationType == models.EntityAnnotationFile {
				files := gjson.GetBytes(ann.Payload, "files").Array()

				annotations[annIdx].FileThumbnails = make([]string, len(files))
				annotations[annIdx].FileContentTypes = make([]string, len(files))

				for fileIdx, file := range files {
					key := file.Get("key").String()
					thumbKey := models.ThumbnailFileName(key)
					thumbnailUrl, err := uc.blobRepository.GenerateSignedUrl(ctx, uc.bucketUrl, thumbKey)

					annotations[annIdx].FileContentTypes[fileIdx] = uc.blobRepository.GetContentType(ctx, uc.bucketUrl, key)

					if err == nil {
						annotations[annIdx].FileThumbnails[fileIdx] = thumbnailUrl
					}
				}
			}
		}
	}

	return annotations, nil
}

func (uc EntityAnnotationUsecase) Get(ctx context.Context, orgId uuid.UUID, id string) (models.EntityAnnotation, error) {
	annotations, err := uc.repository.GetEntityAnnotationById(ctx,
		uc.executorFactory.NewExecutor(), models.AnnotationByIdRequest{
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
	if err := uc.checkObject(ctx, req.OrgId, req.ObjectType); err != nil {
		return nil, errors.Wrap(models.NotFoundError, err.Error())
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
	if err := uc.checkObject(ctx, req.OrgId, req.ObjectType); err != nil {
		return models.EntityAnnotation{}, errors.Wrap(models.NotFoundError, err.Error())
	}

	if err := uc.validateAnnotation(ctx, req); err != nil {
		return models.EntityAnnotation{}, err
	}

	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(
		tx repositories.Transaction,
	) (models.EntityAnnotation, error) {
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
	if err := uc.checkObject(ctx, req.OrgId, req.ObjectType); err != nil {
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

	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(
		tx repositories.Transaction,
	) ([]models.EntityAnnotation, error) {
		annotations := make([]models.EntityAnnotation, len(metadata))

		for idx, file := range metadata {
			fileReq := req

			if err := uc.taskQueueRepository.EnqueueGenerateThumbnailTask(ctx, tx, uc.enforceSecurityAnnotation.OrgId(), uc.bucketUrl, file.Key); err != nil {
				return nil, err
			}

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
				if err := uc.caseUsecase.AttachAnnotationFiles(ctx, tx,
					annotation.Id, req, metadata); err != nil {
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

// checkObject validates that the given object type exists in the organization's data model.
// Returns an error if the object type is not found.
func (uc EntityAnnotationUsecase) checkObject(ctx context.Context, orgId uuid.UUID, objectType string) error {
	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, uc.executorFactory.NewExecutor(), orgId, false, false)
	if err != nil {
		return err
	}

	if _, exists := dataModel.Tables[objectType]; !exists {
		return errors.Wrap(models.NotFoundError, "unknown object type")
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

// validateIngestedObjectExists checks that the object exists in the ingested data.
// Used to validate risk topics can only be attached to existing ingested objects.
func (uc EntityAnnotationUsecase) validateIngestedObjectExists(
	ctx context.Context,
	orgId uuid.UUID,
	objectType string,
	objectId string,
) error {
	exec := uc.executorFactory.NewExecutor()

	execDbClient, err := uc.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return err
	}

	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, orgId, false, true)
	if err != nil {
		return err
	}

	table, ok := dataModel.Tables[objectType]
	if !ok {
		return errors.Wrapf(models.BadParameterError,
			"table %s not found in data model", objectType)
	}

	objects, err := uc.ingestedDataReadRepository.QueryIngestedObject(ctx, execDbClient, table, objectId)
	if err != nil {
		return errors.Wrap(err,
			"failed to fetch ingested object, can not create risk topic for non-existent object")
	}
	if len(objects) == 0 {
		return errors.Wrapf(models.NotFoundError, "ingested object not found")
	}

	return nil
}

// UpsertRiskTopicAnnotation creates or updates a risk topic annotation for an object.
// There is one risk topic annotation per object - topics are merged.
func (uc EntityAnnotationUsecase) UpsertRiskTopicAnnotation(
	ctx context.Context,
	input models.ObjectRiskTopicUpsert,
) (models.EntityAnnotation, error) {
	if err := uc.validateIngestedObjectExists(ctx, input.OrgId, input.ObjectType, input.ObjectId); err != nil {
		return models.EntityAnnotation{}, err
	}

	// Sort topics for deterministic ordering
	slices.Sort(input.Topics)

	// Build payload
	payload := models.EntityAnnotationRiskTopicPayload{
		Topics:        input.Topics,
		SourceType:    input.SourceType,
		SourceDetails: input.SourceDetails,
	}

	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(
		tx repositories.Transaction,
	) (models.EntityAnnotation, error) {
		// Try to find existing risk topic annotation for this object
		existing, err := uc.repository.GetEntityAnnotations(ctx, tx, models.EntityAnnotationRequest{
			OrgId:          input.OrgId,
			ObjectType:     input.ObjectType,
			ObjectId:       input.ObjectId,
			AnnotationType: utils.Ptr(models.EntityAnnotationRiskTopic),
		})
		if err != nil {
			return models.EntityAnnotation{}, err
		}

		if len(existing) > 0 {
			// Update existing annotation, should only be one
			return uc.repository.UpdateEntityAnnotationPayload(ctx, tx,
				input.OrgId, existing[0].Id, payload)
		}

		// Create new annotation
		return uc.repository.CreateEntityAnnotation(ctx, tx, models.CreateEntityAnnotationRequest{
			OrgId:          input.OrgId,
			ObjectType:     input.ObjectType,
			ObjectId:       input.ObjectId,
			AnnotationType: models.EntityAnnotationRiskTopic,
			Payload:        payload,
		})
	})
}

// AppendObjectRiskTopics adds new topics to an object's risk topic annotation.
// If no annotation exists, creates one. Topics are merged and deduplicated.
// For internal use by continuous screening. Skips ingested object validation.
func (uc EntityAnnotationUsecase) AppendObjectRiskTopics(
	ctx context.Context,
	tx repositories.Transaction,
	input models.ObjectRiskTopicUpsert,
) error {
	// Get existing annotation (if any)
	riskTopicType := models.EntityAnnotationRiskTopic
	existing, err := uc.repository.GetEntityAnnotations(ctx, tx, models.EntityAnnotationRequest{
		OrgId:          input.OrgId,
		ObjectType:     input.ObjectType,
		ObjectId:       input.ObjectId,
		AnnotationType: &riskTopicType,
	})
	if err != nil {
		return err
	}

	var existingTopics []models.RiskTopic
	var existingAnnotationId string

	if len(existing) > 0 {
		existingAnnotationId = existing[0].Id
		// Parse existing payload
		var existingPayload models.EntityAnnotationRiskTopicPayload
		if err := json.Unmarshal(existing[0].Payload, &existingPayload); err != nil {
			return errors.Wrap(err, "failed to parse existing risk topic payload")
		}
		existingTopics = existingPayload.Topics
	}

	// Merge topics: existing + new (deduplicated)
	topicSet := make(map[models.RiskTopic]struct{})
	for _, t := range existingTopics {
		topicSet[t] = struct{}{}
	}

	hasNewTopics := false
	for _, t := range input.Topics {
		if _, exists := topicSet[t]; !exists {
			topicSet[t] = struct{}{}
			hasNewTopics = true
		}
	}

	// If no new topics to add, skip entirely
	if !hasNewTopics {
		return nil
	}

	// Build merged topic list and sort for deterministic ordering
	mergedTopics := make([]models.RiskTopic, 0, len(topicSet))
	for t := range topicSet {
		mergedTopics = append(mergedTopics, t)
	}
	slices.Sort(mergedTopics)

	// Build payload
	payload := models.EntityAnnotationRiskTopicPayload{
		Topics:        mergedTopics,
		SourceType:    input.SourceType,
		SourceDetails: input.SourceDetails,
	}

	if existingAnnotationId != "" {
		// Update existing annotation
		_, err = uc.repository.UpdateEntityAnnotationPayload(ctx, tx, input.OrgId, existingAnnotationId, payload)
		return err
	}

	// Create new annotation
	_, err = uc.repository.CreateEntityAnnotation(ctx, tx, models.CreateEntityAnnotationRequest{
		OrgId:          input.OrgId,
		ObjectType:     input.ObjectType,
		ObjectId:       input.ObjectId,
		AnnotationType: models.EntityAnnotationRiskTopic,
		Payload:        payload,
	})
	return err
}
