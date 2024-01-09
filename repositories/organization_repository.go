package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type OrganizationRepository interface {
	AllOrganizations(ctx context.Context, tx Transaction) ([]models.Organization, error)
	GetOrganizationById(ctx context.Context, tx Transaction, organizationId string) (models.Organization, error)
	CreateOrganization(ctx context.Context, tx Transaction, createOrganization models.CreateOrganizationInput, newOrganizationId string) error
	UpdateOrganization(ctx context.Context, tx Transaction, updateOrganization models.UpdateOrganizationInput) error
	DeleteOrganization(ctx context.Context, tx Transaction, organizationId string) error
}

type OrganizationRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql
}

func (repo *OrganizationRepositoryPostgresql) AllOrganizations(ctx context.Context, tx Transaction) ([]models.Organization, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToListOfModels(
		ctx,
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectOrganization...).
			From(dbmodels.TABLE_ORGANIZATION).
			OrderBy("id"),
		dbmodels.AdaptOrganization,
	)
}
func (repo *OrganizationRepositoryPostgresql) GetOrganizationById(ctx context.Context, tx Transaction, organizationId string) (models.Organization, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToModel(
		ctx,
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectOrganization...).
			From(dbmodels.TABLE_ORGANIZATION).
			Where("id = ?", organizationId),
		dbmodels.AdaptOrganization,
	)
}

func (repo *OrganizationRepositoryPostgresql) CreateOrganization(ctx context.Context, tx Transaction, createOrganization models.CreateOrganizationInput, newOrganizationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().Insert(dbmodels.TABLE_ORGANIZATION).
			Columns(
				"id",
				"name",
				"database_name",
			).
			Values(
				newOrganizationId,
				createOrganization.Name,
				createOrganization.DatabaseName,
			),
	)
	return err
}

func (repo *OrganizationRepositoryPostgresql) UpdateOrganization(ctx context.Context, tx Transaction, updateOrganization models.UpdateOrganizationInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	var updateRequest = NewQueryBuilder().Update(dbmodels.TABLE_ORGANIZATION)

	if updateOrganization.Name != nil {
		updateRequest = updateRequest.Set("name", *updateOrganization.Name)
	}
	if updateOrganization.DatabaseName != nil {
		updateRequest = updateRequest.Set("database_name", *updateOrganization.DatabaseName)
	}
	if updateOrganization.ExportScheduledExecutionS3 != nil {
		updateRequest = updateRequest.Set("export_scheduled_execution_s3", *updateOrganization.ExportScheduledExecutionS3)
	}

	updateRequest = updateRequest.Where("id = ?", updateOrganization.Id)

	_, err := pgTx.ExecBuilder(ctx, updateRequest)
	return err
}

func (repo *OrganizationRepositoryPostgresql) DeleteOrganization(ctx context.Context, tx Transaction, organizationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(ctx, NewQueryBuilder().Delete(dbmodels.TABLE_ORGANIZATION).Where("id = ?", organizationId))
	return err
}
