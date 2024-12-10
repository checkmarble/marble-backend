package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func selectLicenses() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.LicenseFields...).
		From(dbmodels.TABLE_LICENSES)
}

func (repo *MarbleDbRepository) GetLicenseById(ctx context.Context, exec Executor, licenseId string) (models.License, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.License{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectLicenses().Where(squirrel.Eq{"id": licenseId}),
		dbmodels.AdaptLicense,
	)
}

func (repo *MarbleDbRepository) GetLicenseByKey(ctx context.Context, exec Executor, licenseKey string) (models.License, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.License{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectLicenses().Where("key = ?", licenseKey),
		dbmodels.AdaptLicense,
	)
}

func (repo *MarbleDbRepository) ListLicenses(ctx context.Context, exec Executor) ([]models.License, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		selectLicenses().OrderBy("created_at DESC"),
		dbmodels.AdaptLicense,
	)
}

func (repo *MarbleDbRepository) CreateLicense(ctx context.Context, exec Executor, license models.License) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_LICENSES).
			Columns(
				"id",
				"key",
				"created_at",
				"expiration_date",
				"name",
				"description",
				"sso_entitlement",
				"workflows_entitlement",
				"analytics_entitlement",
				"data_enrichment",
				"user_roles",
				"webhooks",
				"rule_snoozes",
				"test_run",
			).
			Values(
				license.Id,
				license.Key,
				license.CreatedAt,
				license.ExpirationDate,
				license.OrganizationName,
				license.Description,
				license.LicenseEntitlements.Sso,
				license.LicenseEntitlements.Workflows,
				license.LicenseEntitlements.Analytics,
				license.LicenseEntitlements.DataEnrichment,
				license.LicenseEntitlements.UserRoles,
				license.LicenseEntitlements.Webhooks,
				license.LicenseEntitlements.RuleSnoozes,
				license.LicenseEntitlements.TestRun,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateLicense(ctx context.Context, exec Executor, updateLicenseInput models.UpdateLicenseInput) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().Update(dbmodels.TABLE_LICENSES)

	if updateLicenseInput.Suspend.Valid {
		if updateLicenseInput.Suspend.Bool {
			query = query.Set("suspended_at", squirrel.Expr("NOW()"))
		} else {
			query = query.Set("suspended_at", nil)
		}
	}
	if updateLicenseInput.ExpirationDate.Valid {
		query = query.Set("expiration_date", updateLicenseInput.ExpirationDate.Time)
	}
	if updateLicenseInput.OrganizationName.Valid {
		query = query.Set("name", updateLicenseInput.OrganizationName.String)
	}
	if updateLicenseInput.Description.Valid {
		query = query.Set("description", updateLicenseInput.Description.String)
	}
	if updateLicenseInput.LicenseEntitlements.Valid {
		licenseEntitlements := updateLicenseInput.LicenseEntitlements.V
		query = query.Set("sso_entitlement", licenseEntitlements.Sso)
		query = query.Set("workflows_entitlement", licenseEntitlements.Workflows)
		query = query.Set("analytics_entitlement", licenseEntitlements.Analytics)
		query = query.Set("data_enrichment", licenseEntitlements.DataEnrichment)
		query = query.Set("user_roles", licenseEntitlements.UserRoles)
		query = query.Set("webhooks", licenseEntitlements.Webhooks)
		query = query.Set("rule_snoozes", licenseEntitlements.RuleSnoozes)
		query = query.Set("test_run", licenseEntitlements.TestRun)
	}

	err := ExecBuilder(
		ctx,
		exec,
		query.Where("id = ?", updateLicenseInput.Id),
	)
	return err
}
