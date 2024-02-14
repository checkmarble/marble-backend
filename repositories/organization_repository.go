package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type OrganizationRepository interface {
	AllOrganizations(ctx context.Context, exec Executor) ([]models.Organization, error)
	GetOrganizationById(ctx context.Context, exec Executor, organizationId string) (models.Organization, error)
	CreateOrganization(ctx context.Context, exec Executor, createOrganization models.CreateOrganizationInput, newOrganizationId string) error
	UpdateOrganization(ctx context.Context, exec Executor, updateOrganization models.UpdateOrganizationInput) error
	DeleteOrganization(ctx context.Context, exec Executor, organizationId string) error
}

type OrganizationRepositoryPostgresql struct {
	executorGetter ExecutorGetter
}

func (repo *OrganizationRepositoryPostgresql) AllOrganizations(ctx context.Context, exec Executor) ([]models.Organization, error) {
	exec = repo.executorGetter.ifNil(exec)

	return SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectOrganization...).
			From(dbmodels.TABLE_ORGANIZATION).
			OrderBy("id"),
		dbmodels.AdaptOrganization,
	)
}
func (repo *OrganizationRepositoryPostgresql) GetOrganizationById(ctx context.Context, exec Executor, organizationId string) (models.Organization, error) {
	exec = repo.executorGetter.ifNil(exec)

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ColumnsSelectOrganization...).
			From(dbmodels.TABLE_ORGANIZATION).
			Where("id = ?", organizationId),
		dbmodels.AdaptOrganization,
	)
}

func (repo *OrganizationRepositoryPostgresql) CreateOrganization(ctx context.Context, exec Executor, createOrganization models.CreateOrganizationInput, newOrganizationId string) error {
	exec = repo.executorGetter.ifNil(exec)

	_, err := ExecBuilder(
		ctx,
		exec,
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

func (repo *OrganizationRepositoryPostgresql) UpdateOrganization(ctx context.Context, exec Executor, updateOrganization models.UpdateOrganizationInput) error {
	exec = repo.executorGetter.ifNil(exec)

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

	_, err := ExecBuilder(ctx, exec, updateRequest)
	return err
}

func (repo *OrganizationRepositoryPostgresql) DeleteOrganization(ctx context.Context, exec Executor, organizationId string) error {
	exec = repo.executorGetter.ifNil(exec)

	_, err := ExecBuilder(ctx, exec, NewQueryBuilder().Delete(dbmodels.TABLE_ORGANIZATION).Where("id = ?", organizationId))
	return err
}
