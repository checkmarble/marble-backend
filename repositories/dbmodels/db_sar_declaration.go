package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const (
	TABLE_SAR_DECLARATIONS        = "sar_declarations"
	TABLE_SAR_ORG_FORMAT_SETTINGS = "sar_org_format_settings"
	TABLE_SAR_DECLARATION_EXPORTS = "sar_declaration_exports"
)

type DBSarDeclaration struct {
	Id             uuid.UUID       `db:"id"`
	OrganizationId uuid.UUID       `db:"organization_id"`
	CaseId         uuid.UUID       `db:"case_id"`
	FormatKey      string          `db:"format_key"`
	ReportType     string          `db:"report_type"`
	Document       json.RawMessage `db:"document"`
	Version        int             `db:"version"`
	Analysis       string          `db:"analysis"`
	ExtraInfo      string          `db:"extra_info"`
	CreatedBy      *uuid.UUID      `db:"created_by"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

var SarDeclarationColumns = utils.ColumnList[DBSarDeclaration]()

func AdaptSarDeclaration(db DBSarDeclaration) (models.SarDeclaration, error) {
	return models.SarDeclaration{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		CaseId:         db.CaseId,
		FormatKey:      db.FormatKey,
		ReportType:     db.ReportType,
		Document:       db.Document,
		Version:        db.Version,
		Analysis:       db.Analysis,
		ExtraInfo:      db.ExtraInfo,
		CreatedBy:      db.CreatedBy,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
	}, nil
}

type DBSarOrgFormatSettings struct {
	OrganizationId uuid.UUID       `db:"organization_id"`
	FormatFamily   string          `db:"format_family"`
	Values         json.RawMessage `db:"values"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

// "values" is a SQL keyword: columns are escaped.
var SarOrgFormatSettingsColumns = utils.EscapedColumnList[DBSarOrgFormatSettings]()

func AdaptSarOrgFormatSettings(db DBSarOrgFormatSettings) (models.SarOrgFormatSettings, error) {
	values := map[string]string{}
	if err := json.Unmarshal(db.Values, &values); err != nil {
		return models.SarOrgFormatSettings{}, errors.Wrap(err, "invalid sar org format settings values")
	}
	return models.SarOrgFormatSettings{
		OrganizationId: db.OrganizationId,
		FormatFamily:   db.FormatFamily,
		Values:         values,
		UpdatedAt:      db.UpdatedAt,
	}, nil
}

type DBSarDeclarationExport struct {
	Id                 uuid.UUID       `db:"id"`
	DeclarationId      uuid.UUID       `db:"declaration_id"`
	DeclarationVersion int             `db:"declaration_version"`
	Document           json.RawMessage `db:"document"`
	Outstanding        json.RawMessage `db:"outstanding"`
	BucketName         string          `db:"bucket_name"`
	FileReference      string          `db:"file_reference"`
	Sha256             string          `db:"sha256"`
	CreatedBy          *uuid.UUID      `db:"created_by"`
	CreatedAt          time.Time       `db:"created_at"`
}

var SarDeclarationExportColumns = utils.ColumnList[DBSarDeclarationExport]()

func AdaptSarDeclarationExport(db DBSarDeclarationExport) (models.SarDeclarationExport, error) {
	return models.SarDeclarationExport{
		Id:                 db.Id,
		DeclarationId:      db.DeclarationId,
		DeclarationVersion: db.DeclarationVersion,
		Document:           db.Document,
		Outstanding:        db.Outstanding,
		BucketName:         db.BucketName,
		FileReference:      db.FileReference,
		Sha256:             db.Sha256,
		CreatedBy:          db.CreatedBy,
		CreatedAt:          db.CreatedAt,
	}, nil
}
