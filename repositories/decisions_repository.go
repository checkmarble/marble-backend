package repositories

import (
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
)

type DecisionRepository interface {
	DecisionById(transaction Transaction, decisionId string) (models.Decision, error)
	DecisionsById(transaction Transaction, decisionIds []string) ([]models.Decision, error)
	DecisionsByCaseId(transaction Transaction, caseId string) ([]models.Decision, error)
	DecisionsOfScheduledExecution(scheduledExecutionId string) (<-chan models.Decision, <-chan error)
	StoreDecision(tx Transaction, decision models.Decision, organizationId string, newDecisionId string) error
	DecisionsOfOrganization(transaction Transaction, organizationId string, paginationAndSorting models.PaginationAndSorting, filters models.DecisionFilters) ([]models.DecisionWithRank, error)
	UpdateDecisionCaseId(transaction Transaction, decisionsIds []string, caseId string) error
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

	return SqlToRow(tx,
		selectJoinDecisionAndCase().
			Where(squirrel.Eq{"d.id": decisionId}),
		func(row pgx.CollectableRow) (models.Decision, error) {
			db, err := pgx.RowToStructByPos[dbmodels.DbJoinDecisionAndCase](row)
			if err != nil {
				return models.Decision{}, err
			}

			var decisionCase *models.Case
			if db.DbDecision.CaseId != nil {
				decisionCaseValue, err := dbmodels.AdaptCase(db.DBCase)
				if err != nil {
					return models.Decision{}, err
				}
				decisionCase = &decisionCaseValue
			}
			return dbmodels.AdaptDecision(db.DbDecision, rules, decisionCase), nil
		},
	)
}

func (repo *DecisionRepositoryImpl) DecisionsById(transaction Transaction, decisionIds []string) ([]models.Decision, error) {
	tx := repo.transactionFactory.adaptMarbleDatabaseTransaction(transaction)

	query := selectJoinDecisionAndCase().Where(squirrel.Eq{"d.id": decisionIds})

	return SqlToListOfRow(tx, query, func(row pgx.CollectableRow) (models.Decision, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DbJoinDecisionAndCase](row)
		if err != nil {
			return models.Decision{}, err
		}

		var decisionCase *models.Case
		if db.DbDecision.CaseId != nil {
			decisionCaseValue, err := dbmodels.AdaptCase(db.DBCase)
			if err != nil {
				return models.Decision{}, err
			}
			decisionCase = &decisionCaseValue
		}
		return dbmodels.AdaptDecision(db.DbDecision, []models.RuleExecution{}, decisionCase), nil
	})
}

func (repo *DecisionRepositoryImpl) DecisionsByCaseId(transaction Transaction, caseId string) ([]models.Decision, error) {
	tx := repo.transactionFactory.adaptMarbleDatabaseTransaction(transaction)

	query := selectDecisions().
		Where(squirrel.Eq{"case_id": caseId}).
		OrderBy("created_at DESC")

	decisionsChan, errChan := repo.channelOfDecisions(tx, query)

	decisions := ChanToSlice(decisionsChan)
	err := <-errChan

	return decisions, err
}

func (repo *DecisionRepositoryImpl) DecisionsOfOrganization(
	transaction Transaction,
	organizationId string,
	pagination models.PaginationAndSorting,
	filters models.DecisionFilters,
) ([]models.DecisionWithRank, error) {
	tx := repo.transactionFactory.adaptMarbleDatabaseTransaction(transaction)

	subquery := selectDecisionsWithRank(pagination).
		Where(squirrel.Eq{"d.org_id": organizationId})
	subquery = applyDecisionFilters(subquery, filters)

	paginatedQuery := NewQueryBuilder().
		Select(decisionsWithRankColumns()...).
		FromSelect(subquery, "s").
		Limit(uint64(pagination.Limit))

	paginatedQuery, err := applyDecisionPagination(paginatedQuery, pagination)
	if err != nil {
		return []models.DecisionWithRank{}, err
	}
	query := selectDecisionsWithJoinedFields(paginatedQuery, pagination)

	return SqlToListOfRow(tx, query, func(row pgx.CollectableRow) (models.DecisionWithRank, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DBPaginatedDecisions](row)
		if err != nil {
			return models.DecisionWithRank{}, err
		}

		var decisionCase *models.Case
		if db.DbDecision.CaseId != nil {
			decisionCaseValue, err := dbmodels.AdaptCase(db.DBCase)
			if err != nil {
				return models.DecisionWithRank{}, err
			}
			decisionCase = &decisionCaseValue
		}
		return dbmodels.AdaptDecisionWithRank(db.DbDecision, decisionCase, db.RankNumber, db.Total), nil
	})
}

