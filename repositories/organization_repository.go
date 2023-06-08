package repositories

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type OrganizationRepository interface {
	AllOrganizations(tx Transaction) ([]models.Organization, error)
	GetOrganizationById(tx Transaction, organizationID string) (models.Organization, error)
	CreateOrganization(tx Transaction, createOrganization models.CreateOrganizationInput, newOrganizationId string) error
	UpdateOrganization(tx Transaction, updateOrganization models.UpdateOrganizationInput) error
	DeleteOrganization(tx Transaction, organizationID string) error
}

type OrganizationRepositoryPostgresql struct {
	transactionFactory TransactionFactory
	queryBuilder       squirrel.StatementBuilderType
}

func (repo *OrganizationRepositoryPostgresql) AllOrganizations(tx Transaction) ([]models.Organization, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		repo.queryBuilder.
			Select(dbmodels.OrganizationFields...).
			From(dbmodels.TABLE_ORGANIZATION).
			OrderBy("id"),
		dbmodels.AdaptOrganization,
	)
}
func (repo *OrganizationRepositoryPostgresql) GetOrganizationById(tx Transaction, organizationID string) (models.Organization, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		repo.queryBuilder.
			Select(dbmodels.OrganizationFields...).
			From(dbmodels.TABLE_ORGANIZATION).
			Where("id = ?", organizationID),
		dbmodels.AdaptOrganization,
	)
}

func (repo *OrganizationRepositoryPostgresql) CreateOrganization(tx Transaction, createOrganization models.CreateOrganizationInput, newOrganizationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlInsert(
		pgTx,
		repo.queryBuilder.Insert(dbmodels.TABLE_ORGANIZATION).
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
}

func (repo *OrganizationRepositoryPostgresql) UpdateOrganization(tx Transaction, updateOrganization models.UpdateOrganizationInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	var updateRequest = repo.queryBuilder.Update(dbmodels.TABLE_ORGANIZATION)

	if updateOrganization.Name != nil {
		updateRequest = updateRequest.Set("name", *updateOrganization.Name)
	}
	if updateOrganization.DatabaseName != nil {
		updateRequest = updateRequest.Set("database_name", *updateOrganization.DatabaseName)
	}

	updateRequest = updateRequest.Where("id = ?", updateOrganization.ID)

	return SqlUpdate(pgTx, updateRequest)
}

func (repo *OrganizationRepositoryPostgresql) DeleteOrganization(tx Transaction, organizationID string) error {
	pgTx := repo.adaptMarbleDatabaseTransaction(tx)

	return SqlDelete(pgTx, repo.queryBuilder.Delete(dbmodels.TABLE_ORGANIZATION).Where("id = ?", organizationID))
}
