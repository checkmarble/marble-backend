package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func selectTransferMappings() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectTransferMappingsColumn...).
		From(dbmodels.TABLE_TRANSFER_MAPPINGS)
}

func (repo *MarbleDbRepository) GetTransferMapping(ctx context.Context, exec Executor, id string) (models.TransferMapping, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.TransferMapping{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectTransferMappings().Where(squirrel.Eq{"id": id}),
		dbmodels.AdaptTransferMapping,
	)
}

func (repo *MarbleDbRepository) ListTransferMappings(
	ctx context.Context,
	exec Executor,
	organizationId string,
	partnerId string,
	transferId string,
) ([]models.TransferMapping, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		selectTransferMappings().
			Where(squirrel.Eq{"organization_id": organizationId}).
			Where(squirrel.Eq{"partner_id": partnerId}).
			Where(squirrel.Eq{"client_transfer_id": transferId}),
		dbmodels.AdaptTransferMapping,
	)
}

func (repo *MarbleDbRepository) CreateTransferMapping(
	ctx context.Context,
	exec Executor,
	id string,
	transferMapping models.TransferMappingCreateInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_TRANSFER_MAPPINGS).
			Columns(
				"id",
				"client_transfer_id",
				"organization_id",
				"partner_id",
			).
			Values(
				id,
				transferMapping.ClientTransferId,
				transferMapping.OrganizationId,
				transferMapping.PartnerId,
			),
	)
	return err
}

func (repo *MarbleDbRepository) DeleteTransferMapping(ctx context.Context, exec Executor, id string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Delete(dbmodels.TABLE_TRANSFER_MAPPINGS).
			Where(squirrel.Eq{"id": id}),
	)
	return err
}
