package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListCaseEvents(ctx context.Context, exec Executor, caseId string) ([]models.CaseEvent, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseEventColumn...).
		From(dbmodels.TABLE_CASE_EVENTS).
		Where(squirrel.Eq{"case_id": caseId}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptCaseEvent,
	)
}

func (repo *MarbleDbRepository) CreateCaseEvent(ctx context.Context, exec Executor,
	createCaseEventAttributes models.CreateCaseEventAttributes,
) error {
	return repo.BatchCreateCaseEvents(ctx, exec, []models.CreateCaseEventAttributes{createCaseEventAttributes})
}

func (repo *MarbleDbRepository) BatchCreateCaseEvents(ctx context.Context, exec Executor,
	createCaseEventAttributes []models.CreateCaseEventAttributes,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

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

	err := ExecBuilder(ctx, exec, query)

	return err
}
