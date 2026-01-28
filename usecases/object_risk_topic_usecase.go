package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

type objectRiskTopicRepository interface {
	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID uuid.UUID,
		fetchEnumValues bool,
		useCache bool,
	) (models.DataModel, error)
	GetObjectRiskTopicById(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
	) (models.ObjectRiskTopic, error)
	GetObjectRiskTopicByObjectId(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		objectType string,
		objectId string,
	) (models.ObjectRiskTopic, error)
	ListObjectRiskTopics(
		ctx context.Context,
		exec repositories.Executor,
		filter models.ObjectRiskTopicFilter,
		paginationAndSorting models.PaginationAndSorting,
	) ([]models.ObjectRiskTopic, error)
	UpsertObjectRiskTopic(
		ctx context.Context,
		exec repositories.Executor,
		input models.ObjectRiskTopicCreate,
	) (models.ObjectRiskTopic, error)
	InsertObjectRiskTopicEvent(
		ctx context.Context,
		exec repositories.Executor,
		event models.ObjectRiskTopicEventCreate,
	) error
	ListObjectRiskTopicEvents(
		ctx context.Context,
		exec repositories.Executor,
		objectRiskTopicsId uuid.UUID,
	) ([]models.ObjectRiskTopicEvent, error)
}

type ObjectRiskTopicUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	enforceSecurity    security.EnforceSecurityObjectRiskTopic
	repository         objectRiskTopicRepository
	ingestedDataReader repositories.IngestedDataReadRepository
}

func NewObjectRiskTopicUsecase(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	enforceSecurity security.EnforceSecurityObjectRiskTopic,
	repo objectRiskTopicRepository,
	ingestedDataReader repositories.IngestedDataReadRepository,
) *ObjectRiskTopicUsecase {
	return &ObjectRiskTopicUsecase{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		enforceSecurity:    enforceSecurity,
		repository:         repo,
		ingestedDataReader: ingestedDataReader,
	}
}

func (usecase *ObjectRiskTopicUsecase) ListObjectRiskTopics(
	ctx context.Context,
	filter models.ObjectRiskTopicFilter,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ObjectRiskTopic, error) {
	exec := usecase.executorFactory.NewExecutor()

	objectRisks, err := usecase.repository.ListObjectRiskTopics(ctx, exec, filter, paginationAndSorting)
	if err != nil {
		return nil, err
	}

	for _, objectRisk := range objectRisks {
		if err := usecase.enforceSecurity.ReadObjectRiskTopic(objectRisk); err != nil {
			return nil, err
		}
	}

	return objectRisks, nil
}

func (usecase *ObjectRiskTopicUsecase) GetObjectRiskTopicById(
	ctx context.Context,
	id uuid.UUID,
) (models.ObjectRiskTopic, error) {
	exec := usecase.executorFactory.NewExecutor()

	objectRiskTopic, err := usecase.repository.GetObjectRiskTopicById(ctx, exec, id)
	if err != nil {
		return models.ObjectRiskTopic{}, err
	}

	if err := usecase.enforceSecurity.ReadObjectRiskTopic(objectRiskTopic); err != nil {
		return models.ObjectRiskTopic{}, err
	}

	return objectRiskTopic, nil
}

func (usecase *ObjectRiskTopicUsecase) UpsertObjectRiskTopic(
	ctx context.Context,
	input models.ObjectRiskTopicWithEventUpsert,
) error {
	if err := usecase.enforceSecurity.WriteObjectRiskTopic(input.OrgId); err != nil {
		return err
	}

	execDbClient, err := usecase.executorFactory.NewClientDbExecutor(ctx, input.OrgId)
	if err != nil {
		return err
	}
	exec := usecase.executorFactory.NewExecutor()

	// Fetch datamodel for querying ingested object
	dataModel, err := usecase.repository.GetDataModel(ctx, exec, input.OrgId, false, true)
	if err != nil {
		return err
	}

	// Check if the object type exists in the data model
	table, ok := dataModel.Tables[input.ObjectType]
	if !ok {
		return errors.Wrapf(models.BadParameterError,
			"table %s not found in data model", input.ObjectType)
	}

	// Check if the object exists and valid
	_, err = usecase.ingestedDataReader.QueryIngestedObject(
		ctx,
		execDbClient,
		table,
		input.ObjectId,
	)
	if err != nil {
		return errors.Wrap(err, "failed to fetch ingested object, can not create risk topic for non-existent object")
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return usecase.appendObjectRiskTopicsInternal(ctx, tx, input)
	})
}

