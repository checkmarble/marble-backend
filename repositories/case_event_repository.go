package repositories

import (
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListCaseEvents(tx Transaction, caseId string) ([]models.CaseEvent, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseEventColumn...).
		From(dbmodels.TABLE_CASE_EVENTS).
		Where(squirrel.Eq{"case_id": caseId}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(
		pgTx,
		query,
		dbmodels.AdaptCaseEvent,
	)
}

func (repo *MarbleDbRepository) CreateCaseEvent(tx Transaction, createCaseEventAttributes models.CreateCaseEventAttributes) error {
	return repo.BatchCreateCaseEvents(tx, []models.CreateCaseEventAttributes{createCaseEventAttributes})
}

func (repo *MarbleDbRepository) BatchCreateCaseEvents(tx Transaction, createCaseEventAttributes []models.CreateCaseEventAttributes) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().Insert(dbmodels.TABLE_CASE_EVENTS).
		Columns(
			"case_id",
			"user_id",
			"event_type",
			"additional_note",
			"resource_id",
			"resource_type",
			"new_value",
			"previous_value",
		)

	for _, createCaseEventAttribute := range createCaseEventAttributes {
		query = query.Values(
			createCaseEventAttribute.CaseId,
			createCaseEventAttribute.UserId,
			createCaseEventAttribute.EventType,
			createCaseEventAttribute.AdditionalNote,
			createCaseEventAttribute.ResourceId,
			createCaseEventAttribute.ResourceType,
			createCaseEventAttribute.NewValue,
			createCaseEventAttribute.PreviousValue,
		)
	}

	_, err := pgTx.ExecBuilder(query)

	return err
}
