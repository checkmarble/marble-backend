package repositories

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

func (repo *MarbleDbRepository) GetObjectMetadataById(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
) (models.ObjectMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ObjectMetadata{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectObjectMetadataColumn...).
		From(dbmodels.TABLE_OBJECT_METADATA).
		Where(squirrel.Eq{
			"id":            id,
			"metadata_type": models.MetadataTypeRiskTopics.String(),
		})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptObjectMetadata)
}

func (repo *MarbleDbRepository) ListObjectMetadata(
	ctx context.Context,
	exec Executor,
	filter models.ObjectMetadataFilter,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ObjectMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if paginationAndSorting.Sorting == models.SortingFieldUnknown {
		return nil, errors.Wrapf(models.BadParameterError, "invalid sorting field: %s", paginationAndSorting.Sorting)
	}

	orderCond := fmt.Sprintf(
		"om.%s %s, om.id %s",
		paginationAndSorting.Sorting,
		paginationAndSorting.Order,
		paginationAndSorting.Order,
	)

	var offset models.ObjectMetadata
	if paginationAndSorting.OffsetId != "" {
		var err error
		offsetId, err := uuid.Parse(paginationAndSorting.OffsetId)
		if err != nil {
			return nil, errors.Wrap(models.BadParameterError, "invalid offsetId format")
		}
		offset, err = repo.GetObjectMetadataById(ctx, exec, offsetId)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.Wrap(models.NotFoundError,
				"No row found matching the provided offsetId")
		} else if err != nil {
			return nil, errors.Wrap(err, "failed to fetch object metadata corresponding to the provided offsetId")
		}
	}

	query := NewQueryBuilder().
		Select(columnsNames("om", dbmodels.SelectObjectMetadataColumn)...).
		From(dbmodels.TABLE_OBJECT_METADATA + " om").
		Where(squirrel.Eq{"om.org_id": filter.OrgId})

	if filter.ObjectType != nil {
		query = query.Where(squirrel.Eq{"om.object_type": *filter.ObjectType})
	}
	if len(filter.ObjectIds) > 0 {
		query = query.Where(squirrel.Eq{"om.object_id": filter.ObjectIds})
	}
	if len(filter.MetadataTypes) > 0 {
		metadataTypeStrs := make([]string, 0, len(filter.MetadataTypes))
		for _, mt := range filter.MetadataTypes {
			metadataTypeStrs = append(metadataTypeStrs, mt.String())
		}
		query = query.Where(squirrel.Eq{"om.metadata_type": metadataTypeStrs})
	}

	query = query.OrderBy(orderCond).
		Limit(uint64(paginationAndSorting.Limit))

	query, err := applyObjectMetadataPaginationFilters(query, paginationAndSorting, offset)
	if err != nil {
		return nil, err
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptObjectMetadata)
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
		Select(dbmodels.SelectObjectMetadataColumn...).
		From(dbmodels.TABLE_OBJECT_METADATA).
		Where(squirrel.Eq{
			"org_id":        orgId,
			"object_type":   objectType,
			"object_id":     objectId,
			"metadata_type": models.MetadataTypeRiskTopics.String(),
		})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptObjectRiskTopic)
}

func (repo *MarbleDbRepository) UpsertObjectRiskTopic(
	ctx context.Context,
	exec Executor,
	input models.ObjectRiskTopicUpsert,
) (models.ObjectRiskTopic, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ObjectRiskTopic{}, err
	}

	// Build JSONB metadata
	var sourceDetailsJSON json.RawMessage
	if input.SourceDetails != nil {
		var err error
		sourceDetailsJSON, err = input.SourceDetails.ToJSON()
		if err != nil {
			return models.ObjectRiskTopic{}, errors.Wrap(err, "failed to serialize source details")
		}
	}

	topicStrings := make([]string, 0, len(input.Topics))
	for _, t := range input.Topics {
		topicStrings = append(topicStrings, t.String())
	}

	metadata := dbmodels.DBRiskTopicsMetadata{
		Topics:        topicStrings,
		SourceType:    input.SourceType.String(),
		SourceDetails: sourceDetailsJSON,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return models.ObjectRiskTopic{}, errors.Wrap(err, "failed to serialize metadata")
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_OBJECT_METADATA).
		Columns(
			"org_id",
			"object_type",
			"object_id",
			"metadata_type",
			"metadata",
		).
		Values(
			input.OrgId,
			input.ObjectType,
			input.ObjectId,
			models.MetadataTypeRiskTopics.String(),
			metadataJSON,
		).
		Suffix(`
			ON CONFLICT (org_id, object_type, object_id, metadata_type)
			DO UPDATE SET
				metadata = EXCLUDED.metadata,
				updated_at = NOW()
			RETURNING *`,
		)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptObjectRiskTopic)
}

func applyObjectMetadataPaginationFilters(
	query squirrel.SelectBuilder,
	p models.PaginationAndSorting,
	offset models.ObjectMetadata,
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
		// only ordering and pagination by created_at and updated_at is allowed for now
		return query, fmt.Errorf("invalid sorting field: %w", models.BadParameterError)
	}

	args := []any{offsetValue, p.OffsetId}
	if p.Order == models.SortingOrderDesc {
		query = query.Where(fmt.Sprintf("(om.%s, om.id) < (?, ?)", p.Sorting), args...)
	} else {
		query = query.Where(fmt.Sprintf("(om.%s, om.id) > (?, ?)", p.Sorting), args...)
	}

	return query, nil
}
