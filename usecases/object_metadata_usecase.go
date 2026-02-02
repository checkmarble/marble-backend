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
	ListObjectMetadata(
		ctx context.Context,
		exec repositories.Executor,
		filter models.ObjectMetadataFilter,
		paginationAndSorting models.PaginationAndSorting,
	) ([]models.ObjectMetadata, error)
	GetObjectRiskTopicByObjectId(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		objectType string,
		objectId string,
	) (models.ObjectRiskTopic, error)
	UpsertObjectRiskTopic(
		ctx context.Context,
		exec repositories.Executor,
		input models.ObjectRiskTopicUpsert,
	) (models.ObjectRiskTopic, error)
}

type ObjectMetadataUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	enforceSecurity    security.EnforceSecurityObjectMetadata
	repository         objectMetadataRepository
	ingestedDataReader repositories.IngestedDataReadRepository
}

func NewObjectMetadataUsecase(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	enforceSecurity security.EnforceSecurityObjectMetadata,
	repo objectMetadataRepository,
	ingestedDataReader repositories.IngestedDataReadRepository,
) *ObjectMetadataUsecase {
	return &ObjectMetadataUsecase{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		enforceSecurity:    enforceSecurity,
		repository:         repo,
		ingestedDataReader: ingestedDataReader,
	}
}

func (usecase *ObjectMetadataUsecase) ListObjectMetadata(
	ctx context.Context,
	filter models.ObjectMetadataFilter,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ObjectMetadata, error) {
	exec := usecase.executorFactory.NewExecutor()

	metadata, err := usecase.repository.ListObjectMetadata(ctx, exec, filter, paginationAndSorting)
	if err != nil {
		return nil, err
	}

	// Enforce security for each metadata item
	result := make([]models.ObjectMetadata, 0, len(metadata))
	for _, m := range metadata {
		if err := usecase.enforceSecurity.ReadObjectMetadata(m); err != nil {
			continue // Skip unauthorized items
		}
		result = append(result, m)
	}

	return result, nil
}

func (usecase *ObjectMetadataUsecase) GetObjectRiskTopicByObjectId(
	ctx context.Context,
	orgId uuid.UUID,
	objectType string,
	objectId string,
) (models.ObjectRiskTopic, error) {
	exec := usecase.executorFactory.NewExecutor()

	objectRiskTopic, err := usecase.repository.GetObjectRiskTopicByObjectId(ctx, exec, orgId, objectType, objectId)
	if err != nil {
		return models.ObjectRiskTopic{}, err
	}

	if err := usecase.enforceSecurity.ReadObjectMetadata(objectRiskTopic.ObjectMetadata); err != nil {
		return models.ObjectRiskTopic{}, err
	}

	return objectRiskTopic, nil
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
) (models.ObjectRiskTopic, error) {
	if err := usecase.enforceSecurity.WriteObjectMetadata(input.OrgId); err != nil {
		return models.ObjectRiskTopic{}, err
	}

	if err := usecase.validateIngestedObjectExists(ctx, input.OrgId, input.ObjectType, input.ObjectId); err != nil {
		return models.ObjectRiskTopic{}, err
	}

	// Sort topics for deterministic ordering
	slices.Sort(input.Topics)

	exec := usecase.executorFactory.NewExecutor()
	return usecase.repository.UpsertObjectRiskTopic(ctx, exec, input)
}

// AppendObjectRiskTopics adds new topics to an object within an existing transaction.
// For internal use by other usecases. Skips ingested object validation.
func (usecase *ObjectMetadataUsecase) AppendObjectRiskTopics(
	ctx context.Context,
	tx repositories.Transaction,
	input models.ObjectRiskTopicUpsert,
) error {
	// Get existing topics (if any)
	existing, err := usecase.repository.GetObjectRiskTopicByObjectId(
		ctx,
		tx,
		input.OrgId,
		input.ObjectType,
		input.ObjectId,
	)
	if err != nil && !errors.Is(err, models.NotFoundError) {
		return err
	}

	// Merge topics: existing + new (deduplicated)
	topicSet := make(map[models.RiskTopic]struct{})
	for _, t := range existing.Topics {
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
	_, err = usecase.repository.UpsertObjectRiskTopic(
		ctx,
		tx,
		models.ObjectRiskTopicUpsert{
			OrgId:         input.OrgId,
			ObjectType:    input.ObjectType,
			ObjectId:      input.ObjectId,
			Topics:        mergedTopics,
			SourceType:    input.SourceType,
			SourceDetails: input.SourceDetails,
			UserId:        input.UserId,
		},
	)
	return err
}
