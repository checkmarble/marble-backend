package repositories

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/Masterminds/squirrel"
)

type DecisionRepository interface {
	DecisionById(transaction Transaction, decisionId string) (models.Decision, error)
	DecisionsOfScheduledExecution(scheduledExecutionId string) (<-chan models.Decision, <-chan error)
	StoreDecision(tx Transaction, decision models.Decision, organizationId string, newDecisionId string) error
	DecisionsOfOrganization(transaction Transaction, organizationId string, limit int) ([]models.Decision, error)
}

type DecisionRepositoryImpl struct {
	transactionFactory TransactionFactoryPosgresql
}

// the size of the batch is chosen without any benchmark
const decisionRulesBatchSize = 1000

func (repo *DecisionRepositoryImpl) DecisionById(transaction Transaction, decisionId string) (models.Decision, error) {
	tx := repo.transactionFactory.adaptMarbleDatabaseTransaction(transaction)

	rules, err := repo.rulesOfDecision(tx, decisionId)
	if err != nil {
		return models.Decision{}, err
	}

	decision, err := SqlToModel(tx,
		selectDecisions().
			Where(squirrel.Eq{"id": decisionId}),
		func(dbDecision dbmodels.DbDecision) models.Decision {
			return dbmodels.AdaptDecision(dbDecision, rules)
		},
	)

	if err != nil {
		return models.Decision{}, err
	}

	return decision, err
}

func (repo *DecisionRepositoryImpl) DecisionsOfOrganization(transaction Transaction, organizationId string, limit int) ([]models.Decision, error) {
	tx := repo.transactionFactory.adaptMarbleDatabaseTransaction(transaction)

	decisionsChan, errChan := repo.channelOfDecisions(tx,
		selectDecisions().
			Where(squirrel.Eq{"org_id": organizationId}).
			OrderBy("created_at DESC").
			Limit(uint64(limit)),
	)

	decisions := ChanToSlice(decisionsChan)
	err := <-errChan

	return decisions, err
}

func (repo *DecisionRepositoryImpl) DecisionsOfScheduledExecution(scheduledExecutionId string) (<-chan models.Decision, <-chan error) {

	tx := repo.transactionFactory.adaptMarbleDatabaseTransaction(nil)

	return repo.channelOfDecisions(
		tx,
		selectDecisions().
			Where(squirrel.Eq{"scheduled_execution_id": scheduledExecutionId}).
			OrderBy("created_at DESC"),
	)
}

func (repo *DecisionRepositoryImpl) StoreDecision(tx Transaction, decision models.Decision, organizationId string, newDecisionId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_DECISIONS).
			Columns(
				"id",
				"org_id",
				"outcome",
				"scenario_id",
				"scenario_name",
				"scenario_description",
				"scenario_version",
				"score",
				"error_code",
				"trigger_object",
				"trigger_object_type",
				"scheduled_execution_id",
			).
			Values(
				newDecisionId,
				organizationId,
				decision.Outcome.String(),
				decision.ScenarioId,
				decision.ScenarioName,
				decision.ScenarioDescription,
				decision.ScenarioVersion,
				decision.Score,
				decision.DecisionError,
				decision.ClientObject.Data,
				decision.ClientObject.TableName,
				decision.ScheduledExecutionId,
			),
	)
	if err != nil {
		return err
	}

	builderForRules := NewQueryBuilder().
		Insert(dbmodels.TABLE_DECISION_RULES).
		Columns(
			"id",
			"org_id",
			"decision_id",
			"name",
			"description",
			"score_modifier",
			"result",
			"error_code",
		)

	for _, ruleExecution := range decision.RuleExecutions {
		builderForRules = builderForRules.
			Values(
				utils.NewPrimaryKey(organizationId),
				organizationId,
				newDecisionId,
				ruleExecution.Rule.Name,
				ruleExecution.Rule.Description,
				ruleExecution.ResultScoreModifier,
				ruleExecution.Result,
				models.AdaptRuleExecutionError(ruleExecution.Error),
			)
	}
	_, err = pgTx.ExecBuilder(builderForRules)
	return err
}

