package dbmodels

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

const TABLE_SANCTION_CHECKS = "sanction_checks"

var SelectSanctionChecksColumn = utils.ColumnList[DBSanctionCheck]()

type DBSanctionCheck struct {
	Id              string    `db:"id"`
	DecisionId      string    `db:"decision_id"`
	Status          string    `db:"status"`
	SearchInput     []byte    `db:"search_input"`
	SearchDatasets  []string  `db:"search_datasets"`
	SearchThreshold int       `db:"search_threshold"`
	IsManual        bool      `db:"is_manual"`
	IsPartial       bool      `db:"is_partial"`
	RequestedBy     *string   `db:"requested_by"`
	IsArchived      bool      `db:"is_archived"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

func AdaptSanctionCheck(dto DBSanctionCheck) (models.SanctionCheckExecution, error) {
	cfg := models.OrganizationOpenSanctionsConfig{
		MatchThreshold: &dto.SearchThreshold,
		Datasets:       dto.SearchDatasets,
	}

	var query models.OpenSanctionsQuery

	err := json.NewDecoder(bytes.NewReader(dto.SearchInput)).Decode(&query)
	if err != nil {
		return models.SanctionCheckExecution{},
			errors.Wrap(err, "could not unmarshal sanction check query input")
	}

	return models.SanctionCheckExecution{
		Query:     query,
		OrgConfig: cfg,
		Count:     0,
		Partial:   dto.IsPartial,
	}, nil
}
