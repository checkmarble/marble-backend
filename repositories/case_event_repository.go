package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
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

func (repo *MarbleDbRepository) ListCaseEventsOfTypes(ctx context.Context, exec Executor,
	caseId string, types []models.CaseEventType, paging models.PaginationAndSorting,
) ([]models.CaseEvent, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseEventColumn...).
		From(dbmodels.TABLE_CASE_EVENTS).
		Where(squirrel.Eq{"case_id": caseId}).
		Where(squirrel.Eq{"event_type": types}).
		OrderBy(fmt.Sprintf("created_at %[1]s, id %[1]s", paging.Order)).
		Limit(uint64(paging.Limit))

	if paging.OffsetId != "" {
		offsetCaseEvent, err := repo.GetCaseEventById(ctx, exec, paging.OffsetId)
		if err != nil {
			return nil, err
		}

		if paging.Order == models.SortingOrderDesc {
			query = query.Where(fmt.Sprintf("(%s, id) < (?, ?)", paging.Sorting),
				offsetCaseEvent.CreatedAt, offsetCaseEvent.Id)
		} else {
			query = query.Where(fmt.Sprintf("(%s, id) > (?, ?)", paging.Sorting),
				offsetCaseEvent.CreatedAt, offsetCaseEvent.Id)
		}
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptCaseEvent)
}

func (repo *MarbleDbRepository) ListCaseCommentEvents(ctx context.Context, exec Executor,
	caseId string, paging models.PaginationAndSorting,
) ([]models.CaseCommentEvent, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(
			"ce.id",
			"ce.user_id",
			"ce.created_at",
			"ce.event_type",
			"ce.additional_note",
			"ea.payload as annotation_payload",
		).
		From("case_events ce").
		LeftJoin("entity_annotations ea ON ea.id = ce.resource_id").
		Where("ce.case_id = ?", caseId).
		Where(squirrel.Or{
			squirrel.Eq{"ce.event_type": string(models.CaseCommentAdded)},
			squirrel.And{
				squirrel.Eq{"ce.event_type": string(models.CaseEntityAnnotated)},
				squirrel.Eq{"ce.additional_note": models.EntityAnnotationComment.String()},
			},
		}).
		OrderBy(fmt.Sprintf("ce.created_at %[1]s, ce.id %[1]s", paging.Order)).
		Limit(uint64(paging.Limit))

	if paging.OffsetId != "" {
		offsetCaseEvent, err := repo.GetCaseEventById(ctx, exec, paging.OffsetId)
		if err != nil {
			return nil, err
		}

		if paging.Order == models.SortingOrderDesc {
			query = query.Where("(ce.created_at, ce.id) < (?, ?)",
				offsetCaseEvent.CreatedAt, offsetCaseEvent.Id)
		} else {
			query = query.Where("(ce.created_at, ce.id) > (?, ?)",
				offsetCaseEvent.CreatedAt, offsetCaseEvent.Id)
		}
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptCaseCommentEvent)
}

func (repo *MarbleDbRepository) CreateCaseEvent(ctx context.Context, exec Executor,
	createCaseEventAttributes models.CreateCaseEventAttributes,
) (models.CaseEvent, error) {
	events, err := repo.BatchCreateCaseEvents(ctx, exec, []models.CreateCaseEventAttributes{
		createCaseEventAttributes,
	})
	if err != nil {
		return models.CaseEvent{}, err
	}
	return events[0], nil
}

func (repo *MarbleDbRepository) BatchCreateCaseEvents(ctx context.Context, exec Executor,
	createCaseEventAttributes []models.CreateCaseEventAttributes,
) ([]models.CaseEvent, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().Insert(dbmodels.TABLE_CASE_EVENTS).
		Columns(
			"id",
			"org_id",
			"case_id",
			"user_id",
			"event_type",
			"additional_note",
			"resource_id",
			"resource_type",
			"new_value",
			"previous_value",
		).
		Suffix("returning *")

	for _, createCaseEventAttribute := range createCaseEventAttributes {
		var userId pgtype.Text
		if createCaseEventAttribute.UserId != nil &&
			len(*createCaseEventAttribute.UserId) > 0 {
			userId = pgtype.Text{String: *createCaseEventAttribute.UserId, Valid: true}
		} else {
			userId = pgtype.Text{Valid: false}
		}
		query = query.Values(
			uuid.Must(uuid.NewV7()).String(),
			createCaseEventAttribute.OrgId,
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

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptCaseEvent)
}
