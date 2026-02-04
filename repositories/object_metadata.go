package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetObjectMetadata(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	objectType string,
	objectId string,
	metadataType models.MetadataType,
) (models.ObjectMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ObjectMetadata{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectObjectMetadataColumn...).
		From(dbmodels.TABLE_OBJECT_METADATA).
		Where(squirrel.Eq{
			"org_id":        orgId,
			"object_type":   objectType,
			"object_id":     objectId,
			"metadata_type": metadataType.String(),
		})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptObjectMetadata)
}

func (repo *MarbleDbRepository) UpsertObjectMetadata(
	ctx context.Context,
	exec Executor,
	input models.ObjectMetadataUpsert,
) (models.ObjectMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ObjectMetadata{}, err
	}

	if input.Metadata == nil {
		return models.ObjectMetadata{}, errors.Wrap(models.BadParameterError, "metadata can not be nil")
	}

	metadataJSON, err := input.Metadata.ToJSON()
	if err != nil {
		return models.ObjectMetadata{}, err
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
			input.MetadataType.String(),
			metadataJSON,
		).
		Suffix(`
			ON CONFLICT (org_id, object_type, object_id, metadata_type)
			DO UPDATE SET
				metadata = EXCLUDED.metadata,
				updated_at = NOW()
			RETURNING *`,
		)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptObjectMetadata)
}

// Very specific method for Risk Topics metadata type. Need to find element in metadata array (Topics)
// Have a GIN index on it for performance (ref: idx_object_metadata_risk_topics_gin)
// Have an index on jsonb_array_length for faster filtering of non-empty topics (ref: idx_object_metadata_risk_topics_length)
func (repo *MarbleDbRepository) FindObjectRiskTopicsMetadata(
	ctx context.Context,
	exec Executor,
	filter models.ObjectRiskTopicsMetadataFilter,
) ([]models.ObjectMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if len(filter.ObjectIds) == 0 {
		return nil, errors.Wrap(models.BadParameterError, "object IDs filter can not be empty")
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectObjectMetadataColumn...).
		From(dbmodels.TABLE_OBJECT_METADATA).
		Where(squirrel.Eq{
			"org_id":        filter.OrgId,
			"object_type":   filter.ObjectType,
			"metadata_type": models.MetadataTypeRiskTopics.String(),
			"object_id":     filter.ObjectIds,
		})

	if len(filter.Topics) == 0 {
		query = query.Where("jsonb_array_length(metadata->'topics') > 0")
	} else {
		query = query.Where("metadata->'topics' ??| ?", filter.Topics)
	}
	query = query.Limit(1)

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptObjectMetadata)
}
