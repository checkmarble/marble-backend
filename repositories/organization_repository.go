package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type OrganizationRepository interface {
	// organization
	AllOrganizations(ctx context.Context, exec Executor) ([]models.Organization, error)
	GetOrganizationById(ctx context.Context, exec Executor, organizationId string) (models.Organization, error)
	CreateOrganization(ctx context.Context, exec Executor, newOrganizationId, name string) error
	UpdateOrganization(ctx context.Context, exec Executor, updateOrganization models.UpdateOrganizationInput) error
	DeleteOrganization(ctx context.Context, exec Executor, organizationId string) error
	DeleteOrganizationDecisionRulesAsync(ctx context.Context, exec Executor, organizationId string)

	// organization schema
	OrganizationSchemaOfOrganization(ctx context.Context, exec Executor, organizationId string) (models.OrganizationSchema, error)
	CreateOrganizationSchema(
		ctx context.Context,
		exec Executor,
		organizationId, schemaName string,
	) error
}

type OrganizationRepositoryPostgresql struct{}

func (repo *OrganizationRepositoryPostgresql) AllOrganizations(ctx context.Context, exec Executor) ([]models.Organization, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

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

func (repo *OrganizationRepositoryPostgresql) GetOrganizationById(ctx context.Context,
	exec Executor, organizationId string,
) (models.Organization, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Organization{}, err
	}

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

func (repo *OrganizationRepositoryPostgresql) CreateOrganization(
	ctx context.Context,
	exec Executor,
	newOrganizationId, name string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_ORGANIZATION).
			Columns(
				"id",
				"name",
			).
			Values(
				newOrganizationId,
				name,
			),
	)
	return err
}

func (repo *OrganizationRepositoryPostgresql) UpdateOrganization(ctx context.Context, exec Executor, updateOrganization models.UpdateOrganizationInput) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	updateRequest := NewQueryBuilder().Update(dbmodels.TABLE_ORGANIZATION)

	if updateOrganization.Name != nil {
		updateRequest = updateRequest.Set("name", *updateOrganization.Name)
	}

	updateRequest = updateRequest.Where("id = ?", updateOrganization.Id)

	err := ExecBuilder(ctx, exec, updateRequest)
	return err
}

func (repo *OrganizationRepositoryPostgresql) DeleteOrganization(ctx context.Context, exec Executor, organizationId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(ctx, exec, NewQueryBuilder().Delete(dbmodels.TABLE_ORGANIZATION).Where("id = ?", organizationId))
	return err
}

func (repo *OrganizationRepositoryPostgresql) DeleteOrganizationDecisionRulesAsync(
	ctx context.Context, exec Executor, organizationId string,
) {
	// This is used asynchronously after the organization is deleted, because it is not dramatic if it fails
	go func() {
		err := ExecBuilder(
			ctx,
			exec,
			NewQueryBuilder().
				Delete(dbmodels.TABLE_DECISION_RULES).
				Where("org_id = ?", organizationId),
		)
		if err != nil {
			utils.LogAndReportSentryError(ctx, err)
		}
	}()
}

func (repo *OrganizationRepositoryPostgresql) CreateOrganizationSchema(
	ctx context.Context,
	exec Executor,
	organizationId, schemaName string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.ORGANIZATION_SCHEMA_TABLE).
			Columns(
				dbmodels.OrganizationSchemaFields...,
			).
			Values(
				uuid.NewString(),
				organizationId,
				schemaName,
			),
	)
	return err
}

// TODO: is to be removed because we no longer need it
func (repo *OrganizationRepositoryPostgresql) OrganizationSchemaOfOrganization(
	ctx context.Context, exec Executor, organizationId string,
) (models.OrganizationSchema, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.OrganizationSchema{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.OrganizationSchemaFields...).
			From(dbmodels.ORGANIZATION_SCHEMA_TABLE).
			Where(squirrel.Eq{"org_id": organizationId}),
		dbmodels.AdaptOrganizationSchema,
	)
}