// AppendObjectRiskTopics adds new topics to an object within an existing transaction.
// For internal use by other usecases. Skips ingested object validation.
func (usecase *ObjectRiskTopicUsecase) AppendObjectRiskTopics(
	ctx context.Context,
	exec repositories.Executor,
	input models.ObjectRiskTopicWithEventUpsert,
) error {
	return usecase.appendObjectRiskTopicsInternal(ctx, exec, input)
}

// appendObjectRiskTopicsInternal is the shared logic for appending topics.
// It checks existing topics and only adds new ones.
func (usecase *ObjectRiskTopicUsecase) appendObjectRiskTopicsInternal(
	ctx context.Context,
	exec repositories.Executor,
	input models.ObjectRiskTopicWithEventUpsert,
) error {
	// Get existing topics (if any)
	existing, err := usecase.repository.GetObjectRiskTopicByObjectId(
		ctx, exec, input.OrgId, input.ObjectType, input.ObjectId)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// Merge topics: existing + new (deduplicated)
	topicSet := make(map[models.RiskTopic]struct{})
	for _, t := range existing.Topics {
		topicSet[t] = struct{}{}
	}

	// Track which topics are actually new (for the event)
	newTopics := make([]models.RiskTopic, 0)
	for _, t := range input.Topics {
		if _, exists := topicSet[t]; !exists {
			topicSet[t] = struct{}{}
			newTopics = append(newTopics, t)
		}
	}

	// If no new topics to add, skip entirely
	if len(newTopics) == 0 {
		return nil
	}

	// Build merged topic list
	mergedTopics := make([]models.RiskTopic, 0, len(topicSet))
	for t := range topicSet {
		mergedTopics = append(mergedTopics, t)
	}

	// Upsert with merged topics
	ort, err := usecase.repository.UpsertObjectRiskTopic(ctx, exec,
		models.ObjectRiskTopicCreate{
			OrgId:      input.OrgId,
			ObjectType: input.ObjectType,
			ObjectId:   input.ObjectId,
			Topics:     mergedTopics,
		},
	)
	if err != nil {
		return err
	}

	// Record event with only the NEW topics that were added
	return usecase.repository.InsertObjectRiskTopicEvent(ctx, exec,
		models.ObjectRiskTopicEventCreate{
			OrgId:              input.OrgId,
			ObjectRiskTopicsId: ort.Id,
			Topics:             newTopics,
			SourceType:         input.SourceType,
			SourceDetails:      input.SourceDetails,
			UserId:             &input.UserId,
		},
	)
}

func (usecase *ObjectRiskTopicUsecase) ListObjectRiskTopicEvents(
	ctx context.Context,
	objectRiskTopicsId uuid.UUID,
) ([]models.ObjectRiskTopicEvent, error) {
	exec := usecase.executorFactory.NewExecutor()

	// Check if the user can read the parent object risk topic
	objectRiskTopic, err := usecase.repository.GetObjectRiskTopicById(ctx, exec, objectRiskTopicsId)
	if err != nil {
		return nil, err
	}
	if err := usecase.enforceSecurity.ReadObjectRiskTopic(objectRiskTopic); err != nil {
		return nil, err
	}

	events, err := usecase.repository.ListObjectRiskTopicEvents(ctx, exec, objectRiskTopicsId)
	if err != nil {
		return nil, err
	}

	return events, nil
}
