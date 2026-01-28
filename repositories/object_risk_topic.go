package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

func (repo *MarbleDbRepository) GetObjectRiskTopicById(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
) (models.ObjectRiskTopic, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ObjectRiskTopic{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectObjectRiskTopicColumn...).
		From(dbmodels.TABLE_OBJECT_RISK_TOPICS).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptObjectRiskTopic)
}

func (repo *MarbleDbRepository) GetObjectRiskTopicByObjectId(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	objectType string,
	objectId string,
) (models.ObjectRiskTopic, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ObjectRiskTopic{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectObjectRiskTopicColumn...).
		From(dbmodels.TABLE_OBJECT_RISK_TOPICS).
		Where(squirrel.Eq{
			"org_id":      orgId,
			"object_type": objectType,
			"object_id":   objectId,
		})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptObjectRiskTopic)
}

func (repo *MarbleDbRepository) ListObjectRiskTopics(
	ctx context.Context,
	exec Executor,
	filter models.ObjectRiskTopicFilter,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ObjectRiskTopic, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if paginationAndSorting.Sorting == models.SortingFieldUnknown {
		return nil, errors.Wrapf(models.BadParameterError, "invalid sorting field: %s", paginationAndSorting.Sorting)
	}

	orderCond := fmt.Sprintf(
		"ort.%s %s, ort.id %s",
		paginationAndSorting.Sorting,
		paginationAndSorting.Order,
		paginationAndSorting.Order,
	)

	var offset models.ObjectRiskTopic
	if paginationAndSorting.OffsetId != "" {
		var err error
		offsetId, err := uuid.Parse(paginationAndSorting.OffsetId)
		if err != nil {
			return nil, errors.Wrap(models.BadParameterError, "invalid offsetId format")
		}
		offset, err = repo.GetObjectRiskTopicById(ctx, exec, offsetId)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.Wrap(err, "No row found matching the provided offsetId")
		} else if err != nil {
			return nil, errors.Wrap(err, "failed to fetch object risk topic corresponding to the provided offsetId")
		}
	}

	query := NewQueryBuilder().
		Select(columnsNames("ort", dbmodels.SelectObjectRiskTopicColumn)...).
		From(dbmodels.TABLE_OBJECT_RISK_TOPICS + " ort").
		Where(squirrel.Eq{"ort.org_id": filter.OrgId})

	if filter.ObjectType != nil {
		query = query.Where(squirrel.Eq{"ort.object_type": *filter.ObjectType})
	}
	if filter.ObjectId != nil {
		query = query.Where(squirrel.Eq{"ort.object_id": *filter.ObjectId})
	}
	if len(filter.Topics) > 0 {
		query = query.Where(squirrel.Expr("ort.topics && ?", filter.Topics))
	}

	query = query.OrderBy(orderCond).
		Limit(uint64(paginationAndSorting.Limit))

	query, err := applyObjectRiskTopicPaginationFilters(query, paginationAndSorting, offset)
	if err != nil {
		return nil, err
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptObjectRiskTopic)
}

func (repo *MarbleDbRepository) UpsertObjectRiskTopic(
	ctx context.Context,
	exec Executor,
	input models.ObjectRiskTopicCreate,
) (models.ObjectRiskTopic, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ObjectRiskTopic{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_OBJECT_RISK_TOPICS).
		Columns(
			"org_id",
			"object_type",
			"object_id",
			"topics",
		).
		Values(
			input.OrgId,
			input.ObjectType,
			input.ObjectId,
			input.Topics,
		).
		Suffix(`
			ON CONFLICT (org_id, object_type, object_id)
			DO UPDATE SET
				topics = EXCLUDED.topics,
				updated_at = NOW()
			RETURNING *`,
		)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptObjectRiskTopic)
}

func (repo *MarbleDbRepository) InsertObjectRiskTopicEvent(
	ctx context.Context,
	exec Executor,
	event models.ObjectRiskTopicEventCreate,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	var sourceDetailsJSON []byte
	if event.SourceDetails != nil {
		var err error
		sourceDetailsJSON, err = event.SourceDetails.ToJSON()
		if err != nil {
			return errors.Wrap(err, "failed to serialize source details")
		}
	}

	query := NewQueryBuilder().
		Insert("object_risk_topic_events").
		Columns(
			"id",
			"org_id",
			"object_risk_topics_id",
			"topics",
			"source_type",
			"source_details",
			"user_id",
			"api_key_id",
		).
		Values(
			uuid.Must(uuid.NewV7()),
			event.OrgId,
			event.ObjectRiskTopicsId,
			event.Topics,
			event.SourceType.String(),
			sourceDetailsJSON,
			event.UserId,
			event.ApiKeyId,
		)

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = exec.Exec(ctx, sql, args...)
	return err
}

func (repo *MarbleDbRepository) ListObjectRiskTopicEvents(
	ctx context.Context,
	exec Executor,
	objectRiskTopicsId uuid.UUID,
) ([]models.ObjectRiskTopicEvent, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectObjectRiskTopicEventColumn...).
		From(dbmodels.TABLE_OBJECT_RISK_TOPIC_EVENTS).
		Where(squirrel.Eq{"object_risk_topics_id": objectRiskTopicsId}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptObjectRiskTopicEvent)
}

func applyObjectRiskTopicPaginationFilters(
	query squirrel.SelectBuilder,
	p models.PaginationAndSorting,
	offset models.ObjectRiskTopic,
) (squirrel.SelectBuilder, error) {
	if p.OffsetId == "" {
		return query, nil
	}

	var offsetValue any
	switch p.Sorting {
	case models.SortingFieldCreatedAt:
		offsetValue = offset.CreatedAt
	case models.SortingFieldUpdatedAt:
		offsetValue = offset.UpdatedAt
	default:
		// only ordering and pagination by created_at is allowed for now
		return query, fmt.Errorf("invalid sorting field: %w", models.BadParameterError)
	}

	args := []any{offsetValue, p.OffsetId}
	if p.Order == models.SortingOrderDesc {
		query = query.Where(fmt.Sprintf("(ort.%s, ort.id) < (?, ?)", p.Sorting), args...)
	} else {
		query = query.Where(fmt.Sprintf("(ort.%s, ort.id) > (?, ?)", p.Sorting), args...)
	}

	return query, nil
}