func applyDecisionFilters(query squirrel.SelectBuilder, filters models.DecisionFilters) squirrel.SelectBuilder {
	if len(filters.ScenarioIds) > 0 {
		query = query.Where(squirrel.Eq{"scenario_id": filters.ScenarioIds})
	}
	if len(filters.Outcomes) > 0 {
		query = query.Where(squirrel.Eq{"outcome": filters.Outcomes})
	}
	if len(filters.TriggerObjects) > 0 {
		query = query.Where(squirrel.Eq{"trigger_object_type": filters.TriggerObjects})
	}
	if !filters.StartDate.IsZero() {
		query = query.Where(squirrel.GtOrEq{"created_at": filters.StartDate})
	}
	if !filters.EndDate.IsZero() {
		query = query.Where(squirrel.LtOrEq{"created_at": filters.EndDate})
	}
	if filters.WithCase != nil && *filters.WithCase {
		query = query.Where(squirrel.NotEq{"case_id": nil})
	}
	if filters.WithCase != nil && !*filters.WithCase {
		query = query.Where(squirrel.Eq{"case_id": nil})
	}
	if len(filters.CaseIds) > 0 {
		query = query.Where(squirrel.Eq{"case_id": filters.CaseIds})
	}
	return query
}

func selectDecisionsWithRank(p models.PaginationAndSorting) squirrel.SelectBuilder {
	orderCondition := fmt.Sprintf("d.%s %s, d.id %s", p.Sorting, p.Order, p.Order)

	query := NewQueryBuilder().
		Select(dbmodels.SelectDecisionColumn...).
		Column(fmt.Sprintf("RANK() OVER (ORDER BY %s) as rank_number", orderCondition)).
		Column("COUNT(*) OVER() AS total").
		From(fmt.Sprintf("%s AS d", dbmodels.TABLE_DECISIONS))

	// When fetching the previous page, we want the "last xx decisions", so we need to reverse the order of the query,
	// select the xx items, then reverse again to put them back in the right order
	if p.OffsetId != "" && p.Previous {
		query = query.OrderBy(fmt.Sprintf("d.%s %s, d.id %s", p.Sorting, models.ReverseOrder(p.Order), models.ReverseOrder(p.Order)))
	} else {
		query = query.OrderBy(orderCondition)
	}

	return query
}

func decisionsWithRankColumns() (columns []string) {
	columns = append(columns, dbmodels.SelectDecisionColumn...)

	columns = columnsNames("s", columns)
	columns = append(columns, "rank_number", "total")
	return columns
}

func applyDecisionPagination(query squirrel.SelectBuilder, p models.PaginationAndSorting) (squirrel.SelectBuilder, error) {
	if p.OffsetId == "" {
		return query, nil
	}

	offsetSubquery, args, err := squirrel.
		Select("id", "org_id", string(p.Sorting)).
		From(dbmodels.TABLE_DECISIONS).
		Where(squirrel.Eq{"id": p.OffsetId}).
		ToSql()
	if err != nil {
		return query, err
	}
	query = query.Join(fmt.Sprintf("(%s) AS cursorRecord ON cursorRecord.org_id = s.org_id", offsetSubquery), args...)

	queryConditionBefore := fmt.Sprintf("s.%s < cursorRecord.%s OR (s.%s = cursorRecord.%s AND s.id < cursorRecord.id)", p.Sorting, p.Sorting, p.Sorting, p.Sorting)
	queryConditionAfter := fmt.Sprintf("s.%s > cursorRecord.%s OR (s.%s = cursorRecord.%s AND s.id > cursorRecord.id)", p.Sorting, p.Sorting, p.Sorting, p.Sorting)

	if p.Next {
		if p.Order == "DESC" {
			query = query.Where(queryConditionBefore)
		} else {
			query = query.Where(queryConditionAfter)
		}
	}
	if p.Previous {
		if p.Order == "DESC" {
			query = query.Where(queryConditionAfter)
		} else {
			query = query.Where(queryConditionBefore)
		}
	}

	return query, nil
}

