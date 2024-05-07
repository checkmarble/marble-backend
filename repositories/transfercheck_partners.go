package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func selectPartners() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.PartnerFields...).
		From(dbmodels.TABLE_PARTNERS)
}

func (repo MarbleDbRepository) ListPartners(ctx context.Context, exec Executor) ([]models.Partner, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		selectPartners(),
		dbmodels.AdaptPartner,
	)
}

func (repo MarbleDbRepository) CreatePartner(
	ctx context.Context,
	exec Executor,
	partnerId string,
	partnerCreateInput models.PartnerCreateInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_PARTNERS).
			Columns(
				"id",
				"name",
			).
			Values(
				partnerId,
				partnerCreateInput.Name,
			),
	)
	return err
}

func (repo MarbleDbRepository) UpdatePartner(
	ctx context.Context,
	exec Executor,
	partnerId string,
	partnerUpdateInput models.PartnerUpdateInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_PARTNERS).
		Where(squirrel.Eq{"id": partnerId}).
		Set("name", partnerUpdateInput.Name)

	err := ExecBuilder(ctx, exec, query)
	return err
}

func (repo MarbleDbRepository) GetPartnerById(ctx context.Context, exec Executor, partnerId string) (models.Partner, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Partner{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectPartners().Where(squirrel.Eq{"id": partnerId}),
		dbmodels.AdaptPartner,
	)
}
