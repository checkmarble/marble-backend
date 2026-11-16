package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repo *MarbleDbRepository) CreateSarDeclaration(ctx context.Context, exec Executor, d models.SarDeclaration) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SAR_DECLARATIONS).
		Columns(
			"id", "organization_id", "case_id", "format_key", "report_type",
			"document", "analysis", "extra_info", "created_by",
		).
		Values(
			d.Id, d.OrganizationId, d.CaseId, d.FormatKey, d.ReportType,
			json.RawMessage(d.Document), d.Analysis, d.ExtraInfo, d.CreatedBy,
		)
	return ExecBuilder(ctx, exec, query)
}

func selectSarDeclaration() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SarDeclarationColumns...).
		From(dbmodels.TABLE_SAR_DECLARATIONS)
}

func (repo *MarbleDbRepository) GetSarDeclaration(ctx context.Context, exec Executor, id uuid.UUID) (models.SarDeclaration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SarDeclaration{}, err
	}
	query := selectSarDeclaration().Where(squirrel.Eq{"id": id})
	return SqlToModel(ctx, exec, query, dbmodels.AdaptSarDeclaration)
}

func (repo *MarbleDbRepository) GetSarDeclarationForUpdate(ctx context.Context, exec Executor, id uuid.UUID) (models.SarDeclaration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SarDeclaration{}, err
	}
	query := selectSarDeclaration().Where(squirrel.Eq{"id": id}).Suffix("FOR UPDATE")
	return SqlToModel(ctx, exec, query, dbmodels.AdaptSarDeclaration)
}

func (repo *MarbleDbRepository) ListSarDeclarationsByCase(ctx context.Context, exec Executor, caseId uuid.UUID) ([]models.SarDeclaration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := selectSarDeclaration().
		Where(squirrel.Eq{"case_id": caseId}).
		OrderBy("created_at ASC", "id ASC")
	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptSarDeclaration)
}

func updateSarDeclarationDocumentQuery(
	id uuid.UUID, expectedVersion int, document []byte, analysis, extraInfo string,
) squirrel.UpdateBuilder {
	return NewQueryBuilder().
		Update(dbmodels.TABLE_SAR_DECLARATIONS).
		Set("document", json.RawMessage(document)).
		Set("analysis", analysis).
		Set("extra_info", extraInfo).
		Set("version", squirrel.Expr("version + 1")).
		Set("updated_at", squirrel.Expr("now()")).
		Where("id = ?", id).
		Where("version = ?", expectedVersion).
		Suffix("RETURNING version")
}

// UpdateSarDeclarationDocument performs an optimistic-concurrency update: it
// only succeeds if the stored version equals expectedVersion, otherwise
// models.ErrSarVersionConflict is returned and nothing is written.
func (repo *MarbleDbRepository) UpdateSarDeclarationDocument(
	ctx context.Context, exec Executor, id uuid.UUID, expectedVersion int,
	document []byte, analysis, extraInfo string,
) (int, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return 0, err
	}
	query := updateSarDeclarationDocumentQuery(id, expectedVersion, document, analysis, extraInfo)
	newVersion, err := SqlToOptionalRow(ctx, exec, query, pgx.RowTo[int])
	if err != nil {
		return 0, err
	}
	if newVersion == nil {
		return 0, models.ErrSarVersionConflict
	}
	return *newVersion, nil
}

func (repo *MarbleDbRepository) GetSarOrgFormatSettings(
	ctx context.Context, exec Executor, orgId uuid.UUID, family string,
) (*models.SarOrgFormatSettings, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SarOrgFormatSettingsColumns...).
		From(dbmodels.TABLE_SAR_ORG_FORMAT_SETTINGS).
		Where(squirrel.Eq{"organization_id": orgId, "format_family": family})
	return SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptSarOrgFormatSettings)
}

// UpsertSarOrgFormatSettings replaces the whole Values map for (org, family).
func (repo *MarbleDbRepository) UpsertSarOrgFormatSettings(ctx context.Context, exec Executor, s models.SarOrgFormatSettings) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	values := s.Values
	if values == nil {
		values = map[string]string{}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return errors.Wrap(err, "cannot marshal sar org format settings")
	}
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SAR_ORG_FORMAT_SETTINGS).
		Columns("organization_id", "format_family", `"values"`).
		Values(s.OrganizationId, s.FormatFamily, json.RawMessage(raw)).
		Suffix(`ON CONFLICT (organization_id, format_family) DO UPDATE SET "values" = EXCLUDED."values", updated_at = now()`)
	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) CreateSarDeclarationExport(ctx context.Context, exec Executor, e models.SarDeclarationExport) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	outstanding := json.RawMessage(e.Outstanding)
	if len(outstanding) == 0 {
		outstanding = json.RawMessage("[]")
	}
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SAR_DECLARATION_EXPORTS).
		Columns(
			"id", "declaration_id", "declaration_version", "document", "outstanding",
			"bucket_name", "file_reference", "sha256", "created_by",
		).
		Values(
			e.Id, e.DeclarationId, e.DeclarationVersion, json.RawMessage(e.Document), outstanding,
			e.BucketName, e.FileReference, e.Sha256, e.CreatedBy,
		)
	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) GetSarDeclarationExport(ctx context.Context, exec Executor, id uuid.UUID) (models.SarDeclarationExport, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SarDeclarationExport{}, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SarDeclarationExportColumns...).
		From(dbmodels.TABLE_SAR_DECLARATION_EXPORTS).
		Where(squirrel.Eq{"id": id})
	return SqlToModel(ctx, exec, query, dbmodels.AdaptSarDeclarationExport)
}

func (repo *MarbleDbRepository) ListSarDeclarationExports(ctx context.Context, exec Executor, declarationId uuid.UUID) ([]models.SarDeclarationExport, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SarDeclarationExportColumns...).
		From(dbmodels.TABLE_SAR_DECLARATION_EXPORTS).
		Where(squirrel.Eq{"declaration_id": declarationId}).
		OrderBy("created_at DESC", "id DESC")
	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptSarDeclarationExport)
}

// GetSarDeclarationExportsSummary returns the number of exports of a
// declaration and the date of the latest one (nil when none), without loading
// any document.
func (repo *MarbleDbRepository) GetSarDeclarationExportsSummary(
	ctx context.Context, exec Executor, declarationId uuid.UUID,
) (int, *time.Time, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return 0, nil, err
	}
	query := NewQueryBuilder().
		Select("count(*)", "max(created_at)").
		From(dbmodels.TABLE_SAR_DECLARATION_EXPORTS).
		Where(squirrel.Eq{"declaration_id": declarationId})
	sql, args, err := query.ToSql()
	if err != nil {
		return 0, nil, errors.Wrap(err, "cannot build sar export summary query")
	}
	var count int
	var lastAt *time.Time
	if err := exec.QueryRow(ctx, sql, args...).Scan(&count, &lastAt); err != nil {
		return 0, nil, errors.Wrap(err, "cannot read sar export summary")
	}
	return count, lastAt, nil
}
