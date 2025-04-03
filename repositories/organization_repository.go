package repositories

import (
	"context"
	"fmt"

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
	GetOrganizationFeatureAccess(ctx context.Context, exec Executor, organizationId string) (
		models.DbStoredOrganizationFeatureAccess, error,
	)
	UpdateOrganizationFeatureAccess(
		ctx context.Context,
		exec Executor,
		updateFeatureAccess models.UpdateOrganizationFeatureAccessInput,
	) error
	HasOrganizations(ctx context.Context, exec Executor) (bool, error)
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
	if err != nil {
		return err
	}

	newErr := ExecBuilder(ctx, exec, NewQueryBuilder().
		Insert(dbmodels.TABLE_ORGANIZATION_FEATURE_ACCESS).
		Columns("org_id").
		Values(newOrganizationId))

	return newErr
}

func (repo *OrganizationRepositoryPostgresql) UpdateOrganization(ctx context.Context, exec Executor, updateOrganization models.UpdateOrganizationInput) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	updateRequest := NewQueryBuilder().Update(dbmodels.TABLE_ORGANIZATION)
	hasUpdates := false

	if updateOrganization.DefaultScenarioTimezone != nil {
		updateRequest = updateRequest.Set(
			"default_scenario_timezone",
			*updateOrganization.DefaultScenarioTimezone)
		hasUpdates = true
	}
	if updateOrganization.SanctionCheckConfig.MatchThreshold != nil {
		updateRequest = updateRequest.Set("sanctions_threshold",
			*updateOrganization.SanctionCheckConfig.MatchThreshold)
		hasUpdates = true
	}
	if updateOrganization.SanctionCheckConfig.MatchLimit != nil {
		updateRequest = updateRequest.Set("sanctions_limit",
			*updateOrganization.SanctionCheckConfig.MatchLimit)
		hasUpdates = true
	}

	if !hasUpdates {
		return nil
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

func (repo *OrganizationRepositoryPostgresql) GetOrganizationFeatureAccess(ctx context.Context, exec Executor,
	organizationId string,
) (models.DbStoredOrganizationFeatureAccess, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.DbStoredOrganizationFeatureAccess{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectOrganizationFeatureAccessColumn...).
			From(dbmodels.TABLE_ORGANIZATION_FEATURE_ACCESS).
			Where(squirrel.Eq{"org_id": organizationId}),
		dbmodels.AdaptOrganizationFeatureAccess,
	)
}

func (repo *OrganizationRepositoryPostgresql) UpdateOrganizationFeatureAccess(
	ctx context.Context,
	exec Executor,
	updateFeatureAccess models.UpdateOrganizationFeatureAccessInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_ORGANIZATION_FEATURE_ACCESS).
		Where(squirrel.Eq{"org_id": updateFeatureAccess.OrganizationId})

	nbUpdated := 0
	if updateFeatureAccess.TestRun != nil {
		query = query.Set("test_run", *updateFeatureAccess.TestRun)
		nbUpdated++
	}
	if updateFeatureAccess.Sanctions != nil {
		query = query.Set("sanctions", *updateFeatureAccess.Sanctions)
		nbUpdated++
	}

	if nbUpdated == 0 {
		return nil
	}

	query.Set("updated_at", squirrel.Expr("NOW()"))

	err := ExecBuilder(ctx, exec, query)
	return err
}

func (repo *OrganizationRepositoryPostgresql) HasOrganizations(ctx context.Context, exec Executor) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}

	var exists bool
	err := exec.QueryRow(ctx, fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s LIMIT 1)",
		dbmodels.TABLE_ORGANIZATION)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
