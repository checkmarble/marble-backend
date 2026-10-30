package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

const TABLE_TRACFIN_CONFIGURATION = "tracfin_configuration"
const TABLE_TRACFIN_DRAFTS = "tracfin_drafts"

type DBTracfinConfiguration struct {
	OrgID          string          `db:"org_id"`
	Settings       json.RawMessage `db:"settings"`
	Mappings       json.RawMessage `db:"mappings"`
	ProfileVersion string          `db:"profile_version"`
	ProfileHash    string          `db:"profile_hash"`
	Revision       int64           `db:"revision"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
	UpdatedBy      string          `db:"updated_by"`
}

var TracfinConfigurationColumns = utils.ColumnList[DBTracfinConfiguration]()

func AdaptTracfinConfiguration(db DBTracfinConfiguration) (models.TracfinConfiguration, error) {
	result := models.TracfinConfiguration{OrgID: db.OrgID, ProfileVersion: db.ProfileVersion, ProfileHash: db.ProfileHash, Revision: db.Revision, CreatedAt: db.CreatedAt, UpdatedAt: db.UpdatedAt, UpdatedBy: db.UpdatedBy}
	if err := json.Unmarshal(db.Settings, &result.Settings); err != nil {
		return models.TracfinConfiguration{}, errors.Wrap(err, "decode TRACFIN settings")
	}
	if err := json.Unmarshal(db.Mappings, &result.Mappings); err != nil {
		return models.TracfinConfiguration{}, errors.Wrap(err, "decode TRACFIN mappings")
	}
	return result, nil
}

// Content deliberately excludes scope, edit metadata and last_export. Only this
// payload changes when editing or explicitly refreshing the copied configuration.
type TracfinDraftContent struct {
	Document      models.TracfinDocument      `json:"document"`
	Configuration models.TracfinConfiguration `json:"configuration"`
}

type DBTracfinDraft struct {
	CaseID     string          `db:"case_id"`
	OrgID      string          `db:"org_id"`
	Content    json.RawMessage `db:"content"`
	Revision   int64           `db:"revision"`
	LastExport json.RawMessage `db:"last_export"`
	CreatedAt  time.Time       `db:"created_at"`
	UpdatedAt  time.Time       `db:"updated_at"`
	UpdatedBy  string          `db:"updated_by"`
}

var TracfinDraftColumns = utils.ColumnList[DBTracfinDraft]()

func AdaptTracfinDraft(db DBTracfinDraft) (models.TracfinDraft, error) {
	var content TracfinDraftContent
	if err := json.Unmarshal(db.Content, &content); err != nil {
		return models.TracfinDraft{}, errors.Wrap(err, "decode TRACFIN draft")
	}
	result := models.TracfinDraft{CaseID: db.CaseID, OrgID: db.OrgID, Document: content.Document, Configuration: content.Configuration, Revision: db.Revision, CreatedAt: db.CreatedAt, UpdatedAt: db.UpdatedAt, UpdatedBy: db.UpdatedBy}
	if len(db.LastExport) > 0 {
		if err := json.Unmarshal(db.LastExport, &result.LastExport); err != nil {
			return models.TracfinDraft{}, errors.Wrap(err, "decode TRACFIN export")
		}
	}
	return result, nil
}
