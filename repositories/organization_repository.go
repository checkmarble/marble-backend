package repositories

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"
)

type OrganizationRepository interface {
	AllOrganizations(tx Transaction) ([]models.Organization, error)
	GetOrganizationById(tx Transaction, organizationId string) (models.Organization, error)
	CreateOrganization(tx Transaction, createOrganization models.CreateOrganizationInput, newOrganizationId string) error
	UpdateOrganization(tx Transaction, updateOrganization models.UpdateOrganizationInput) error
	DeleteOrganization(tx Transaction, organizationId string) error
}

type OrganizationRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func (repo *OrganizationRepositoryPostgresql) AllOrganizations(tx Transaction) ([]models.Organization, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectOrganization...).
			From(dbmodels.TABLE_ORGANIZATION).
			OrderBy("id"),
		dbmodels.AdaptOrganization,
	)
}
func (repo *OrganizationRepositoryPostgresql) GetOrganizationById(tx Transaction, organizationId string) (models.Organization, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectOrganization...).
			From(dbmodels.TABLE_ORGANIZATION).
			Where("id = ?", organizationId),
		dbmodels.AdaptOrganization,
	)
}

func (repo *OrganizationRepositoryPostgresql) CreateOrganization(tx Transaction, createOrganization models.CreateOrganizationInput, newOrganizationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
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

func (repo *OrganizationRepositoryPostgresql) UpdateOrganization(tx Transaction, updateOrganization models.UpdateOrganizationInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

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

	_, err := pgTx.ExecBuilder(updateRequest)
	return err
}

func (repo *OrganizationRepositoryPostgresql) DeleteOrganization(tx Transaction, organizationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(NewQueryBuilder().Delete(dbmodels.TABLE_ORGANIZATION).Where("id = ?", organizationId))
	return err
}
