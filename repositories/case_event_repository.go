package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/jackc/pgx/v5/pgtype"
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

func (repo *MarbleDbRepository) GetCaseEventById(ctx context.Context, exec Executor, id string) (models.CaseEvent, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.CaseEvent{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseEventColumn...).
		From(dbmodels.TABLE_CASE_EVENTS).
		Where("id = ?", id)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptCaseEvent)
}

func (repo *MarbleDbRepository) ListCaseEventsOfTypes(ctx context.Context, exec Executor, caseId string, types []models.CaseEventType, paging models.PaginationAndSorting) ([]models.CaseEvent, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseEventColumn...).
		From(dbmodels.TABLE_CASE_EVENTS).
		Where(squirrel.Eq{"case_id": caseId}).
		Where(squirrel.Eq{"event_type": types}).
		OrderBy(fmt.Sprintf("created_at %[1]s, id %[1]s", paging.Order)).
		Limit(uint64(paging.Limit) + 1)

	if paging.OffsetId != "" {
		offsetCaseEvent, err := repo.GetCaseEventById(ctx, exec, paging.OffsetId)
		if err != nil {
			return nil, err
		}

		if paging.Order == models.SortingOrderDesc {
			query = query.Where(fmt.Sprintf("(%s, id) < (?, ?)", paging.Sorting), offsetCaseEvent.CreatedAt, offsetCaseEvent.Id)
		} else {
			query = query.Where(fmt.Sprintf("(%s, id) > (?, ?)", paging.Sorting), offsetCaseEvent.CreatedAt, offsetCaseEvent.Id)
		}
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptCaseEvent)
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
		var userId pgtype.Text
		if createCaseEventAttribute.UserId != nil &&
			len(*createCaseEventAttribute.UserId) > 0 {
			userId = pgtype.Text{String: *createCaseEventAttribute.UserId, Valid: true}
		} else {
			userId = pgtype.Text{Valid: false}
		}
		query = query.Values(
			createCaseEventAttribute.CaseId,
			userId,
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
