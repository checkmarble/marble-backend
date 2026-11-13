package repositories

import (
	"context"
	"encoding/json"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
)

// TracfinRepository accepts a pool or the caller's transaction. Writes do not
// commit; callers can read back database-assigned revisions/dates in the same tx.
// Reads return models.NotFoundError when absent, including no configuration.
// Configuration expectedRevision=0 means create, not an unconditional upsert.
// Draft creation always starts at revision 1; other content saves increment it.
// UpdatedBy must come from the authenticated server-side actor. CreatedAt and
// UpdatedAt come from PostgreSQL, never from the supplied model's dates.
type TracfinRepository interface {
	GetTracfinDecisionSources(context.Context, Executor, string, string, []string) ([]models.TracfinDecisionSource, error)
	GetTracfinConfiguration(context.Context, Executor, string) (models.TracfinConfiguration, error)
	SaveTracfinConfiguration(context.Context, Executor, string, models.TracfinConfiguration, int64) error
	GetTracfinDraft(context.Context, Executor, string, string) (models.TracfinDraft, error)
	CreateTracfinDraft(context.Context, Executor, string, string, models.TracfinDraft) error
	SaveTracfinDraft(context.Context, Executor, string, string, models.TracfinDraft, int64) error
	SaveTracfinExport(context.Context, Executor, string, string, models.TracfinExport, int64, *string) error
}

var _ TracfinRepository = (*MarbleDbRepository)(nil)

func validateTracfinScope(exec Executor, orgID string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if orgID == "" {
		return errors.Wrap(models.BadParameterError, "TRACFIN organization is required")
	}
	return nil
}
func validateTracfinDraftScope(exec Executor, orgID, caseID string) error {
	if err := validateTracfinScope(exec, orgID); err != nil {
		return err
	}
	if caseID == "" {
		return errors.Wrap(models.BadParameterError, "TRACFIN case is required")
	}
	return nil
}
func (repo *MarbleDbRepository) GetTracfinConfiguration(ctx context.Context, exec Executor, orgID string) (models.TracfinConfiguration, error) {
	if err := validateTracfinScope(exec, orgID); err != nil {
		return models.TracfinConfiguration{}, err
	}
	return SqlToModel(ctx, exec, NewQueryBuilder().Select(dbmodels.TracfinConfigurationColumns...).From(dbmodels.TABLE_TRACFIN_CONFIGURATION).Where("org_id = ?", orgID), dbmodels.AdaptTracfinConfiguration)
}
func (repo *MarbleDbRepository) SaveTracfinConfiguration(ctx context.Context, exec Executor, orgID string, config models.TracfinConfiguration, expectedRevision int64) error {
	if err := validateTracfinScope(exec, orgID); err != nil {
		return err
	}
	if config.OrgID != orgID || expectedRevision < 0 {
		return errors.Wrap(models.BadParameterError, "invalid TRACFIN configuration scope or revision")
	}
	settings, err := json.Marshal(config.Settings)
	if err != nil {
		return err
	}
	mappings, err := json.Marshal(config.Mappings)
	if err != nil {
		return err
	}
	if expectedRevision == 0 {
		return execTracfinWrite(ctx, exec, NewQueryBuilder().Insert(dbmodels.TABLE_TRACFIN_CONFIGURATION).
			Columns("org_id", "settings", "mappings", "profile_version", "profile_hash", "revision", "updated_by").
			Values(orgID, settings, mappings, config.ProfileVersion, config.ProfileHash, 1, config.UpdatedBy).
			Suffix("ON CONFLICT (org_id) DO NOTHING"))
	}
	return execTracfinWrite(ctx, exec, NewQueryBuilder().Update(dbmodels.TABLE_TRACFIN_CONFIGURATION).
		Set("settings", settings).Set("mappings", mappings).Set("profile_version", config.ProfileVersion).Set("profile_hash", config.ProfileHash).
		Set("revision", squirrel.Expr("revision + 1")).Set("updated_at", squirrel.Expr("CURRENT_TIMESTAMP")).Set("updated_by", config.UpdatedBy).
		Where("org_id = ?", orgID).Where("revision = ?", expectedRevision))
}
func (repo *MarbleDbRepository) GetTracfinDraft(ctx context.Context, exec Executor, orgID, caseID string) (models.TracfinDraft, error) {
	if err := validateTracfinDraftScope(exec, orgID, caseID); err != nil {
		return models.TracfinDraft{}, err
	}
	columns := make([]string, len(dbmodels.TracfinDraftColumns))
	for i, column := range dbmodels.TracfinDraftColumns {
		columns[i] = "d." + column
	}
	return SqlToModel(ctx, exec, NewQueryBuilder().Select(columns...).From(dbmodels.TABLE_TRACFIN_DRAFTS+" AS d").
		Join("cases AS c ON c.id = d.case_id AND c.org_id = d.org_id").Where("d.org_id = ?", orgID).Where("d.case_id = ?", caseID), dbmodels.AdaptTracfinDraft)
}
func (repo *MarbleDbRepository) CreateTracfinDraft(ctx context.Context, exec Executor, orgID, caseID string, draft models.TracfinDraft) error {
	if err := validateTracfinDraftScope(exec, orgID, caseID); err != nil {
		return err
	}
	if draft.OrgID != orgID || draft.CaseID != caseID {
		return errors.Wrap(models.BadParameterError, "invalid TRACFIN draft scope")
	}
	content, err := json.Marshal(dbmodels.TracfinDraftContent{Document: draft.Document, Configuration: draft.Configuration})
	if err != nil {
		return err
	}
	// No configuration FK: entirely manual drafts remain available without settings.
	return execTracfinWrite(ctx, exec, NewQueryBuilder().Insert(dbmodels.TABLE_TRACFIN_DRAFTS).
		Columns("case_id", "org_id", "content", "revision", "updated_by").
		Select(NewQueryBuilder().Select("id", "org_id").Column("?::jsonb", content).Column("1").Column("?::uuid", draft.UpdatedBy).
			From("cases").Where("id = ?", caseID).Where("org_id = ?", orgID)).
		Suffix("ON CONFLICT (case_id) DO NOTHING"))
}
func (repo *MarbleDbRepository) SaveTracfinDraft(ctx context.Context, exec Executor, orgID, caseID string, draft models.TracfinDraft, expectedRevision int64) error {
	if err := validateTracfinDraftScope(exec, orgID, caseID); err != nil {
		return err
	}
	if draft.OrgID != orgID || draft.CaseID != caseID || expectedRevision < 1 {
		return errors.Wrap(models.BadParameterError, "invalid TRACFIN draft scope or revision")
	}
	content, err := json.Marshal(dbmodels.TracfinDraftContent{Document: draft.Document, Configuration: draft.Configuration})
	if err != nil {
		return err
	}
	return execTracfinWrite(ctx, exec, NewQueryBuilder().Update(dbmodels.TABLE_TRACFIN_DRAFTS).
		Set("content", content).Set("revision", squirrel.Expr("revision + 1")).Set("updated_at", squirrel.Expr("CURRENT_TIMESTAMP")).Set("updated_by", draft.UpdatedBy).
		Where("org_id = ?", orgID).Where("case_id = ?", caseID).Where("revision = ?", expectedRevision).
		Where("EXISTS (SELECT 1 FROM cases WHERE cases.id = tracfin_drafts.case_id AND cases.org_id = tracfin_drafts.org_id)"))
}

