package repositories

import (
	"context"
	"fmt"
	"net"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type OrganizationRepository interface {
	// organization
	AllOrganizations(ctx context.Context, exec Executor) ([]models.Organization, error)
	GetOrganizationById(ctx context.Context, exec Executor, organizationId uuid.UUID) (models.Organization, error)
	CreateOrganization(ctx context.Context, exec Executor, newOrganizationId uuid.UUID, name string) error
	UpdateOrganization(ctx context.Context, exec Executor, updateOrganization models.UpdateOrganizationInput) error
	DeleteOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) error
	DeleteOrganizationDecisionRulesAsync(ctx context.Context, exec Executor, organizationId uuid.UUID)
	GetOrganizationFeatureAccess(ctx context.Context, exec Executor, organizationId uuid.UUID) (
		models.DbStoredOrganizationFeatureAccess, error,
	)
	UpdateOrganizationFeatureAccess(
		ctx context.Context,
		exec Executor,
		updateFeatureAccess models.UpdateOrganizationFeatureAccessInput,
	) error
	HasOrganizations(ctx context.Context, exec Executor) (bool, error)
	UpdateOrganizationAllowedNetworks(ctx context.Context, exec Executor, orgId uuid.UUID,
		subnets []net.IPNet) ([]net.IPNet, error)
}

func (repo *MarbleDbRepository) AllOrganizations(ctx context.Context, exec Executor) ([]models.Organization, error) {
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

func (repo *MarbleDbRepository) GetOrganizationById(ctx context.Context,
	exec Executor, organizationId uuid.UUID,
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

func (repo *MarbleDbRepository) CreateOrganization(
	ctx context.Context,
	exec Executor,
	newOrganizationId uuid.UUID,
	name string,
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

func (repo *MarbleDbRepository) UpdateOrganization(ctx context.Context, exec Executor, updateOrganization models.UpdateOrganizationInput) error {
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
	if updateOrganization.ScreeningConfig.MatchThreshold != nil {
		updateRequest = updateRequest.Set("sanctions_threshold",
			*updateOrganization.ScreeningConfig.MatchThreshold)
		hasUpdates = true
	}
	if updateOrganization.ScreeningConfig.MatchLimit != nil {
		updateRequest = updateRequest.Set("sanctions_limit",
			*updateOrganization.ScreeningConfig.MatchLimit)
		hasUpdates = true
	}
	if updateOrganization.AutoAssignQueueLimit != nil {
		updateRequest = updateRequest.Set("auto_assign_queue_limit",
			updateOrganization.AutoAssignQueueLimit)
		hasUpdates = true
	}
	updateRequest = updateRequest.Set("sentry_replay_enabled",
		updateOrganization.SentryReplayEnabled)
	hasUpdates = true

	if !hasUpdates {
		return nil
	}
	updateRequest = updateRequest.Where("id = ?", updateOrganization.Id)

	err := ExecBuilder(ctx, exec, updateRequest)
	return err
}

func (repo *MarbleDbRepository) DeleteOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(ctx, exec, NewQueryBuilder().Delete(dbmodels.TABLE_ORGANIZATION).Where("id = ?", organizationId))
	return err
}

func (repo *MarbleDbRepository) DeleteOrganizationDecisionRulesAsync(
	ctx context.Context, exec Executor, organizationId uuid.UUID,
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

func (repo *MarbleDbRepository) GetOrganizationFeatureAccess(ctx context.Context, exec Executor,
	organizationId uuid.UUID,
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

func (repo *MarbleDbRepository) UpdateOrganizationFeatureAccess(
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

func (repo *MarbleDbRepository) HasOrganizations(ctx context.Context, exec Executor) (bool, error) {
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

func (repo *MarbleDbRepository) GetOrganizationAllowedNetworks(ctx context.Context, exec Executor, orgId uuid.UUID) ([]net.IPNet, error) {
	sql := NewQueryBuilder().
		Select("allowed_networks").
		From(dbmodels.TABLE_ORGANIZATION).
		Where("id = ?", orgId)

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptOrganizationWhitelistedSubnets)
}

func (repo *MarbleDbRepository) UpdateOrganizationAllowedNetworks(ctx context.Context,
	exec Executor, orgId uuid.UUID, subnets []net.IPNet,
) ([]net.IPNet, error) {
	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_ORGANIZATION).
		Set("allowed_networks", subnets).
		Where("id = ?", orgId).
		Suffix("returning allowed_networks")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptOrganizationWhitelistedSubnets)
}
