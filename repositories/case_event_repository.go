package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListCaseEvents(ctx context.Context, tx Transaction_deprec, caseId string) ([]models.CaseEvent, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseEventColumn...).
		From(dbmodels.TABLE_CASE_EVENTS).
		Where(squirrel.Eq{"case_id": caseId}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(
		ctx,
		pgTx,
		query,
		dbmodels.AdaptCaseEvent,
	)
}

func (repo *MarbleDbRepository) CreateCaseEvent(ctx context.Context, tx Transaction_deprec, createCaseEventAttributes models.CreateCaseEventAttributes) error {
	return repo.BatchCreateCaseEvents(ctx, tx, []models.CreateCaseEventAttributes{createCaseEventAttributes})
}

func (repo *MarbleDbRepository) BatchCreateCaseEvents(ctx context.Context, tx Transaction_deprec, createCaseEventAttributes []models.CreateCaseEventAttributes) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

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

	_, err := pgTx.ExecBuilder(ctx, query)

	return err
}
