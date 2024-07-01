package repositories

import (
	"context"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func selectPartners() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.PartnerFields...).
		From(dbmodels.TABLE_PARTNERS)
}

func (repo MarbleDbRepository) ListPartners(ctx context.Context, exec Executor, filters models.PartnerFilters) ([]models.Partner, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectPartners().OrderBy("created_at DESC")
	if filters.Bic.Valid {
		query = query.Where(squirrel.Eq{"UPPER(bic)": strings.ToUpper(filters.Bic.String)})
	}

	return SqlToListOfModels(
		ctx,
		exec,
		query,
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
				"bic",
			).
			Values(
				partnerId,
				partnerCreateInput.Name,
				partnerCreateInput.Bic,
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
		Where(squirrel.Eq{"id": partnerId})

	if partnerUpdateInput.Name.Valid {
		query = query.Set("name", partnerUpdateInput.Name)
	}
	if partnerUpdateInput.Bic.Valid {
		query = query.Set("bic", partnerUpdateInput.Bic)
	}

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