// Export publication only changes last_export. It cannot advance the edit revision
// or clear the previous successful export on generation/storage failure.
func (repo *MarbleDbRepository) SaveTracfinExport(ctx context.Context, exec Executor, orgID, caseID string, export models.TracfinExport, expectedRevision int64, expectedPreviousExportID *string) error {
	if err := validateTracfinDraftScope(exec, orgID, caseID); err != nil {
		return err
	}
	if expectedRevision < 1 || export.DraftRevision != expectedRevision || export.ID == "" || (expectedPreviousExportID != nil && *expectedPreviousExportID == "") {
		return errors.Wrap(models.BadParameterError, "invalid TRACFIN export revision or ID")
	}
	encoded, err := json.Marshal(export)
	if err != nil {
		return err
	}
	var previous any
	if expectedPreviousExportID != nil {
		previous = *expectedPreviousExportID
	}
	return execTracfinWrite(ctx, exec, NewQueryBuilder().Update(dbmodels.TABLE_TRACFIN_DRAFTS).Set("last_export", encoded).
		Where("org_id = ?", orgID).Where("case_id = ?", caseID).Where("revision = ?", expectedRevision).
		Where("last_export->>'id' IS NOT DISTINCT FROM ?::text", previous).
		Where("EXISTS (SELECT 1 FROM cases WHERE cases.id = tracfin_drafts.case_id AND cases.org_id = tracfin_drafts.org_id)"))
}
func execTracfinWrite(ctx context.Context, exec Executor, query squirrel.Sqlizer) error {
	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}
	tag, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.Wrap(models.ConflictError, "TRACFIN record changed, exists already, or is outside the case scope")
	}
	return nil
}

// GetTracfinDecisionSources deliberately bypasses adaptCoreDecision: that adapter
// decodes numbers through float64. Historical snapshots need no current table.
func (repo *MarbleDbRepository) GetTracfinDecisionSources(ctx context.Context, exec Executor, orgID, caseID string, ids []string) ([]models.TracfinDecisionSource, error) {
	if err := validateTracfinDraftScope(exec, orgID, caseID); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []models.TracfinDecisionSource{}, nil
	}
	if len(ids) > 100 {
		return nil, errors.Wrap(models.BadParameterError, "too many TRACFIN source decisions")
	}
	return SqlToListOfModels(ctx, exec, NewQueryBuilder().Select("d.id", "d.trigger_object_type", "d.trigger_object").From("decisions AS d").
		Join("cases AS c ON c.id = d.case_id AND c.org_id = d.org_id").Where("d.org_id = ?", orgID).Where("d.case_id = ?", caseID).Where(squirrel.Eq{"d.id": ids}), func(row models.TracfinDecisionSource) (models.TracfinDecisionSource, error) { return row, nil })
}
