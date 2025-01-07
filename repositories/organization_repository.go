package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
)

type OrganizationRepository interface {
	// organization
	AllOrganizations(ctx context.Context, exec Executor) ([]models.Organization, error)
	GetOrganizationById(ctx context.Context, exec Executor, organizationId string) (models.Organization, error)
	CreateOrganization(ctx context.Context, exec Executor, newOrganizationId, name string) error
	UpdateOrganization(ctx context.Context, exec Executor, updateOrganization models.UpdateOrganizationInput) error
	DeleteOrganization(ctx context.Context, exec Executor, organizationId string) error
	DeleteOrganizationDecisionRulesAsync(ctx context.Context, exec Executor, organizationId string)
	GetOrganizationEntitlements(ctx context.Context, exec Executor, organizationId string) (
		[]models.OrganizationEntitlement, error)
	UpdateOrganizationEntitlements(ctx context.Context, exec Executor, organizationId string,
		entitlements models.UpdateOrganizationEntitlementInput) error
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

	if updateOrganization.DefaultScenarioTimezone != nil {
		updateRequest = updateRequest.Set(
			"default_scenario_timezone",
			*updateOrganization.DefaultScenarioTimezone)
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

func (repo *OrganizationRepositoryPostgresql) GetOrganizationEntitlements(ctx context.Context, exec Executor,
	organizationId string,
) ([]models.OrganizationEntitlement, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectOrganizationEntitlementColumn...).
		From(dbmodels.TABLE_ORGANIZATION_ENTITLEMENTS).
		Where("organization_id = ?", organizationId).
		OrderBy("created_at DESC")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptOrganizationEntitlement)
}

func (repo *OrganizationRepositoryPostgresql) UpdateOrganizationEntitlements(ctx context.Context, exec Executor,
	organizationId string, entitlement models.UpdateOrganizationEntitlementInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_ORGANIZATION_ENTITLEMENTS).
		Where(squirrel.And{
			squirrel.Eq{"organization_id": organizationId},
			squirrel.Eq{"feature_id": entitlement.FeatureId},
		}).
		Set("availability", entitlement.Availability)

	err := ExecBuilder(ctx, exec, query)
	return err
}
