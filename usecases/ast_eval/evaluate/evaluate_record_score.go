package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type evaluateRecordRiskLevelRulesetUsecase interface {
	GetRuleset(ctx context.Context, recordType string, status models.ScoreRulesetStatus, version int) (models.ScoringRuleset, error)
}

type evaluateRecordRiskLevelUsecase interface {
	InternalComputeScore(ctx context.Context, exec repositories.Executor, orgId uuid.UUID,
		ruleset models.ScoringRuleset,
		recordType, recordId string,
	) (*models.ScoringEvaluation, error)
}

type evaluateRecordRiskLevelRepository interface {
	ListPivots(
		ctx context.Context,
		exec repositories.Executor,
		organizationId uuid.UUID,
		tableId *string,
		useCache bool,
	) ([]models.PivotMetadata, error)
	GetActiveScore(
		ctx context.Context,
		exec repositories.Executor,
		record models.ScoringRecordRef,
	) (*models.ScoringScore, error)
	InsertScore(
		ctx context.Context,
		tx repositories.Transaction,
		req models.InsertScoreRequest,
	) (models.ScoringScore, error)
}

type RecordRiskLevel struct {
	orgId                 uuid.UUID
	executorFactory       executor_factory.ExecutorFactory
	transactionFactory    executor_factory.TransactionFactory
	scoringRulesetUsecase evaluateRecordRiskLevelRulesetUsecase
	scoringScoreUsecase   evaluateRecordRiskLevelUsecase
	repository            evaluateRecordRiskLevelRepository
	dataModel             models.DataModel
	clientObject          models.ClientObject
}

func NewRecordRiskLevelEvaluator(
	orgId uuid.UUID,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	scoringRulesetUsecase evaluateRecordRiskLevelRulesetUsecase,
	scoringScoreUsecase evaluateRecordRiskLevelUsecase,
	repository evaluateRecordRiskLevelRepository,
	dataModel models.DataModel,
	clientObject models.ClientObject,
) RecordRiskLevel {
	return RecordRiskLevel{
		orgId:                 orgId,
		executorFactory:       executorFactory,
		transactionFactory:    transactionFactory,
		scoringRulesetUsecase: scoringRulesetUsecase,
		scoringScoreUsecase:   scoringScoreUsecase,
		repository:            repository,
		dataModel:             dataModel,
		clientObject:          clientObject,
	}
}

func (rs RecordRiskLevel) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	tableName := rs.clientObject.TableName
	objectId := rs.clientObject.Data["object_id"].(string)

	exec := rs.executorFactory.NewExecutor()

	ruleset, err := rs.scoringRulesetUsecase.GetRuleset(ctx, tableName, models.ScoreRulesetCommitted, 0)
	if err != nil && !errors.Is(err, models.NotFoundError) {
		return MakeEvaluateError(err)
	}

	// If the trigger object does not have a scoring ruleset, change all parameters to refer to its pivot
	if errors.Is(err, models.NotFoundError) {
		ruleset, tableName, objectId, err = rs.getPivotRuleset(ctx, exec, tableName)
		if err != nil {
			if errors.Is(err, models.NotFoundError) {
				return 0, nil
			}

			return MakeEvaluateError(err)
		}
	}

	riskLevel, err := executor_factory.TransactionReturnValue(ctx, rs.transactionFactory, func(tx repositories.Transaction) (int, error) {
		score, err := rs.repository.GetActiveScore(ctx, tx, models.ScoringRecordRef{
			OrgId:      rs.orgId,
			RecordType: tableName,
			RecordId:   objectId,
		})
		if err != nil {
			return 0, err
		}

		// If the record does not have a score yet, we *MUST* compute it online.
		if score == nil {
			eval, err := rs.scoringScoreUsecase.InternalComputeScore(ctx, tx, rs.orgId, ruleset, tableName, objectId)
			if err != nil {
				// If the record does not exist, it has no score and a score cannot be computed, return 0.
				if errors.Is(err, models.NotFoundError) || eval == nil {
					return 0, nil
				}

				return 0, err
			}

			req := models.InsertScoreRequest{
				OrgId:      rs.orgId,
				RecordType: tableName,
				RecordId:   objectId,
				RiskLevel:  eval.RiskLevel,
				Source:     models.ScoreSourceRuleset,
				RulesetId:  &ruleset.Id,
			}

			score, err := rs.repository.InsertScore(ctx, tx, req)
			if err != nil {
				return 0, err
			}

			return score.RiskLevel, nil
		}

		return score.RiskLevel, nil
	})
	if err != nil {
		return MakeEvaluateError(err)
	}

	return riskLevel, nil
}

func (rs RecordRiskLevel) getPivotRuleset(ctx context.Context, exec repositories.Executor, tableName string) (models.ScoringRuleset, string, string, error) {
	table, ok := rs.dataModel.Tables[tableName]
	if !ok {
		return models.ScoringRuleset{}, "", "", models.NotFoundError
	}

	pivots, err := rs.repository.ListPivots(ctx, exec, rs.orgId, &table.ID, false)
	if err != nil {
		return models.ScoringRuleset{}, "", "", err
	}

	if len(pivots) == 0 {
		return models.ScoringRuleset{}, "", "", models.NotFoundError
	}

	pivot := pivots[0].Enrich(rs.dataModel)

	if len(pivot.PathLinks) == 0 {
		return models.ScoringRuleset{}, "", "", models.NotFoundError
	}
	link, ok := table.LinksToSingle[pivot.PathLinks[0]]
	if !ok {
		return models.ScoringRuleset{}, "", "", models.NotFoundError
	}

	pivotValue, pivotValueOk := rs.clientObject.Data[link.ChildFieldName].(string)
	if !pivotValueOk {
		return models.ScoringRuleset{}, "", "", models.NotFoundError
	}

	tableName = pivot.PivotTable

	ruleset, err := rs.scoringRulesetUsecase.GetRuleset(ctx, tableName, models.ScoreRulesetCommitted, 0)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return models.ScoringRuleset{}, "", "", models.NotFoundError
		}

		return models.ScoringRuleset{}, "", "", err
	}

	return ruleset, tableName, pivotValue, nil
}
