package repositories

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"
	"marble/marble-backend/utils"

	"github.com/Masterminds/squirrel"
)

type ScenarioIterationRuleRepositoryLegacy interface {
	CreateRule(ctx context.Context, organizationId string, rule models.CreateRuleInput) (models.Rule, error)
}

type RuleRepository interface {
	GetRuleById(tx Transaction, ruleId string) (models.Rule, error)
	ListRulesByIterationId(tx Transaction, iterationId string) ([]models.Rule, error)
	UpdateRule(tx Transaction, rule models.UpdateRuleInput) error
	DeleteRule(tx Transaction, ruleID string) error
}

type RuleRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func selectRules() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectRulesColumn...).
		From(dbmodels.TABLE_RULES)
}

func (repo *RuleRepositoryPostgresql) GetRuleById(tx Transaction, ruleId string) (models.Rule, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModelAdapterWithErr(
		pgTx,
		selectRules().Where(squirrel.Eq{"id": ruleId}),
		dbmodels.AdaptRule,
	)
}

func (repo *RuleRepositoryPostgresql) ListRulesByIterationId(tx Transaction, iterationId string) ([]models.Rule, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModelsAdapterWithErr(
		pgTx,
		selectRules().Where(squirrel.Eq{"scenarioIterationId": iterationId}),
		dbmodels.AdaptRule,
	)
}

func (repo *RuleRepositoryPostgresql) UpdateRule(tx Transaction, rule models.UpdateRuleInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	dbUpdateRuleInput, err := dbmodels.AdaptDBUpdateRuleInput(rule)
	if err != nil {
		return err
	}

	var updateRequest = NewQueryBuilder().
		Update(dbmodels.TABLE_RULES).
		SetMap(utils.ColumnValueMap(dbUpdateRuleInput)).
		Where("id = ?", rule.Id)

	_, err = pgTx.ExecBuilder(updateRequest)
	return err
}

func (repo *RuleRepositoryPostgresql) DeleteRule(tx Transaction, ruleID string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(NewQueryBuilder().Delete(dbmodels.TABLE_RULES).Where("id = ?", ruleID))
	return err
}