func (repo *DecisionRepositoryImpl) rulesOfDecision(transaction Transaction, decisionId string) ([]models.RuleExecution, error) {
	tx := repo.transactionFactory.adaptMarbleDatabaseTransaction(transaction)

	return SqlToListOfModels(
		tx,
		NewQueryBuilder().Select(dbmodels.SelectDecisionRuleColumn...).
			From(dbmodels.TABLE_DECISION_RULE).
			Where(squirrel.Eq{"decision_id": decisionId}).
			OrderBy("id"),
		dbmodels.AdaptRuleExecution,
	)
}

type RulesOfDecision struct {
	rules []models.RuleExecution
}

// Return an array of RulesOfDecision that correspond to the decisionIds
func (repo *DecisionRepositoryImpl) rulesOfDecisionsBatch(transaction Transaction, decisionIds []string) ([]RulesOfDecision, error) {
	tx := repo.transactionFactory.adaptMarbleDatabaseTransaction(transaction)

	allRules, err := SqlToListOfModels(
		tx,
		NewQueryBuilder().Select(dbmodels.SelectDecisionRuleColumn...).
			From(dbmodels.TABLE_DECISION_RULE).
			Where(squirrel.Eq{"decision_id": decisionIds}).
			OrderBy("decision_id"),
		func(r dbmodels.DbDecisionRule) dbmodels.DbDecisionRule { return r },
	)
	if err != nil {
		return nil, err
	}

	decisionsRulesMap := make(map[string]*RulesOfDecision, len(decisionIds))
	for _, decisionId := range decisionIds {
		decisionsRulesMap[decisionId] = &RulesOfDecision{}
	}

	// dispatch rules to their corresponding decision
	for _, dbRule := range allRules {
		rulesOfDecision := decisionsRulesMap[dbRule.DecisionId]
		rulesOfDecision.rules = append(rulesOfDecision.rules, dbmodels.AdaptRuleExecution(dbRule))
	}

	// return an array of RulesOfDecision that match the input array decisionIds
	return utils.Map(decisionIds, func(decisionId string) RulesOfDecision {
		return *decisionsRulesMap[decisionId]
	}), nil
}

func (repo *DecisionRepositoryImpl) channelOfDecisions(tx TransactionPostgres, query squirrel.Sqlizer) (<-chan models.Decision, <-chan error) {

	decisionsChannel := make(chan models.Decision, 100)
	errChannel := make(chan error, 1)

	go func() {
		defer close(decisionsChannel)
		defer close(errChannel)

		dbDecisionsChannel, dbErrChannel := SqlToChannelOfDbModel[dbmodels.DbDecision](tx, query)

		var allErrors []error

		// Let's keep the non optimized version
		// for dbDecision := range dbDecisionsChannel {
		// 	rules, err := repo.rulesOfDecision(tx, dbDecision.Id)
		// 	if err != nil {
		// 		allErrors = append(allErrors, err)
		// 		// do not send invalid decisions
		// 		continue
		// 	}
		// 	decisionsChannel <- dbmodels.AdaptDecision(dbDecision, rules)
		// }

		for dbDecisions := range BatchChannel(dbDecisionsChannel, decisionRulesBatchSize) {

			// fetch rules of all decisions
			rules, err := repo.rulesOfDecisionsBatch(
				nil,
				utils.Map(dbDecisions, func(d dbmodels.DbDecision) string { return d.Id }),
			)

			if err != nil {
				allErrors = append(allErrors, err)
				// do not send invalid decisions
				continue
			}

			for i := 0; i < len(dbDecisions); i++ {
				decisionsChannel <- dbmodels.AdaptDecision(dbDecisions[i], rules[i].rules)
			}
		}

		// wait for Db to finish
		allErrors = append(allErrors, <-dbErrChannel)

		errChannel <- errors.Join(allErrors...)
	}()

	return decisionsChannel, errChannel
}

func selectDecisions() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectDecisionColumn...).
		From(dbmodels.TABLE_DECISIONS)
}

func BatchChannel[Value any](inChannel <-chan Value, batchSize int) <-chan []Value {

	out := make(chan []Value, batchSize)

	go func() {
		defer close(out)

		buf := make([]Value, 0, batchSize)

		flush := func() {
			if len(buf) > 0 {
				out <- buf
				buf = make([]Value, 0, batchSize)
			}
		}

		for v := range inChannel {
			buf = append(buf, v)
			if len(buf) == batchSize {
				flush()
			}
		}

		flush()
	}()

	return out
}

func ChanToSlice[Model any](channel <-chan Model) []Model {
	var slice []Model
	for item := range channel {
		slice = append(slice, item)
	}
	return slice
}
