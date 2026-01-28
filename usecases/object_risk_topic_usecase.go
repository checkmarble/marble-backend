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
	ListObjectRiskTopic(
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

func (usecase *ObjectRiskTopicUsecase) ListObjectRiskTopic(
	ctx context.Context,
	filter models.ObjectRiskTopicFilter,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ObjectRiskTopic, error) {
	exec := usecase.executorFactory.NewExecutor()

	objectRisks, err := usecase.repository.ListObjectRiskTopic(ctx, exec, filter, paginationAndSorting)
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

func (usecase *ObjectRiskTopicUsecase) UpsertObjectRiskTopic(
	ctx context.Context,
	input models.ObjectRiskTopicWithEventUpsert,
) error {
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

	// Check if the object exists and valid
	_, err = usecase.ingestedDataReader.QueryIngestedObject(
		ctx,
		execDbClient,
		dataModel.Tables[input.ObjectType],
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
					SourceType:         input.SourceDetails.SourceDetailType(),
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
