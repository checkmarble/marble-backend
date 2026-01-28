package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
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
		return errors.Wrap(err, "failed fetch ingested object")
	}

	err = usecase.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			ort, err := usecase.repository.UpsertObjectRiskTopic(
				ctx,
				tx,
				models.ObjectRiskTopicCreate{
					OrgId:      input.OrgId,
					ObjectType: input.ObjectType,
					ObjectId:   input.ObjectId,
					Topics:     input.Topics,
				},
			)
			if err != nil {
				return err
			}

			err = usecase.repository.InsertObjectRiskTopicEvent(
				ctx,
				tx,
				models.ObjectRiskTopicEventCreate{
					OrgId:              input.OrgId,
					ObjectRiskTopicsId: ort.Id,
					Topics:             input.Topics,
					SourceType:         input.SourceType,
					SourceDetails:      input.SourceDetails,
					UserId:             &input.UserId,
				},
			)
			if err != nil {
				return err
			}
			return nil
		},
	)

	return err
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
