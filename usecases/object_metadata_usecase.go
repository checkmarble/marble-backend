package usecases

import (
	"context"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type objectMetadataRepository interface {
	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID uuid.UUID,
		fetchEnumValues bool,
		useCache bool,
	) (models.DataModel, error)
	GetObjectMetadata(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		objectType string,
		objectId string,
		metadataType models.MetadataType,
	) (models.ObjectMetadata, error)
	UpsertObjectMetadata(
		ctx context.Context,
		exec repositories.Executor,
		input models.ObjectMetadataUpsert,
	) (models.ObjectMetadata, error)
}

type ObjectMetadataUsecase struct {
	executorFactory executor_factory.ExecutorFactory

	enforceSecurity    security.EnforceSecurityObjectMetadata
	repository         objectMetadataRepository
	ingestedDataReader repositories.IngestedDataReadRepository
}

func NewObjectMetadataUsecase(
	executorFactory executor_factory.ExecutorFactory,
	enforceSecurity security.EnforceSecurityObjectMetadata,
	repo objectMetadataRepository,
	ingestedDataReader repositories.IngestedDataReadRepository,
) *ObjectMetadataUsecase {
	return &ObjectMetadataUsecase{
		executorFactory:    executorFactory,
		enforceSecurity:    enforceSecurity,
		repository:         repo,
		ingestedDataReader: ingestedDataReader,
	}
}

func (usecase *ObjectMetadataUsecase) GetObjectMetadata(
	ctx context.Context,
	orgId uuid.UUID,
	objectType string,
	objectId string,
	metadataType models.MetadataType,
) (models.ObjectMetadata, error) {
	exec := usecase.executorFactory.NewExecutor()

	objectMetadata, err := usecase.repository.GetObjectMetadata(
		ctx,
		exec,
		orgId,
		objectType,
		objectId,
		metadataType,
	)
	if err != nil {
		return models.ObjectMetadata{}, err
	}

	if err := usecase.enforceSecurity.ReadObjectMetadata(objectMetadata); err != nil {
		return models.ObjectMetadata{}, err
	}

	return objectMetadata, nil
}

// validateIngestedObjectExists checks that the object type exists in the data model
// and the object exists in the ingested data. This is used to validate metadata
// can only be attached to existing ingested objects.
func (usecase *ObjectMetadataUsecase) validateIngestedObjectExists(
	ctx context.Context,
	orgId uuid.UUID,
	objectType string,
	objectId string,
) error {
	exec := usecase.executorFactory.NewExecutor()

	execDbClient, err := usecase.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return err
	}

	dataModel, err := usecase.repository.GetDataModel(ctx, exec, orgId, false, true)
	if err != nil {
		return err
	}

	table, ok := dataModel.Tables[objectType]
	if !ok {
		return errors.Wrapf(models.BadParameterError,
			"table %s not found in data model", objectType)
	}

	_, err = usecase.ingestedDataReader.QueryIngestedObject(ctx, execDbClient, table, objectId)
	if err != nil {
		return errors.Wrap(err,
			"failed to fetch ingested object, can not create metadata for non-existent object")
	}

	return nil
}

func (usecase *ObjectMetadataUsecase) UpsertObjectRiskTopic(
	ctx context.Context,
	input models.ObjectRiskTopicUpsert,
) (models.ObjectMetadata, error) {
	if err := usecase.enforceSecurity.WriteObjectMetadata(input.OrgId); err != nil {
		return models.ObjectMetadata{}, err
	}

	if err := usecase.validateIngestedObjectExists(ctx, input.OrgId, input.ObjectType, input.ObjectId); err != nil {
		return models.ObjectMetadata{}, err
	}

	// Sort topics for deterministic ordering
	slices.Sort(input.Topics)

	upsertInput := models.ObjectMetadataUpsert{
		OrgId:        input.OrgId,
		ObjectType:   input.ObjectType,
		ObjectId:     input.ObjectId,
		MetadataType: models.MetadataTypeRiskTopics,
		Metadata: models.RiskTopicsMetadata{
			Topics:        input.Topics,
			SourceType:    input.SourceType,
			SourceDetails: input.SourceDetails,
		},
	}

	return usecase.repository.UpsertObjectMetadata(ctx, usecase.executorFactory.NewExecutor(), upsertInput)
}

// AppendObjectRiskTopics adds new topics to an object within an existing transaction.
// For internal use by other usecases. Skips ingested object validation.
func (usecase *ObjectMetadataUsecase) AppendObjectRiskTopics(
	ctx context.Context,
	tx repositories.Transaction,
	input models.ObjectRiskTopicUpsert,
) error {
	// Get existing topics (if any)
	existing, err := usecase.repository.GetObjectMetadata(
		ctx,
		tx,
		input.OrgId,
		input.ObjectType,
		input.ObjectId,
		models.MetadataTypeRiskTopics,
	)
	if err != nil && !errors.Is(err, models.NotFoundError) {
		return err
	}
	var existingTopics []models.RiskTopic
	if existing.Metadata != nil {
		metadataContent, ok := existing.Metadata.(*models.RiskTopicsMetadata)
		if !ok {
			return errors.Errorf("invalid metadata content type for risk topics: %T", existing.Metadata)
		}
		existingTopics = metadataContent.Topics
	}

	// Merge topics: existing + new (deduplicated)
	topicSet := make(map[models.RiskTopic]struct{})
	for _, t := range existingTopics {
		topicSet[t] = struct{}{}
	}

	// Track which topics are actually new
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

	// Upsert with merged topics
	// Audit trail captures the change automatically
	_, err = usecase.repository.UpsertObjectMetadata(
		ctx,
		tx,
		models.ObjectMetadataUpsert{
			OrgId:        input.OrgId,
			ObjectType:   input.ObjectType,
			ObjectId:     input.ObjectId,
			MetadataType: models.MetadataTypeRiskTopics,
			Metadata: models.RiskTopicsMetadata{
				Topics:        mergedTopics,
				SourceType:    input.SourceType,
				SourceDetails: input.SourceDetails,
			},
		},
	)
	return err
}
