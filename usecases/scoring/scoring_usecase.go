package scoring

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type scoringRepository interface {
	GetScoreHistory(ctx context.Context, exec repositories.Executor, entityRef models.ScoringEntityRef) ([]models.ScoringScore, error)
	GetActiveScore(ctx context.Context, exec repositories.Executor, entityRef models.ScoringEntityRef) (*models.ScoringScore, error)
	InsertScore(ctx context.Context, tx repositories.Transaction, req models.InsertScoreRequest) (models.ScoringScore, error)
}

type scoringIngestedDataReader interface {
	QueryIngestedObject(ctx context.Context, exec repositories.Executor,
		table models.Table, objectId string, metadataFields ...string) ([]models.DataModelObject, error)
}

type ScoringUsecase struct {
	enforceSecurity     security.EnforceSecurityScoring
	executorFactory     executor_factory.ExecutorFactory
	transactionFactory  executor_factory.TransactionFactory
	repository          scoringRepository
	dataModelRepository repositories.DataModelRepository
	ingestedDataReader  scoringIngestedDataReader
}

func NewScoringUsecase(
	enforceSecurity security.EnforceSecurityScoring,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repository scoringRepository,
	dataModelRepository repositories.DataModelRepository,
	ingestedDataReader scoringIngestedDataReader,
) ScoringUsecase {
	return ScoringUsecase{
		enforceSecurity:     enforceSecurity,
		executorFactory:     executorFactory,
		transactionFactory:  transactionFactory,
		repository:          repository,
		dataModelRepository: dataModelRepository,
		ingestedDataReader:  ingestedDataReader,
	}
}

func (uc ScoringUsecase) GetScoreHistory(ctx context.Context, entityRef models.ScoringEntityRef) ([]models.ScoringScore, error) {
	entityRef.OrgId = uc.enforceSecurity.OrgId()

	scores, err := uc.repository.GetScoreHistory(ctx, uc.executorFactory.NewExecutor(), entityRef)
	if err != nil {
		return nil, err
	}

	for _, score := range scores {
		if err := uc.enforceSecurity.ReadEntityScore(score); err != nil {
			return nil, err
		}
	}

	return scores, nil
}

func (uc ScoringUsecase) GetActiveScore(ctx context.Context, entityRef models.ScoringEntityRef) (*models.ScoringScore, error) {
	entityRef.OrgId = uc.enforceSecurity.OrgId()

	score, err := uc.repository.GetActiveScore(ctx, uc.executorFactory.NewExecutor(), entityRef)
	if err != nil || score == nil {
		return nil, err
	}

	if err := uc.enforceSecurity.ReadEntityScore(*score); err != nil {
		return nil, err
	}

	return score, nil
}

func (uc ScoringUsecase) OverrideScore(ctx context.Context, req models.InsertScoreRequest) (models.ScoringScore, error) {
	exec := uc.executorFactory.NewExecutor()

	req.OrgId = uc.enforceSecurity.OrgId()

	if req.Source == models.ScoreSourceOverride {
		switch {
		case uc.enforceSecurity.UserId() != nil:
			req.OverridenBy = utils.Ptr(uuid.MustParse(string(*uc.enforceSecurity.UserId())))
		case uc.enforceSecurity.ApiKeyId() != nil:
			req.OverridenBy = utils.Ptr(uuid.MustParse(string(*uc.enforceSecurity.ApiKeyId())))
		}

		dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, req.OrgId, false, false)
		if err != nil {
			return models.ScoringScore{}, err
		}
		table, ok := dataModel.Tables[req.EntityType]
		if !ok {
			return models.ScoringScore{}, errors.Newf("unknown entity type '%s'", req.EntityType)
		}

		clientExec, err := uc.executorFactory.NewClientDbExecutor(ctx, req.OrgId)
		if err != nil {
			return models.ScoringScore{}, err
		}

		objs, err := uc.ingestedDataReader.QueryIngestedObject(ctx, clientExec, table, req.EntityId)
		if err != nil {
			return models.ScoringScore{}, err
		}
		if len(objs) == 0 {
			return models.ScoringScore{}, errors.Wrap(models.NotFoundError,
				fmt.Sprintf("no ingested object %s/%s", req.EntityType, req.EntityId))
		}

		if err := uc.enforceSecurity.OverrideScore(req.ToEntityRef()); err != nil {
			return models.ScoringScore{}, err
		}
	}

	score, err := executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(tx repositories.Transaction) (models.ScoringScore, error) {
		return uc.repository.InsertScore(ctx, tx, req)
	})

	return score, err
}

func (uc ScoringUsecase) ComputeScore(ctx context.Context, req models.InsertScoreRequest) (models.ScoringScore, error) {
	return models.ScoringScore{}, errors.New("not yet implemented")
}