func selectDecisionsWithJoinedFields(query squirrel.SelectBuilder, p models.PaginationAndSorting) squirrel.SelectBuilder {
	var columns []string
	columns = append(columns, columnsNames("d", dbmodels.SelectDecisionColumn)...)
	columns = append(columns, columnsNames("c", dbmodels.SelectCaseColumn)...)
	return squirrel.
		Select(columns...).
		Column("rank_number").
		Column("total").
		FromSelect(query, "d").
		LeftJoin(fmt.Sprintf("%s AS c ON c.id = d.case_id", dbmodels.TABLE_CASES)).
		OrderBy(fmt.Sprintf("d.%s %s, d.id %s", p.Sorting, p.Order, p.Order)).
		PlaceholderFormat(squirrel.Dollar)
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
				0, // TODO: cleanup, remove the field in db (it's not nullable now)
				decision.ClientObject.Data,
				decision.ClientObject.TableName,
				decision.ScheduledExecutionId,
			),
	)
	if err != nil {
		return err
	}

	if len(decision.RuleExecutions) == 0 {
		return nil
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

func (repo *DecisionRepositoryImpl) UpdateDecisionCaseId(transaction Transaction, decisionIds []string, caseId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(transaction)
	var query = NewQueryBuilder().
		Update(dbmodels.TABLE_DECISIONS).
		Set("case_id", caseId).
		Where(squirrel.Eq{"id": decisionIds})

	_, err := pgTx.ExecBuilder(query)
	return err
}

func selectJoinDecisionAndCase() squirrel.SelectBuilder {
	var columns []string
	columns = append(columns, columnsNames("d", dbmodels.SelectDecisionColumn)...)
	columns = append(columns, columnsNames("c", dbmodels.SelectCaseColumn)...)
	return NewQueryBuilder().
		Select(columns...).
		From(fmt.Sprintf("%s AS d", dbmodels.TABLE_DECISIONS)).
		LeftJoin(fmt.Sprintf("%s AS c ON c.id = d.case_id", dbmodels.TABLE_CASES)).
		OrderBy("d.created_at DESC")
}

func (repo *DecisionRepositoryImpl) rulesOfDecision(transaction Transaction, decisionId string) ([]models.RuleExecution, error) {
	tx := repo.transactionFactory.adaptMarbleDatabaseTransaction(transaction)

	return SqlToListOfModels(
		tx,
		NewQueryBuilder().Select(dbmodels.SelectDecisionRuleColumn...).
			From(dbmodels.TABLE_DECISION_RULE).
			Where(squirrel.Eq{"decision_id": decisionId}).
			OrderBy("id"),
		func(r dbmodels.DbDecisionRule) (models.RuleExecution, error) {
			return dbmodels.AdaptRuleExecution(r), nil
		},
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
		func(r dbmodels.DbDecisionRule) (dbmodels.DbDecisionRule, error) { return r, nil },
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

		dbDecisionsChannel, dbErrChannel := SqlToChannelOfModels(tx, query, func(row pgx.CollectableRow) (dbmodels.DbDecision, error) {
			return pgx.RowToStructByName[dbmodels.DbDecision](row)
		})

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
				decisionsChannel <- dbmodels.AdaptDecision(dbDecisions[i], rules[i].rules, nil)
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
	slice := make([]Model, 0)
	for item := range channel {
		slice = append(slice, item)
	}
	return slice
}
