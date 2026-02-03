package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
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
